package indicators

import (
	"errors"
	"fmt"
	"math"
)

// ---------------------------------------------------------------------------
// Helper: Extract OHLC arrays from bar data
// ---------------------------------------------------------------------------

// extractOHLC converts raw bar data into typed slices.
// Bars must be in ascending chronological order (oldest first).
// Returns opens, highs, lows, closes slices.
func extractOHLC(bars []map[string]interface{}) (opens, highs, lows, closes []float64) {
	n := len(bars)
	opens = make([]float64, n)
	highs = make([]float64, n)
	lows = make([]float64, n)
	closes = make([]float64, n)
	for i, bar := range bars {
		opens[i] = getFloatFromBar(bar, "open")
		highs[i] = getFloatFromBar(bar, "high")
		lows[i] = getFloatFromBar(bar, "low")
		closes[i] = getFloatFromBar(bar, "close")
	}
	return
}

// ---------------------------------------------------------------------------
// Internal helpers (unexported)
// ---------------------------------------------------------------------------

// calcEMASeries returns a full EMA series over the input data.
// The first `period` values are used to seed the EMA with their SMA.
// Returns a slice starting from index (period-1) of the input,
// so the returned slice has length len(data) - period + 1.
// If len(data) < period, returns nil.
func calcEMASeries(data []float64, period int) []float64 {
	if len(data) < period {
		return nil
	}

	multiplier := 2.0 / float64(period+1)

	// Seed with SMA of first `period` values
	sum := 0.0
	for i := 0; i < period; i++ {
		sum += data[i]
	}
	ema := sum / float64(period)

	result := make([]float64, 0, len(data)-period+1)
	result = append(result, ema)

	for i := period; i < len(data); i++ {
		ema = (data[i]-ema)*multiplier + ema
		result = append(result, ema)
	}
	return result
}

// calcRSISeries returns a full series of RSI values using Wilder's smoothing.
// Requires at least period+1 closes. Returns a slice of RSI values, one for
// each close starting from index `period` (i.e., length = len(closes) - period).
// If insufficient data, returns nil.
func calcRSISeries(closes []float64, period int) []float64 {
	n := len(closes)
	if n < period+1 {
		return nil
	}

	// Calculate price changes
	changes := make([]float64, n-1)
	for i := 1; i < n; i++ {
		changes[i-1] = closes[i] - closes[i-1]
	}

	// First average gain/loss from first `period` changes
	var sumGain, sumLoss float64
	for i := 0; i < period; i++ {
		if changes[i] > 0 {
			sumGain += changes[i]
		} else {
			sumLoss += -changes[i]
		}
	}
	avgGain := sumGain / float64(period)
	avgLoss := sumLoss / float64(period)

	// Compute first RSI
	rsiValues := make([]float64, 0, len(changes)-period+1)
	rsiValues = append(rsiValues, computeRSIFromAvg(avgGain, avgLoss))

	// Wilder's smoothing for remaining changes
	for i := period; i < len(changes); i++ {
		gain := math.Max(changes[i], 0)
		loss := math.Abs(math.Min(changes[i], 0))
		avgGain = (avgGain*float64(period-1) + gain) / float64(period)
		avgLoss = (avgLoss*float64(period-1) + loss) / float64(period)
		rsiValues = append(rsiValues, computeRSIFromAvg(avgGain, avgLoss))
	}

	return rsiValues
}

// computeRSIFromAvg converts average gain/loss to RSI value.
func computeRSIFromAvg(avgGain, avgLoss float64) float64 {
	if avgLoss == 0 {
		if avgGain > 0 {
			return 100
		}
		return 50 // all-same prices
	}
	rs := avgGain / avgLoss
	return 100 - (100 / (1 + rs))
}

// ---------------------------------------------------------------------------
// Exported calculation functions
// ---------------------------------------------------------------------------

// CalcSMA calculates the Simple Moving Average over the last `period` closes.
func CalcSMA(closes []float64, period int) (float64, error) {
	if period <= 0 {
		return 0, errors.New("CalcSMA: period must be positive")
	}
	n := len(closes)
	if n < period {
		return 0, fmt.Errorf("CalcSMA: insufficient data, need %d closes, got %d", period, n)
	}
	sum := 0.0
	for i := n - period; i < n; i++ {
		sum += closes[i]
	}
	return sum / float64(period), nil
}

