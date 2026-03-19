package schwab

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

// =============================================================================
// Option Symbol Conversion
//
// OCC Format:    AAPL250117C00150000
//   - Underlying (variable length, no padding)
//   - YYMMDD (6 digits)
//   - C or P
//   - 8-digit strike × 1000 (zero-padded)
//
// Schwab Format (space-padded): AAPL  250117C00150000
//   - Underlying padded to 6 chars with trailing spaces
//   - YYMMDD (6 digits)
//   - C or P
//   - 8-digit strike × 1000 (zero-padded)
//
// Schwab Format (dot-prefix): .AAPL250117C150
//   - Leading dot
//   - Underlying (no padding)
//   - YYMMDD (6 digits)
//   - C or P
//   - Strike as plain number (not ×1000, not zero-padded)
// =============================================================================

// convertSchwabOptionToOCC converts a Schwab option symbol to standard OCC format.
//
// Handles two Schwab formats:
//  1. Space-padded: "AAPL  250117C00150000" → strip spaces → "AAPL250117C00150000"
//  2. Dot-prefix:   ".AAPL250117C150"       → remove dot, parse strike, reformat
//
// If the symbol is already in OCC format (no spaces, no dot), it is returned unchanged.
func convertSchwabOptionToOCC(schwabSymbol string) string {
	if schwabSymbol == "" {
		return schwabSymbol
	}

	// Handle dot-prefix format: .AAPL250117C150
	if strings.HasPrefix(schwabSymbol, ".") {
		return convertDotPrefixToOCC(schwabSymbol[1:]) // strip the dot
	}

	// Handle space-padded format: strip all spaces
	if strings.Contains(schwabSymbol, " ") {
		return strings.ReplaceAll(schwabSymbol, " ", "")
	}

	// Already in OCC format or unrecognized — return as-is
	return schwabSymbol
}

// convertDotPrefixToOCC converts a dot-prefix symbol (without the dot) to OCC format.
// Input example: "AAPL250117C150" → "AAPL250117C00150000"
func convertDotPrefixToOCC(symbol string) string {
	// Find where the underlying ends and the date begins.
	// The underlying is all leading alpha characters.
	dateStart := 0
	for i, ch := range symbol {
		if unicode.IsDigit(ch) {
			dateStart = i
			break
		}
	}
	if dateStart == 0 {
		return symbol // no digits found — not parseable
	}

	underlying := symbol[:dateStart]

	// After underlying: YYMMDD (6 digits) + C/P (1 char) + strike
	rest := symbol[dateStart:]
	if len(rest) < 8 { // minimum: 6-digit date + C/P + at least 1 digit strike
		return symbol // too short, return as-is
	}

	datePart := rest[:6]          // YYMMDD
	optionType := string(rest[6]) // C or P
	strikeStr := rest[7:]         // strike as plain number (e.g., "150", "2.5")

	// Parse strike and convert to OCC format (×1000, zero-padded to 8 digits)
	strikeFloat, err := strconv.ParseFloat(strikeStr, 64)
	if err != nil {
		return symbol // can't parse strike, return as-is
	}

	strikeOCC := int64(strikeFloat * 1000)
	return fmt.Sprintf("%s%s%s%08d", underlying, datePart, optionType, strikeOCC)
}

// convertOCCToSchwab converts a standard OCC option symbol to Schwab's space-padded format.
//
// OCC format: underlying + YYMMDD + C/P + 8-digit strike (total suffix = 15 chars)
// Schwab format: underlying padded to 6 chars with spaces + same 15-char suffix
//
// Example: "AAPL250117C00150000" → "AAPL  250117C00150000"
//
//	"F250620C00015000"    → "F     250620C00015000"
func convertOCCToSchwab(occSymbol string) string {
	if occSymbol == "" {
		return occSymbol
	}

	// The last 15 characters are: YYMMDD (6) + C/P (1) + strike (8)
	const suffixLen = 15
	if len(occSymbol) <= suffixLen {
		return occSymbol // too short to be a valid OCC symbol
	}

	underlying := occSymbol[:len(occSymbol)-suffixLen]
	suffix := occSymbol[len(occSymbol)-suffixLen:]

	// Pad underlying to 6 characters with trailing spaces
	const padWidth = 6
	padded := underlying
	if len(padded) < padWidth {
		padded = padded + strings.Repeat(" ", padWidth-len(padded))
	}

	return padded + suffix
}

// isOptionSymbol returns true if the symbol appears to be an option symbol
// (as opposed to a plain equity ticker).
//
// Heuristic: option symbols are ≥15 chars in OCC format, or contain spaces
// (Schwab format), or start with a dot (Schwab dot-prefix format).
// We also check for the C/P + 8-digit strike pattern at the end.
func isOptionSymbol(symbol string) bool {
	if symbol == "" {
		return false
	}

	// Schwab dot-prefix format
	if strings.HasPrefix(symbol, ".") {
		return true
	}

	// Schwab space-padded format
	if strings.Contains(symbol, " ") {
		return true
	}

	// OCC format: minimum length is 1-char underlying + 6-digit date + C/P + 8-digit strike = 16
	if len(symbol) < 16 {
		return false
	}

	// Check that the 15-char suffix has the right structure:
	// positions [-15:-9] should be digits (YYMMDD)
	// position [-9] should be C or P
	// positions [-8:] should be digits (strike)
	suffix := symbol[len(symbol)-15:]

	// Date: 6 digits
	for _, ch := range suffix[:6] {
		if !unicode.IsDigit(ch) {
			return false
		}
	}

	// Option type: C or P
	optType := suffix[6]
	if optType != 'C' && optType != 'P' {
		return false
	}

	// Strike: 8 digits
	for _, ch := range suffix[7:] {
		if !unicode.IsDigit(ch) {
			return false
		}
	}

	return true
}

// classifySymbols splits a list of symbols into equities and options.
func classifySymbols(symbols []string) (equities []string, options []string) {
	for _, sym := range symbols {
		if isOptionSymbol(sym) {
			options = append(options, sym)
		} else {
			equities = append(equities, sym)
		}
	}
	return equities, options
}
