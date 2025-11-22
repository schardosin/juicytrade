package ivx

import (
	"math"
	"time"

	"trade-backend-go/internal/models"
)

// Calculator handles IVx calculations for options data
type Calculator struct{}

// NewCalculator creates a new IVx calculator instance
func NewCalculator() *Calculator {
	return &Calculator{}
}

// CalculateIVxForExpiration calculates IVx for a single expiration using ATM average method
func (c *Calculator) CalculateIVxForExpiration(expirationDate string, optionsChain []models.OptionContract, underlyingPrice float64, greeksMap map[string]map[string]interface{}) *models.IVxExpiration {
	if len(optionsChain) == 0 || underlyingPrice <= 0 {
		return nil
	}

	// 1. Find ATM strike
	atmStrike := c.findATMStrike(optionsChain, underlyingPrice)
	if atmStrike == 0 {
		return nil
	}

	// 2. Find ATM call and put contracts
	var atmCall, atmPut *models.OptionContract
	for i := range optionsChain {
		contract := &optionsChain[i]
		if contract.StrikePrice == atmStrike {
			if contract.Type == "call" {
				atmCall = contract
			} else if contract.Type == "put" {
				atmPut = contract
			}
		}
	}

	if atmCall == nil || atmPut == nil {
		return nil
	}

	// 3. Get IVs from the provided greeks map
	callIV := c.getIVFromGreeks(greeksMap, atmCall.Symbol)
	putIV := c.getIVFromGreeks(greeksMap, atmPut.Symbol)

	var validIVs []float64
	if callIV > 0 {
		validIVs = append(validIVs, callIV)
	}
	if putIV > 0 {
		validIVs = append(validIVs, putIV)
	}

	if len(validIVs) == 0 {
		return nil
	}

	// 4. Calculate Average IV
	ivx := 0.0
	for _, iv := range validIVs {
		ivx += iv
	}
	ivx /= float64(len(validIVs))

	// 5. Calculate DTE and Expected Move
	dte, isExpired := c.calculateDaysToExpiration(expirationDate)
	
	// Handle expired options
	adjustedIVx := ivx
	calculationMethod := "atm_iv_average"
	
	if isExpired {
		// For expired options, IVx should be near zero since there's no time value
		adjustedIVx = 0.001 // Very small value (0.1%)
		calculationMethod = "atm_iv_average_expired_adjusted"
	}
	
	// Ensure DTE is positive for calculation
	calcDte := dte
	if calcDte < 0.0001 {
		calcDte = 0.0001
	}

	expectedMove := underlyingPrice * adjustedIVx * math.Sqrt(calcDte/365)

	return &models.IVxExpiration{
		ExpirationDate:      expirationDate,
		DaysToExpiration:    dte,
		IVxPercent:          math.Round(adjustedIVx*100*100) / 100,
		ExpectedMoveDollars: math.Round(expectedMove*100) / 100,
		CalculationMethod:   calculationMethod,
		OptionsCount:        len(optionsChain),
		IsExpired:           isExpired,
	}
}

// findATMStrike finds the at-the-money strike price
func (c *Calculator) findATMStrike(optionsChain []models.OptionContract, underlyingPrice float64) float64 {
	if len(optionsChain) == 0 {
		return 0
	}

	minDiff := math.MaxFloat64
	var atmStrike float64

	for _, contract := range optionsChain {
		diff := math.Abs(contract.StrikePrice - underlyingPrice)
		if diff < minDiff {
			minDiff = diff
			atmStrike = contract.StrikePrice
		}
	}

	return atmStrike
}

// getIVFromGreeks extracts IV from the greeks map for a symbol
func (c *Calculator) getIVFromGreeks(greeksMap map[string]map[string]interface{}, symbol string) float64 {
	if greeks, ok := greeksMap[symbol]; ok {
		if iv, ok := greeks["implied_volatility"].(float64); ok {
			return iv
		}
	}
	return 0
}

// calculateDaysToExpiration calculates DTE with Eastern Time logic
func (c *Calculator) calculateDaysToExpiration(expirationDate string) (float64, bool) {
	// Parse expiration date
	expDate, err := time.Parse("2006-01-02", expirationDate)
	if err != nil {
		return 0, false
	}

	// Always use Eastern Time for all date calculations
	et, err := time.LoadLocation("America/New_York")
	if err != nil {
		// Fallback to UTC if ET fails, though this shouldn't happen in valid env
		et = time.UTC
	}

	now := time.Now().In(et)
	today := now.Truncate(24 * time.Hour)
	expDate = expDate.In(et)

	// For same-day expiration (0DTE)
	if expDate.Equal(today) {
		// Market close is 4:00 PM ET
		marketClose := time.Date(today.Year(), today.Month(), today.Day(), 16, 0, 0, 0, et)

		// If after market close
		if now.After(marketClose) {
			return 0.0001, true
		}
		
		// Calculate fractional time remaining until market close
		timeRemaining := marketClose.Sub(now)
		dteHours := timeRemaining.Hours()
		return dteHours / 24, false
	}

	// For future expirations
	fullDays := expDate.Sub(today).Hours() / 24

	// Add fractional day based on current time (assume 4:00 PM ET expiration)
	marketCloseToday := time.Date(today.Year(), today.Month(), today.Day(), 16, 0, 0, 0, et)
	if now.Before(marketCloseToday) {
		timeRemainingToday := marketCloseToday.Sub(now)
		fractionalDay := timeRemainingToday.Hours() / 24
		fullDays += fractionalDay
	}

	if fullDays <= 0 {
		fullDays = 0.0001
	}

	return fullDays, false
}