// CalcEMA calculates the Exponential Moving Average.
func CalcEMA(closes []float64, period int) (float64, error) {
	if period <= 0 {
		return 0, errors.New("CalcEMA: period must be positive")
	}
	n := len(closes)
	if n < period {
		return 0, fmt.Errorf("CalcEMA: insufficient data, need %d closes, got %d", period, n)
	}

	multiplier := 2.0 / float64(period+1)

	// Seed with SMA of first `period` values
	sum := 0.0
	for i := 0; i < period; i++ {
		sum += closes[i]
	}
	ema := sum / float64(period)

	// Apply EMA formula from period onward
	for i := period; i < n; i++ {
		ema = (closes[i]-ema)*multiplier + ema
	}
	return ema, nil
}

// CalcRSI calculates the Relative Strength Index using Wilder's smoothing.
func CalcRSI(closes []float64, period int) (float64, error) {
	if period <= 0 {
		return 0, errors.New("CalcRSI: period must be positive")
	}
	n := len(closes)
	if n < period+1 {
		return 0, fmt.Errorf("CalcRSI: insufficient data, need %d closes, got %d", period+1, n)
	}

	// Calculate price changes
	changes := make([]float64, n-1)
	for i := 1; i < n; i++ {
		changes[i-1] = closes[i] - closes[i-1]
	}

	// First average gain/loss (simple average of first `period` changes)
	var sumGain, sumLoss float64
	for i := 0; i < period; i++ {
		if changes[i] > 0 {
			sumGain += changes[i]
		} else {
			sumLoss += -changes[i]
		}
	}
	avgGain := sumGain / float64(period)
	avgLoss := sumLoss / float64(period)

	// Wilder's smoothing for remaining changes
	for i := period; i < len(changes); i++ {
		gain := math.Max(changes[i], 0)
		loss := math.Abs(math.Min(changes[i], 0))
		avgGain = (avgGain*float64(period-1) + gain) / float64(period)
		avgLoss = (avgLoss*float64(period-1) + loss) / float64(period)
	}

	return computeRSIFromAvg(avgGain, avgLoss), nil
}

// CalcMACD calculates the MACD line value (fast EMA - slow EMA).
// Returns the most recent MACD line value (not histogram or signal).
func CalcMACD(closes []float64, fastPeriod, slowPeriod, signalPeriod int) (float64, error) {
	if fastPeriod <= 0 || slowPeriod <= 0 || signalPeriod <= 0 {
		return 0, errors.New("CalcMACD: all periods must be positive")
	}
	needed := slowPeriod + signalPeriod
	if len(closes) < needed {
		return 0, fmt.Errorf("CalcMACD: insufficient data, need %d closes, got %d", needed, len(closes))
	}

	// Calculate fast and slow EMA series
	fastEMA := calcEMASeries(closes, fastPeriod)
	slowEMA := calcEMASeries(closes, slowPeriod)

	if fastEMA == nil || slowEMA == nil {
		return 0, errors.New("CalcMACD: failed to compute EMA series")
	}

	// Align: slowEMA starts at index (slowPeriod-1) of closes,
	// fastEMA starts at index (fastPeriod-1) of closes.
	// MACD line = fastEMA - slowEMA, aligned from slowPeriod-1 onward.
	//
	// fastEMA[i] corresponds to closes[fastPeriod-1+i]
	// slowEMA[j] corresponds to closes[slowPeriod-1+j]
	// We need fastEMA at the same close index as slowEMA.
	// offset = slowPeriod - fastPeriod (number of fastEMA values to skip)
	offset := slowPeriod - fastPeriod
	if offset < 0 {
		offset = 0
	}

	macdLen := len(slowEMA)
	if offset+macdLen > len(fastEMA) {
		macdLen = len(fastEMA) - offset
	}

	macdLine := make([]float64, macdLen)
	for i := 0; i < macdLen; i++ {
		macdLine[i] = fastEMA[offset+i] - slowEMA[i]
	}

	if len(macdLine) == 0 {
		return 0, errors.New("CalcMACD: no MACD line values computed")
	}

	// Return the most recent MACD line value
	return macdLine[len(macdLine)-1], nil
}

