package ivx

import (
	"context"
	"fmt"
	"log"
	"sort"
	"sync"
	"time"

	"trade-backend-go/internal/models"
	"trade-backend-go/internal/providers"
)

// Service handles IVx calculations
type Service struct {
	calculator      *Calculator
	providerManager *providers.ProviderManager
	cache           *Cache
}

// getSymbolsToFetch returns all symbols to fetch for IVx calculation, including weekly variants
func (s *Service) getSymbolsToFetch(symbol string) []string {
	// Map of weekly symbols to their base symbol
	weeklySymbolMap := map[string]string{
		"SPX": "SPXW", // SPX → include SPXW (SPX Weeklys)
		"NDX": "NDXP", // NDX → include NDXP  
		"RUT": "RUTW", // RUT → include RUTW
	}
	
	symbols := []string{symbol}
	
	// If this symbol has a weekly variant, include it
	if weeklySymbol, exists := weeklySymbolMap[symbol]; exists {
		symbols = append(symbols, weeklySymbol)
	}
	
	return symbols
}

// NewService creates a new IVx service
func NewService(providerManager *providers.ProviderManager, cache *Cache) *Service {
	return &Service{
		calculator:      NewCalculator(),
		providerManager: providerManager,
		cache:           cache,
	}
}

// StreamUpdate represents an update in the IVx stream
type StreamUpdate struct {
	Type    string      `json:"type"` // "status", "partial", "complete", "error"
	Payload interface{} `json:"payload"`
}

// GetIVxStream streams IVx data for a symbol
func (s *Service) GetIVxStream(ctx context.Context, symbol string, updates chan<- StreamUpdate) {
	defer close(updates)

	startTime := time.Now()

	// 1. Check cache first
	if cached := s.cache.Get(symbol); cached != nil {
		// Send cached results immediately
		for _, exp := range cached {
			updates <- StreamUpdate{Type: "data", Payload: exp}
		}
		
		// Send completion with cached flag
		updates <- StreamUpdate{Type: "complete", Payload: models.IVxResponse{
			Symbol:          symbol,
			Expirations:     cached,
			Cached:          true,
			CalculationTime: nil, // No calculation time for cached data
		}}
		return
	}

	// 2. Send initial status
	updates <- StreamUpdate{Type: "status", Payload: "Fetching underlying price..."}

	// 1. Get underlying price
	quote, err := s.providerManager.GetStockQuote(ctx, symbol)
	if err != nil {
		updates <- StreamUpdate{Type: "error", Payload: fmt.Sprintf("Failed to get quote: %v", err)}
		return
	}

	if quote.Bid == nil || quote.Ask == nil {
		updates <- StreamUpdate{Type: "error", Payload: "Invalid quote data"}
		return
	}

	underlyingPrice := (*quote.Bid + *quote.Ask) / 2
	updates <- StreamUpdate{Type: "status", Payload: fmt.Sprintf("Underlying price: %.2f", underlyingPrice)}

	// 2. Get all symbols to fetch (base + weekly variants like SPXW)
	symbolsToFetch := s.getSymbolsToFetch(symbol)
	// slog.Info("Fetching options for symbols", "base", symbol, "all", symbolsToFetch) // Removed slog as it's not imported

	// 3. Fetch options chain for all symbols
	var allContracts []*models.OptionContract
	for _, sym := range symbolsToFetch {
		contracts, err := s.providerManager.GetOptionsChainBasic(ctx, sym, "", &underlyingPrice, 0, nil, nil)
		if err != nil {
			// slog.Warn("Failed to get options chain", "symbol", sym, "error", err) // Removed slog as it's not imported
			log.Printf("Failed to get options chain for %s: %v", sym, err) // Using log.Printf as a fallback
			continue
		}
		allContracts = append(allContracts, contracts...)
		// slog.Info("Fetched contracts", "symbol", sym, "count", len(contracts)) // Removed slog as it's not imported
		log.Printf("Fetched %d contracts for %s", len(contracts), sym) // Using log.Printf as a fallback
	}

	if len(allContracts) == 0 {
		updates <- StreamUpdate{Type: "error", Payload: "No contracts found for any symbol"}
		return
	}

	updates <- StreamUpdate{Type: "status", Payload: fmt.Sprintf("Found %d total contracts", len(allContracts))}

	// 4. Group contracts by expiration
	expirationMap := make(map[string][]*models.OptionContract)
	for _, contract := range allContracts {
		if contract.ExpirationDate != "" {
			expirationMap[contract.ExpirationDate] = append(expirationMap[contract.ExpirationDate], contract)
		}
	}

	// 5. Filter to valid expirations (with both calls and puts, ATM strikes)
	type expData struct {
		dateStr      string
		optionsChain []models.OptionContract
		atmStrike    float64
		atmCall      *models.OptionContract
		atmPut       *models.OptionContract
	}

	validExpirations := make([]*expData, 0, len(expirationMap))

	for date, contracts := range expirationMap {
		chain := make([]models.OptionContract, len(contracts))
		for i, ptr := range contracts {
			chain[i] = *ptr
		}

		atmStrike := s.calculator.findATMStrike(chain, underlyingPrice)
		if atmStrike == 0 {
			continue
		}

		var atmCall, atmPut *models.OptionContract
		for i := range chain {
			if chain[i].StrikePrice == atmStrike {
				if chain[i].Type == "call" {
					atmCall = &chain[i]
				} else if chain[i].Type == "put" {
					atmPut = &chain[i]
				}
			}
		}

		if atmCall != nil && atmPut != nil {
			validExpirations = append(validExpirations, &expData{
				dateStr:      date,
				optionsChain: chain,
				atmStrike:    atmStrike,
				atmCall:      atmCall,
				atmPut:       atmPut,
			})
		}
	}

	if len(validExpirations) == 0 {
		updates <- StreamUpdate{Type: "error", Payload: "No valid expirations found"}
		return
	}

	updates <- StreamUpdate{Type: "status", Payload: fmt.Sprintf("Found %d valid expirations. Fetching Greeks...", len(validExpirations))}

	// 6. Process in batches and stream results
	batchSize := 1
	var allResults []models.IVxExpiration
	var resultsMutex sync.Mutex
	var wg sync.WaitGroup
	
	// Semaphore to limit concurrent batch requests
	sem := make(chan struct{}, 5)

	for i := 0; i < len(validExpirations); i += batchSize {
		end := i + batchSize
		if end > len(validExpirations) {
			end = len(validExpirations)
		}

		batch := validExpirations[i:end]
		wg.Add(1)

		go func(batchData []*expData) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			// Collect symbols for this batch
			symbolsToFetch := make([]string, 0, len(batchData)*2)
			for _, data := range batchData {
				symbolsToFetch = append(symbolsToFetch, data.atmCall.Symbol, data.atmPut.Symbol)
			}

			// Fetch Greeks for batch
			batchCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()

			greeksMap, err := s.providerManager.GetOptionsGreeksBatch(batchCtx, symbolsToFetch)
			if err != nil {
				log.Printf("Error fetching greeks batch: %v", err)
				// Continue with partial/empty map if possible, or just skip
			}
			if greeksMap == nil {
				greeksMap = make(map[string]map[string]interface{})
			}

			// Calculate IVx for this batch
			for _, data := range batchData {
				ivxResult := s.calculator.CalculateIVxForExpiration(data.dateStr, data.optionsChain, underlyingPrice, greeksMap)
				if ivxResult != nil {
					// Stream individual result immediately
					updates <- StreamUpdate{Type: "data", Payload: ivxResult}
					
					resultsMutex.Lock()
					allResults = append(allResults, *ivxResult)
					resultsMutex.Unlock()
				}
			}
		}(batch)
	}

	wg.Wait()

	// 7. Sort and Cache Final Results
	sort.Slice(allResults, func(i, j int) bool {
		return allResults[i].ExpirationDate < allResults[j].ExpirationDate
	})

	calculationTime := time.Since(startTime).Seconds()
	
	if len(allResults) > 0 {
		s.cache.Set(symbol, allResults)
	}

	finalResponse := models.IVxResponse{
		Symbol:          symbol,
		Expirations:     allResults,
		Cached:          false,
		CalculationTime: &calculationTime,
	}

	updates <- StreamUpdate{Type: "complete", Payload: finalResponse}
}

