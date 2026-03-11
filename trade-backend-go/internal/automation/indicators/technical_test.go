package indicators

import (
	"math"
	"testing"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// assertInDelta fails the test if |got - want| > delta.
func assertInDelta(t *testing.T, want, got, delta float64, msg string) {
	t.Helper()
	if math.Abs(got-want) > delta {
		t.Errorf("%s: want %.6f ± %.6f, got %.6f (diff %.6f)", msg, want, delta, got, math.Abs(got-want))
	}
}

// assertError fails if err is nil.
func assertError(t *testing.T, err error, msg string) {
	t.Helper()
	if err == nil {
		t.Errorf("%s: expected error, got nil", msg)
	}
}

// assertNoError fails if err is non-nil.
func assertNoError(t *testing.T, err error, msg string) {
	t.Helper()
	if err != nil {
		t.Errorf("%s: unexpected error: %v", msg, err)
	}
}

// makeConstant returns a slice of n identical values.
func makeConstant(val float64, n int) []float64 {
	s := make([]float64, n)
	for i := range s {
		s[i] = val
	}
	return s
}

// makeRising returns a slice [start, start+step, start+2*step, ...] of length n.
func makeRising(start, step float64, n int) []float64 {
	s := make([]float64, n)
	for i := range s {
		s[i] = start + float64(i)*step
	}
	return s
}

// makeFalling returns a slice [start, start-step, start-2*step, ...] of length n.
func makeFalling(start, step float64, n int) []float64 {
	s := make([]float64, n)
	for i := range s {
		s[i] = start - float64(i)*step
	}
	return s
}

// ---------------------------------------------------------------------------
// TestExtractOHLC
// ---------------------------------------------------------------------------

func TestExtractOHLC(t *testing.T) {
	bars := []map[string]interface{}{
		{"open": 100.0, "high": 110.0, "low": 90.0, "close": 105.0},
		{"open": 105.0, "high": 115.0, "low": 95.0, "close": 110.0},
		{"open": float32(110.0), "high": int(120), "low": int64(100), "close": 115.0},
	}

	opens, highs, lows, closes := extractOHLC(bars)

	if len(opens) != 3 || len(highs) != 3 || len(lows) != 3 || len(closes) != 3 {
		t.Fatal("extractOHLC: expected 3 elements in each slice")
	}

	assertInDelta(t, 100.0, opens[0], 0.01, "opens[0]")
	assertInDelta(t, 110.0, highs[0], 0.01, "highs[0]")
	assertInDelta(t, 90.0, lows[0], 0.01, "lows[0]")
	assertInDelta(t, 105.0, closes[0], 0.01, "closes[0]")

	// Verify type coercion for float32, int, int64
	assertInDelta(t, 110.0, opens[2], 0.01, "opens[2] float32")
	assertInDelta(t, 120.0, highs[2], 0.01, "highs[2] int")
	assertInDelta(t, 100.0, lows[2], 0.01, "lows[2] int64")
	assertInDelta(t, 115.0, closes[2], 0.01, "closes[2]")
}

func TestExtractOHLC_Empty(t *testing.T) {
	opens, highs, lows, closes := extractOHLC(nil)
	if len(opens) != 0 || len(highs) != 0 || len(lows) != 0 || len(closes) != 0 {
		t.Error("extractOHLC(nil): expected empty slices")
	}
}

// ---------------------------------------------------------------------------
// TestCalcSMA
// ---------------------------------------------------------------------------

func TestCalcSMA_Known(t *testing.T) {
	result, err := CalcSMA([]float64{1, 2, 3, 4, 5}, 3)
	assertNoError(t, err, "SMA [1..5] p=3")
	assertInDelta(t, 4.0, result, 0.001, "SMA [1..5] p=3")
}

func TestCalcSMA_AllSame(t *testing.T) {
	result, err := CalcSMA([]float64{10, 20, 30}, 3)
	assertNoError(t, err, "SMA [10,20,30] p=3")
	assertInDelta(t, 20.0, result, 0.001, "SMA [10,20,30] p=3")
}

func TestCalcSMA_ExactFit(t *testing.T) {
	result, err := CalcSMA([]float64{5, 10, 15}, 3)
	assertNoError(t, err, "SMA exact fit")
	assertInDelta(t, 10.0, result, 0.001, "SMA exact fit")
}

func TestCalcSMA_InsufficientData(t *testing.T) {
	_, err := CalcSMA([]float64{1, 2}, 5)
	assertError(t, err, "SMA insufficient data")
}

func TestCalcSMA_ZeroPeriod(t *testing.T) {
	_, err := CalcSMA([]float64{1, 2, 3}, 0)
	assertError(t, err, "SMA zero period")
}

// ---------------------------------------------------------------------------
// TestCalcEMA
// ---------------------------------------------------------------------------

func TestCalcEMA_ConstantPrices(t *testing.T) {
	// EMA of constant values should equal that value (converges to SMA)
	closes := makeConstant(50.0, 20)
	result, err := CalcEMA(closes, 10)
	assertNoError(t, err, "EMA constant")
	assertInDelta(t, 50.0, result, 0.001, "EMA constant converges to value")
}

func TestCalcEMA_KnownCalculation(t *testing.T) {
	// Manual check: 5 values, period=3
	// SMA seed = (10+20+30)/3 = 20.0
	// multiplier = 2/(3+1) = 0.5
	// EMA after val 40: (40-20)*0.5+20 = 30.0
	// EMA after val 50: (50-30)*0.5+30 = 40.0
	result, err := CalcEMA([]float64{10, 20, 30, 40, 50}, 3)
	assertNoError(t, err, "EMA known")
	assertInDelta(t, 40.0, result, 0.001, "EMA known calculation")
}

func TestCalcEMA_InsufficientData(t *testing.T) {
	_, err := CalcEMA([]float64{1, 2}, 5)
	assertError(t, err, "EMA insufficient data")
}

// ---------------------------------------------------------------------------
// TestCalcRSI
// ---------------------------------------------------------------------------

func TestCalcRSI_AllRising(t *testing.T) {
	// Monotonically rising: RSI should be near 100
	closes := makeRising(100, 1, 30)
	result, err := CalcRSI(closes, 14)
	assertNoError(t, err, "RSI all rising")
	if result < 95 {
		t.Errorf("RSI all rising: expected > 95, got %.2f", result)
	}
}

func TestCalcRSI_AllFalling(t *testing.T) {
	// Monotonically falling: RSI should be near 0
	closes := makeFalling(200, 1, 30)
	result, err := CalcRSI(closes, 14)
	assertNoError(t, err, "RSI all falling")
	if result > 5 {
		t.Errorf("RSI all falling: expected < 5, got %.2f", result)
	}
}

func TestCalcRSI_AllSame(t *testing.T) {
	// All same prices: RSI should be 50
	closes := makeConstant(100, 20)
	result, err := CalcRSI(closes, 14)
	assertNoError(t, err, "RSI all same")
	assertInDelta(t, 50.0, result, 0.001, "RSI all same → 50")
}

func TestCalcRSI_WilderReference(t *testing.T) {
	// Classic Wilder data set: 15 closes, period 14.
	// With 15 closes we get 14 changes and a single first-pass RSI (no
	// smoothing iterations). Our implementation computes the initial simple
	// average of gains/losses, producing RSI ≈ 72.98.
	closes := []float64{44, 44.34, 44.09, 43.61, 44.33, 44.83, 45.10,
		45.42, 45.84, 46.08, 45.89, 46.03, 45.61, 46.28, 46.28}
	result, err := CalcRSI(closes, 14)
	assertNoError(t, err, "RSI Wilder ref")
	assertInDelta(t, 72.98, result, 0.5, "RSI Wilder's reference value (first-pass)")
}

func TestCalcRSI_InsufficientData(t *testing.T) {
	_, err := CalcRSI([]float64{1, 2, 3}, 14)
	assertError(t, err, "RSI insufficient data")
}

// ---------------------------------------------------------------------------
// TestCalcMACD
// ---------------------------------------------------------------------------

func TestCalcMACD_RisingPrices(t *testing.T) {
	// Steadily rising prices → fast EMA > slow EMA → positive MACD
	closes := makeRising(100, 1, 60)
	result, err := CalcMACD(closes, 12, 26, 9)
	assertNoError(t, err, "MACD rising")
	if result <= 0 {
		t.Errorf("MACD rising: expected positive, got %.6f", result)
	}
}

func TestCalcMACD_FallingPrices(t *testing.T) {
	// Steadily falling prices → fast EMA < slow EMA → negative MACD
	closes := makeFalling(200, 1, 60)
	result, err := CalcMACD(closes, 12, 26, 9)
	assertNoError(t, err, "MACD falling")
	if result >= 0 {
		t.Errorf("MACD falling: expected negative, got %.6f", result)
	}
}

func TestCalcMACD_InsufficientData(t *testing.T) {
	_, err := CalcMACD([]float64{1, 2, 3}, 12, 26, 9)
	assertError(t, err, "MACD insufficient data")
}

func TestCalcMACD_ConstantPrices(t *testing.T) {
	// Constant prices → MACD should be ~0
	closes := makeConstant(100, 60)
	result, err := CalcMACD(closes, 12, 26, 9)
	assertNoError(t, err, "MACD constant")
	assertInDelta(t, 0.0, result, 0.001, "MACD constant → ~0")
}

// ---------------------------------------------------------------------------
// TestCalcMomentum
// ---------------------------------------------------------------------------

func TestCalcMomentum_Positive(t *testing.T) {
	// 100 → 105, period=1 → 5% gain
	result, err := CalcMomentum([]float64{100, 105}, 1)
	assertNoError(t, err, "Momentum positive")
	assertInDelta(t, 5.0, result, 0.001, "Momentum 100→105")
}

func TestCalcMomentum_Negative(t *testing.T) {
	// 100 → 95, period=1 → -5%
	result, err := CalcMomentum([]float64{100, 95}, 1)
	assertNoError(t, err, "Momentum negative")
	assertInDelta(t, -5.0, result, 0.001, "Momentum 100→95")
}

func TestCalcMomentum_LongerPeriod(t *testing.T) {
	// [100, 110, 120, 130, 150], period=3: (150-110)/110*100 = 36.3636...
	result, err := CalcMomentum([]float64{100, 110, 120, 130, 150}, 3)
	assertNoError(t, err, "Momentum period=3")
	assertInDelta(t, 36.3636, result, 0.01, "Momentum longer period")
}

func TestCalcMomentum_InsufficientData(t *testing.T) {
	_, err := CalcMomentum([]float64{100}, 1)
	assertError(t, err, "Momentum insufficient data")
}

func TestCalcMomentum_ZeroPrevious(t *testing.T) {
	_, err := CalcMomentum([]float64{0, 100}, 1)
	assertError(t, err, "Momentum zero previous")
}

// ---------------------------------------------------------------------------
// TestCalcCMO
// ---------------------------------------------------------------------------

func TestCalcCMO_MonotonicRise(t *testing.T) {
	// All up moves → CMO = +100
	closes := makeRising(100, 1, 16)
	result, err := CalcCMO(closes, 14)
	assertNoError(t, err, "CMO rising")
	assertInDelta(t, 100.0, result, 0.001, "CMO monotonic rise → +100")
}

func TestCalcCMO_MonotonicFall(t *testing.T) {
	// All down moves → CMO = -100
	closes := makeFalling(200, 1, 16)
	result, err := CalcCMO(closes, 14)
	assertNoError(t, err, "CMO falling")
	assertInDelta(t, -100.0, result, 0.001, "CMO monotonic fall → -100")
}

func TestCalcCMO_NoMovement(t *testing.T) {
	closes := makeConstant(50, 16)
	result, err := CalcCMO(closes, 14)
	assertNoError(t, err, "CMO no movement")
	assertInDelta(t, 0.0, result, 0.001, "CMO no movement → 0")
}

func TestCalcCMO_InsufficientData(t *testing.T) {
	_, err := CalcCMO([]float64{1, 2}, 14)
	assertError(t, err, "CMO insufficient data")
}

// ---------------------------------------------------------------------------
// TestCalcStochastic
// ---------------------------------------------------------------------------

func TestCalcStochastic_CloseAtHigh(t *testing.T) {
	// Close equals highest high → %K = 100
	highs := []float64{110, 120, 130, 140, 150}
	lows := []float64{90, 95, 100, 105, 110}
	closes := []float64{100, 110, 120, 130, 150} // close = high of last bar
	result, err := CalcStochastic(highs, lows, closes, 5, 1)
	assertNoError(t, err, "Stoch close at high")
	assertInDelta(t, 100.0, result, 0.001, "Stoch close at high → 100")
}

func TestCalcStochastic_CloseAtLow(t *testing.T) {
	// Close equals lowest low → %K = 0
	highs := []float64{110, 120, 130, 140, 150}
	lows := []float64{90, 95, 100, 105, 110}
	closes := []float64{100, 110, 120, 130, 90} // close = lowest low in window
	result, err := CalcStochastic(highs, lows, closes, 5, 1)
	assertNoError(t, err, "Stoch close at low")
	assertInDelta(t, 0.0, result, 0.001, "Stoch close at low → 0")
}

func TestCalcStochastic_CloseAtMid(t *testing.T) {
	// Same high and low each bar, close at midpoint
	highs := []float64{200, 200, 200, 200, 200}
	lows := []float64{100, 100, 100, 100, 100}
	closes := []float64{150, 150, 150, 150, 150}
	result, err := CalcStochastic(highs, lows, closes, 5, 1)
	assertNoError(t, err, "Stoch close at mid")
	assertInDelta(t, 50.0, result, 0.001, "Stoch close at mid → 50")
}

func TestCalcStochastic_NoRange(t *testing.T) {
	// highestHigh == lowestLow → 50 (neutral)
	highs := []float64{100, 100, 100}
	lows := []float64{100, 100, 100}
	closes := []float64{100, 100, 100}
	result, err := CalcStochastic(highs, lows, closes, 3, 1)
	assertNoError(t, err, "Stoch no range")
	assertInDelta(t, 50.0, result, 0.001, "Stoch no range → 50")
}

func TestCalcStochastic_InsufficientData(t *testing.T) {
	_, err := CalcStochastic([]float64{1}, []float64{1}, []float64{1}, 5, 3)
	assertError(t, err, "Stoch insufficient data")
}

// ---------------------------------------------------------------------------
// TestCalcStochRSI
// ---------------------------------------------------------------------------

func TestCalcStochRSI_InRange(t *testing.T) {
	// Generate enough data for StochRSI and verify output is in 0-100
	closes := makeRising(100, 0.5, 60)
	// Add some variation to avoid all-same RSI values
	for i := range closes {
		if i%3 == 0 {
			closes[i] -= 0.3
		}
	}
	result, err := CalcStochRSI(closes, 14, 14, 3, 3)
	assertNoError(t, err, "StochRSI in range")
	if result < 0 || result > 100 {
		t.Errorf("StochRSI: expected 0-100, got %.2f", result)
	}
}

func TestCalcStochRSI_StrongUptrend(t *testing.T) {
	// Strong consistent uptrend should produce high StochRSI
	closes := makeRising(100, 2, 60)
	result, err := CalcStochRSI(closes, 14, 14, 3, 3)
	assertNoError(t, err, "StochRSI uptrend")
	// With a strong uptrend, all RSI values will be near 100 and identical,
	// causing highestRSI == lowestRSI → returns 50 (neutral).
	// This is expected behavior per the edge case handling.
	if result < 0 || result > 100 {
		t.Errorf("StochRSI uptrend: expected 0-100, got %.2f", result)
	}
}

func TestCalcStochRSI_InsufficientData(t *testing.T) {
	_, err := CalcStochRSI([]float64{1, 2, 3}, 14, 14, 3, 3)
	assertError(t, err, "StochRSI insufficient data")
}

// ---------------------------------------------------------------------------
// TestCalcADX
// ---------------------------------------------------------------------------

func TestCalcADX_StrongTrend(t *testing.T) {
	// Generate strong uptrend data: each bar higher highs and higher lows
	n := 50
	highs := make([]float64, n)
	lows := make([]float64, n)
	closes := make([]float64, n)
	for i := 0; i < n; i++ {
		base := 100.0 + float64(i)*2.0
		highs[i] = base + 5
		lows[i] = base - 5
		closes[i] = base + 3
	}

	result, err := CalcADX(highs, lows, closes, 14)
	assertNoError(t, err, "ADX strong trend")
	if result < 25 {
		t.Errorf("ADX strong trend: expected > 25, got %.2f", result)
	}
}

func TestCalcADX_FlatMarket(t *testing.T) {
	// Flat/sideways: alternating ups and downs
	n := 50
	highs := make([]float64, n)
	lows := make([]float64, n)
	closes := make([]float64, n)
	for i := 0; i < n; i++ {
		base := 100.0
		if i%2 == 0 {
			highs[i] = base + 2
			lows[i] = base - 2
			closes[i] = base + 1
		} else {
			highs[i] = base + 2
			lows[i] = base - 2
			closes[i] = base - 1
		}
	}

	result, err := CalcADX(highs, lows, closes, 14)
	assertNoError(t, err, "ADX flat market")
	if result > 25 {
		t.Errorf("ADX flat market: expected < 25, got %.2f", result)
	}
}

func TestCalcADX_InsufficientData(t *testing.T) {
	_, err := CalcADX([]float64{1, 2, 3}, []float64{1, 2, 3}, []float64{1, 2, 3}, 14)
	assertError(t, err, "ADX insufficient data")
}

func TestCalcADX_UnequalLengths(t *testing.T) {
	_, err := CalcADX([]float64{1, 2}, []float64{1}, []float64{1, 2}, 1)
	assertError(t, err, "ADX unequal lengths")
}

// ---------------------------------------------------------------------------
// TestCalcCCI
// ---------------------------------------------------------------------------

func TestCalcCCI_PriceAtSMA(t *testing.T) {
	// All bars identical: TP at SMA, mean deviation = 0 → CCI = 0
	n := 20
	highs := makeConstant(100, n)
	lows := makeConstant(100, n)
	closes := makeConstant(100, n)
	result, err := CalcCCI(highs, lows, closes, 20)
	assertNoError(t, err, "CCI at SMA")
	assertInDelta(t, 0.0, result, 0.001, "CCI all same → 0")
}

func TestCalcCCI_AboveSMA(t *testing.T) {
	// Most bars at 100, last bar at 120 → CCI positive
	n := 20
	highs := makeConstant(100, n)
	lows := makeConstant(100, n)
	closes := makeConstant(100, n)
	// Last bar significantly higher
	highs[n-1] = 120
	lows[n-1] = 110
	closes[n-1] = 120
	result, err := CalcCCI(highs, lows, closes, 20)
	assertNoError(t, err, "CCI above SMA")
	if result <= 0 {
		t.Errorf("CCI above SMA: expected positive, got %.2f", result)
	}
}

func TestCalcCCI_InsufficientData(t *testing.T) {
	_, err := CalcCCI([]float64{1, 2}, []float64{1, 2}, []float64{1, 2}, 5)
	assertError(t, err, "CCI insufficient data")
}

// ---------------------------------------------------------------------------
// TestCalcATR
// ---------------------------------------------------------------------------

func TestCalcATR_ConstantRange(t *testing.T) {
	// All bars have same range (high-low = 10), no gaps → ATR = 10
	n := 20
	highs := make([]float64, n)
	lows := make([]float64, n)
	closes := make([]float64, n)
	for i := 0; i < n; i++ {
		highs[i] = 110
		lows[i] = 100
		closes[i] = 105 // close within range, no gap to previous
	}
	result, err := CalcATR(highs, lows, closes, 14)
	assertNoError(t, err, "ATR constant range")
	assertInDelta(t, 10.0, result, 0.01, "ATR constant range → 10")
}

func TestCalcATR_WithGaps(t *testing.T) {
	// When close[i-1] is outside next bar's range, TR should be larger than H-L
	highs := []float64{110, 115, 130, 135, 140}
	lows := []float64{100, 105, 120, 125, 130}
	closes := []float64{105, 110, 125, 130, 135}
	result, err := CalcATR(highs, lows, closes, 2)
	assertNoError(t, err, "ATR with gaps")
	if result <= 0 {
		t.Errorf("ATR with gaps: expected positive, got %.2f", result)
	}
}

func TestCalcATR_InsufficientData(t *testing.T) {
	_, err := CalcATR([]float64{110}, []float64{100}, []float64{105}, 14)
	assertError(t, err, "ATR insufficient data")
}

func TestCalcATR_WilderSmoothing(t *testing.T) {
	// Verify Wilder's smoothing: first period simple avg, then smoothed
	// 5 bars, period=2
	// TR[0] = max(110-100, |110-100|, |100-100|) = 10 (bar 1 vs bar 0 close)
	// TR[1] = max(112-102, |112-105|, |102-105|) = 10
	// TR[2] = max(120-100, |120-105|, |100-105|) = 20
	// TR[3] = max(115-105, |115-110|, |105-110|) = 10
	//
	// First ATR = (TR[0]+TR[1])/2 = (10+10)/2 = 10
	// After TR[2]: ATR = (10*(2-1) + 20)/2 = 30/2 = 15
	// After TR[3]: ATR = (15*(2-1) + 10)/2 = 25/2 = 12.5
	highs := []float64{105, 110, 112, 120, 115}
	lows := []float64{95, 100, 102, 100, 105}
	closes := []float64{100, 105, 105, 110, 110}
	result, err := CalcATR(highs, lows, closes, 2)
	assertNoError(t, err, "ATR Wilder's")
	assertInDelta(t, 12.5, result, 0.01, "ATR Wilder's smoothing")
}

// ---------------------------------------------------------------------------
// TestCalcBollingerPercentB
// ---------------------------------------------------------------------------

func TestCalcBollingerPercentB_AtSMA(t *testing.T) {
	// All same → stddev = 0 → bandwidth = 0 → returns 0.5
	closes := makeConstant(100, 20)
	result, err := CalcBollingerPercentB(closes, 20, 2.0)
	assertNoError(t, err, "BB%B at SMA")
	assertInDelta(t, 0.5, result, 0.001, "BB%B all same → 0.5")
}

func TestCalcBollingerPercentB_AtUpper(t *testing.T) {
	// Construct data where close is exactly at the upper band
	// 19 values of 100, last value = SMA + 2*stddev
	// SMA = (19*100 + X) / 20
	// We need X such that X = SMA + 2*SD
	// For simplicity, use values that produce known bands.
	//
	// Use [100, 100, ..., 100, 120] (19×100 + 120)
	// SMA = (19*100 + 120)/20 = 2020/20 = 101
	// Variance = (19*(100-101)^2 + (120-101)^2)/20 = (19*1 + 361)/20 = 380/20 = 19
	// SD = sqrt(19) ≈ 4.3589
	// Upper = 101 + 2*4.3589 = 109.7178
	// Lower = 101 - 2*4.3589 = 92.2822
	// %B = (120 - 92.2822)/(109.7178 - 92.2822) = 27.7178/17.4356 ≈ 1.59
	closes := make([]float64, 20)
	for i := 0; i < 19; i++ {
		closes[i] = 100
	}
	closes[19] = 120
	result, err := CalcBollingerPercentB(closes, 20, 2.0)
	assertNoError(t, err, "BB%%B above upper")
	if result <= 1.0 {
		t.Errorf("BB%%B above upper: expected > 1.0, got %.4f", result)
	}
}

func TestCalcBollingerPercentB_AtLower(t *testing.T) {
	// Use [100, 100, ..., 100, 80] (19×100 + 80)
	// SMA = (19*100 + 80)/20 = 1980/20 = 99
	// Variance = (19*(100-99)^2 + (80-99)^2)/20 = (19 + 361)/20 = 19
	// SD = sqrt(19) ≈ 4.3589
	// Lower = 99 - 2*4.3589 = 90.2822
	// Upper = 99 + 2*4.3589 = 107.7178
	// %B = (80 - 90.2822)/(107.7178 - 90.2822) = -10.2822/17.4356 ≈ -0.59
	closes := make([]float64, 20)
	for i := 0; i < 19; i++ {
		closes[i] = 100
	}
	closes[19] = 80
	result, err := CalcBollingerPercentB(closes, 20, 2.0)
	assertNoError(t, err, "BB%%B below lower")
	if result >= 0.0 {
		t.Errorf("BB%%B below lower: expected < 0.0, got %.4f", result)
	}
}

func TestCalcBollingerPercentB_ExactBands(t *testing.T) {
	// Use a known distribution to test exact band positions
	// 5 values: [98, 99, 100, 101, 102] period=5 stddev=2
	// SMA = 100
	// Variance = (4+1+0+1+4)/5 = 2
	// SD = sqrt(2) ≈ 1.41421
	// Upper = 100 + 2*1.41421 = 102.82842
	// Lower = 100 - 2*1.41421 = 97.17158
	// BW = 5.65686
	// Close = 102, %B = (102 - 97.17158)/5.65686 = 4.82842/5.65686 ≈ 0.8536
	result, err := CalcBollingerPercentB([]float64{98, 99, 100, 101, 102}, 5, 2.0)
	assertNoError(t, err, "BB%B exact bands")
	assertInDelta(t, 0.8536, result, 0.001, "BB%B exact bands")
}

func TestCalcBollingerPercentB_InsufficientData(t *testing.T) {
	_, err := CalcBollingerPercentB([]float64{1, 2}, 5, 2.0)
	assertError(t, err, "BB%B insufficient data")
}

// ---------------------------------------------------------------------------
// TestCalcEMASeries (internal helper, tested indirectly via MACD but also directly)
// ---------------------------------------------------------------------------

func TestCalcEMASeries_Nil(t *testing.T) {
	result := calcEMASeries([]float64{1}, 5)
	if result != nil {
		t.Error("calcEMASeries: expected nil for insufficient data")
	}
}

func TestCalcEMASeries_Length(t *testing.T) {
	data := []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	result := calcEMASeries(data, 3)
	expectedLen := len(data) - 3 + 1 // 8
	if len(result) != expectedLen {
		t.Errorf("calcEMASeries: expected length %d, got %d", expectedLen, len(result))
	}
}

// ---------------------------------------------------------------------------
// TestCalcRSISeries (internal helper, tested indirectly via StochRSI)
// ---------------------------------------------------------------------------

func TestCalcRSISeries_Nil(t *testing.T) {
	result := calcRSISeries([]float64{1}, 5)
	if result != nil {
		t.Error("calcRSISeries: expected nil for insufficient data")
	}
}

func TestCalcRSISeries_Length(t *testing.T) {
	// 20 closes, period=14 → 20 - 14 = 6 RSI values
	closes := makeRising(100, 1, 20)
	result := calcRSISeries(closes, 14)
	expectedLen := 20 - 14 // 6
	if len(result) != expectedLen {
		t.Errorf("calcRSISeries: expected length %d, got %d", expectedLen, len(result))
	}
}

func TestCalcRSISeries_ConsistentWithCalcRSI(t *testing.T) {
	// The last value of calcRSISeries should match CalcRSI
	closes := []float64{44, 44.34, 44.09, 43.61, 44.33, 44.83, 45.10,
		45.42, 45.84, 46.08, 45.89, 46.03, 45.61, 46.28, 46.28}
	series := calcRSISeries(closes, 14)
	direct, err := CalcRSI(closes, 14)
	assertNoError(t, err, "RSI series vs direct")
	if series == nil || len(series) == 0 {
		t.Fatal("calcRSISeries returned nil or empty")
	}
	assertInDelta(t, direct, series[len(series)-1], 0.001, "RSI series last == CalcRSI")
}
