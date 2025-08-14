import { describe, it, expect, beforeEach, vi } from 'vitest';
import { SelectedLegsStore } from '../src/services/selectedLegsStore.js';

describe('SelectedLegsStore - Data Integrity & Cascade Protection', () => {
  let store;

  beforeEach(() => {
    store = new SelectedLegsStore();
  });

  describe('Data Source Validation', () => {
    describe('Options Chain Data', () => {
      it('handles complete options chain data correctly', () => {
        const optionData = {
          symbol: 'SPY_240119C00500000',
          strike_price: 500,
          type: 'call',
          expiry: '2024-01-19',
          bid: 4.50,
          ask: 4.60,
          current_price: 4.55,
          side: 'buy'
        };

        const result = store.addLeg(optionData, 'options_chain');
        expect(result).toBe(true);

        const leg = store.getLeg(optionData.symbol);
        expect(leg).toMatchObject({
          symbol: optionData.symbol,
          side: 'buy',
          action: 'buy_to_open',
          quantity: 1,
          strike_price: 500,
          type: 'call',
          expiry: '2024-01-19',
          current_price: 4.55,
          bid: 4.50,
          ask: 4.60,
          source: 'options_chain'
        });
      });

      it('handles missing price data gracefully', () => {
        const optionData = {
          symbol: 'SPY_240119C00500000',
          strike_price: 500,
          type: 'call',
          expiry: '2024-01-19',
          side: 'buy'
          // Missing: bid, ask, current_price
        };

        const result = store.addLeg(optionData, 'options_chain');
        expect(result).toBe(true);

        const leg = store.getLeg(optionData.symbol);
        expect(leg.current_price).toBe(0);
        expect(leg.bid).toBe(0);
        expect(leg.ask).toBe(0);
      });

      it('calculates current_price from bid/ask when missing', () => {
        const optionData = {
          symbol: 'SPY_240119C00500000',
          strike_price: 500,
          type: 'call',
          expiry: '2024-01-19',
          bid: 4.50,
          ask: 4.60,
          side: 'buy'
          // Missing: current_price
        };

        const result = store.addLeg(optionData, 'options_chain');
        expect(result).toBe(true);

        const leg = store.getLeg(optionData.symbol);
        expect(leg.current_price).toBe(4.55); // (4.50 + 4.60) / 2
      });
    });

    describe('Position Data', () => {
      it('handles complete position data correctly', () => {
        const positionData = {
          symbol: 'SPY_240119C00500000',
          strike_price: 500,
          type: 'call', // Use normalized field name
          expiry: '2024-01-19', // Use normalized field name
          side: 'buy', // Use normalized side
          original_quantity: 5,
          quantity: 3,
          avg_entry_price: 4.25,
          current_price: 4.55,
          unrealized_pl: 150,
          bid: 4.50,
          ask: 4.60
        };

        const result = store.addLeg(positionData, 'positions');
        expect(result).toBe(true);

        const leg = store.getLeg(positionData.symbol);
        expect(leg).toMatchObject({
          symbol: positionData.symbol,
          side: 'buy',
          action: 'buy_to_close', // Position closing action
          quantity: 3,
          strike_price: 500,
          type: 'call',
          expiry: '2024-01-19',
          current_price: 4.55,
          original_quantity: 5,
          avg_entry_price: 4.25,
          unrealized_pl: 150,
          source: 'positions'
        });
      });

      it('handles position data with missing prices', () => {
        const positionData = {
          symbol: 'SPY_240119C00500000',
          strike_price: 500,
          type: 'call', // Use normalized field
          expiry: '2024-01-19', // Use normalized field
          side: 'buy', // Use normalized side
          original_quantity: 5
          // Missing: current_price, bid, ask, avg_entry_price
        };

        const result = store.addLeg(positionData, 'positions');
        expect(result).toBe(true);

        const leg = store.getLeg(positionData.symbol);
        expect(leg.current_price).toBe(0);
        expect(leg.bid).toBe(0);
        expect(leg.ask).toBe(0);
        expect(leg.avg_entry_price).toBe(null);
      });

      it('uses original_quantity as default quantity for positions', () => {
        const positionData = {
          symbol: 'SPY_240119C00500000',
          strike_price: 500,
          type: 'call', // Use normalized field
          expiry: '2024-01-19', // Use normalized field
          side: 'buy', // Use normalized side
          original_quantity: 10
          // Missing: quantity
        };

        const result = store.addLeg(positionData, 'positions');
        expect(result).toBe(true);

        const leg = store.getLeg(positionData.symbol);
        expect(leg.quantity).toBe(10);
      });
    });

    describe('Provider Data Format Compatibility', () => {
      it('handles Tastytrade-style data format', () => {
        const tastytradeData = {
          symbol: 'SPY   240119C00500000',
          strike: 500.0,
          option_type: 'C',
          expiration_date: '2024-01-19',
          bid: 4.50,
          ask: 4.60,
          mark: 4.55
        };

        const result = store.addLeg({
          ...tastytradeData,
          current_price: tastytradeData.mark,
          type: tastytradeData.option_type.toLowerCase() === 'c' ? 'call' : 'put',
          strike_price: tastytradeData.strike,
          expiry: tastytradeData.expiration_date
        }, 'options_chain');

        expect(result).toBe(true);
        const leg = store.getLeg(tastytradeData.symbol);
        expect(leg.type).toBe('call');
        expect(leg.strike_price).toBe(500);
        expect(leg.current_price).toBe(4.55);
      });

      it('handles Alpaca-style data format', () => {
        const alpacaData = {
          symbol: 'SPY240119C00500000',
          strike_price: 500,
          option_type: 'call',
          expiry_date: '2024-01-19',
          bid_price: 4.50,
          ask_price: 4.60,
          last_price: 4.55
        };

        const result = store.addLeg({
          ...alpacaData,
          bid: alpacaData.bid_price,
          ask: alpacaData.ask_price,
          current_price: alpacaData.last_price,
          type: alpacaData.option_type,
          expiry: alpacaData.expiry_date
        }, 'options_chain');

        expect(result).toBe(true);
        const leg = store.getLeg(alpacaData.symbol);
        expect(leg.type).toBe('call');
        expect(leg.current_price).toBe(4.55);
        expect(leg.bid).toBe(4.50);
        expect(leg.ask).toBe(4.60);
      });

      it('handles mixed case option types', () => {
        const validTestCases = [
          { type: 'call', expected: 'call' },
          { type: 'put', expected: 'put' },
          { type: 'C', expected: 'C' },
          { type: 'c', expected: 'c' },
          { type: 'P', expected: 'P' },
          { type: 'p', expected: 'p' }
        ];

        // Test valid cases
        validTestCases.forEach(({ type, expected }) => {
          const optionData = {
            symbol: `TEST_${type}`,
            strike_price: 500,
            type: type,
            expiry: '2024-01-19'
          };

          const result = store.addLeg(optionData, 'options_chain');
          expect(result).toBe(true);

          const leg = store.getLeg(optionData.symbol);
          expect(leg.type).toBe(expected);
        });

        // Test that invalid cases are rejected (like 'Call', 'CALL', etc.)
        const invalidTestCases = ['Call', 'CALL', 'Put', 'PUT'];
        invalidTestCases.forEach(type => {
          const optionData = {
            symbol: `INVALID_${type}`,
            strike_price: 500,
            type: type,
            expiry: '2024-01-19'
          };

          const result = store.addLeg(optionData, 'options_chain');
          expect(result).toBe(false); // Should be rejected
        });
      });
    });
  });

  describe('State Management Integrity', () => {
    describe('Leg Addition and Removal', () => {
      it('prevents duplicate legs with same symbol', () => {
        const optionData = {
          symbol: 'SPY_240119C00500000',
          strike_price: 500,
          type: 'call',
          expiry: '2024-01-19',
          current_price: 4.55
        };

        // Add first leg
        const result1 = store.addLeg(optionData, 'options_chain');
        expect(result1).toBe(true);
        expect(store.getAllLegs()).toHaveLength(1);

        // Add same leg again - should replace
        const updatedData = { ...optionData, current_price: 5.00 };
        const result2 = store.addLeg(updatedData, 'options_chain');
        expect(result2).toBe(true);
        expect(store.getAllLegs()).toHaveLength(1);

        const leg = store.getLeg(optionData.symbol);
        expect(leg.current_price).toBe(5.00);
      });

      it('handles leg removal correctly', () => {
        const optionData = {
          symbol: 'SPY_240119C00500000',
          strike_price: 500,
          type: 'call',
          expiry: '2024-01-19'
        };

        store.addLeg(optionData, 'options_chain');
        expect(store.hasLegs()).toBe(true);

        const removed = store.removeLeg(optionData.symbol);
        expect(removed).toBe(true);
        expect(store.hasLegs()).toBe(false);
        expect(store.getLeg(optionData.symbol)).toBe(null);
      });

      it('handles removal of non-existent leg', () => {
        const removed = store.removeLeg('NON_EXISTENT_SYMBOL');
        expect(removed).toBe(false);
      });
    });

    describe('Quantity Management', () => {
      beforeEach(() => {
        const optionData = {
          symbol: 'SPY_240119C00500000',
          strike_price: 500,
          type: 'call',
          expiry: '2024-01-19',
          quantity: 5
        };
        store.addLeg(optionData, 'options_chain');
      });

      it('updates leg quantity correctly', () => {
        const result = store.updateLegQuantity('SPY_240119C00500000', 10);
        expect(result).toBe(true);

        const leg = store.getLeg('SPY_240119C00500000');
        expect(leg.quantity).toBe(10);
      });

      it('enforces minimum quantity of 1', () => {
        const result = store.updateLegQuantity('SPY_240119C00500000', 0);
        expect(result).toBe(true);

        const leg = store.getLeg('SPY_240119C00500000');
        expect(leg.quantity).toBe(1);
      });

      it('enforces maximum quantity for options chain legs', () => {
        const result = store.updateLegQuantity('SPY_240119C00500000', 150);
        expect(result).toBe(true);

        const leg = store.getLeg('SPY_240119C00500000');
        expect(leg.quantity).toBe(100); // Max for options chain
      });

      it('enforces original quantity limit for position legs', () => {
        const positionData = {
          symbol: 'SPY_240119P00480000',
          strike_price: 480,
          type: 'put', // Use normalized field
          expiry: '2024-01-19', // Use normalized field
          side: 'buy', // Use normalized side
          original_quantity: 8,
          quantity: 5
        };

        store.addLeg(positionData, 'positions');

        // Try to update beyond original quantity
        const result = store.updateLegQuantity('SPY_240119P00480000', 12);
        expect(result).toBe(true);

        const leg = store.getLeg('SPY_240119P00480000');
        expect(leg.quantity).toBe(8); // Limited to original_quantity
      });

      it('handles quantity update for non-existent leg', () => {
        const result = store.updateLegQuantity('NON_EXISTENT', 5);
        expect(result).toBe(false);
      });
    });

    describe('Leg Replacement', () => {
      beforeEach(() => {
        const originalLeg = {
          symbol: 'SPY_240119C00500000',
          strike_price: 500,
          type: 'call',
          expiry: '2024-01-19',
          quantity: 3,
          side: 'buy',
          current_price: 4.55
        };
        store.addLeg(originalLeg, 'options_chain');
      });

      it('replaces leg while preserving quantity and side', () => {
        const newLegData = {
          symbol: 'SPY_240119C00510000',
          strike_price: 510,
          type: 'call',
          expiry: '2024-01-19',
          current_price: 3.25
        };

        const result = store.replaceLeg('SPY_240119C00500000', newLegData);
        expect(result).toBe(true);

        // Old leg should be gone
        expect(store.getLeg('SPY_240119C00500000')).toBe(null);

        // New leg should exist with preserved properties
        const newLeg = store.getLeg('SPY_240119C00510000');
        expect(newLeg).toBeTruthy();
        expect(newLeg.quantity).toBe(3); // Preserved
        expect(newLeg.side).toBe('buy'); // Preserved
        expect(newLeg.strike_price).toBe(510); // New
        expect(newLeg.current_price).toBe(3.25); // New
      });

      it('handles replacement of non-existent leg', () => {
        const newLegData = {
          symbol: 'SPY_240119C00510000',
          strike_price: 510,
          type: 'call',
          expiry: '2024-01-19'
        };

        const result = store.replaceLeg('NON_EXISTENT', newLegData);
        expect(result).toBe(false);
      });

      it('handles invalid new leg data', () => {
        const result = store.replaceLeg('SPY_240119C00500000', null);
        expect(result).toBe(false);

        const result2 = store.replaceLeg('SPY_240119C00500000', { invalid: 'data' });
        expect(result2).toBe(false);
      });
    });
  });

  describe('Reactive Metadata Calculations', () => {
    beforeEach(() => {
      // Add multiple legs for testing
      const legs = [
        {
          symbol: 'SPY_240119C00500000',
          strike_price: 500,
          type: 'call',
          expiry: '2024-01-19',
          side: 'buy',
          quantity: 2,
          current_price: 4.55
        },
        {
          symbol: 'SPY_240119C00510000',
          strike_price: 510,
          type: 'call',
          expiry: '2024-01-19',
          side: 'sell',
          quantity: 2,
          current_price: 3.25
        },
        {
          symbol: 'SPY_240119P00490000',
          strike_price: 490,
          type: 'put', // Use normalized field
          expiry: '2024-01-19', // Use normalized field
          side: 'buy', // Use normalized side
          original_quantity: 1,
          avg_entry_price: 2.50
        }
      ];

      store.addLeg(legs[0], 'options_chain');
      store.addLeg(legs[1], 'options_chain');
      store.addLeg(legs[2], 'positions');
    });

    it('calculates total legs correctly', () => {
      const metadata = store.metadata.value;
      expect(metadata.totalLegs).toBe(3);
    });

    it('calculates total quantity correctly', () => {
      const metadata = store.metadata.value;
      expect(metadata.totalQuantity).toBe(5); // 2 + 2 + 1
    });

      it('calculates net premium correctly', () => {
        const metadata = store.metadata.value;
        // Buy 2 @ 4.55 = -9.10, Sell 2 @ 3.25 = +6.50, Buy 1 @ 0 = 0
        // Net = -9.10 + 6.50 + 0 = -2.60 (store works in dollars, not cents)
        expect(metadata.netPremium).toBeCloseTo(-2.6, 1);
      });

    it('identifies credit and debit correctly', () => {
      const metadata = store.metadata.value;
      expect(metadata.hasCredit).toBe(true); // Has sell side
      expect(metadata.hasDebit).toBe(true); // Has buy side
    });

    it('tracks sources correctly', () => {
      const metadata = store.metadata.value;
      expect(metadata.sources).toContain('options_chain');
      expect(metadata.sources).toContain('positions');
      expect(metadata.sources).toHaveLength(2);
    });

    it('groups legs by source correctly', () => {
      const metadata = store.metadata.value;
      expect(metadata.bySource.options_chain).toHaveLength(2);
      expect(metadata.bySource.positions).toHaveLength(1);
    });

    it('updates metadata reactively when legs change', () => {
      let initialTotal = store.metadata.value.totalLegs;
      expect(initialTotal).toBe(3);

      // Add another leg
      store.addLeg({
        symbol: 'SPY_240119P00480000',
        strike_price: 480,
        type: 'put',
        expiry: '2024-01-19'
      }, 'options_chain');

      let newTotal = store.metadata.value.totalLegs;
      expect(newTotal).toBe(4);

      // Remove a leg
      store.removeLeg('SPY_240119C00500000');
      newTotal = store.metadata.value.totalLegs;
      expect(newTotal).toBe(3);
    });
  });

  describe('Source-Based Management', () => {
    beforeEach(() => {
      // Add legs from different sources
      const optionsChainLegs = [
        { symbol: 'OC_LEG_1', strike_price: 500, type: 'call', expiry: '2024-01-19' },
        { symbol: 'OC_LEG_2', strike_price: 510, type: 'call', expiry: '2024-01-19' }
      ];

      const positionLegs = [
        { symbol: 'POS_LEG_1', strike_price: 490, type: 'put', expiry: '2024-01-19', side: 'buy' },
        { symbol: 'POS_LEG_2', strike_price: 480, type: 'put', expiry: '2024-01-19', side: 'sell' }
      ];

      const strategyLegs = [
        { symbol: 'STRAT_LEG_1', strike_price: 520, type: 'call', expiry: '2024-01-19' }
      ];

      optionsChainLegs.forEach(leg => store.addLeg(leg, 'options_chain'));
      positionLegs.forEach(leg => store.addLeg(leg, 'positions'));
      strategyLegs.forEach(leg => store.addLeg(leg, 'strategies'));
    });

    it('gets legs from specific source', () => {
      const optionsChainLegs = store.getLegsFromSource('options_chain');
      const positionLegs = store.getLegsFromSource('positions');
      const strategyLegs = store.getLegsFromSource('strategies');

      expect(optionsChainLegs).toHaveLength(2);
      expect(positionLegs).toHaveLength(2);
      expect(strategyLegs).toHaveLength(1);
    });

    it('clears legs by source', () => {
      expect(store.getAllLegs()).toHaveLength(5);

      store.clearLegsBySource('options_chain');
      expect(store.getAllLegs()).toHaveLength(3);
      expect(store.getLegsFromSource('options_chain')).toHaveLength(0);
      expect(store.getLegsFromSource('positions')).toHaveLength(2);
      expect(store.getLegsFromSource('strategies')).toHaveLength(1);
    });

    it('clears all legs', () => {
      expect(store.getAllLegs()).toHaveLength(5);

      store.clearAllLegs();
      expect(store.getAllLegs()).toHaveLength(0);
      expect(store.hasLegs()).toBe(false);
    });
  });

  describe('Data Validation', () => {
    it('rejects legs without symbol', () => {
      const invalidLeg = {
        strike_price: 500,
        type: 'call',
        expiry: '2024-01-19'
        // Missing: symbol
      };

      const result = store.addLeg(invalidLeg, 'options_chain');
      expect(result).toBe(false);
      expect(store.hasLegs()).toBe(false);
    });

    it('rejects legs with invalid side', () => {
      const invalidLeg = {
        symbol: 'SPY_240119C00500000',
        strike_price: 500,
        type: 'call',
        expiry: '2024-01-19',
        side: 'invalid_side'
      };

      const result = store.addLeg(invalidLeg, 'options_chain');
      expect(result).toBe(false);
    });

    it('rejects legs with invalid option type', () => {
      const invalidLeg = {
        symbol: 'SPY_240119C00500000',
        strike_price: 500,
        type: 'invalid_type',
        expiry: '2024-01-19'
      };

      const result = store.addLeg(invalidLeg, 'options_chain');
      expect(result).toBe(false);
    });

    it('accepts valid option types in various formats', () => {
      const validTypes = ['call', 'put', 'C', 'P', 'c', 'p'];

      validTypes.forEach((type, index) => {
        const leg = {
          symbol: `TEST_${index}`,
          strike_price: 500,
          type: type,
          expiry: '2024-01-19'
        };

        const result = store.addLeg(leg, 'options_chain');
        expect(result).toBe(true);
      });

      expect(store.getAllLegs()).toHaveLength(validTypes.length);
    });
  });

  describe('Summary and Statistics', () => {
    beforeEach(() => {
      // Create a complex multi-leg position
      const legs = [
        {
          symbol: 'SPY_240119C00500000',
          strike_price: 500,
          type: 'call',
          expiry: '2024-01-19',
          side: 'buy',
          quantity: 2,
          current_price: 4.55,
          bid: 4.50,
          ask: 4.60
        },
        {
          symbol: 'SPY_240119C00510000',
          strike_price: 510,
          type: 'call',
          expiry: '2024-01-19',
          side: 'sell',
          quantity: 2,
          current_price: 3.25,
          bid: 3.20,
          ask: 3.30
        }
      ];

      legs.forEach(leg => store.addLeg(leg, 'options_chain'));
    });

    it('provides comprehensive summary', () => {
      const summary = store.getSummary();

      expect(summary).toMatchObject({
        totalLegs: 2,
        totalQuantity: 4,
        netPremium: -2.5999999999999996, // Floating point precision - use toBeCloseTo in real tests
        isCredit: false,
        isDebit: true,
        sources: ['options_chain'],
        breakdown: {
          options_chain: 2,
          positions: 0,
          strategies: 0
        }
      });
    });

    it('correctly identifies credit vs debit strategies', () => {
      // Clear existing legs
      store.clearAllLegs();

      // Add a credit spread (sell higher strike, buy lower strike calls)
      const creditSpread = [
        {
          symbol: 'SPY_240119C00500000',
          strike_price: 500,
          type: 'call',
          expiry: '2024-01-19',
          side: 'buy',
          quantity: 1,
          current_price: 2.00
        },
        {
          symbol: 'SPY_240119C00510000',
          strike_price: 510,
          type: 'call',
          expiry: '2024-01-19',
          side: 'sell',
          quantity: 1,
          current_price: 3.00
        }
      ];

      creditSpread.forEach(leg => store.addLeg(leg, 'options_chain'));

      const summary = store.getSummary();
      expect(summary.netPremium).toBe(1); // -2.00 + 3.00 = 1.00 (store works in dollars)
      expect(summary.isCredit).toBe(true);
      expect(summary.isDebit).toBe(false);
    });
  });

  describe('Edge Cases and Error Handling', () => {
    it('handles null and undefined inputs gracefully', () => {
      expect(store.addLeg(null, 'options_chain')).toBe(false);
      expect(store.addLeg(undefined, 'options_chain')).toBe(false);
      expect(store.addLeg({}, 'options_chain')).toBe(false);
    });

    it('handles empty string symbol', () => {
      const leg = {
        symbol: '',
        strike_price: 500,
        type: 'call',
        expiry: '2024-01-19'
      };

      expect(store.addLeg(leg, 'options_chain')).toBe(false);
    });

    it('handles missing strike price', () => {
      const leg = {
        symbol: 'SPY_240119C00500000',
        type: 'call',
        expiry: '2024-01-19'
        // Missing: strike_price
      };

      const result = store.addLeg(leg, 'options_chain');
      expect(result).toBe(true);

      const addedLeg = store.getLeg(leg.symbol);
      expect(addedLeg.strike_price).toBe(0);
    });

    it('handles missing expiry date', () => {
      const leg = {
        symbol: 'SPY_240119C00500000',
        strike_price: 500,
        type: 'call'
        // Missing: expiry
      };

      const result = store.addLeg(leg, 'options_chain');
      expect(result).toBe(true);

      const addedLeg = store.getLeg(leg.symbol);
      expect(addedLeg.expiry).toBeUndefined();
    });

      it('handles very large quantities', () => {
        const leg = {
          symbol: 'SPY_240119C00500000',
          strike_price: 500,
          type: 'call',
          expiry: '2024-01-19',
          quantity: 999999
        };

        store.addLeg(leg, 'options_chain');
        const addedLeg = store.getLeg(leg.symbol);
        // The store doesn't enforce quantity limits during addLeg, only during updateLegQuantity
        expect(addedLeg.quantity).toBe(999999);
      });

    it('handles negative quantities', () => {
      const leg = {
        symbol: 'SPY_240119C00500000',
        strike_price: 500,
        type: 'call',
        expiry: '2024-01-19',
        quantity: -5
      };

      store.addLeg(leg, 'options_chain');
      const addedLeg = store.getLeg(leg.symbol);
      expect(addedLeg.quantity).toBe(1); // Should be set to minimum of 1
    });
  });

  describe('Concurrent Operations', () => {
    it('handles rapid leg additions and removals', () => {
      const symbols = Array.from({ length: 10 }, (_, i) => `TEST_SYMBOL_${i}`);
      
      // Add all legs rapidly
      symbols.forEach((symbol, i) => {
        store.addLeg({
          symbol,
          strike_price: 500 + i,
          type: 'call',
          expiry: '2024-01-19'
        }, 'options_chain');
      });

      expect(store.getAllLegs()).toHaveLength(10);

      // Remove every other leg
      symbols.filter((_, i) => i % 2 === 0).forEach(symbol => {
        store.removeLeg(symbol);
      });

      expect(store.getAllLegs()).toHaveLength(5);
    });

    it('handles concurrent quantity updates', () => {
      const symbol = 'SPY_240119C00500000';
      store.addLeg({
        symbol,
        strike_price: 500,
        type: 'call',
        expiry: '2024-01-19',
        quantity: 1
      }, 'options_chain');

      // Simulate rapid quantity updates
      const updates = [5, 10, 3, 8, 1];
      updates.forEach(qty => {
        store.updateLegQuantity(symbol, qty);
      });

      const finalLeg = store.getLeg(symbol);
      expect(finalLeg.quantity).toBe(1); // Last update
    });
  });

  describe('Provider-Specific Edge Cases', () => {
    it('handles provider data with missing required fields', () => {
      // Simulate data from a provider that doesn't send all fields
      const incompleteProviderData = {
        symbol: 'INCOMPLETE_DATA',
        strike: 500, // Different field name
        // Missing: type, expiry, prices
      };

      const result = store.addLeg({
        ...incompleteProviderData,
        strike_price: incompleteProviderData.strike,
        type: 'call', // Default
        expiry: '2024-01-19' // Default
      }, 'options_chain');

      expect(result).toBe(true);
      const leg = store.getLeg('INCOMPLETE_DATA');
      expect(leg.strike_price).toBe(500);
      expect(leg.type).toBe('call');
      expect(leg.current_price).toBe(0);
    });

    it('handles provider data format inconsistencies', () => {
      // Test different ways providers might send the same data
      const providerVariations = [
        {
          symbol: 'PROVIDER_A',
          strike_price: 500,
          option_type: 'call',
          expiration_date: '2024-01-19',
          last_price: 4.55
        },
        {
          symbol: 'PROVIDER_B',
          strike: 500,
          type: 'C',
          expiry: '2024-01-19',
          mark: 4.55
        },
        {
          symbol: 'PROVIDER_C',
          strike_price: 500,
          call_put: 'call',
          exp_date: '2024-01-19',
          mid_price: 4.55
        }
      ];

      // Normalize each variation
      providerVariations.forEach((data, index) => {
        const normalized = {
          symbol: data.symbol,
          strike_price: data.strike_price || data.strike,
          type: data.option_type || (data.type === 'C' ? 'call' : data.type) || data.call_put,
          expiry: data.expiration_date || data.expiry || data.exp_date,
          current_price: data.last_price || data.mark || data.mid_price
        };

        const result = store.addLeg(normalized, 'options_chain');
        expect(result).toBe(true);
      });

      expect(store.getAllLegs()).toHaveLength(3);
    });
  });

  describe('Real-World Cascade Scenarios', () => {
    it('simulates complete leg selection to payoff chart flow', () => {
      // Step 1: User selects legs from options chain
      const chainLegs = [
        {
          symbol: 'SPY_240119C00500000',
          strike_price: 500,
          type: 'call',
          expiry: '2024-01-19',
          bid: 4.50,
          ask: 4.60,
          current_price: 4.55,
          side: 'buy',
          quantity: 1
        },
        {
          symbol: 'SPY_240119C00510000',
          strike_price: 510,
          type: 'call',
          expiry: '2024-01-19',
          bid: 3.20,
          ask: 3.30,
          current_price: 3.25,
          side: 'sell',
          quantity: 1
        }
      ];

      chainLegs.forEach(leg => {
        const result = store.addLeg(leg, 'options_chain');
        expect(result).toBe(true);
      });

      // Step 2: Verify data is ready for BottomTradingPanel calculations
      const allLegs = store.getAllLegs();
      expect(allLegs).toHaveLength(2);
      
      // Each leg should have all required fields for calculations
      allLegs.forEach(leg => {
        expect(leg.symbol).toBeTruthy();
        expect(leg.strike_price).toBeGreaterThan(0);
        expect(leg.type).toBeTruthy();
        expect(leg.side).toBeTruthy();
        expect(leg.quantity).toBeGreaterThan(0);
        expect(leg.current_price).toBeGreaterThanOrEqual(0);
        expect(leg.bid).toBeGreaterThanOrEqual(0);
        expect(leg.ask).toBeGreaterThanOrEqual(0);
      });

      // Step 3: Verify metadata calculations for trading panel
      const metadata = store.metadata.value;
      expect(metadata.netPremium).toBeCloseTo(-1.3, 1); // (4.55 * -1) + (3.25 * 1) = -1.30 (store works in dollars)
      expect(metadata.hasCredit).toBe(true);
      expect(metadata.hasDebit).toBe(true);

      // Step 4: Simulate strike price adjustment (common user action)
      const newLegData = {
        symbol: 'SPY_240119C00505000',
        strike_price: 505,
        type: 'call',
        expiry: '2024-01-19',
        bid: 3.80,
        ask: 3.90,
        current_price: 3.85
      };

      const replaced = store.replaceLeg('SPY_240119C00510000', newLegData);
      expect(replaced).toBe(true);

      // Verify the cascade continues to work
      const updatedLegs = store.getAllLegs();
      expect(updatedLegs).toHaveLength(2);
      expect(store.getLeg('SPY_240119C00505000')).toBeTruthy();
      expect(store.getLeg('SPY_240119C00510000')).toBe(null);
    });

    it('simulates position closing workflow', () => {
      // Step 1: Add existing positions
      const positions = [
        {
          symbol: 'SPY_240119C00500000',
          strike_price: 500,
          type: 'call', // Use normalized field
          expiry: '2024-01-19', // Use normalized field
          side: 'buy', // Use normalized side
          original_quantity: 10,
          quantity: 5, // Partial close
          avg_entry_price: 4.00,
          current_price: 4.55,
          unrealized_pl: 275, // (4.55 - 4.00) * 5 * 100
          bid: 4.50,
          ask: 4.60
        }
      ];

      positions.forEach(pos => {
        const result = store.addLeg(pos, 'positions');
        expect(result).toBe(true);
      });

      // Step 2: Verify position data is correctly normalized
      const leg = store.getLeg('SPY_240119C00500000');
      expect(leg.action).toBe('buy_to_close'); // Correct closing action
      expect(leg.source).toBe('positions');
      expect(leg.original_quantity).toBe(10);
      expect(leg.avg_entry_price).toBe(4.00);
      expect(leg.unrealized_pl).toBe(275);

      // Step 3: Simulate quantity adjustment (partial close)
      const updated = store.updateLegQuantity('SPY_240119C00500000', 3);
      expect(updated).toBe(true);

      const updatedLeg = store.getLeg('SPY_240119C00500000');
      expect(updatedLeg.quantity).toBe(3);

      // Step 4: Try to exceed original quantity (should be prevented)
      const overUpdate = store.updateLegQuantity('SPY_240119C00500000', 15);
      expect(overUpdate).toBe(true);

      const cappedLeg = store.getLeg('SPY_240119C00500000');
      expect(cappedLeg.quantity).toBe(10); // Capped at original_quantity
    });

    it('simulates provider data update scenario', () => {
      // Step 1: Add leg with initial provider data
      const initialData = {
        symbol: 'SPY_240119C00500000',
        strike_price: 500,
        type: 'call',
        expiry: '2024-01-19',
        bid: 4.50,
        ask: 4.60,
        current_price: 4.55
      };

      store.addLeg(initialData, 'options_chain');

      // Step 2: Simulate provider price update (same symbol, new prices)
      const updatedData = {
        symbol: 'SPY_240119C00500000',
        strike_price: 500,
        type: 'call',
        expiry: '2024-01-19',
        bid: 4.75,
        ask: 4.85,
        current_price: 4.80
      };

      store.addLeg(updatedData, 'options_chain');

      // Step 3: Verify prices were updated (not duplicated)
      expect(store.getAllLegs()).toHaveLength(1);
      const leg = store.getLeg('SPY_240119C00500000');
      expect(leg.current_price).toBe(4.80);
      expect(leg.bid).toBe(4.75);
      expect(leg.ask).toBe(4.85);
    });
  });
});
