package schwab

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strings"

	"trade-backend-go/internal/models"
)

// =============================================================================
// Account Info
// =============================================================================

// GetAccount gets account information including balances and buying power.
// Uses GET /trader/v1/accounts/{accountHash}?fields=positions
func (s *SchwabProvider) GetAccount(ctx context.Context) (*models.Account, error) {
	reqURL := s.buildTraderURL("/accounts/" + s.accountHash + "?fields=positions")

	body, _, err := s.doAuthenticatedRequest(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("schwab: GetAccount failed: %w", err)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("schwab: failed to parse account response: %w", err)
	}

	return transformSchwabAccount(response, s.accountHash), nil
}

// transformSchwabAccount transforms a Schwab account response into an Account model.
//
// Schwab response structure:
//
//	{
//	  "securitiesAccount": {
//	    "type": "MARGIN",
//	    "accountNumber": "12345678",
//	    "currentBalances": {
//	      "liquidationValue": 50000.0,
//	      "buyingPower": 25000.0,
//	      "cashBalance": 10000.0,
//	      "equity": 40000.0,
//	      "longMarketValue": 35000.0,
//	      "shortMarketValue": 0.0,
//	      "dayTradingBuyingPower": 100000.0,
//	      "availableFunds": 25000.0,
//	      "maintenanceRequirement": 15000.0
//	    }
//	  }
//	}
func transformSchwabAccount(response map[string]interface{}, accountHash string) *models.Account {
	account := models.NewAccount()
	account.AccountID = accountHash
	account.Status = "active"

	// Navigate to securitiesAccount
	secAcct := extractMap(response, "securitiesAccount")
	if secAcct == nil {
		// Some responses may not have the wrapper
		secAcct = response
	}

	if acctNum, ok := extractString(secAcct, "accountNumber"); ok {
		account.AccountNumber = &acctNum
	}

	// Extract balances from currentBalances
	balances := extractMap(secAcct, "currentBalances")
	if balances == nil {
		// Try initialBalances as fallback
		balances = extractMap(secAcct, "initialBalances")
	}
	if balances == nil {
		return account
	}

	account.BuyingPower = extractFloat64Ptr(balances, "buyingPower")
	account.Cash = extractFloat64Ptr(balances, "cashBalance")
	account.PortfolioValue = extractFloat64Ptr(balances, "liquidationValue")
	account.Equity = extractFloat64Ptr(balances, "equity")
	account.DayTradingBuyingPower = extractFloat64Ptr(balances, "dayTradingBuyingPower")
	account.LongMarketValue = extractFloat64Ptr(balances, "longMarketValue")
	account.ShortMarketValue = extractFloat64Ptr(balances, "shortMarketValue")
	account.MaintenanceMargin = extractFloat64Ptr(balances, "maintenanceRequirement")

	// Map availableFunds to regt buying power if not explicitly present
	if account.BuyingPower == nil {
		account.BuyingPower = extractFloat64Ptr(balances, "availableFunds")
	}

	return account
}

// =============================================================================
// Positions
// =============================================================================

// GetPositions gets all current positions.
// Uses GET /trader/v1/accounts/{accountHash}?fields=positions and extracts positions.
func (s *SchwabProvider) GetPositions(ctx context.Context) ([]*models.Position, error) {
	reqURL := s.buildTraderURL("/accounts/" + s.accountHash + "?fields=positions")

	body, _, err := s.doAuthenticatedRequest(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("schwab: GetPositions failed: %w", err)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("schwab: failed to parse positions response: %w", err)
	}

	return transformSchwabPositions(response), nil
}

// GetPositionsEnhanced gets enhanced positions with hierarchical grouping.
// Fetches positions then delegates to BaseProviderImpl for grouping.
func (s *SchwabProvider) GetPositionsEnhanced(ctx context.Context) (*models.EnhancedPositionsResponse, error) {
	positions, err := s.GetPositions(ctx)
	if err != nil {
		return models.NewEnhancedPositionsResponse(), err
	}
	if len(positions) == 0 {
		return models.NewEnhancedPositionsResponse(), nil
	}
	return s.BaseProviderImpl.ConvertPositionsToEnhanced(positions), nil
}

