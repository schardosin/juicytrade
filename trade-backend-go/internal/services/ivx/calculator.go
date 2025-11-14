package ivx

import (
	"context"
	"log"
	"math"
	"sort"

	"trade-backend-go/internal/models"
)

// Configuration constants for IVx calculation - matching Python implementation
const (
	IVxCalculationMethod     = "price_based"
	MinStrikesRequired       = 3
	MinOptionPrice           = 0.01
	MaxBidAskSpreadRatio     = 0.5
	EnableCalculationLogging = true
	MinVarianceThreshold     = 0.0001
	MaxVolatilityThreshold   = 5.0
)

// Calculator handles IVx calculations for options data
type Calculator struct{}

// NewCalculator creates a new IVx calculator instance
func NewCalculator() *Calculator {
	return &Calculator{}
}

// CalculateIVxData calculates IVx and expected move for an options chain
// Uses price-based calculation by default, falls back to IV average if insufficient data
func (c *Calculator) CalculateIVxData(optionsChain []models.OptionContract, underlyingPrice float64, dte float64) *models.IVxExpiration {
	if EnableCalculationLogging {
		log.Printf("🔍 CalculateIVxData: chain_len=%d, underlying=%.2f, dte=%.2f", len(optionsChain), underlyingPrice, dte)
	}

	if len(optionsChain) == 0 || underlyingPrice <= 0 || dte <= 0 {
		if EnableCalculationLogging {
			log.Printf("Invalid inputs for IVx: chain_len=%d, underlying=%.2f, dte=%.2f", len(optionsChain), underlyingPrice, dte)
		}
		return nil
	}

	// Log first few options for debugging
	if EnableCalculationLogging {
		for i := 0; i < len(optionsChain) && i < 3; i++ {
			opt := optionsChain[i]
			log.Printf("🔍 Option %d: Strike=%.2f, Type=%s, Bid=%v, Ask=%v, Close=%v", 
				i, opt.StrikePrice, opt.Type, opt.Bid, opt.Ask, opt.ClosePrice)
		}
	}

	var ivx *float64
	calculationMethod := ""

	// Try price-based calculation first
	if IVxCalculationMethod == "price_based" {
		if EnableCalculationLogging {
			log.Printf("🔍 Attempting price-based IVx calculation")
		}
		calculatedIVx := c.calculatePriceBasedIVx(optionsChain, underlyingPrice, dte)
		if calculatedIVx != nil {
			ivx = calculatedIVx
			calculationMethod = "price_based"
			if EnableCalculationLogging {
				log.Printf("✅ Price-based IVx successful: %.4f", *ivx)
			}
		} else {
			if EnableCalculationLogging {
				log.Printf("❌ Price-based IVx failed")
			}
		}
	}

	// Fallback to IV average method if price-based fails
	if ivx == nil {
		if EnableCalculationLogging {
			log.Printf("🔍 Attempting IV average fallback")
		}
		calculatedIVx := c.getATMImpliedVolatility(optionsChain, underlyingPrice)
		if calculatedIVx != nil {
			ivx = calculatedIVx
			calculationMethod = "iv_average"
			if EnableCalculationLogging {
				log.Printf("✅ IV average fallback successful: %.4f", *ivx)
			}
		} else {
			if EnableCalculationLogging {
				log.Printf("❌ IV average fallback failed")
			}
		}
	}

	if ivx == nil {
		if EnableCalculationLogging {
			log.Printf("❌ All IVx calculation methods failed")
		}
		return nil
	}

	// Calculate expected move
	expectedMove := c.calculateExpectedMove(underlyingPrice, *ivx, dte)

	return &models.IVxExpiration{
		DaysToExpiration:     dte,
		IVxPercent:           math.Round(*ivx*100*100) / 100, // Round to 2 decimal places
		ExpectedMoveDollars:  math.Round(expectedMove*100) / 100, // Round to 2 decimal places
		CalculationMethod:    calculationMethod,
		OptionsCount:         len(optionsChain),
		IsExpired:            dte <= 0.0001,
	}
}

