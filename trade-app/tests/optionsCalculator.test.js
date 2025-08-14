import { describe, it, expect } from 'vitest';
import { calculateMultiLegProfitLoss } from '../src/services/optionsCalculator';

describe('calculateMultiLegProfitLoss', () => {
  it('should calculate the profit and loss for a single long call option', () => {
    const selectedOptions = [
      {
        symbol: 'SPY251219C00500000',
        quantity: 1,
        side: 'buy',
        strike_price: 500,
        type: 'call',
        expiry: '2025-12-19',
      },
    ];

    const optionsData = [
      {
        symbol: 'SPY251219C00500000',
        bid: 10.0,
        ask: 10.5,
      },
    ];

    const underlyingPrice = 505;
    const limitPrice = 10.5;

    const result = calculateMultiLegProfitLoss(
      selectedOptions,
      optionsData,
      underlyingPrice,
      limitPrice
    );

    expect(result.maxProfit).toBe(Infinity);
    expect(result.maxLoss).toBe(-1050); // -10.5 * 1 * 100
    expect(result.breakEvenPoints).toEqual([510.5]);
  });
});
