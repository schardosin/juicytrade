package automation

import (
	"testing"

	"trade-backend-go/internal/automation/types"
)

func TestIndicatorSubscriptionSymbol(t *testing.T) {
	tests := []struct {
		name string
		ind  types.IndicatorConfig
		want string
	}{
		{name: "vix change fixed default", ind: types.IndicatorConfig{Type: types.IndicatorVIXChange}, want: "VIX"},
		{name: "vix change ignores stale symbol", ind: types.IndicatorConfig{Type: types.IndicatorVIXChange, Symbol: "STALE"}, want: "VIX"},
		{name: "vix level default", ind: types.IndicatorConfig{Type: types.IndicatorVIX}, want: "VIX"},
		{name: "vix level custom", ind: types.IndicatorConfig{Type: types.IndicatorVIX, Symbol: "UVXY"}, want: "UVXY"},
		{name: "gap default", ind: types.IndicatorConfig{Type: types.IndicatorGap}, want: "QQQ"},
		{name: "gap custom", ind: types.IndicatorConfig{Type: types.IndicatorGap, Symbol: "SPY"}, want: "SPY"},
		{name: "calendar", ind: types.IndicatorConfig{Type: types.IndicatorCalendar}, want: ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := indicatorSubscriptionSymbol(tc.ind); got != tc.want {
				t.Fatalf("indicatorSubscriptionSymbol(%+v) = %q, want %q", tc.ind, got, tc.want)
			}
		})
	}
}

func TestIndicatorSymbolCollectionSkipsDisabled(t *testing.T) {
	indicators := []types.IndicatorConfig{
		{Type: types.IndicatorVIXChange, Enabled: false},
		{Type: types.IndicatorGap, Enabled: true},
	}

	var symbols []string
	for _, ind := range indicators {
		if !ind.Enabled {
			continue
		}
		if symbol := indicatorSubscriptionSymbol(ind); symbol != "" {
			symbols = append(symbols, symbol)
		}
	}

	if len(symbols) != 1 || symbols[0] != "QQQ" {
		t.Fatalf("collected symbols = %v, want [QQQ]", symbols)
	}
}
