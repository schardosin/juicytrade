package models

// SubscriptionStatusResponse represents the subscription status data structure
// that matches the Python implementation exactly
type SubscriptionStatusResponse struct {
	QuoteSubscriptions       []string `json:"quote_subscriptions"`
	GreeksSubscriptions      []string `json:"greeks_subscriptions"`
	TotalQuoteSubscriptions  int      `json:"total_quote_subscriptions"`
	TotalGreeksSubscriptions int      `json:"total_greeks_subscriptions"`
	IsConnected              bool     `json:"is_connected"`
	QuoteProviders           []string `json:"quote_providers"`
	GreeksProviders          []string `json:"greeks_providers"`
}
