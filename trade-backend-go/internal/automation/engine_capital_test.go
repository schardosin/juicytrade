package automation

import (
	"testing"

	"trade-backend-go/internal/models"
)

// --- helpers ---

func fptr(f float64) *float64 { return &f }

// ---- Step 5: readNetLiq ----

func TestReadNetLiq(t *testing.T) {
	cases := []struct {
		name    string
		acct    *models.Account
		wantVal float64
		wantOK  bool
	}{
		{"nil account", nil, 0, false},
		{"portfolio value set", &models.Account{PortfolioValue: fptr(10000)}, 10000, true},
		{"equity fallback", &models.Account{Equity: fptr(8000)}, 8000, true},
		{"portfolio preferred over equity", &models.Account{PortfolioValue: fptr(10000), Equity: fptr(8000)}, 10000, true},
		{"zero portfolio falls through to equity", &models.Account{PortfolioValue: fptr(0), Equity: fptr(8000)}, 8000, true},
		{"both nil", &models.Account{}, 0, false},
		{"both non-positive", &models.Account{PortfolioValue: fptr(0), Equity: fptr(-1)}, 0, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			val, ok := readNetLiq(c.acct)
			if val != c.wantVal || ok != c.wantOK {
				t.Errorf("readNetLiq() = (%v, %v), want (%v, %v)", val, ok, c.wantVal, c.wantOK)
			}
		})
	}
}
