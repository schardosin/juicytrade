package handlers

import (
	"testing"

	"trade-backend-go/internal/automation/types"
)

func TestValidateTradeCapital(t *testing.T) {
	cases := []struct {
		name    string
		tc      types.TradeConfiguration
		wantErr bool
	}{
		{
			name: "percent 60 ok",
			tc:   types.TradeConfiguration{MaxCapitalMode: types.MaxCapitalModePercent, MaxCapitalPercent: 60},
		},
		{
			name:    "percent 0 error",
			tc:      types.TradeConfiguration{MaxCapitalMode: types.MaxCapitalModePercent, MaxCapitalPercent: 0},
			wantErr: true,
		},
		{
			name:    "percent 150 error",
			tc:      types.TradeConfiguration{MaxCapitalMode: types.MaxCapitalModePercent, MaxCapitalPercent: 150},
			wantErr: true,
		},
		{
			name: "percent 100 boundary ok",
			tc:   types.TradeConfiguration{MaxCapitalMode: types.MaxCapitalModePercent, MaxCapitalPercent: 100},
		},
		{
			name: "percent 1 boundary ok",
			tc:   types.TradeConfiguration{MaxCapitalMode: types.MaxCapitalModePercent, MaxCapitalPercent: 1},
		},
		{
			name: "fixed 5000 ok",
			tc:   types.TradeConfiguration{MaxCapitalMode: types.MaxCapitalModeFixed, MaxCapital: 5000},
		},
		{
			name:    "fixed 50 error",
			tc:      types.TradeConfiguration{MaxCapitalMode: types.MaxCapitalModeFixed, MaxCapital: 50},
			wantErr: true,
		},
		{
			name: "empty mode 5000 ok",
			tc:   types.TradeConfiguration{MaxCapital: 5000}, // MaxCapitalMode == "" defaults to fixed
		},
		{
			name:    "empty mode 50 error",
			tc:      types.TradeConfiguration{MaxCapital: 50},
			wantErr: true,
		},
		{
			name: "fixed 100 boundary ok",
			tc:   types.TradeConfiguration{MaxCapitalMode: types.MaxCapitalModeFixed, MaxCapital: 100},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := validateTradeCapital(c.tc)
			if c.wantErr && err == nil {
				t.Errorf("expected error, got nil")
			}
			if !c.wantErr && err != nil {
				t.Errorf("expected no error, got %v", err)
			}
		})
	}
}
