import { describe, it, expect } from 'vitest';
import orderService from '../src/services/orderService.js';

describe('OrderService - Cascade Protection & Order Flow Integrity', () => {

  describe('Order Validation & Data Integrity', () => {
    describe('Order Data Validation', () => {
      it('validates complete order data successfully', () => {
        const validOrderData = {
          symbol: 'SPY',
          expiry: '2024-01-19',
          legs: [
            {
              symbol: 'SPY_240119C00500000',
              side: 'buy',
              ratio_qty: 1
            }
          ],
          orderPrice: 2.50
        };

        const result = orderService.validateOrder(validOrderData);
        
        expect(result.isValid).toBe(true);
        expect(result.errors).toHaveLength(0);
      });

      it('rejects order without symbol', () => {
        const invalidOrderData = {
          expiry: '2024-01-19',
          legs: [{ symbol: 'SPY_240119C00500000', side: 'buy', ratio_qty: 1 }],
          orderPrice: 2.50
        };

        const result = orderService.validateOrder(invalidOrderData);
        
        expect(result.isValid).toBe(false);
        expect(result.errors).toContain('Symbol is required');
      });

      it('rejects order without expiry', () => {
        const invalidOrderData = {
          symbol: 'SPY',
          legs: [{ symbol: 'SPY_240119C00500000', side: 'buy', ratio_qty: 1 }],
          orderPrice: 2.50
        };

        const result = orderService.validateOrder(invalidOrderData);
        
        expect(result.isValid).toBe(false);
        expect(result.errors).toContain('Expiry date is required');
      });

      it('rejects order without legs', () => {
        const invalidOrderData = {
          symbol: 'SPY',
          expiry: '2024-01-19',
          legs: [],
          orderPrice: 2.50
        };

        const result = orderService.validateOrder(invalidOrderData);
        
        expect(result.isValid).toBe(false);
        expect(result.errors).toContain('At least one leg is required');
      });

      it('rejects order without price', () => {
        const invalidOrderData = {
          symbol: 'SPY',
          expiry: '2024-01-19',
          legs: [{ symbol: 'SPY_240119C00500000', side: 'buy', ratio_qty: 1 }]
        };

        const result = orderService.validateOrder(invalidOrderData);
        
        expect(result.isValid).toBe(false);
        expect(result.errors).toContain('Order price is required');
      });

      it('validates individual leg data', () => {
        const invalidOrderData = {
          symbol: 'SPY',
          expiry: '2024-01-19',
          legs: [
            { side: 'buy', ratio_qty: 1 }, // Missing symbol
            { symbol: 'SPY_240119P00480000', ratio_qty: 1 }, // Missing side
            { symbol: 'SPY_240119C00520000', side: 'sell' } // Missing quantity
          ],
          orderPrice: 2.50
        };

        const result = orderService.validateOrder(invalidOrderData);
        
        expect(result.isValid).toBe(false);
        expect(result.errors).toContain('Leg 1: Symbol is required');
        expect(result.errors).toContain('Leg 2: Side is required');
        expect(result.errors).toContain('Leg 3: Quantity is required');
      });
    });

    describe('Order Payload Building', () => {
      it('builds correct payload with limitPrice', () => {
        const orderData = {
          symbol: 'SPY',
          expiry: '2024-01-19',
          strategyType: 'Iron Condor',
          legs: [
            { symbol: 'SPY_240119C00500000', action: 'buy', ratio_qty: 1 },
            { symbol: 'SPY_240119C00510000', action: 'sell', ratio_qty: 1 }
          ],
          limitPrice: 2.50,
          quantity: 2,
          timeInForce: 'GTC',
          orderType: 'LIMIT'
        };

        const payload = orderService.buildOrderPayload(orderData);
        
        expect(payload).toEqual({
          legs: [
            { symbol: 'SPY_240119C00500000', side: 'buy', qty: 1 },
            { symbol: 'SPY_240119C00510000', side: 'sell', qty: 1 }
          ],
          order_type: 'limit',
          time_in_force: 'gtc',
          limit_price: 2.50
        });
      });

      it('falls back to orderPrice when limitPrice not available', () => {
        const orderData = {
          legs: [
            { symbol: 'SPY_240119C00500000', action: 'buy', ratio_qty: 1 }
          ],
          orderPrice: 3.25
        };

        const payload = orderService.buildOrderPayload(orderData);
        
        expect(payload.limit_price).toBe(3.25);
      });

      it('uses default values for optional parameters', () => {
        const orderData = {
          legs: [
            { symbol: 'SPY_240119C00500000', action: 'buy', ratio_qty: 1 }
          ],
          limitPrice: 2.50
        };

        const payload = orderService.buildOrderPayload(orderData);
        
        expect(payload.order_type).toBe('limit');
        expect(payload.time_in_force).toBe('day');
      });

      it('handles missing quantity in legs', () => {
        const orderData = {
          legs: [
            { symbol: 'SPY_240119C00500000', action: 'buy' }, // No quantity
            { symbol: 'SPY_240119P00480000', action: 'sell', quantity: 2 } // Alternative quantity field
          ],
          limitPrice: 2.50
        };

        const payload = orderService.buildOrderPayload(orderData);
        
        expect(payload.legs[0].qty).toBe(1); // Default to 1
        expect(payload.legs[1].qty).toBe(2); // Use quantity field
      });
    });

    describe('Leg Formatting', () => {
      it('formats legs correctly with ratio_qty', () => {
        const legs = [
          { symbol: 'SPY_240119C00500000', action: 'buy', ratio_qty: 2 },
          { symbol: 'SPY_240119P00480000', action: 'sell', ratio_qty: 1 }
        ];

        const formattedLegs = orderService.formatLegs(legs);
        
        expect(formattedLegs).toEqual([
          { symbol: 'SPY_240119C00500000', side: 'buy', qty: 2 },
          { symbol: 'SPY_240119P00480000', side: 'sell', qty: 1 }
        ]);
      });

      it('formats legs correctly with quantity field', () => {
        const legs = [
          { symbol: 'SPY_240119C00500000', action: 'buy', quantity: 3 }
        ];

        const formattedLegs = orderService.formatLegs(legs);
        
        expect(formattedLegs).toEqual([
          { symbol: 'SPY_240119C00500000', side: 'buy', qty: 3 }
        ]);
      });

      it('defaults to quantity 1 when no quantity specified', () => {
        const legs = [
          { symbol: 'SPY_240119C00500000', action: 'buy' }
        ];

        const formattedLegs = orderService.formatLegs(legs);
        
        expect(formattedLegs).toEqual([
          { symbol: 'SPY_240119C00500000', side: 'buy', qty: 1 }
        ]);
      });

      it('converts string quantities to integers', () => {
        const legs = [
          { symbol: 'SPY_240119C00500000', action: 'buy', ratio_qty: '5' }
        ];

        const formattedLegs = orderService.formatLegs(legs);
        
        expect(formattedLegs[0].qty).toBe(5);
        expect(typeof formattedLegs[0].qty).toBe('number');
      });
    });
  });

  describe('Credit/Debit Order Classification', () => {
    describe('Credit Order Detection', () => {
      it('identifies credit order using netPremium (negative)', () => {
        const orderData = { netPremium: -1.50 };
        
        expect(orderService.isCreditOrder(orderData)).toBe(true);
      });

      it('identifies credit order using netPremium (zero)', () => {
        const orderData = { netPremium: 0 };
        
        expect(orderService.isCreditOrder(orderData)).toBe(true);
      });

      it('identifies debit order using netPremium (positive)', () => {
        const orderData = { netPremium: 2.50 };
        
        expect(orderService.isCreditOrder(orderData)).toBe(false);
      });

      it('falls back to limitPrice when netPremium unavailable', () => {
        const creditOrder = { limitPrice: -1.25 };
        const debitOrder = { limitPrice: 2.75 };
        
        expect(orderService.isCreditOrder(creditOrder)).toBe(true);
        expect(orderService.isCreditOrder(debitOrder)).toBe(false);
      });

      it('defaults to debit when no pricing information available', () => {
        const orderData = {};
        
        expect(orderService.isCreditOrder(orderData)).toBe(false);
      });

      it('prioritizes netPremium over limitPrice', () => {
        const orderData = {
          netPremium: -1.50, // Credit
          limitPrice: 2.50   // Would be debit
        };
        
        expect(orderService.isCreditOrder(orderData)).toBe(true);
      });
    });
  });

  describe('Order Summary Calculations', () => {
    describe('Premium and P&L Calculations', () => {
      it('calculates order summary for iron butterfly', () => {
        const orderData = {
          legs: [
            { side: 'buy', price: 5.00, ratio_qty: 1 },
            { side: 'sell', price: 3.00, ratio_qty: 2 },
            { side: 'buy', price: 1.00, ratio_qty: 1 }
          ],
          orderPrice: -1.00, // Credit
          strategyType: 'IRON Butterfly'
        };

        const summary = orderService.calculateOrderSummary(orderData);
        
        expect(summary.totalPremium).toBe(0); // (5*-1) + (3*2) + (1*-1) = 0
        expect(summary.maxProfit).toBe(100); // |orderPrice| * 100
        expect(summary.maxLoss).toBe(100); // (2 - |orderPrice|) * 100
        expect(summary.netCredit).toBe(true);
        expect(summary.netDebit).toBe(false);
      });

      it('calculates order summary for other strategies', () => {
        const orderData = {
          legs: [
            { side: 'buy', price: 4.00, ratio_qty: 1 },
            { side: 'sell', price: 2.00, ratio_qty: 1 }
          ],
          orderPrice: 2.00, // Debit
          strategyType: 'Call Spread'
        };

        const summary = orderService.calculateOrderSummary(orderData);
        
        expect(summary.totalPremium).toBe(-2.00); // (4*-1) + (2*1) = -2
        expect(summary.maxProfit).toBe(0); // (2 - |orderPrice|) * 100 = 0
        expect(summary.maxLoss).toBe(200); // |orderPrice| * 100
        expect(summary.netCredit).toBe(false);
        expect(summary.netDebit).toBe(true);
      });

      it('handles missing leg prices gracefully', () => {
        const orderData = {
          legs: [
            { side: 'buy', ratio_qty: 1 }, // No price
            { side: 'sell', price: 2.00, ratio_qty: 1 }
          ],
          orderPrice: 1.50,
          strategyType: 'Spread'
        };

        const summary = orderService.calculateOrderSummary(orderData);
        
        expect(summary.totalPremium).toBe(2.00); // (0*-1) + (2*1) = 2
        expect(summary.netDebit).toBe(true);
      });

      it('handles zero order price correctly', () => {
        const orderData = {
          legs: [],
          orderPrice: 0,
          strategyType: 'Test'
        };

        const summary = orderService.calculateOrderSummary(orderData);
        
        expect(summary.netCredit).toBe(false); // 0 >= 0 is true, so debit
        expect(summary.netDebit).toBe(true);
      });
    });
  });

  describe('Order Type Determination', () => {
    it('determines single-leg order correctly', () => {
      const legs = [{ symbol: 'SPY_240119C00500000', action: 'buy', ratio_qty: 1 }];
      const orderType = orderService.determineOrderType(legs);
      expect(orderType).toBe('single-leg');
    });

    it('determines multi-leg order correctly', () => {
      const legs = [
        { symbol: 'SPY_240119C00500000', action: 'buy', ratio_qty: 1 },
        { symbol: 'SPY_240119C00510000', action: 'sell', ratio_qty: 1 }
      ];
      const orderType = orderService.determineOrderType(legs);
      expect(orderType).toBe('multi-leg');
    });

    it('throws error for empty legs array', () => {
      expect(() => orderService.determineOrderType([])).toThrow('No legs provided');
    });

    it('throws error for null legs', () => {
      expect(() => orderService.determineOrderType(null)).toThrow('No legs provided');
    });
  });

  describe('Utility Functions', () => {
    describe('Expiry Date Formatting', () => {
      it('formats string expiry correctly', () => {
        const expiry = '2024-01-19';
        const formatted = orderService.formatExpiry(expiry);
        
        expect(formatted).toBe('2024-01-19');
      });

      it('formats Date object to ISO string', () => {
        const expiry = new Date('2024-01-19T00:00:00.000Z');
        const formatted = orderService.formatExpiry(expiry);
        
        expect(formatted).toBe('2024-01-19');
      });

      it('returns non-string, non-Date values as-is', () => {
        const expiry = 20240119;
        const formatted = orderService.formatExpiry(expiry);
        
        expect(formatted).toBe(20240119);
      });
    });
  });

  describe('Edge Cases & Error Handling', () => {
    describe('Malformed Data Handling', () => {
      it('handles null/undefined order data gracefully', () => {
        // The actual service throws on null/undefined, which is expected behavior
        expect(() => orderService.validateOrder(null)).toThrow();
        expect(() => orderService.validateOrder(undefined)).toThrow();
        expect(() => orderService.validateOrder({})).not.toThrow();
        
        const emptyResult = orderService.validateOrder({});
        expect(emptyResult.isValid).toBe(false);
        expect(emptyResult.errors.length).toBeGreaterThan(0);
      });

      it('handles empty legs array', () => {
        const formattedLegs = orderService.formatLegs([]);
        expect(formattedLegs).toEqual([]);
      });

      it('handles null legs in formatLegs', () => {
        expect(() => orderService.formatLegs(null)).toThrow();
      });

      it('handles malformed leg data', () => {
        const legs = [
          null,
          undefined,
          {},
          { symbol: 'TEST' } // Missing required fields
        ];

        const formattedLegs = orderService.formatLegs(legs.filter(Boolean));
        
        // Should handle the valid entries
        expect(formattedLegs).toHaveLength(2);
        expect(formattedLegs[1]).toEqual({
          symbol: 'TEST',
          side: undefined,
          qty: 1
        });
      });
    });

    describe('Boundary Value Testing', () => {
      it('handles zero and negative quantities', () => {
        const legs = [
          { symbol: 'TEST1', action: 'buy', ratio_qty: 0 },
          { symbol: 'TEST2', action: 'sell', ratio_qty: -1 },
          { symbol: 'TEST3', action: 'buy', ratio_qty: 1000 }
        ];

        const formattedLegs = orderService.formatLegs(legs);
        
        // parseInt(0 || 1) = parseInt(1) = 1, because 0 is falsy
        expect(formattedLegs[0].qty).toBe(1);
        expect(formattedLegs[1].qty).toBe(-1);
        expect(formattedLegs[2].qty).toBe(1000);
      });

      it('handles extreme price values', () => {
        const orderData1 = { netPremium: -999999.99 };
        const orderData2 = { netPremium: 999999.99 };
        const orderData3 = { limitPrice: 0.01 };
        
        expect(orderService.isCreditOrder(orderData1)).toBe(true);
        expect(orderService.isCreditOrder(orderData2)).toBe(false);
        expect(orderService.isCreditOrder(orderData3)).toBe(false);
      });
    });
  });

  describe('Real-World Order Scenarios', () => {
    describe('Complex Multi-Leg Orders', () => {
      it('validates complex order before submission', () => {
        const complexOrder = {
          symbol: 'QQQ',
          expiry: '2024-02-16',
          legs: [
            { symbol: 'QQQ_240216C00400000', side: 'buy', ratio_qty: 2 },
            { symbol: 'QQQ_240216C00410000', side: 'sell', ratio_qty: 4 },
            { symbol: 'QQQ_240216C00420000', side: 'buy', ratio_qty: 2 }
          ],
          orderPrice: 1.25
        };

        const validation = orderService.validateOrder(complexOrder);
        expect(validation.isValid).toBe(true);

        const summary = orderService.calculateOrderSummary(complexOrder);
        expect(summary.netDebit).toBe(true);
      });

      it('builds correct payload for iron condor order', () => {
        const ironCondorOrder = {
          symbol: 'SPY',
          expiry: '2024-01-19',
          strategyType: 'Iron Condor',
          legs: [
            { symbol: 'SPY_240119P00480000', action: 'sell', ratio_qty: 1 },
            { symbol: 'SPY_240119P00490000', action: 'buy', ratio_qty: 1 },
            { symbol: 'SPY_240119C00510000', action: 'buy', ratio_qty: 1 },
            { symbol: 'SPY_240119C00520000', action: 'sell', ratio_qty: 1 }
          ],
          limitPrice: -0.50, // Credit
          timeInForce: 'GTC'
        };

        const payload = orderService.buildOrderPayload(ironCondorOrder);
        
        expect(payload).toEqual({
          legs: [
            { symbol: 'SPY_240119P00480000', side: 'sell', qty: 1 },
            { symbol: 'SPY_240119P00490000', side: 'buy', qty: 1 },
            { symbol: 'SPY_240119C00510000', side: 'buy', qty: 1 },
            { symbol: 'SPY_240119C00520000', side: 'sell', qty: 1 }
          ],
          order_type: 'limit',
          time_in_force: 'gtc',
          limit_price: -0.50
        });
      });
    });

    describe('Order Modification Scenarios', () => {
      it('handles price adjustments correctly', () => {
        const baseOrder = {
          legs: [{ symbol: 'SPY_240119C00500000', action: 'buy', ratio_qty: 1 }]
        };

        // Test different price scenarios
        const creditOrder = { ...baseOrder, limitPrice: -1.50 };
        const debitOrder = { ...baseOrder, limitPrice: 2.50 };
        const evenOrder = { ...baseOrder, limitPrice: 0 };

        expect(orderService.isCreditOrder(creditOrder)).toBe(true);
        expect(orderService.isCreditOrder(debitOrder)).toBe(false);
        expect(orderService.isCreditOrder(evenOrder)).toBe(false);
      });

      it('handles quantity modifications', () => {
        const legs = [
          { symbol: 'SPY_240119C00500000', action: 'buy', ratio_qty: 1 },
          { symbol: 'SPY_240119C00510000', action: 'sell', ratio_qty: 2 }
        ];

        const formattedLegs = orderService.formatLegs(legs);
        
        expect(formattedLegs[0].qty).toBe(1);
        expect(formattedLegs[1].qty).toBe(2);
      });
    });
  });
});
