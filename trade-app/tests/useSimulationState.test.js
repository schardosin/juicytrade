import { describe, it, expect, vi, beforeEach } from 'vitest';
import { ref, computed, nextTick } from 'vue';
import { useSimulationState } from '../src/composables/useSimulationState.js';

// Mock the theoreticalPayoff module so the composable doesn't need real Black-Scholes
vi.mock('../src/utils/theoreticalPayoff.js', () => ({
  calculateWeightedAverageIV: vi.fn(() => 0.30),
}));

/**
 * Helper: build a positions computed ref with a single option leg
 * whose expiry_date is the given YYYY-MM-DD string.
 */
function makePositions(expiryDate) {
  return computed(() => [
    {
      symbol: 'SPY_250815C00550000',
      asset_class: 'us_option',
      qty: 1,
      strike_price: 550,
      option_type: 'call',
      expiry_date: expiryDate,
      avg_entry_price: 5.0,
    },
  ]);
}

/**
 * Minimal marketData stub — getOptionGreeks returns a ref-like object
 * with implied_volatility for any symbol.
 */
function makeMarketData() {
  return {
    getOptionGreeks: () => ({
      value: { implied_volatility: '0.30' },
    }),
  };
}

/**
 * Return a YYYY-MM-DD string for a date that is `daysFromNow` calendar
 * days in the future (or past if negative).
 */
function futureDateStr(daysFromNow) {
  const d = new Date();
  d.setHours(0, 0, 0, 0);
  d.setDate(d.getDate() + daysFromNow);
  const yyyy = d.getFullYear();
  const mm = String(d.getMonth() + 1).padStart(2, '0');
  const dd = String(d.getDate()).padStart(2, '0');
  return `${yyyy}-${mm}-${dd}`;
}

/** Midnight-normalized "today" for comparison. */
function todayMidnight() {
  const d = new Date();
  d.setHours(0, 0, 0, 0);
  return d;
}

describe('useSimulationState — date navigation boundary fixes', () => {
  let positions;
  let marketData;

  beforeEach(() => {
    // Expiry is 5 days from now — gives room to increment/decrement.
    positions = makePositions(futureDateStr(5));
    marketData = makeMarketData();
  });

  // ─── Initial state ───────────────────────────────────────────────

  it('initial selectedDate is midnight-normalized', () => {
    const { selectedDate } = useSimulationState(positions, marketData);
    const d = selectedDate.value;

    expect(d.getHours()).toBe(0);
    expect(d.getMinutes()).toBe(0);
    expect(d.getSeconds()).toBe(0);
    expect(d.getMilliseconds()).toBe(0);
  });

  // ─── incrementDate ───────────────────────────────────────────────

  it('incrementDate from day-before-expiry reaches expiry day', () => {
    // Expiry is 5 days out.  Advance 4 times → day before expiry.
    const { selectedDate, incrementDate } = useSimulationState(positions, marketData);

    for (let i = 0; i < 4; i++) {
      incrementDate();
    }

    const dayBeforeExpiry = new Date(selectedDate.value);
    // One more increment should land exactly on expiry day.
    incrementDate();

    const expected = todayMidnight();
    expected.setDate(expected.getDate() + 5);

    expect(selectedDate.value.getFullYear()).toBe(expected.getFullYear());
    expect(selectedDate.value.getMonth()).toBe(expected.getMonth());
    expect(selectedDate.value.getDate()).toBe(expected.getDate());
    expect(selectedDate.value.getHours()).toBe(0);
  });

  it('incrementDate from expiry day does NOT advance past expiry', () => {
    const { selectedDate, incrementDate } = useSimulationState(positions, marketData);

    // Advance to expiry day (5 increments).
    for (let i = 0; i < 5; i++) {
      incrementDate();
    }
    const atExpiry = new Date(selectedDate.value);

    // One more should be a no-op.
    incrementDate();
    expect(selectedDate.value.getTime()).toBe(atExpiry.getTime());
  });

  // ─── decrementDate ──────────────────────────────────────────────

  it('decrementDate from today does NOT go before today', () => {
    const { selectedDate, decrementDate } = useSimulationState(positions, marketData);

    const today = todayMidnight();
    // selectedDate starts at today; decrement should be a no-op.
    decrementDate();
    expect(selectedDate.value.getTime()).toBe(today.getTime());
  });

  it('decrementDate from day-after-today reaches today', () => {
    const { selectedDate, incrementDate, decrementDate } = useSimulationState(positions, marketData);

    // Go forward one day, then back one day.
    incrementDate();
    const tomorrow = new Date(selectedDate.value);
    expect(tomorrow.getDate()).toBe(todayMidnight().getDate() + 1);

    decrementDate();
    const today = todayMidnight();
    expect(selectedDate.value.getFullYear()).toBe(today.getFullYear());
    expect(selectedDate.value.getMonth()).toBe(today.getMonth());
    expect(selectedDate.value.getDate()).toBe(today.getDate());
  });

  // ─── reset ──────────────────────────────────────────────────────

  it('reset sets date to today (midnight-normalized)', () => {
    const { selectedDate, incrementDate, reset } = useSimulationState(positions, marketData);

    // Move forward a few days, then reset.
    incrementDate();
    incrementDate();
    incrementDate();
    expect(selectedDate.value.getTime()).not.toBe(todayMidnight().getTime());

    reset();

    const d = selectedDate.value;
    const today = todayMidnight();
    expect(d.getFullYear()).toBe(today.getFullYear());
    expect(d.getMonth()).toBe(today.getMonth());
    expect(d.getDate()).toBe(today.getDate());
    expect(d.getHours()).toBe(0);
    expect(d.getMinutes()).toBe(0);
    expect(d.getSeconds()).toBe(0);
    expect(d.getMilliseconds()).toBe(0);
  });
});