// calculatePriceBasedIVx calculates IVx using option prices (VIX-style methodology)
func (c *Calculator) calculatePriceBasedIVx(optionsChain []models.OptionContract, underlyingPrice, dte float64) *float64 {
	if len(optionsChain) == 0 || underlyingPrice <= 0 || dte <= 0 {
		return nil
	}

	// Select relevant options for calculation
	selectedOptions := c.selectOptionStrikes(optionsChain, underlyingPrice)

	if len(selectedOptions) < MinStrikesRequired {
		if EnableCalculationLogging {
			log.Printf("Insufficient strikes for price-based IVx: %d < %d", len(selectedOptions), MinStrikesRequired)
		}
		return nil
	}

	// Calculate time to expiration in years
	T := dte / 365.0
	if T <= 0 {
		return nil
	}

	// Sort options by strike
	sort.Slice(selectedOptions, func(i, j int) bool {
		return selectedOptions[i].Strike < selectedOptions[j].Strike
	})

	strikes := make([]float64, len(selectedOptions))
	for i, opt := range selectedOptions {
		strikes[i] = opt.Strike
	}

	// Calculate variance contribution from each strike
	totalVarianceContribution := 0.0
	contributionsCount := 0

	if EnableCalculationLogging {
		log.Printf("Calculating price-based IVx with %d strikes, T=%.4f", len(selectedOptions), T)
	}

	for i, option := range selectedOptions {
		strike := option.Strike
		price := option.Price

		// Calculate ΔK for this strike
		deltaK := c.calculateDeltaK(strikes, i)

		if deltaK > 0 && strike > 0 {
			// Add contribution: (ΔK/K²) * price
			contribution := (deltaK / (strike * strike)) * price
			totalVarianceContribution += contribution
			contributionsCount++

			if EnableCalculationLogging {
				log.Printf("Strike %.2f: price=%.4f, ΔK=%.2f, contribution=%.8f", strike, price, deltaK, contribution)
			}
		}
	}

	if totalVarianceContribution <= 0 || contributionsCount == 0 {
		if EnableCalculationLogging {
			log.Printf("No valid contributions: total=%.8f, count=%d", totalVarianceContribution, contributionsCount)
		}
		return nil
	}

	// Calculate variance: σ² = (2/T) * Σ(ΔK/K²) * price
	variance := (2 / T) * totalVarianceContribution

	// Quality control checks
	if variance < MinVarianceThreshold {
		if EnableCalculationLogging {
			log.Printf("Variance too low: %.8f < %.8f", variance, MinVarianceThreshold)
		}
		return nil
	}

	// Convert to annualized volatility
	if variance > 0 {
		volatility := math.Sqrt(variance)

		// Additional quality control
		if volatility > MaxVolatilityThreshold {
			if EnableCalculationLogging {
				log.Printf("Volatility too high: %.4f > %.4f", volatility, MaxVolatilityThreshold)
			}
			return nil
		}

		if EnableCalculationLogging {
			log.Printf("Price-based IVx calculated: %.4f (%.2f%%) from %d strikes", volatility, volatility*100, contributionsCount)
		}

		return &volatility // Return as decimal
	}

	return nil
}

// selectedOption represents an option selected for IVx calculation
type selectedOption struct {
	Strike float64
	Price  float64
	Type   string
}