// transformSchwabPositions extracts and transforms positions from a Schwab account response.
func transformSchwabPositions(response map[string]interface{}) []*models.Position {
	// Navigate to securitiesAccount.positions
	secAcct := extractMap(response, "securitiesAccount")
	if secAcct == nil {
		secAcct = response
	}

	positionsRaw, ok := secAcct["positions"]
	if !ok {
		return []*models.Position{}
	}
	positionsArr, ok := positionsRaw.([]interface{})
	if !ok {
		return []*models.Position{}
	}

	var positions []*models.Position
	for _, pRaw := range positionsArr {
		pData, ok := pRaw.(map[string]interface{})
		if !ok {
			continue
		}
		pos := transformSchwabPosition(pData)
		if pos != nil {
			positions = append(positions, pos)
		}
	}

	return positions
}

// transformSchwabPosition transforms a single Schwab position into a Position model.
//
// Schwab position structure:
//
//	{
//	  "instrument": {"symbol": "AAPL", "assetType": "EQUITY", ...},
//	  "longQuantity": 100.0,
//	  "shortQuantity": 0.0,
//	  "averagePrice": 150.0,
//	  "marketValue": 15025.0,
//	  "currentDayProfitLoss": 25.0,
//	  "currentDayProfitLossPercentage": 0.17
//	}
func transformSchwabPosition(data map[string]interface{}) *models.Position {
	instrument := extractMap(data, "instrument")
	if instrument == nil {
		return nil
	}

	rawSymbol, _ := extractString(instrument, "symbol")
	if rawSymbol == "" {
		return nil
	}

	assetType, _ := extractString(instrument, "assetType")

	// Determine asset class and convert symbol
	var assetClass, symbol string
	switch strings.ToUpper(assetType) {
	case "OPTION":
		assetClass = "us_option"
		symbol = convertSchwabOptionToOCC(rawSymbol)
	default:
		assetClass = "us_equity"
		symbol = rawSymbol
	}

	// Calculate net quantity
	longQty, _ := extractFloat64(data, "longQuantity")
	shortQty, _ := extractFloat64(data, "shortQuantity")
	qty := longQty - shortQty

	side := "long"
	if qty < 0 {
		side = "short"
	}

	avgPrice, _ := extractFloat64(data, "averagePrice")
	marketValue, _ := extractFloat64(data, "marketValue")
	unrealizedPL, _ := extractFloat64(data, "currentDayProfitLoss")
	costBasis, _ := extractFloat64(data, "currentDayCost")

	// Calculate current price from market value and quantity
	var currentPrice float64
	if math.Abs(qty) > 0 {
		currentPrice = math.Abs(marketValue / qty)
	}

	unrealizedPLPC := extractFloat64Ptr(data, "currentDayProfitLossPercentage")

	pos := &models.Position{
		Symbol:         symbol,
		Qty:            math.Abs(qty),
		Side:           side,
		MarketValue:    marketValue,
		CostBasis:      costBasis,
		UnrealizedPL:   unrealizedPL,
		UnrealizedPLPC: unrealizedPLPC,
		CurrentPrice:   currentPrice,
		AvgEntryPrice:  avgPrice,
		AssetClass:     assetClass,
	}

	// For option positions, parse the OCC symbol for additional fields
	if assetClass == "us_option" {
		parseOCCSymbolIntoPosition(pos, symbol)
	}

	return pos
}

// parseOCCSymbolIntoPosition extracts option-specific fields from an OCC symbol
// and sets them on the position.
//
// OCC format: AAPL250117C00150000
//
//	underlying (variable) + YYMMDD (6) + C/P (1) + strike×1000 (8) = 15-char suffix
func parseOCCSymbolIntoPosition(pos *models.Position, occSymbol string) {
	const suffixLen = 15
	if len(occSymbol) <= suffixLen {
		return
	}

	underlying := occSymbol[:len(occSymbol)-suffixLen]
	suffix := occSymbol[len(occSymbol)-suffixLen:]

	pos.UnderlyingSymbol = &underlying

	// Option type from position 6 of suffix (index 6)
	optType := string(suffix[6])
	if optType == "C" {
		call := "call"
		pos.OptionType = &call
	} else if optType == "P" {
		put := "put"
		pos.OptionType = &put
	}

	// Expiration date: YYMMDD from positions 0-5
	if len(suffix) >= 6 {
		yy := suffix[0:2]
		mm := suffix[2:4]
		dd := suffix[4:6]
		expiry := "20" + yy + "-" + mm + "-" + dd
		pos.ExpiryDate = &expiry
	}

	// Strike price: last 8 digits / 1000
	if len(suffix) >= 15 {
		strikeStr := suffix[7:15]
		strikeInt := 0
		for _, ch := range strikeStr {
			if ch >= '0' && ch <= '9' {
				strikeInt = strikeInt*10 + int(ch-'0')
			}
		}
		strike := float64(strikeInt) / 1000.0
		pos.StrikePrice = &strike
	}
}
