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

	var underlyingPrice float64
	if quote.Bid != nil && quote.Ask != nil && *quote.Bid > 0 && *quote.Ask > 0 {
		underlyingPrice = (*quote.Bid + *quote.Ask) / 2
	} else if quote.Last != nil && *quote.Last > 0 {
		underlyingPrice = *quote.Last
	} else {
		updates <- StreamUpdate{Type: "error", Payload: "Invalid quote data: missing bid/ask and last price"}
		return
	}
	updates <- StreamUpdate{Type: "status", Payload: fmt.Sprintf("Underlying price: %.2f", underlyingPrice)}

	// 2. Get all symbols to fetch (base + weekly variants like SPXW)
	symbolsToFetch := s.getSymbolsToFetch(symbol)
	log.Printf("Fetching IVx for symbols: %v", symbolsToFetch)

	// 3. Fetch FULL chain ONCE per symbol (major optimization: 50+ calls → 1-2 calls)
	// Pass expiry="" to get all expirations in a single API call
	updates <- StreamUpdate{Type: "status", Payload: "Fetching full options chain..."}
	
	var allContracts []*models.OptionContract
	for _, sym := range symbolsToFetch {
		// Fetch full chain: no expiry filter, no strike filter (we'll filter in-memory)
		contracts, err := s.providerManager.GetOptionsChainBasic(ctx, sym, "", &underlyingPrice, 0, nil, nil)
		if err != nil {
			log.Printf("Failed to get full options chain for %s: %v", sym, err)
			continue
		}
		if len(contracts) > 0 {
			allContracts = append(allContracts, contracts...)
			log.Printf("✅ Fetched %d contracts for %s (full chain, all expirations)", len(contracts), sym)
		}
	}

	if len(allContracts) == 0 {
		errorMsg := fmt.Sprintf("No options contracts found for %s. Symbol may not have options available or data unavailable from provider.", symbol)
		log.Printf("❌ IVx failed for %s: no contracts from any symbol (%v)", symbol, symbolsToFetch)
		updates <- StreamUpdate{Type: "error", Payload: errorMsg}
		return
	}

	updates <- StreamUpdate{Type: "status", Payload: fmt.Sprintf("Found %d total contracts. Grouping by expiration...", len(allContracts))}

	// 4. Group contracts by expiration (in-memory, very fast)
	expirationMap := make(map[string][]*models.OptionContract)
	for _, contract := range allContracts {
		if contract.ExpirationDate != "" {
			expirationMap[contract.ExpirationDate] = append(expirationMap[contract.ExpirationDate], contract)
		}
	}

	if len(expirationMap) == 0 {
		updates <- StreamUpdate{Type: "error", Payload: "No expirations found"}
		return
	}

	log.Printf("📊 Grouped into %d unique expirations", len(expirationMap))
	updates <- StreamUpdate{Type: "status", Payload: fmt.Sprintf("Found %d expirations. Finding ATM strikes...", len(expirationMap))}

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

	// Sort validExpirations by date (earliest first) so batches process nearest exps first
	sort.Slice(validExpirations, func(i, j int) bool {
		return validExpirations[i].dateStr < validExpirations[j].dateStr
	})

	log.Printf("✅ Found %d valid expirations with ATM strikes", len(validExpirations))
	updates <- StreamUpdate{Type: "status", Payload: fmt.Sprintf("Found %d valid expirations. Fetching Greeks...", len(validExpirations))}

	// Process in concurrent batches of expirations (e.g., 2 exps per batch → 4 symbols)
	// Each batch fetches its own Greeks, calculates, and streams "data" immediately
	expBatchSize := 2 // Expirations per batch (adjust for ~4 symbols)
	var allResults []models.IVxExpiration
	var resultsMutex sync.Mutex
	var wg sync.WaitGroup

	// Semaphore to limit concurrent batches (e.g., 5 max)
	maxConcurrent := 5
	sem := make(chan struct{}, maxConcurrent)

	for i := 0; i < len(validExpirations); i += expBatchSize {
		end := i + expBatchSize
		if end > len(validExpirations) {
			end = len(validExpirations)
		}
		batchData := validExpirations[i:end]

		wg.Add(1)
		go func(batch []*expData) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			// Collect symbols for this batch
			var batchSymbols []string
			for _, data := range batch {
				batchSymbols = append(batchSymbols, data.atmCall.Symbol, data.atmPut.Symbol)
			}

			// Fetch Greeks for this batch only
			batchCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
			defer cancel()
			localGreeks, err := s.providerManager.GetOptionsGreeksBatch(batchCtx, batchSymbols)
			if err != nil {
				log.Printf("⚠️ Batch fetch error for symbols %v: %v (skipping batch)", batchSymbols, err)
				return
			}
			if localGreeks == nil {
				localGreeks = make(map[string]map[string]interface{})
			}

			// Calculate and stream for this batch immediately
			for _, data := range batch {
				ivxResult := s.calculator.CalculateIVxForExpiration(data.dateStr, data.optionsChain, underlyingPrice, localGreeks)
				if ivxResult != nil {
					// Stream individual result as soon as calculated
					updates <- StreamUpdate{Type: "data", Payload: ivxResult}

					// Collect for final
					resultsMutex.Lock()
					allResults = append(allResults, *ivxResult)
					resultsMutex.Unlock()
				} else {
					log.Printf("⚠️ Skipping exp %s: no valid IVs (likely timeout or no Greeks)", data.dateStr)
				}
			}
		}(batchData)
	}

	wg.Wait()

	log.Printf("✅ Completed IVx calculation for %d/%d expirations", len(allResults), len(validExpirations))

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