// selectOptionStrikes selects and organizes options for price-based calculation
func (c *Calculator) selectOptionStrikes(optionsChain []models.OptionContract, underlyingPrice float64) []selectedOption {
	if len(optionsChain) == 0 {
		if EnableCalculationLogging {
			log.Printf("🔍 selectOptionStrikes: Empty options chain")
		}
		return []selectedOption{}
	}

	if EnableCalculationLogging {
		log.Printf("🔍 selectOptionStrikes: Processing %d options, underlying=%.2f", len(optionsChain), underlyingPrice)
	}

	// Find ATM strike
	atmStrike := c.findATMStrike(optionsChain, underlyingPrice)
	if atmStrike == nil {
		if EnableCalculationLogging {
			log.Printf("🔍 selectOptionStrikes: Could not find ATM strike")
		}
		return []selectedOption{}
	}

	if EnableCalculationLogging {
		log.Printf("🔍 selectOptionStrikes: ATM strike found: %.2f", *atmStrike)
	}

	selectedOptions := []selectedOption{}

	// Group options by strike and type
	optionsByStrike := make(map[float64]map[string][]models.OptionContract)
	for i, option := range optionsChain {
		strike := option.StrikePrice
		if _, exists := optionsByStrike[strike]; !exists {
			optionsByStrike[strike] = map[string][]models.OptionContract{
				"calls": {},
				"puts":  {},
			}
		}

		if EnableCalculationLogging && i < 5 { // Log first 5 options for debugging
			log.Printf("🔍 Option %d: Strike=%.2f, Type=%s, Bid=%v, Ask=%v, Close=%v", 
				i, option.StrikePrice, option.Type, option.Bid, option.Ask, option.ClosePrice)
		}

		if option.Type == "call" {
			optionsByStrike[strike]["calls"] = append(optionsByStrike[strike]["calls"], option)
		} else if option.Type == "put" {
			optionsByStrike[strike]["puts"] = append(optionsByStrike[strike]["puts"], option)
		}
	}

	if EnableCalculationLogging {
		log.Printf("🔍 selectOptionStrikes: Grouped into %d strikes", len(optionsByStrike))
	}

	// Process each strike
	for strike := range optionsByStrike {
		calls := optionsByStrike[strike]["calls"]
		puts := optionsByStrike[strike]["puts"]

		if strike < *atmStrike {
			// OTM puts for strikes below ATM
			for _, put := range puts {
				price := c.calculateOptionPrice(put)
				if price != nil && *price > 0 {
					selectedOptions = append(selectedOptions, selectedOption{
						Strike: strike,
						Price:  *price,
						Type:   "put",
					})
					break // Use first valid put at this strike
				}
			}
		} else if strike > *atmStrike {
			// OTM calls for strikes above ATM
			for _, call := range calls {
				price := c.calculateOptionPrice(call)
				if price != nil && *price > 0 {
					selectedOptions = append(selectedOptions, selectedOption{
						Strike: strike,
						Price:  *price,
						Type:   "call",
					})
					break // Use first valid call at this strike
				}
			}
		} else { // strike == atmStrike
			// ATM: Average call and put prices
			callPrice := c.findBestPrice(calls)
			putPrice := c.findBestPrice(puts)

			// Use average if both available, otherwise use whichever is available
			if callPrice != nil && putPrice != nil {
				avgPrice := (*callPrice + *putPrice) / 2
				selectedOptions = append(selectedOptions, selectedOption{
					Strike: strike,
					Price:  avgPrice,
					Type:   "atm",
				})
			} else if callPrice != nil {
				selectedOptions = append(selectedOptions, selectedOption{
					Strike: strike,
					Price:  *callPrice,
					Type:   "call",
				})
			} else if putPrice != nil {
				selectedOptions = append(selectedOptions, selectedOption{
					Strike: strike,
					Price:  *putPrice,
					Type:   "put",
				})
			}
		}
	}

	return selectedOptions
}

// findBestPrice finds the best available price for a list of options at the same strike
func (c *Calculator) findBestPrice(options []models.OptionContract) *float64 {
	for _, option := range options {
		price := c.calculateOptionPrice(option)
		if price != nil && *price > 0 {
			return price
		}
	}
	return nil
}

