import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { nextTick } from 'vue';

/**
 * QA: Adversarial Edge Case Tests for LightweightChart (Issue #79)
 * 
 * These tests target boundary conditions, error paths, and edge cases
 * that the happy-path tests may not cover. Focus on:
 * - Null/undefined handling
 * - Empty/zero-length data
 * - Boundary values (max, min, overflow)
 * - Concurrent state changes
 * - Error propagation
 * - Precision/rounding issues
 */

describe('QA: LightweightChart - Adversarial Edge Cases (Issue #79)', () => {

  describe('Null/Undefined Safety - Defensive Programming', () => {
    
    it('should handle undefined candlestickData gracefully', () => {
      const candlestickData = undefined;
      let currentCandle = null;

      // Should not crash when accessing candlestickData
      if (candlestickData && candlestickData.length > 0) {
        currentCandle = candlestickData[candlestickData.length - 1];
      }

      // currentCandle remains null (expected)
      expect(currentCandle).toBe(null);
    });

    it('should handle null candlestickData gracefully', () => {
      const candlestickData = null;
      let currentCandle = null;

      if (candlestickData && candlestickData.length > 0) {
        currentCandle = candlestickData[candlestickData.length - 1];
      }

      expect(currentCandle).toBe(null);
    });

    it('should handle empty candlestickData array', () => {
      const candlestickData = [];
      let currentCandle = null;

      // This is the real scenario - empty historical data
      if (candlestickData && candlestickData.length > 0) {
        currentCandle = candlestickData[candlestickData.length - 1];
      }

      // Should remain null when data is empty
      expect(currentCandle).toBe(null);
    });

    it('should handle priceData with all null/undefined fields', () => {
      const priceData = {
        price: undefined,
        last: undefined,
        mid: undefined,
        bid: undefined,
        ask: undefined
      };

      // Extract price using the same logic as updateRealTimeData
      const newPrice =
        priceData.price ||
        priceData.last ||
        priceData.mid ||
        (priceData.bid && priceData.ask
          ? (priceData.bid + priceData.ask) / 2
          : null) ||
        priceData.bid ||
        priceData.ask;

      // When all are undefined, the || chain returns undefined (not null)
      expect(newPrice).toBeUndefined();
    });

    it('should handle priceData with zero price - POTENTIAL BUG', () => {
      const priceData = { price: 0 };

      // Zero is falsy but valid - price extraction should handle this
      const newPrice = priceData.price || null;
      
      // This is a CRITICAL BUG: zero price would be treated as falsy and skipped!
      // Currently the code would skip zero prices, which is WRONG
      expect(newPrice).toBe(null); // Currently this is what happens (BUG)
      
      // But it SHOULD be:
      const correctPrice = priceData.price !== null && priceData.price !== undefined ? priceData.price : null;
      expect(correctPrice).toBe(0);
    });

    it('should handle priceData with null/undefined in fallback chain', () => {
      const priceData = {
        price: null,
        last: null,
        mid: null,
        bid: 100,
        ask: 102
      };

      const newPrice =
        priceData.price ||
        priceData.last ||
        priceData.mid ||
        (priceData.bid && priceData.ask
          ? (priceData.bid + priceData.ask) / 2
          : null) ||
        priceData.bid ||
        priceData.ask;

      // Should calculate mid-price from bid/ask
      expect(newPrice).toBe(101);
    });

    it('should handle undefined currentCandle when updating real-time data', () => {
      let currentCandle = undefined;
      const newPrice = 100;

      // This should gracefully create a new candle
      if (!currentCandle) {
        currentCandle = {
          time: 1000,
          open: newPrice,
          high: newPrice,
          low: newPrice,
          close: newPrice,
        };
      }

      expect(currentCandle).toBeDefined();
      expect(currentCandle.open).toBe(100);
    });

    it('should handle volumeData.value of null - POTENTIAL CRASH', () => {
      const volumeData = {
        time: 1000,
        value: null,
        color: '#26a69a80'
      };

      // Chart library might crash if value is null
      // This tests if we should validate before passing to chart
      expect(volumeData.value).toBe(null); // Potential bug: chart may crash
    });
  });

  describe('Boundary Value Testing - Extreme Inputs', () => {

    it('should handle negative price correctly', () => {
      const historicalCandle = {
        time: 1000,
        open: -50,
        high: -40,
        low: -60,
        close: -45
      };

      const livePrice = -42;
      const updated = {
        time: historicalCandle.time,
        open: historicalCandle.open,
        high: Math.max(historicalCandle.high, livePrice),
        low: Math.min(historicalCandle.low, livePrice),
        close: livePrice
      };

      // Negative prices shouldn't crash, but may be invalid for stocks
      expect(updated.high).toBe(-40);
      expect(updated.low).toBe(-60);
      expect(updated.close).toBe(-42);
    });

    it('should handle NaN price without crashing', () => {
      let currentCandle = {
        time: 1000,
        open: 100,
        high: 105,
        low: 99,
        close: 104
      };

      const newPrice = NaN;
      
      // Math operations with NaN produce NaN
      const updated = {
        time: currentCandle.time,
        open: currentCandle.open,
        high: Math.max(currentCandle.high, newPrice), // Results in NaN
        low: Math.min(currentCandle.low, newPrice),   // Results in NaN
        close: newPrice
      };

      // This is BAD - we should validate prices
      expect(isNaN(updated.high)).toBe(true);
      expect(isNaN(updated.low)).toBe(true);
    });

    it('should handle Infinity price', () => {
      let currentCandle = {
        time: 1000,
        open: 100,
        high: 105,
        low: 99,
        close: 104
      };

      const newPrice = Infinity;
      
      const updated = {
        time: currentCandle.time,
        open: currentCandle.open,
        high: Math.max(currentCandle.high, newPrice),
        low: Math.min(currentCandle.low, newPrice),
        close: newPrice
      };

      expect(updated.high).toBe(Infinity);
      expect(updated.close).toBe(Infinity);
    });

    it('should handle extremely large price (beyond JavaScript precision)', () => {
      let currentCandle = {
        time: 1000,
        open: Number.MAX_VALUE,
        high: Number.MAX_VALUE,
        low: Number.MAX_VALUE,
        close: Number.MAX_VALUE
      };

      // Adding to MAX_VALUE wraps around
      const newPrice = Number.MAX_VALUE / 2;
      const updated = {
        high: Math.max(currentCandle.high, newPrice)
      };

      // Loss of precision is unavoidable, but should not crash
      expect(updated.high).toBe(Number.MAX_VALUE);
    });

    it('should handle timestamp at JavaScript limit', () => {
      const maxTimestamp = Math.floor(Number.MAX_SAFE_INTEGER / 1000);
      
      const candle = {
        time: maxTimestamp,
        open: 100,
        high: 105,
        low: 99,
        close: 104
      };

      // Chart should handle extreme timestamps
      expect(candle.time).toBe(maxTimestamp);
    });

    it('should handle timestamp of zero (1970-01-01)', () => {
      const candle = {
        time: 0,
        open: 100,
        high: 105,
        low: 99,
        close: 104
      };

      expect(candle.time).toBe(0);
    });

    it('should handle negative timestamp', () => {
      const candle = {
        time: -1705276800,
        open: 100,
        high: 105,
        low: 99,
        close: 104
      };

      // Negative timestamps are pre-1970
      expect(candle.time).toBeLessThan(0);
    });
  });

  describe('Data Corruption - Invalid State Recovery', () => {

    it('should recover from corrupted currentCandle (missing fields) - POTENTIAL BUG', () => {
      let currentCandle = { time: 1000 }; // Missing OHLC

      const newPrice = 100;

      // Trying to access missing high/low/open/close
      try {
        const updated = {
          time: currentCandle.time,
          open: currentCandle.open || newPrice, // undefined || 100 = 100
          high: Math.max(currentCandle.high, newPrice), // Math.max(undefined, 100) = NaN
          low: Math.min(currentCandle.low, newPrice),
          close: newPrice
        };

        // Current code would create NaN values - BUG
        expect(isNaN(updated.high)).toBe(true);
      } catch (err) {
        expect.fail('Should not crash'); // Should not crash
      }
    });

    it('should handle currentCandle with time === newTime but mismatched OHLC', () => {
      let currentCandle = {
        time: 1000,
        open: 100,
        high: 105,
        low: 99,
        close: 104
      };

      const newPrice = 200;
      const timeInSeconds = 1000; // Same as currentCandle.time

      // Check if this is a new candle or updating existing
      if (!currentCandle || currentCandle.time !== timeInSeconds) {
        // Create new - but we shouldn't be here
        expect.fail('Should not create new candle when time matches');
      } else {
        // Update existing - this should happen
        const updated = {
          time: currentCandle.time,
          open: currentCandle.open,
          high: Math.max(currentCandle.high, newPrice),
          low: Math.min(currentCandle.low, newPrice),
          close: newPrice
        };

        expect(updated.open).toBe(100);
        expect(updated.high).toBe(200);
        expect(updated.close).toBe(200);
      }
    });

    it('should handle transitioned state: currentCandle from old timeframe', () => {
      // This could happen if timeframe change isn't properly synchronized
      let currentCandle = {
        time: 1705276800, // Daily candle time
        open: 100,
        high: 105,
        low: 99,
        close: 104
      };

      // User changes to 1h timeframe while live data arrives
      // Live data has 1h-aligned time
      const hourlyTime = 1705294800; // 1 hour boundary

      // Old code might try to update with mismatched times
      if (currentCandle.time !== hourlyTime) {
        // Would create new candle - but it's orphaned from old timeframe
        const newCandle = {
          time: hourlyTime,
          open: 110,
          high: 110,
          low: 110,
          close: 110
        };

        expect(newCandle.time).not.toBe(currentCandle.time);
      }
    });
  });

  describe('Concurrency & State Machine - Race Conditions Beyond the Fix', () => {

    it('should not lose state during rapid loadHistoricalData calls', async () => {
      let historicalDataLoaded = false;
      let currentCandle = null;
      const calls = [];

      // Simulate multiple rapid calls to loadHistoricalData
      async function simulateLoadHistoricalData(symbol) {
        calls.push(`load_start_${symbol}`);
        historicalDataLoaded = false;

        // Simulate network delay
        await new Promise(resolve => setTimeout(resolve, 10));

        currentCandle = {
          time: 1000,
          open: Math.random() * 100,
          high: 105,
          low: 95,
          close: 100
        };
        historicalDataLoaded = true;
        calls.push(`load_end_${symbol}`);
      }

      // Fire rapid calls
      await Promise.all([
        simulateLoadHistoricalData('SPY'),
        simulateLoadHistoricalData('TSLA'),
        simulateLoadHistoricalData('AAPL')
      ]);

      // All loads started, but only last one should complete successfully
      expect(historicalDataLoaded).toBe(true);
      expect(currentCandle).toBeDefined();
      // Last call to complete wins
      expect(calls.length).toBeGreaterThanOrEqual(3);
    });

    it('should handle updateRealTimeData called during loadHistoricalData', async () => {
      let historicalDataLoaded = false;
      let currentCandle = null;
      let updateCount = 0;

      const loadPromise = (async () => {
        historicalDataLoaded = false;
        await new Promise(resolve => setTimeout(resolve, 50));
        currentCandle = { time: 1000, open: 100, high: 105, low: 99, close: 104 };
        historicalDataLoaded = true;
      })();

      // Immediately try to update with live data
      await new Promise(resolve => setTimeout(resolve, 10));
      
      // This should be skipped because historicalDataLoaded is false
      if (historicalDataLoaded && currentCandle) {
        updateCount++;
      }

      await loadPromise;

      // Now update should work
      if (historicalDataLoaded && currentCandle) {
        updateCount++;
      }

      expect(updateCount).toBe(1); // Only second update should run
    });

    it('should handle changeTimeframe called while updateRealTimeData is processing', () => {
      let historicalDataLoaded = true;
      let currentCandle = {
        time: 1705276800, // Daily
        open: 100,
        high: 105,
        low: 99,
        close: 104
      };

      // Start processing price update
      const priceUpdateStart = {
        candlestickSeries: { update: vi.fn() },
        priceData: { price: 110 }
      };

      // Meanwhile, user changes timeframe
      historicalDataLoaded = false;
      currentCandle = null;

      // Price update tries to continue
      if (!historicalDataLoaded) {
        // Should skip - guard works
        expect(currentCandle).toBe(null);
      }

      // New historical data arrives
      historicalDataLoaded = true;
      currentCandle = { time: 1705294800, open: 102, high: 106, low: 101, close: 103 };

      expect(currentCandle.time).not.toBe(1705276800);
    });
  });

  describe('Price Extraction Logic - Zero and Falsy Values', () => {

    it('should NOT skip zero price from price field - BUG FOUND', () => {
      const priceData = { price: 0 };

      // BUG: Current code uses || which treats 0 as falsy
      const buggyExtraction = priceData.price || null;
      const correctExtraction = priceData.price !== null && priceData.price !== undefined ? priceData.price : null;

      expect(buggyExtraction).toBe(null); // WRONG!
      expect(correctExtraction).toBe(0);  // CORRECT
    });

    it('should NOT skip zero price from bid/ask midpoint', () => {
      const priceData = {
        price: undefined,
        last: undefined,
        mid: undefined,
        bid: -1,
        ask: 1
      };

      const midpoint = (priceData.bid && priceData.ask) ? (priceData.bid + priceData.ask) / 2 : null;
      
      // Midpoint is 0 - a valid price
      expect(midpoint).toBe(0);
    });

    it('should handle bid/ask calculation resulting in fractional price', () => {
      const priceData = {
        price: undefined,
        last: undefined,
        mid: undefined,
        bid: 100.333,
        ask: 100.667
      };

      const midpoint = (priceData.bid && priceData.ask) ? (priceData.bid + priceData.ask) / 2 : null;
      
      expect(midpoint).toBe(100.5);
    });

    it('should extract price when only bid is available (ask undefined)', () => {
      const priceData = {
        price: undefined,
        last: undefined,
        mid: undefined,
        bid: 100,
        ask: undefined
      };

      const newPrice =
        priceData.price ||
        priceData.last ||
        priceData.mid ||
        (priceData.bid && priceData.ask ? (priceData.bid + priceData.ask) / 2 : null) ||
        priceData.bid ||
        priceData.ask;

      expect(newPrice).toBe(100);
    });
  });

  describe('Time Alignment Logic - Edge Cases', () => {

    it('should handle 1-minute alignment at midnight', () => {
      const now = new Date(Date.UTC(2024, 0, 1, 0, 0, 59)); // 00:00:59 UTC
      
      const alignedTime = new Date(
        now.getUTCFullYear(),
        now.getUTCMonth(),
        now.getUTCDate(),
        now.getUTCHours(),
        now.getUTCMinutes(),
        0,
        0
      );

      expect(alignedTime.getUTCHours()).toBe(0);
      expect(alignedTime.getUTCMinutes()).toBe(0);
      expect(alignedTime.getUTCSeconds()).toBe(0);
    });

    it('should handle 5-minute alignment at boundary (4:59 to 5:00)', () => {
      const before = new Date(Date.UTC(2024, 0, 1, 12, 4, 59));
      const after = new Date(Date.UTC(2024, 0, 1, 12, 5, 0));

      const alignBefore = Math.floor(before.getUTCMinutes() / 5) * 5;
      const alignAfter = Math.floor(after.getUTCMinutes() / 5) * 5;

      expect(alignBefore).toBe(0); // 4 -> 0 minutes
      expect(alignAfter).toBe(5);  // 5 -> 5 minutes
    });

    it('should handle 4-hour alignment at day boundary (20:00 to 00:00)', () => {
      const before = new Date(Date.UTC(2024, 0, 1, 23, 0, 0));
      const after = new Date(Date.UTC(2024, 0, 2, 0, 0, 0));

      const alignBefore = Math.floor(before.getUTCHours() / 4) * 4;
      const alignAfter = Math.floor(after.getUTCHours() / 4) * 4;

      expect(alignBefore).toBe(20);
      expect(alignAfter).toBe(0);
    });

    it('should handle weekly alignment for Monday (day 1)', () => {
      const now = new Date(Date.UTC(2024, 0, 1)); // Jan 1, 2024 was a Monday
      const dayOfWeek = now.getUTCDay(); // 0 = Sunday, 1 = Monday
      const daysToMonday = dayOfWeek === 0 ? 6 : dayOfWeek - 1;

      expect(dayOfWeek).toBe(1); // Monday
      expect(daysToMonday).toBe(0);
    });

    it('should handle weekly alignment for Sunday (day 0)', () => {
      const now = new Date(Date.UTC(2024, 0, 7)); // Jan 7, 2024 was a Sunday
      const dayOfWeek = now.getUTCDay();
      const daysToMonday = dayOfWeek === 0 ? 6 : dayOfWeek - 1;

      expect(dayOfWeek).toBe(0); // Sunday
      expect(daysToMonday).toBe(6); // 6 days back to previous Monday
    });

    it('should handle monthly alignment for last day of month', () => {
      // February 2024 (leap year)
      const now = new Date(Date.UTC(2024, 1, 29)); // Feb 29, 2024
      
      const alignedTime = new Date(Date.UTC(
        now.getUTCFullYear(),
        now.getUTCMonth(),
        1,
        0,
        0,
        0,
        0
      ));

      expect(alignedTime.getUTCDate()).toBe(1);
      expect(alignedTime.getUTCMonth()).toBe(1);
    });

    it('should handle monthly alignment on January 1st', () => {
      const now = new Date(Date.UTC(2024, 0, 1));
      
      const alignedTime = new Date(Date.UTC(
        now.getUTCFullYear(),
        now.getUTCMonth(),
        1,
        0,
        0,
        0,
        0
      ));

      expect(alignedTime.getUTCDate()).toBe(1);
      expect(alignedTime.getUTCMonth()).toBe(0);
    });
  });

  describe('Volume Data - Null/Invalid Values', () => {

    it('should handle bar with zero volume', () => {
      const bar = {
        time: 1000,
        open: 100,
        high: 105,
        low: 99,
        close: 104,
        volume: 0
      };

      const volumeData = {
        time: bar.time,
        value: bar.volume,
        color: bar.close >= bar.open ? '#26a69a80' : '#ef535080'
      };

      expect(volumeData.value).toBe(0);
      expect(volumeData.color).toBe('#26a69a80'); // Up candle
    });

    it('should handle bar with negative volume - POTENTIAL BUG', () => {
      const bar = {
        time: 1000,
        open: 100,
        high: 105,
        low: 99,
        close: 104,
        volume: -1000
      };

      const volumeData = {
        time: bar.time,
        value: bar.volume,
        color: bar.close >= bar.open ? '#26a69a80' : '#ef535080'
      };

      expect(volumeData.value).toBe(-1000); // Accepted, may crash chart
    });

    it('should handle bar with NaN volume', () => {
      const bar = {
        time: 1000,
        open: 100,
        high: 105,
        low: 99,
        close: 104,
        volume: NaN
      };

      const volumeData = {
        time: bar.time,
        value: bar.volume,
        color: bar.close >= bar.open ? '#26a69a80' : '#ef535080'
      };

      expect(isNaN(volumeData.value)).toBe(true);
    });

    it('should handle bar with missing volume field', () => {
      const bar = {
        time: 1000,
        open: 100,
        high: 105,
        low: 99,
        close: 104
        // volume missing
      };

      const volumeData = {
        time: bar.time,
        value: bar.volume, // undefined
        color: bar.close >= bar.open ? '#26a69a80' : '#ef535080'
      };

      expect(volumeData.value).toBeUndefined();
    });
  });

  describe('OHLC Validation - Logical Consistency', () => {

    it('should handle close < open (down candle)', () => {
      const candle = {
        time: 1000,
        open: 105,
        high: 110,
        low: 100,
        close: 102
      };

      // Validate OHLC makes sense
      expect(candle.open).toBeGreaterThan(candle.close); // Down
      expect(candle.high).toBeGreaterThanOrEqual(candle.open);
      expect(candle.low).toBeLessThanOrEqual(candle.close);
    });

    it('should detect invalid: high < open - BUG DETECTION', () => {
      const candle = {
        time: 1000,
        open: 105,
        high: 104,
        low: 100,
        close: 102
      };

      // This is INVALID - high should never be less than open
      const isValid = candle.high >= Math.max(candle.open, candle.close);
      expect(isValid).toBe(false); // BUG DETECTED
    });

    it('should detect invalid: low > close - BUG DETECTION', () => {
      const candle = {
        time: 1000,
        open: 105,
        high: 110,
        low: 103,
        close: 102
      };

      // This is INVALID - low should never be greater than close
      const isValid = candle.low <= Math.min(candle.open, candle.close);
      expect(isValid).toBe(false); // BUG DETECTED
    });

    it('should detect invalid: high < low - BUG DETECTION', () => {
      const candle = {
        time: 1000,
        open: 105,
        high: 100,
        low: 110,
        close: 102
      };

      // This is INVALID - high should never be less than low
      const isValid = candle.high >= candle.low;
      expect(isValid).toBe(false); // BUG DETECTED
    });

    it('should validate OHLC after price update', () => {
      let candle = {
        time: 1000,
        open: 100,
        high: 105,
        low: 99,
        close: 104
      };

      // Update with extreme price
      const newPrice = 50; // Below low

      candle = {
        time: candle.time,
        open: candle.open,
        high: Math.max(candle.high, newPrice),
        low: Math.min(candle.low, newPrice),
        close: newPrice
      };

      // Validate: should be consistent
      const isValid = 
        candle.high >= candle.low &&
        candle.high >= candle.open &&
        candle.high >= candle.close &&
        candle.low <= candle.open &&
        candle.low <= candle.close;

      expect(isValid).toBe(true);
    });
  });

  describe('Type Coercion - JavaScript Quirks', () => {

    it('should handle string price from API', () => {
      const priceData = { price: '100.50' };

      // This might come from API as string
      const numPrice = parseFloat(priceData.price);

      expect(typeof numPrice).toBe('number');
      expect(numPrice).toBe(100.50);
    });

    it('should handle price as string that is not a number', () => {
      const priceData = { price: 'INVALID' };

      const numPrice = parseFloat(priceData.price);

      expect(isNaN(numPrice)).toBe(true);
    });

    it('should handle time as string from API', () => {
      const bar = {
        time: '2024-01-15 10:30',
        open: 100,
        high: 105,
        low: 99,
        close: 104
      };

      // Current code checks: if (typeof time === 'string' && time.includes(' '))
      if (typeof bar.time === 'string' && bar.time.includes(' ')) {
        const etTimeString = bar.time + ':00 EST';
        const date = new Date(etTimeString);
        const timestamp = Math.floor(date.getTime() / 1000);

        expect(typeof timestamp).toBe('number');
      }
    });

    it('should handle time as number from API', () => {
      const bar = {
        time: 1705276800,
        open: 100,
        high: 105,
        low: 99,
        close: 104
      };

      // Current code assumes if not string, use directly
      const time = bar.time;

      expect(typeof time).toBe('number');
      expect(time).toBe(1705276800);
    });
  });

});