// CalcMomentum calculates price momentum as a percentage rate of change.
// momentum = ((current - previous) / previous) * 100
func CalcMomentum(closes []float64, period int) (float64, error) {
	if period <= 0 {
		return 0, errors.New("CalcMomentum: period must be positive")
	}
	n := len(closes)
	if n < period+1 {
		return 0, fmt.Errorf("CalcMomentum: insufficient data, need %d closes, got %d", period+1, n)
	}

	current := closes[n-1]
	previous := closes[n-1-period]

	if previous == 0 {
		return 0, errors.New("CalcMomentum: zero division — previous close is 0")
	}

	return ((current - previous) / previous) * 100, nil
}

// CalcCMO calculates the Chande Momentum Oscillator.
// Returns a value from -100 to +100.
func CalcCMO(closes []float64, period int) (float64, error) {
	if period <= 0 {
		return 0, errors.New("CalcCMO: period must be positive")
	}
	n := len(closes)
	if n < period+1 {
		return 0, fmt.Errorf("CalcCMO: insufficient data, need %d closes, got %d", period+1, n)
	}

	// Use the last (period+1) closes to compute `period` changes
	start := n - period - 1
	var sumUp, sumDown float64
	for i := start + 1; i < n; i++ {
		change := closes[i] - closes[i-1]
		if change > 0 {
			sumUp += change
		} else {
			sumDown += -change
		}
	}

	total := sumUp + sumDown
	if total == 0 {
		return 0, nil // No movement
	}

	return ((sumUp - sumDown) / total) * 100, nil
}

// CalcStochastic calculates the Stochastic Oscillator (fast %K value).
func CalcStochastic(highs, lows, closes []float64, kPeriod, dPeriod int) (float64, error) {
	if kPeriod <= 0 || dPeriod <= 0 {
		return 0, errors.New("CalcStochastic: periods must be positive")
	}
	n := len(closes)
	if n != len(highs) || n != len(lows) {
		return 0, errors.New("CalcStochastic: highs, lows, closes must have equal length")
	}
	needed := kPeriod + dPeriod - 1
	if n < needed {
		return 0, fmt.Errorf("CalcStochastic: insufficient data, need %d bars, got %d", needed, n)
	}

	// Calculate raw %K for the most recent bar
	lastIdx := n - 1
	startIdx := lastIdx - kPeriod + 1

	highestHigh := highs[startIdx]
	lowestLow := lows[startIdx]
	for i := startIdx + 1; i <= lastIdx; i++ {
		if highs[i] > highestHigh {
			highestHigh = highs[i]
		}
		if lows[i] < lowestLow {
			lowestLow = lows[i]
		}
	}

	if highestHigh == lowestLow {
		return 50, nil // No range, neutral
	}

	rawK := ((closes[lastIdx] - lowestLow) / (highestHigh - lowestLow)) * 100
	return rawK, nil
}

// CalcStochRSI calculates the Stochastic RSI (%K value).
func CalcStochRSI(closes []float64, rsiPeriod, stochPeriod, kPeriod, dPeriod int) (float64, error) {
	if rsiPeriod <= 0 || stochPeriod <= 0 || kPeriod <= 0 || dPeriod <= 0 {
		return 0, errors.New("CalcStochRSI: all periods must be positive")
	}
	minBars := rsiPeriod + stochPeriod + kPeriod + dPeriod
	n := len(closes)
	if n < minBars {
		return 0, fmt.Errorf("CalcStochRSI: insufficient data, need %d closes, got %d", minBars, n)
	}

	// Step 1: Calculate full RSI series
	rsiSeries := calcRSISeries(closes, rsiPeriod)
	if rsiSeries == nil || len(rsiSeries) < stochPeriod {
		return 0, errors.New("CalcStochRSI: insufficient RSI data for stochastic calculation")
	}

	// Step 2: Apply Stochastic formula to the last stochPeriod RSI values
	rsiLen := len(rsiSeries)
	window := rsiSeries[rsiLen-stochPeriod:]

	highestRSI := window[0]
	lowestRSI := window[0]
	for _, v := range window[1:] {
		if v > highestRSI {
			highestRSI = v
		}
		if v < lowestRSI {
			lowestRSI = v
		}
	}

	if highestRSI == lowestRSI {
		return 50, nil // Neutral
	}

	stochRSI := ((rsiSeries[rsiLen-1] - lowestRSI) / (highestRSI - lowestRSI)) * 100
	return stochRSI, nil
}