// calculateOptionPrice calculates option price using best available data with quality checks
func (c *Calculator) calculateOptionPrice(option models.OptionContract) *float64 {
	price := 0.0

	// Priority: 1) Midpoint of bid/ask, 2) Close price
	if option.Bid != nil && option.Ask != nil && *option.Bid > 0 && *option.Ask > 0 {
		midpoint := (*option.Bid + *option.Ask) / 2
		spread := *option.Ask - *option.Bid

		// Check if spread is reasonable (not too wide)
		if midpoint > 0 && spread/midpoint <= MaxBidAskSpreadRatio {
			price = midpoint
		}
	}

	// Fallback to close price if bid/ask is not suitable
	if price == 0 && option.ClosePrice != nil && *option.ClosePrice > 0 {
		price = *option.ClosePrice
	}

	// Validate minimum price threshold
	if price >= MinOptionPrice {
		return &price
	}

	return nil
}

// calculateDeltaK calculates the strike interval (ΔK) for a given strike
func (c *Calculator) calculateDeltaK(strikes []float64, currentIndex int) float64 {
	if len(strikes) < 2 {
		return 0
	}

	if currentIndex == 0 { // First strike
		return strikes[1] - strikes[0]
	} else if currentIndex == len(strikes)-1 { // Last strike
		return strikes[len(strikes)-1] - strikes[len(strikes)-2]
	} else { // Middle strikes
		return (strikes[currentIndex+1] - strikes[currentIndex-1]) / 2
	}
}

// findATMStrike finds the at-the-money strike price
func (c *Calculator) findATMStrike(optionsChain []models.OptionContract, underlyingPrice float64) *float64 {
	if len(optionsChain) == 0 {
		return nil
	}

	minDiff := math.MaxFloat64
	var atmStrike *float64

	for _, contract := range optionsChain {
		diff := math.Abs(contract.StrikePrice - underlyingPrice)
		if diff < minDiff {
			minDiff = diff
			atmStrike = &contract.StrikePrice
		}
	}

	return atmStrike
}

// getATMImpliedVolatility gets the average implied volatility of the ATM options
func (c *Calculator) getATMImpliedVolatility(optionsChain []models.OptionContract, underlyingPrice float64) *float64 {
	atmStrike := c.findATMStrike(optionsChain, underlyingPrice)
	if atmStrike == nil {
		return nil
	}

	atmIVs := []float64{}
	for _, contract := range optionsChain {
		if contract.StrikePrice == *atmStrike && contract.ImpliedVolatility != nil {
			atmIVs = append(atmIVs, *contract.ImpliedVolatility)
		}
	}

	if len(atmIVs) == 0 {
		return nil
	}

	// Return average IV
	avgIV := 0.0
	for _, iv := range atmIVs {
		avgIV += iv
	}
	avgIV /= float64(len(atmIVs))

	return &avgIV
}

// calculateExpectedMove calculates the expected move in dollars
func (c *Calculator) calculateExpectedMove(underlyingPrice, ivx, dte float64) float64 {
	if underlyingPrice <= 0 || ivx <= 0 || dte <= 0 {
		return 0
	}

	// Formula: Expected Move = Stock Price * Implied Volatility * sqrt(Days to Expiration / 365)
	return underlyingPrice * ivx * math.Sqrt(dte/365)
}

// ProcessExpiration processes a single expiration for IVx calculation
func (c *Calculator) ProcessExpiration(ctx context.Context, symbol, expirationDate string, underlyingPrice float64, provider interface{}) (*models.IVxExpiration, error) {
	// This method will be called by the main handler to process individual expirations
	// The provider interface will need to be properly typed based on the actual provider implementation

	// For now, return a placeholder - this will be implemented when we integrate with the main handler
	return &models.IVxExpiration{
		ExpirationDate:      expirationDate,
		DaysToExpiration:    1.0,
		IVxPercent:          20.0,
		ExpectedMoveDollars: 10.0,
		CalculationMethod:   "placeholder",
		OptionsCount:        0,
		IsExpired:           false,
	}, nil
}
