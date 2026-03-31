package tastytrade

import (
	"testing"

	"trade-backend-go/internal/models"
)

func floatPtr(v float64) *float64 {
	return &v
}

func TestComputeUnderlyingPriceFromQuote(t *testing.T) {
	tests := []struct {
		name  string
		quote *models.StockQuote
		want  *float64
	}{
		{
			name:  "nil quote returns nil",
			quote: nil,
			want:  nil,
		},
		{
			name:  "all nil fields returns nil",
			quote: &models.StockQuote{Symbol: "NDX"},
			want:  nil,
		},
		{
			name:  "both bid and ask present returns midpoint",
			quote: &models.StockQuote{Symbol: "SPY", Bid: floatPtr(100.0), Ask: floatPtr(102.0), Last: floatPtr(101.5)},
			want:  floatPtr(101.0),
		},
		{
			name:  "only bid present no last returns nil",
			quote: &models.StockQuote{Symbol: "SPY", Bid: floatPtr(100.0)},
			want:  nil,
		},
		{
			name:  "only ask present no last returns nil",
			quote: &models.StockQuote{Symbol: "SPY", Ask: floatPtr(102.0)},
			want:  nil,
		},
		{
			name:  "no bid/ask but last present returns last (NDX case)",
			quote: &models.StockQuote{Symbol: "NDX", Last: floatPtr(18542.50)},
			want:  floatPtr(18542.50),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := computeUnderlyingPriceFromQuote(tt.quote)
			if tt.want == nil {
				if got != nil {
					t.Errorf("expected nil, got %v", *got)
				}
				return
			}
			if got == nil {
				t.Errorf("expected %v, got nil", *tt.want)
				return
			}
			if *got != *tt.want {
				t.Errorf("expected %v, got %v", *tt.want, *got)
			}
		})
	}
}
