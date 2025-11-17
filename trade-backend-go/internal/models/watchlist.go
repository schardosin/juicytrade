package models

import (
	"fmt"
	"strings"
	"time"
)

// Watchlist represents a watchlist with its metadata
type Watchlist struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Symbols   []string  `json:"symbols"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// CreateWatchlistRequest represents a request to create a new watchlist
type CreateWatchlistRequest struct {
	Name    string   `json:"name" binding:"required,min=1,max=100"`
	Symbols []string `json:"symbols"`
}

// Validate validates and cleans the create watchlist request
func (r *CreateWatchlistRequest) Validate() error {
	r.Name = strings.TrimSpace(r.Name)
	if r.Name == "" {
		return fmt.Errorf("watchlist name cannot be empty")
	}
	
	// Clean and validate symbols
	cleanSymbols := []string{}
	for _, symbol := range r.Symbols {
		if cleanSymbol := strings.TrimSpace(strings.ToUpper(symbol)); cleanSymbol != "" {
			if len(cleanSymbol) >= 1 && len(cleanSymbol) <= 10 && isAlphanumeric(cleanSymbol) {
				cleanSymbols = append(cleanSymbols, cleanSymbol)
			}
		}
	}
	r.Symbols = cleanSymbols
	
	return nil
}

// UpdateWatchlistRequest represents a request to update a watchlist
type UpdateWatchlistRequest struct {
	Name    *string  `json:"name,omitempty"`
	Symbols []string `json:"symbols,omitempty"`
}

// Validate validates and cleans the update watchlist request
func (r *UpdateWatchlistRequest) Validate() error {
	if r.Name != nil {
		*r.Name = strings.TrimSpace(*r.Name)
		if *r.Name == "" {
			return fmt.Errorf("watchlist name cannot be empty")
		}
	}
	
	if r.Symbols != nil {
		// Clean and validate symbols
		cleanSymbols := []string{}
		for _, symbol := range r.Symbols {
			if cleanSymbol := strings.TrimSpace(strings.ToUpper(symbol)); cleanSymbol != "" {
				if len(cleanSymbol) >= 1 && len(cleanSymbol) <= 10 && isAlphanumeric(cleanSymbol) {
					cleanSymbols = append(cleanSymbols, cleanSymbol)
				}
			}
		}
		r.Symbols = cleanSymbols
	}
	
	return nil
}

// AddSymbolRequest represents a request to add a symbol to a watchlist
type AddSymbolRequest struct {
	Symbol string `json:"symbol" binding:"required,min=1,max=10"`
}

// Validate validates and cleans the add symbol request
func (r *AddSymbolRequest) Validate() error {
	r.Symbol = strings.TrimSpace(strings.ToUpper(r.Symbol))
	if r.Symbol == "" {
		return fmt.Errorf("symbol cannot be empty")
	}
	
	if !isAlphanumeric(r.Symbol) {
		return fmt.Errorf("symbol must be alphanumeric")
	}
	
	if len(r.Symbol) < 1 || len(r.Symbol) > 10 {
		return fmt.Errorf("symbol must be 1-10 characters long")
	}
	
	return nil
}

// SetActiveWatchlistRequest represents a request to set the active watchlist
type SetActiveWatchlistRequest struct {
	WatchlistID string `json:"watchlist_id" binding:"required,min=1"`
}

// SearchWatchlistsRequest represents a request to search watchlists
type SearchWatchlistsRequest struct {
	Query string `json:"query" binding:"required,min=1"`
}

// Validate validates the search request
func (r *SearchWatchlistsRequest) Validate() error {
	r.Query = strings.TrimSpace(r.Query)
	if r.Query == "" {
		return fmt.Errorf("search query cannot be empty")
	}
	return nil
}

// WatchlistsResponse represents the response for getting all watchlists
type WatchlistsResponse struct {
	Watchlists      map[string]*Watchlist `json:"watchlists"`
	ActiveWatchlist string                `json:"active_watchlist"`
	TotalWatchlists int                   `json:"total_watchlists"`
	Version         string                `json:"version"`
}

// Helper function to check if a string is alphanumeric
func isAlphanumeric(s string) bool {
	for _, r := range s {
		if !((r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')) {
			return false
		}
	}
	return true
}