// GetIVxSnapshot returns the full IVx data for a symbol (on-demand)
func (s *Service) GetIVxSnapshot(ctx context.Context, symbol string) (*models.IVxResponse, error) {
	updates := make(chan StreamUpdate)
	
	// Start streaming in background
	go s.GetIVxStream(ctx, symbol, updates)

	// Consume stream and wait for completion
	for update := range updates {
		if update.Type == "complete" {
			if response, ok := update.Payload.(models.IVxResponse); ok {
				return &response, nil
			}
		} else if update.Type == "error" {
			return nil, fmt.Errorf("%v", update.Payload)
		}
	}

	return nil, fmt.Errorf("stream closed without completion")
}

// getUnderlyingPrice gets the underlying price with fallback logic
func (s *Service) getUnderlyingPrice(ctx context.Context, symbol string) (*float64, error) {
	// Try direct quote first
	quote, err := s.providerManager.GetStockQuote(ctx, symbol)
	if err == nil && quote != nil {
		if quote.Last != nil && *quote.Last > 0 {
			return quote.Last, nil
		}
		if quote.Bid != nil && quote.Ask != nil && *quote.Bid > 0 && *quote.Ask > 0 {
			mid := (*quote.Bid + *quote.Ask) / 2
			return &mid, nil
		}
	}

	// Fallback: Try to estimate from options chain (common for indices like SPX)
	// We just grab the first available expiration to estimate price
	expirationDatesRaw, err := s.providerManager.GetExpirationDates(ctx, symbol)
	if err != nil || len(expirationDatesRaw) == 0 {
		return nil, fmt.Errorf("could not get expiration dates for fallback price")
	}

	firstExpiry := ""
	if date, ok := expirationDatesRaw[0]["date"].(string); ok {
		firstExpiry = date
	}

	if firstExpiry == "" {
		return nil, fmt.Errorf("invalid expiration date for fallback price")
	}

	// Get basic chain to estimate price from strikes
	// We ask for a few strikes; usually providers return strikes around the current price
	chain, err := s.providerManager.GetOptionsChainBasic(ctx, symbol, firstExpiry, nil, 5, nil, &symbol)
	if err != nil || len(chain) == 0 {
		return nil, fmt.Errorf("could not get options chain for fallback price")
	}

	// Estimate from middle strike
	strikes := make([]float64, 0, len(chain))
	for _, c := range chain {
		if c != nil {
			strikes = append(strikes, c.StrikePrice)
		}
	}
	sort.Float64s(strikes)
	
	if len(strikes) > 0 {
		median := strikes[len(strikes)/2]
		return &median, nil
	}

	return nil, fmt.Errorf("could not estimate price from options chain")
}
