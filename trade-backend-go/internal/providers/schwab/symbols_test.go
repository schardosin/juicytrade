package schwab

import (
	"testing"
)

// =============================================================================
// convertSchwabOptionToOCC tests
// =============================================================================

func TestConvertSchwabOptionToOCC(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		// Space-padded format (primary Schwab format)
		{"AAPL call", "AAPL  250117C00150000", "AAPL250117C00150000"},
		{"SPY put", "SPY   250321P00500000", "SPY250321P00500000"},
		{"F single-char underlying", "F     250620C00015000", "F250620C00015000"},
		{"GOOGL 5-char underlying", "GOOGL 250117C00200000", "GOOGL250117C00200000"},
		{"BRKB 4-char underlying", "BRKB  250117P00400000", "BRKB250117P00400000"},
		{"SPXW weekly", "SPXW  250117C00500000", "SPXW250117C00500000"},
		{"Low strike <$1", "SIRI  250117C00000500", "SIRI250117C00000500"},
		{"High strike >$1000", "AMZN  250117C01500000", "AMZN250117C01500000"},

		// Dot-prefix format
		{"dot prefix AAPL call", ".AAPL250117C150", "AAPL250117C00150000"},
		{"dot prefix SPY put", ".SPY250321P500", "SPY250321P00500000"},
		{"dot prefix low strike", ".F250620C15", "F250620C00015000"},
		{"dot prefix decimal strike", ".AAPL250117C2.5", "AAPL250117C00002500"},

		// Already OCC format (passthrough)
		{"OCC passthrough", "AAPL250117C00150000", "AAPL250117C00150000"},
		{"OCC passthrough short underlying", "F250620C00015000", "F250620C00015000"},

		// Empty
		{"empty string", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := convertSchwabOptionToOCC(tt.input)
			if got != tt.expected {
				t.Errorf("convertSchwabOptionToOCC(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

// =============================================================================
// convertOCCToSchwab tests
// =============================================================================

func TestConvertOCCToSchwab(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"AAPL 4-char", "AAPL250117C00150000", "AAPL  250117C00150000"},
		{"SPY 3-char", "SPY250321P00500000", "SPY   250321P00500000"},
		{"F 1-char", "F250620C00015000", "F     250620C00015000"},
		{"GOOGL 5-char", "GOOGL250117C00200000", "GOOGL 250117C00200000"},
		{"BRKB 4-char", "BRKB250117P00400000", "BRKB  250117P00400000"},
		{"SPXW 4-char weekly", "SPXW250117C00500000", "SPXW  250117C00500000"},
		{"6-char underlying no pad", "ABCDEF250117C00100000", "ABCDEF250117C00100000"},

		// Edge cases
		{"empty string", "", ""},
		{"too short", "SHORT", "SHORT"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := convertOCCToSchwab(tt.input)
			if got != tt.expected {
				t.Errorf("convertOCCToSchwab(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

// =============================================================================
// Round-trip tests
// =============================================================================

func TestConvertRoundTrip_OCCToSchwabToOCC(t *testing.T) {
	occSymbols := []string{
		"AAPL250117C00150000",
		"SPY250321P00500000",
		"F250620C00015000",
		"GOOGL250117C00200000",
		"BRKB250117P00400000",
		"SPXW250117C00500000",
	}

	for _, occ := range occSymbols {
		t.Run(occ, func(t *testing.T) {
			schwab := convertOCCToSchwab(occ)
			backToOCC := convertSchwabOptionToOCC(schwab)
			if backToOCC != occ {
				t.Errorf("round trip failed: %q → %q → %q (expected %q)", occ, schwab, backToOCC, occ)
			}
		})
	}
}

func TestConvertRoundTrip_SchwabToOCCToSchwab(t *testing.T) {
	schwabSymbols := []string{
		"AAPL  250117C00150000",
		"SPY   250321P00500000",
		"F     250620C00015000",
		"GOOGL 250117C00200000",
		"BRKB  250117P00400000",
	}

	for _, schwab := range schwabSymbols {
		t.Run(schwab, func(t *testing.T) {
			occ := convertSchwabOptionToOCC(schwab)
			backToSchwab := convertOCCToSchwab(occ)
			if backToSchwab != schwab {
				t.Errorf("round trip failed: %q → %q → %q (expected %q)", schwab, occ, backToSchwab, schwab)
			}
		})
	}
}

// =============================================================================
// isOptionSymbol tests
// =============================================================================

func TestIsOptionSymbol(t *testing.T) {
	tests := []struct {
		symbol   string
		expected bool
	}{
		// Equities — should be false
		{"AAPL", false},
		{"SPY", false},
		{"F", false},
		{"GOOGL", false},
		{"BRKB", false},
		{"SPXW", false},
		{"", false},
		{"A", false},
		{"VERY_LONG_TICKER", false},

		// OCC format options — should be true
		{"AAPL250117C00150000", true},
		{"SPY250321P00500000", true},
		{"F250620C00015000", true},
		{"GOOGL250117C00200000", true},

		// Schwab space-padded format — should be true
		{"AAPL  250117C00150000", true},
		{"SPY   250321P00500000", true},
		{"F     250620C00015000", true},

		// Schwab dot-prefix format — should be true
		{".AAPL250117C150", true},
		{".SPY250321P500", true},
	}

	for _, tt := range tests {
		t.Run(tt.symbol, func(t *testing.T) {
			got := isOptionSymbol(tt.symbol)
			if got != tt.expected {
				t.Errorf("isOptionSymbol(%q) = %v, want %v", tt.symbol, got, tt.expected)
			}
		})
	}
}

// =============================================================================
// classifySymbols tests
// =============================================================================

func TestClassifySymbols(t *testing.T) {
	symbols := []string{
		"AAPL",
		"AAPL250117C00150000",
		"SPY",
		"SPY250321P00500000",
		"F",
		"F250620C00015000",
		"GOOGL",
	}

	equities, options := classifySymbols(symbols)

	expectedEquities := []string{"AAPL", "SPY", "F", "GOOGL"}
	expectedOptions := []string{"AAPL250117C00150000", "SPY250321P00500000", "F250620C00015000"}

	if len(equities) != len(expectedEquities) {
		t.Fatalf("expected %d equities, got %d: %v", len(expectedEquities), len(equities), equities)
	}
	for i, eq := range equities {
		if eq != expectedEquities[i] {
			t.Errorf("equity[%d] = %q, want %q", i, eq, expectedEquities[i])
		}
	}

	if len(options) != len(expectedOptions) {
		t.Fatalf("expected %d options, got %d: %v", len(expectedOptions), len(options), options)
	}
	for i, opt := range options {
		if opt != expectedOptions[i] {
			t.Errorf("option[%d] = %q, want %q", i, opt, expectedOptions[i])
		}
	}
}

func TestClassifySymbols_Empty(t *testing.T) {
	equities, options := classifySymbols(nil)
	if len(equities) != 0 || len(options) != 0 {
		t.Errorf("expected empty slices for nil input, got equities=%v, options=%v", equities, options)
	}

	equities, options = classifySymbols([]string{})
	if len(equities) != 0 || len(options) != 0 {
		t.Errorf("expected empty slices for empty input, got equities=%v, options=%v", equities, options)
	}
}

func TestClassifySymbols_AllEquities(t *testing.T) {
	symbols := []string{"AAPL", "SPY", "GOOGL", "MSFT"}
	equities, options := classifySymbols(symbols)

	if len(equities) != 4 {
		t.Errorf("expected 4 equities, got %d", len(equities))
	}
	if len(options) != 0 {
		t.Errorf("expected 0 options, got %d", len(options))
	}
}

func TestClassifySymbols_AllOptions(t *testing.T) {
	symbols := []string{
		"AAPL250117C00150000",
		"SPY250321P00500000",
	}
	equities, options := classifySymbols(symbols)

	if len(equities) != 0 {
		t.Errorf("expected 0 equities, got %d", len(equities))
	}
	if len(options) != 2 {
		t.Errorf("expected 2 options, got %d", len(options))
	}
}

// =============================================================================
// convertDotPrefixToOCC edge case tests
// =============================================================================

func TestConvertDotPrefixToOCC_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		input    string // dot already stripped
		expected string
	}{
		{"normal call", "AAPL250117C150", "AAPL250117C00150000"},
		{"normal put", "SPY250321P500", "SPY250321P00500000"},
		{"single char underlying", "F250620C15", "F250620C00015000"},
		{"decimal strike", "AAPL250117P7.5", "AAPL250117P00007500"},
		{"high strike", "AMZN250117C2000", "AMZN250117C02000000"},
		{"sub-dollar strike", "SIRI250117C0.5", "SIRI250117C00000500"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := convertDotPrefixToOCC(tt.input)
			if got != tt.expected {
				t.Errorf("convertDotPrefixToOCC(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}
