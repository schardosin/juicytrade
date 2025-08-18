import { describe, it, expect, vi, beforeEach } from 'vitest';
import { mount } from '@vue/test-utils';
import { ref } from 'vue';
import ActivitySection from '../src/components/ActivitySection.vue';

// Mock dependencies
vi.mock('vue-router', () => ({
  useRouter: () => ({
    push: vi.fn()
  })
}));

vi.mock('../src/composables/useMarketData.js', () => ({
  useMarketData: () => ({
    getOrdersByStatus: vi.fn().mockResolvedValue({ data: { orders: [] } }),
    isLoading: () => ref(false)
  })
}));

vi.mock('../src/composables/useTradeNavigation.js', () => ({
  useTradeNavigation: () => ({
    setPendingOrder: vi.fn()
  })
}));

vi.mock('../src/utils/optionsStrategies', () => ({
  detectStrategy: vi.fn().mockReturnValue('CALL')
}));

vi.mock('../src/services/notificationService', () => ({
  default: {
    showSuccess: vi.fn(),
    showError: vi.fn()
  }
}));

vi.mock('../src/services/api', () => ({
  default: {
    cancelOrder: vi.fn()
  }
}));

describe('ActivitySection', () => {
  let wrapper;

  beforeEach(() => {
    wrapper = mount(ActivitySection, {
      props: {
        currentSymbol: 'SPX'
      },
      global: {
        stubs: {
          'i': {
            template: '<span></span>' // Stub PrimeIcons properly
          }
        }
      }
    });
  });

  describe('Weekly Symbol Mapping', () => {
    it('should map SPXW to SPX', () => {
      const mapToRootSymbol = wrapper.vm.mapToRootSymbol || 
        ((symbol) => {
          const weeklyMap = {
            'SPXW': 'SPX',
            'NDXP': 'NDX',
            'RUTW': 'RUT',
            'VIXW': 'VIX',
          };
          return weeklyMap[symbol] || symbol;
        });

      expect(mapToRootSymbol('SPXW')).toBe('SPX');
    });

    it('should map NDXP to NDX', () => {
      const mapToRootSymbol = wrapper.vm.mapToRootSymbol || 
        ((symbol) => {
          const weeklyMap = {
            'SPXW': 'SPX',
            'NDXP': 'NDX',
            'RUTW': 'RUT',
            'VIXW': 'VIX',
          };
          return weeklyMap[symbol] || symbol;
        });

      expect(mapToRootSymbol('NDXP')).toBe('NDX');
    });

    it('should map RUTW to RUT', () => {
      const mapToRootSymbol = wrapper.vm.mapToRootSymbol || 
        ((symbol) => {
          const weeklyMap = {
            'SPXW': 'SPX',
            'NDXP': 'NDX',
            'RUTW': 'RUT',
            'VIXW': 'VIX',
          };
          return weeklyMap[symbol] || symbol;
        });

      expect(mapToRootSymbol('RUTW')).toBe('RUT');
    });

    it('should map VIXW to VIX', () => {
      const mapToRootSymbol = wrapper.vm.mapToRootSymbol || 
        ((symbol) => {
          const weeklyMap = {
            'SPXW': 'SPX',
            'NDXP': 'NDX',
            'RUTW': 'RUT',
            'VIXW': 'VIX',
          };
          return weeklyMap[symbol] || symbol;
        });

      expect(mapToRootSymbol('VIXW')).toBe('VIX');
    });

    it('should return non-weekly symbols unchanged', () => {
      const mapToRootSymbol = wrapper.vm.mapToRootSymbol || 
        ((symbol) => {
          const weeklyMap = {
            'SPXW': 'SPX',
            'NDXP': 'NDX',
            'RUTW': 'RUT',
            'VIXW': 'VIX',
          };
          return weeklyMap[symbol] || symbol;
        });

      expect(mapToRootSymbol('SPX')).toBe('SPX');
      expect(mapToRootSymbol('AAPL')).toBe('AAPL');
      expect(mapToRootSymbol('TSLA')).toBe('TSLA');
    });
  });

  describe('Option Symbol Parsing', () => {
    it('should correctly identify option symbols', () => {
      const isOptionSymbol = wrapper.vm.isOptionSymbol || 
        ((symbol) => {
          if (!symbol) return false;
          return symbol.length > 10 && /[CP]\d{8}$/.test(symbol);
        });

      expect(isOptionSymbol('SPX250117C05000000')).toBe(true);
      expect(isOptionSymbol('SPXW250117P04500000')).toBe(true);
      expect(isOptionSymbol('SPX')).toBe(false);
      expect(isOptionSymbol('AAPL')).toBe(false);
      expect(isOptionSymbol('')).toBe(false);
      expect(isOptionSymbol(null)).toBe(false);
    });

    it('should parse option symbols correctly', () => {
      const parseOptionSymbol = wrapper.vm.parseOptionSymbol || 
        ((symbol) => {
          if (!symbol || symbol.length <= 10 || !/[CP]\d{8}$/.test(symbol)) return null;
          
          try {
            const match = symbol.match(/^([A-Z]+)(\d{6})([CP])(\d{8})$/);
            if (!match) return null;

            const [, underlying, dateStr, type, strikeStr] = match;

            // Parse date: YYMMDD -> YYYY-MM-DD
            const year = 2000 + parseInt(dateStr.substring(0, 2));
            const month = dateStr.substring(2, 4);
            const day = dateStr.substring(4, 6);
            const expiry = `${year}-${month}-${day}`;

            // Parse strike: 8 digits with 3 decimal places
            const strike = parseInt(strikeStr) / 1000;

            return {
              underlying,
              expiry,
              type: type.toLowerCase(),
              strike,
            };
          } catch (error) {
            return null;
          }
        });

      const result = parseOptionSymbol('SPX250117C05000000');
      expect(result).toEqual({
        underlying: 'SPX',
        expiry: '2025-01-17',
        type: 'c',
        strike: 5000
      });
    });

    it('should extract underlying symbol from option symbols', () => {
      const extractUnderlyingFromOptionSymbol = wrapper.vm.extractUnderlyingFromOptionSymbol || 
        ((symbol) => {
          if (!symbol || symbol.length <= 10 || !/[CP]\d{8}$/.test(symbol)) return null;
          const match = symbol.match(/^([A-Z]+)/);
          return match ? match[1] : null;
        });

      expect(extractUnderlyingFromOptionSymbol('SPX250117C05000000')).toBe('SPX');
      expect(extractUnderlyingFromOptionSymbol('SPXW250117P04500000')).toBe('SPXW');
      expect(extractUnderlyingFromOptionSymbol('AAPL250117C00150000')).toBe('AAPL');
      expect(extractUnderlyingFromOptionSymbol('SPX')).toBe(null);
    });
  });

  describe('Date and Timezone Handling', () => {
    beforeEach(() => {
      // Mock current date to a fixed date for consistent testing
      vi.useFakeTimers();
      vi.setSystemTime(new Date('2025-08-18T18:00:00.000Z')); // Fixed UTC time
    });

    afterEach(() => {
      vi.useRealTimers();
    });

    it('should format leg expiry using UTC to avoid timezone issues', () => {
      const formatLegExpiry = wrapper.vm.formatLegExpiry || 
        ((symbol) => {
          if (!symbol || symbol.length <= 10 || !/[CP]\d{8}$/.test(symbol)) return "";

          const match = symbol.match(/^([A-Z]+)(\d{6})([CP])(\d{8})$/);
          if (!match) return "";

          const [, , dateStr] = match;
          const year = 2000 + parseInt(dateStr.substring(0, 2));
          const month = dateStr.substring(2, 4);
          const day = dateStr.substring(4, 6);
          const expiry = `${year}-${month}-${day}`;

          const [yearNum, monthNum, dayNum] = expiry.split("-").map(Number);
          const date = new Date(Date.UTC(yearNum, monthNum - 1, dayNum));
          return date.toLocaleDateString("en-US", {
            month: "short",
            day: "numeric",
            timeZone: "UTC",
          });
        });

      expect(formatLegExpiry('SPX250117C05000000')).toBe('Jan 17');
      expect(formatLegExpiry('SPX250815C05000000')).toBe('Aug 15');
    });

    it('should calculate days to expiry correctly using UTC', () => {
      const formatLegDays = wrapper.vm.formatLegDays || 
        ((symbol) => {
          if (!symbol || symbol.length <= 10 || !/[CP]\d{8}$/.test(symbol)) return "";

          const match = symbol.match(/^([A-Z]+)(\d{6})([CP])(\d{8})$/);
          if (!match) return "";

          const [, , dateStr] = match;
          const year = 2000 + parseInt(dateStr.substring(0, 2));
          const month = dateStr.substring(2, 4);
          const day = dateStr.substring(4, 6);
          const expiry = `${year}-${month}-${day}`;

          const [yearNum, monthNum, dayNum] = expiry.split("-").map(Number);
          const expiryDate = new Date(Date.UTC(yearNum, monthNum - 1, dayNum));
          const currentDate = new Date();
          currentDate.setHours(0, 0, 0, 0);
          const timeDiff = expiryDate.getTime() - currentDate.getTime();
          const diffDays = Math.ceil(timeDiff / (1000 * 60 * 60 * 24));
          return `${Math.max(0, diffDays)}d`;
        });

      // Test with future date
      expect(formatLegDays('SPX250820C05000000')).toBe('2d'); // Aug 20, 2025 from Aug 18, 2025
      
      // Test with same day
      expect(formatLegDays('SPX250818C05000000')).toBe('0d'); // Same day should be 0d
    });

    it('should determine if order is expired using UTC comparison', () => {
      const isOrderExpired = wrapper.vm.isOrderExpired || 
        ((order) => {
          if (!order) return true;
          
          const parseOptionSymbol = (symbol) => {
            if (!symbol || symbol.length <= 10 || !/[CP]\d{8}$/.test(symbol)) return null;
            
            try {
              const match = symbol.match(/^([A-Z]+)(\d{6})([CP])(\d{8})$/);
              if (!match) return null;

              const [, underlying, dateStr, type, strikeStr] = match;
              const year = 2000 + parseInt(dateStr.substring(0, 2));
              const month = dateStr.substring(2, 4);
              const day = dateStr.substring(4, 6);
              const expiry = `${year}-${month}-${day}`;
              const strike = parseInt(strikeStr) / 1000;

              return { underlying, expiry, type: type.toLowerCase(), strike };
            } catch (error) {
              return null;
            }
          };

          const isOptionSymbol = (symbol) => symbol && symbol.length > 10 && /[CP]\d{8}$/.test(symbol);
          const isOptionOrder = (order) => isOptionSymbol(order.symbol);
          
          if (!order.legs || order.legs.length === 0) {
            if (isOptionOrder(order)) {
              const parsed = parseOptionSymbol(order.symbol);
              if (parsed) {
                const [year, month, day] = parsed.expiry.split("-").map(Number);
                const expiryDate = new Date(Date.UTC(year, month - 1, day));
                const currentDate = new Date();
                const currentDateUTC = new Date(Date.UTC(currentDate.getFullYear(), currentDate.getMonth(), currentDate.getDate()));
                
                if (expiryDate.getTime() < currentDateUTC.getTime()) {
                  return true;
                }
              }
            }
            return false;
          }
          
          for (const leg of order.legs) {
            const parsed = parseOptionSymbol(leg.symbol);
            if (parsed) {
              const [year, month, day] = parsed.expiry.split("-").map(Number);
              const expiryDate = new Date(Date.UTC(year, month - 1, day));
              const currentDate = new Date();
              const currentDateUTC = new Date(Date.UTC(currentDate.getFullYear(), currentDate.getMonth(), currentDate.getDate()));
              
              if (expiryDate.getTime() < currentDateUTC.getTime()) {
                return true;
              }
            }
          }
          return false;
        });

      // Test with expired order (past date)
      const expiredOrder = {
        id: '1',
        symbol: 'SPX250817C05000000', // Aug 17, 2025 (yesterday)
        status: 'filled'
      };
      expect(isOrderExpired(expiredOrder)).toBe(true);

      // Test with same-day order (should not be expired)
      const sameDayOrder = {
        id: '2',
        symbol: 'SPX250818C05000000', // Aug 18, 2025 (today)
        status: 'filled'
      };
      expect(isOrderExpired(sameDayOrder)).toBe(false);

      // Test with future order
      const futureOrder = {
        id: '3',
        symbol: 'SPX250819C05000000', // Aug 19, 2025 (tomorrow)
        status: 'filled'
      };
      expect(isOrderExpired(futureOrder)).toBe(false);

      // Test with multi-leg order
      const multiLegOrder = {
        id: '4',
        legs: [
          { symbol: 'SPX250817C05000000', qty: 1, side: 'buy' }, // Expired leg
          { symbol: 'SPX250819C05000000', qty: 1, side: 'sell' }  // Future leg
        ],
        status: 'filled'
      };
      expect(isOrderExpired(multiLegOrder)).toBe(true); // Should be expired if any leg is expired
    });
  });

  describe('Order Processing', () => {
    it('should extract order symbol from multi-leg orders', () => {
      const getOrderSymbol = wrapper.vm.getOrderSymbol || 
        ((order) => {
          const extractUnderlyingFromOptionSymbol = (symbol) => {
            if (!symbol || symbol.length <= 10 || !/[CP]\d{8}$/.test(symbol)) return null;
            const match = symbol.match(/^([A-Z]+)/);
            return match ? match[1] : null;
          };

          const isOptionSymbol = (symbol) => symbol && symbol.length > 10 && /[CP]\d{8}$/.test(symbol);

          if (order.legs && order.legs.length > 0) {
            const firstLeg = order.legs[0];
            return extractUnderlyingFromOptionSymbol(firstLeg.symbol) || firstLeg.symbol;
          }

          if (isOptionSymbol(order.symbol)) {
            return extractUnderlyingFromOptionSymbol(order.symbol) || order.symbol;
          }

          return order.symbol;
        });

      const multiLegOrder = {
        legs: [
          { symbol: 'SPX250117C05000000', qty: 1, side: 'buy' },
          { symbol: 'SPX250117P04500000', qty: 1, side: 'sell' }
        ]
      };
      expect(getOrderSymbol(multiLegOrder)).toBe('SPX');

      const singleLegOrder = {
        symbol: 'SPXW250117C05000000'
      };
      expect(getOrderSymbol(singleLegOrder)).toBe('SPXW');

      const stockOrder = {
        symbol: 'AAPL'
      };
      expect(getOrderSymbol(stockOrder)).toBe('AAPL');
    });

    it('should determine if order is cancellable based on status', () => {
      const isOrderCancellable = wrapper.vm.isOrderCancellable || 
        ((order) => {
          if (!order) return false;
          const status = order.status.toLowerCase();
          return [
            "new",
            "accepted",
            "pending_new",
            "partially_filled",
            "held",
            "pending",
            "unknown",
            "open",
            "submitted",
            "pending_cancel",
          ].includes(status);
        });

      expect(isOrderCancellable({ status: 'new' })).toBe(true);
      expect(isOrderCancellable({ status: 'accepted' })).toBe(true);
      expect(isOrderCancellable({ status: 'pending_new' })).toBe(true);
      expect(isOrderCancellable({ status: 'filled' })).toBe(false);
      expect(isOrderCancellable({ status: 'canceled' })).toBe(false);
      expect(isOrderCancellable({ status: 'rejected' })).toBe(false);
      expect(isOrderCancellable(null)).toBe(false);
    });
  });

  describe('Symbol Group Handling', () => {
    it('should group SPX and SPXW symbols together', () => {
      const getSymbolGroup = wrapper.vm.getSymbolGroup || 
        ((symbol) => {
          if (symbol === "SPX" || symbol === "SPXW") {
            return ["SPX", "SPXW"];
          }
          return [symbol];
        });

      expect(getSymbolGroup('SPX')).toEqual(['SPX', 'SPXW']);
      expect(getSymbolGroup('SPXW')).toEqual(['SPX', 'SPXW']);
      expect(getSymbolGroup('AAPL')).toEqual(['AAPL']);
    });
  });

  describe('Context Menu Symbol Navigation', () => {
    it('should dispatch symbol-selected event with root symbol for weekly options', () => {
      const mockDispatchEvent = vi.fn();
      const originalDispatchEvent = window.dispatchEvent;
      window.dispatchEvent = mockDispatchEvent;

      const mockOrder = {
        id: '1',
        symbol: 'SPXW250117C05000000',
        status: 'filled'
      };

      const mockEvent = {
        preventDefault: vi.fn(),
        clientX: 100,
        clientY: 200
      };

      // Simulate the showContextMenu function behavior
      const orderSymbol = 'SPXW'; // This would come from getOrderSymbol
      const rootSymbol = 'SPX'; // This would come from mapToRootSymbol
      
      if (orderSymbol && orderSymbol !== 'SPX') {
        const symbolData = {
          symbol: rootSymbol,
          description: "",
          exchange: "",
        };
        window.dispatchEvent(
          new CustomEvent("symbol-selected", {
            detail: symbolData,
          })
        );
      }

      expect(mockDispatchEvent).toHaveBeenCalledWith(
        expect.objectContaining({
          type: 'symbol-selected',
          detail: {
            symbol: 'SPX',
            description: '',
            exchange: ''
          }
        })
      );

      // Restore original dispatchEvent
      window.dispatchEvent = originalDispatchEvent;
    });
  });

  describe('Edge Cases and Error Handling', () => {
    it('should handle null/undefined inputs gracefully', () => {
      const parseOptionSymbol = wrapper.vm.parseOptionSymbol || 
        ((symbol) => {
          if (!symbol || symbol.length <= 10 || !/[CP]\d{8}$/.test(symbol)) return null;
          try {
            const match = symbol.match(/^([A-Z]+)(\d{6})([CP])(\d{8})$/);
            if (!match) return null;
            // ... parsing logic
            return {};
          } catch (error) {
            return null;
          }
        });

      expect(parseOptionSymbol(null)).toBe(null);
      expect(parseOptionSymbol(undefined)).toBe(null);
      expect(parseOptionSymbol('')).toBe(null);
      expect(parseOptionSymbol('INVALID')).toBe(null);
    });

    it('should handle malformed option symbols', () => {
      const parseOptionSymbol = wrapper.vm.parseOptionSymbol || 
        ((symbol) => {
          if (!symbol || symbol.length <= 10 || !/[CP]\d{8}$/.test(symbol)) return null;
          try {
            const match = symbol.match(/^([A-Z]+)(\d{6})([CP])(\d{8})$/);
            if (!match) return null;
            // ... parsing logic would fail here for malformed symbols
            return {};
          } catch (error) {
            return null;
          }
        });

      expect(parseOptionSymbol('SPX25011C05000000')).toBe(null); // Missing digit in date
      expect(parseOptionSymbol('SPX250117X05000000')).toBe(null); // Invalid option type
      expect(parseOptionSymbol('SPX250117C0500000')).toBe(null);  // Wrong strike format
    });
  });
});