// CalcADX calculates the Average Directional Index.
func CalcADX(highs, lows, closes []float64, period int) (float64, error) {
	if period <= 0 {
		return 0, errors.New("CalcADX: period must be positive")
	}
	n := len(closes)
	if n != len(highs) || n != len(lows) {
		return 0, errors.New("CalcADX: highs, lows, closes must have equal length")
	}
	needed := period*2 + 1
	if n < needed {
		return 0, fmt.Errorf("CalcADX: insufficient data, need %d bars, got %d", needed, n)
	}

	// Step 1: Calculate True Range, +DM, -DM for each bar (starting from index 1)
	trValues := make([]float64, n-1)
	plusDM := make([]float64, n-1)
	minusDM := make([]float64, n-1)

	for i := 1; i < n; i++ {
		// True Range
		hl := highs[i] - lows[i]
		hpc := math.Abs(highs[i] - closes[i-1])
		lpc := math.Abs(lows[i] - closes[i-1])
		trValues[i-1] = math.Max(hl, math.Max(hpc, lpc))

		// Directional Movement
		upMove := highs[i] - highs[i-1]
		downMove := lows[i-1] - lows[i]

		if upMove > downMove && upMove > 0 {
			plusDM[i-1] = upMove
		}
		if downMove > upMove && downMove > 0 {
			minusDM[i-1] = downMove
		}
	}

	// Step 2: Wilder's smoothed averages of TR, +DM, -DM
	// First values: simple average of first `period` items
	var sumTR, sumPlusDM, sumMinusDM float64
	for i := 0; i < period; i++ {
		sumTR += trValues[i]
		sumPlusDM += plusDM[i]
		sumMinusDM += minusDM[i]
	}
	smoothedTR := sumTR / float64(period)
	smoothedPlusDM := sumPlusDM / float64(period)
	smoothedMinusDM := sumMinusDM / float64(period)

	// Calculate DX values for ADX smoothing
	dxValues := make([]float64, 0, len(trValues)-period+1)

	// First DX from initial smoothed values
	if smoothedTR > 0 {
		plusDI := (smoothedPlusDM / smoothedTR) * 100
		minusDI := (smoothedMinusDM / smoothedTR) * 100
		diSum := plusDI + minusDI
		if diSum > 0 {
			dxValues = append(dxValues, math.Abs(plusDI-minusDI)/diSum*100)
		} else {
			dxValues = append(dxValues, 0)
		}
	} else {
		dxValues = append(dxValues, 0)
	}

	// Subsequent smoothed values using Wilder's method
	for i := period; i < len(trValues); i++ {
		smoothedTR = (smoothedTR*float64(period-1) + trValues[i]) / float64(period)
		smoothedPlusDM = (smoothedPlusDM*float64(period-1) + plusDM[i]) / float64(period)
		smoothedMinusDM = (smoothedMinusDM*float64(period-1) + minusDM[i]) / float64(period)

		if smoothedTR > 0 {
			plusDI := (smoothedPlusDM / smoothedTR) * 100
			minusDI := (smoothedMinusDM / smoothedTR) * 100
			diSum := plusDI + minusDI
			if diSum > 0 {
				dxValues = append(dxValues, math.Abs(plusDI-minusDI)/diSum*100)
			} else {
				dxValues = append(dxValues, 0)
			}
		} else {
			dxValues = append(dxValues, 0)
		}
	}

	// Step 5: ADX = Wilder's smoothing of DX over `period`
	if len(dxValues) < period {
		return 0, fmt.Errorf("CalcADX: insufficient DX values for ADX smoothing, need %d, got %d", period, len(dxValues))
	}

	// First ADX = simple average of first `period` DX values
	var sumDX float64
	for i := 0; i < period; i++ {
		sumDX += dxValues[i]
	}
	adx := sumDX / float64(period)

	// Wilder's smoothing for remaining DX values
	for i := period; i < len(dxValues); i++ {
		adx = (adx*float64(period-1) + dxValues[i]) / float64(period)
	}

	return adx, nil
}

