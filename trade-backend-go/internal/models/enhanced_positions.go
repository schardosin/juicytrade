package models

// EnhancedPositionsResponse represents the enhanced positions response structure
// Exact conversion of Python enhanced positions format
type EnhancedPositionsResponse struct {
	Enhanced     bool          `json:"enhanced"`
	SymbolGroups []SymbolGroup `json:"symbol_groups"`
}

// SymbolGroup represents a group of positions for the same underlying symbol
type SymbolGroup struct {
	Symbol     string     `json:"symbol"`
	AssetClass string     `json:"asset_class"`
	Strategies []Strategy `json:"strategies"`
}

// Strategy represents a trading strategy with its legs
type Strategy struct {
	Name         string         `json:"name"`
	TotalQty     float64        `json:"total_qty"`
	CostBasis    float64        `json:"cost_basis"`
	DTE          *int           `json:"dte"`
	Legs         []PositionLeg  `json:"legs"`
	DateAcquired *string        `json:"date_acquired,omitempty"`
}

// PositionLeg represents a single leg within a strategy
type PositionLeg struct {
	Symbol          string   `json:"symbol"`
	Qty             float64  `json:"qty"`
	AvgEntryPrice   float64  `json:"avg_entry_price"`
	CostBasis       float64  `json:"cost_basis"`
	AssetClass      string   `json:"asset_class"`
	LastdayPrice    *float64 `json:"lastday_price"`
	DateAcquired    *string  `json:"date_acquired"`
	
	// Option-specific fields
	UnderlyingSymbol *string  `json:"underlying_symbol,omitempty"`
	OptionType       *string  `json:"option_type,omitempty"`
	StrikePrice      *float64 `json:"strike_price,omitempty"`
	ExpiryDate       *string  `json:"expiry_date,omitempty"`
}

// NewEnhancedPositionsResponse creates a new enhanced positions response
func NewEnhancedPositionsResponse() *EnhancedPositionsResponse {
	return &EnhancedPositionsResponse{
		Enhanced:     true,
		SymbolGroups: make([]SymbolGroup, 0),
	}
}

// NewSymbolGroup creates a new symbol group
func NewSymbolGroup(symbol, assetClass string) *SymbolGroup {
	return &SymbolGroup{
		Symbol:     symbol,
		AssetClass: assetClass,
		Strategies: make([]Strategy, 0),
	}
}

// NewStrategy creates a new strategy
func NewStrategy(name string) *Strategy {
	return &Strategy{
		Name: name,
		Legs: make([]PositionLeg, 0),
	}
}

// AddLeg adds a position leg to the strategy
func (s *Strategy) AddLeg(leg PositionLeg) {
	s.Legs = append(s.Legs, leg)
	s.TotalQty += leg.Qty
	s.CostBasis += leg.CostBasis
}

// AddStrategy adds a strategy to the symbol group
func (sg *SymbolGroup) AddStrategy(strategy Strategy) {
	sg.Strategies = append(sg.Strategies, strategy)
}

// AddSymbolGroup adds a symbol group to the response
func (epr *EnhancedPositionsResponse) AddSymbolGroup(group SymbolGroup) {
	epr.SymbolGroups = append(epr.SymbolGroups, group)
}