// CalcCCI calculates the Commodity Channel Index.
func CalcCCI(highs, lows, closes []float64, period int) (float64, error) {
	if period <= 0 {
		return 0, errors.New("CalcCCI: period must be positive")
	}
	n := len(closes)
	if n != len(highs) || n != len(lows) {
		return 0, errors.New("CalcCCI: highs, lows, closes must have equal length")
	}
	if n < period {
		return 0, fmt.Errorf("CalcCCI: insufficient data, need %d bars, got %d", period, n)
	}

	// Typical price for last `period` bars
	start := n - period
	tp := make([]float64, period)
	var tpSum float64
	for i := 0; i < period; i++ {
		idx := start + i
		tp[i] = (highs[idx] + lows[idx] + closes[idx]) / 3.0
		tpSum += tp[i]
	}
	tpSMA := tpSum / float64(period)

	// Mean deviation
	var meanDevSum float64
	for i := 0; i < period; i++ {
		meanDevSum += math.Abs(tp[i] - tpSMA)
	}
	meanDev := meanDevSum / float64(period)

	if meanDev == 0 {
		return 0, nil
	}

	// Lambert's constant 0.015
	cci := (tp[period-1] - tpSMA) / (0.015 * meanDev)
	return cci, nil
}

// CalcATR calculates the Average True Range using Wilder's smoothing.
func CalcATR(highs, lows, closes []float64, period int) (float64, error) {
	if period <= 0 {
		return 0, errors.New("CalcATR: period must be positive")
	}
	n := len(closes)
	if n != len(highs) || n != len(lows) {
		return 0, errors.New("CalcATR: highs, lows, closes must have equal length")
	}
	if n < period+1 {
		return 0, fmt.Errorf("CalcATR: insufficient data, need %d bars, got %d", period+1, n)
	}

	// True Range for each bar starting from index 1
	trValues := make([]float64, n-1)
	for i := 1; i < n; i++ {
		hl := highs[i] - lows[i]
		hpc := math.Abs(highs[i] - closes[i-1])
		lpc := math.Abs(lows[i] - closes[i-1])
		trValues[i-1] = math.Max(hl, math.Max(hpc, lpc))
	}

	// First ATR = simple average of first `period` TRs
	var sum float64
	for i := 0; i < period; i++ {
		sum += trValues[i]
	}
	atr := sum / float64(period)

	// Wilder's smoothing for remaining
	for i := period; i < len(trValues); i++ {
		atr = (atr*float64(period-1) + trValues[i]) / float64(period)
	}

	return atr, nil
}

// CalcBollingerPercentB calculates the Bollinger Band %B indicator.
// %B = (Close - Lower Band) / (Upper Band - Lower Band)
// Returns 0.5 when bandwidth is zero (flat bands).
func CalcBollingerPercentB(closes []float64, period int, stdDev float64) (float64, error) {
	if period <= 0 {
		return 0, errors.New("CalcBollingerPercentB: period must be positive")
	}
	n := len(closes)
	if n < period {
		return 0, fmt.Errorf("CalcBollingerPercentB: insufficient data, need %d closes, got %d", period, n)
	}

	// SMA of last `period` closes
	window := closes[n-period:]
	var sum float64
	for _, v := range window {
		sum += v
	}
	sma := sum / float64(period)

	// Standard deviation (population)
	var variance float64
	for _, v := range window {
		diff := v - sma
		variance += diff * diff
	}
	variance /= float64(period)
	sd := math.Sqrt(variance)

	// Bollinger Bands
	upperBand := sma + (stdDev * sd)
	lowerBand := sma - (stdDev * sd)

	bandWidth := upperBand - lowerBand
	if bandWidth == 0 {
		return 0.5, nil // Flat bands, price at middle
	}

	percentB := (closes[n-1] - lowerBand) / bandWidth
	return percentB, nil
}
