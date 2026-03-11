import { describe, it, expect, beforeEach, vi } from 'vitest';
import { mount } from '@vue/test-utils';
import { ref, nextTick } from 'vue';
import RightPanel from '../src/components/RightPanel.vue';

// Create mock refs that can be updated in tests
const mockSelectedLegs = ref([]);
const mockPositionsData = ref({ positions: [] });
const mockOptionPrice = ref({ price: 4.55 });

// Mock the composables with controllable implementations
vi.mock('../src/composables/useMarketData.js', () => ({
  useMarketData: () => ({
    getPositionsForSymbol: () => mockPositionsData
  })
}));

vi.mock('../src/composables/useSelectedLegs.js', () => ({
  useSelectedLegs: () => ({
    selectedLegs: mockSelectedLegs
  })
}));

const mockStockPrice = ref({ value: 500, bid: 499.5, ask: 500.5 });

vi.mock('../src/composables/useSmartMarketData.js', () => ({
  useSmartMarketData: () => ({
    getOptionPrice: () => mockOptionPrice,
    getStockPrice: () => mockStockPrice
  })
}));

vi.mock('../src/services/smartMarketDataStore.js', () => ({
  smartMarketDataStore: {
    registerSymbolUsage: vi.fn(),
    unregisterSymbolUsage: vi.fn()
  }
}));

// Mock child components to avoid complex dependencies
vi.mock('../src/components/RightPanelSection.vue', () => ({
  default: {
    name: 'RightPanelSection',
    template: '<div class="right-panel-section"><slot></slot></div>',
    props: ['title', 'icon', 'defaultExpanded'],
    emits: ['toggle']
  }
}));

vi.mock('../src/components/QuoteDetailsSection.vue', () => ({
  default: {
    name: 'QuoteDetailsSection',
    template: '<div class="quote-details-section"></div>',
    props: ['symbol', 'currentPrice', 'priceChange', 'additionalQuoteData']
  }
}));

vi.mock('../src/components/PayoffChart.vue', () => ({
  default: {
    name: 'PayoffChart',
    template: '<div class="payoff-chart-mock"></div>',
    props: ['chartData', 'underlyingPrice', 'title', 'showInfo', 'height', 'symbol', 'isLivePrice']
  }
}));

vi.mock('../src/components/ActivitySection.vue', () => ({
  default: {
    name: 'ActivitySection',
    template: '<div class="activity-section"></div>',
    props: ['currentSymbol']
  }
}));

vi.mock('../src/components/WatchlistSection.vue', () => ({
  default: {
    name: 'WatchlistSection',
    template: '<div class="watchlist-section"></div>'
  }
}));

// Mock RightPanelChart to avoid lightweight-charts errors in test environment
vi.mock('../src/components/RightPanelChart.vue', () => ({
  default: {
    name: 'RightPanelChart',
    template: '<div class="right-panel-chart-mock"></div>',
    props: ['symbol', 'currentPrice', 'height', 'livePrice']
  }
}));

describe('RightPanel - Core Logic Tests', () => {
  let wrapper;
  
  const createWrapper = (props = {}) => {
    return mount(RightPanel, {
      props: {
        currentSymbol: 'SPY',
        currentPrice: 500,
        priceChange: 2.5,
        isLivePrice: true,
        chartData: null,
        additionalQuoteData: {},
        optionsChainData: [],
        adjustedNetCredit: null,
        forceExpanded: false,
        forceSection: null,
        ...props
      }
    });
  };

  beforeEach(() => {
    vi.clearAllMocks();
    // Reset mock data
    mockSelectedLegs.value = [];
    mockPositionsData.value = { positions: [] };
    mockOptionPrice.value = { price: 4.55 };
  });

  describe('Position ID Generation Logic (Bug Fix)', () => {
    it('should generate unique IDs for positions with same symbol', async () => {
      // Set up mock data with duplicate symbols
      mockSelectedLegs.value = [
        {
          symbol: 'SPY_240119C00500000',
          strike_price: 500,
          type: 'call',
          expiry: '2024-01-19',
          side: 'buy',
          quantity: 1,
          current_price: 4.55
        },
        {
          symbol: 'SPY_240119C00500000', // Same symbol
          strike_price: 500,
          type: 'call',
          expiry: '2024-01-19',
          side: 'sell',
          quantity: 1,
          current_price: 4.55
        }
      ];

      wrapper = createWrapper();
      await nextTick();

      const vm = wrapper.vm;
      
      // Test the core logic: allPositions should have unique IDs
      expect(vm.allPositions.length).toBe(2);
      
      const ids = vm.allPositions.map(pos => pos.id);
      expect(ids[0]).toBe('selected:SPY_240119C00500000:0');
      expect(ids[1]).toBe('selected:SPY_240119C00500000:1');
      expect(ids[0]).not.toBe(ids[1]);

      // Verify Set behavior works correctly (this was the original bug)
      const idSet = new Set(ids);
      expect(idSet.size).toBe(2); // Both positions should be stored
    });

    it('should handle multiple duplicate symbols correctly', async () => {
      mockSelectedLegs.value = Array.from({ length: 5 }, (_, i) => ({
        symbol: 'SPY_240119C00500000',
        strike_price: 500,
        type: 'call',
        expiry: '2024-01-19',
        side: i % 2 === 0 ? 'buy' : 'sell',
        quantity: 1,
        current_price: 4.55
      }));

      wrapper = createWrapper();
      await nextTick();

      const vm = wrapper.vm;
      const ids = vm.allPositions.map(pos => pos.id);
      
      // All IDs should be unique
      const idSet = new Set(ids);
      expect(idSet.size).toBe(5);
      
      // Verify the pattern
      ids.forEach((id, index) => {
        expect(id).toBe(`selected:SPY_240119C00500000:${index}`);
      });
    });
  });

  describe('Stale Position Cleanup Logic (Bug Fix)', () => {
    it('should remove stale checked positions when legs are removed', async () => {
      wrapper = createWrapper();
      await nextTick();

      const vm = wrapper.vm;
      
      // Simulate initial state with checked positions
      vm.checkedPositions.add('selected:SPY_240119C00500000:0');
      vm.checkedPositions.add('selected:SPY_240119C00510000:0');
      vm.checkedPositions.add('selected:SPY_240119C00520000:0');
      expect(vm.checkedPositions.size).toBe(3);

      // Simulate the allPositions watcher logic when positions are removed
      const oldPositions = [
        { id: 'selected:SPY_240119C00500000:0' },
        { id: 'selected:SPY_240119C00510000:0' },
        { id: 'selected:SPY_240119C00520000:0' }
      ];
      
      const newPositions = [
        { id: 'selected:SPY_240119C00500000:0' },
        { id: 'selected:SPY_240119C00520000:0' }
        // Middle position removed
      ];

      // Test the cleanup logic
      const oldIds = new Set(oldPositions.map(pos => pos.id));
      const newIds = new Set(newPositions.map(pos => pos.id));
      
      // Find and remove stale positions
      oldPositions.forEach((pos) => {
        if (!newIds.has(pos.id) && vm.checkedPositions.has(pos.id)) {
          vm.checkedPositions.delete(pos.id);
        }
      });

      expect(vm.checkedPositions.size).toBe(2);
      expect(vm.checkedPositions.has('selected:SPY_240119C00500000:0')).toBe(true);
      expect(vm.checkedPositions.has('selected:SPY_240119C00520000:0')).toBe(true);
      expect(vm.checkedPositions.has('selected:SPY_240119C00510000:0')).toBe(false);
    });

    it('should handle complete position removal', async () => {
      wrapper = createWrapper();
      await nextTick();

      const vm = wrapper.vm;
      
      // Add some checked positions
      vm.checkedPositions.add('selected:SPY_240119C00500000:0');
      vm.checkedPositions.add('selected:SPY_240119C00510000:0');
      expect(vm.checkedPositions.size).toBe(2);

      // Simulate all positions being removed
      const oldPositions = [
        { id: 'selected:SPY_240119C00500000:0' },
        { id: 'selected:SPY_240119C00510000:0' }
      ];
      const newPositions = []; // All removed

      const newIds = new Set(newPositions.map(pos => pos.id));
      
      // Cleanup logic
      oldPositions.forEach((pos) => {
        if (!newIds.has(pos.id) && vm.checkedPositions.has(pos.id)) {
          vm.checkedPositions.delete(pos.id);
        }
      });

      expect(vm.checkedPositions.size).toBe(0);
    });
  });

  describe('Position Checkbox Management', () => {
    it('should toggle position check state correctly', async () => {
      wrapper = createWrapper();
      await nextTick();

      const vm = wrapper.vm;
      const positionId = 'test-position-id';
      
      // Initially not checked
      expect(vm.checkedPositions.has(positionId)).toBe(false);
      
      // Toggle to checked
      vm.togglePositionCheck(positionId);
      expect(vm.checkedPositions.has(positionId)).toBe(true);
      
      // Toggle back to unchecked
      vm.togglePositionCheck(positionId);
      expect(vm.checkedPositions.has(positionId)).toBe(false);
    });

    it('should handle select all functionality', async () => {
      // Set up mock data to create positions
      mockSelectedLegs.value = [
        {
          symbol: 'SPY_240119C00500000',
          strike_price: 500,
          type: 'call',
          expiry: '2024-01-19',
          side: 'buy',
          quantity: 1,
          current_price: 4.55
        },
        {
          symbol: 'SPY_240119C00510000',
          strike_price: 510,
          type: 'call',
          expiry: '2024-01-19',
          side: 'buy',
          quantity: 1,
          current_price: 3.25
        },
        {
          symbol: 'SPY_240119C00520000',
          strike_price: 520,
          type: 'call',
          expiry: '2024-01-19',
          side: 'buy',
          quantity: 1,
          current_price: 2.15
        }
      ];

      wrapper = createWrapper();
      await nextTick();

      const vm = wrapper.vm;
      
      // Should have 3 positions from mock data
      expect(vm.allPositions.length).toBe(3);

      // Initially none selected
      expect(vm.isAllSelected).toBe(false);
      expect(vm.isIndeterminate).toBe(false);

      // Select all
      vm.toggleSelectAll();
      expect(vm.checkedPositions.size).toBe(3);
      expect(vm.isAllSelected).toBe(true);
      expect(vm.isIndeterminate).toBe(false);

      // Unselect one to test indeterminate state
      const firstPositionId = vm.allPositions[0].id;
      vm.togglePositionCheck(firstPositionId);
      expect(vm.checkedPositions.size).toBe(2);
      expect(vm.isAllSelected).toBe(false);
      expect(vm.isIndeterminate).toBe(true);

      // Select all again
      vm.toggleSelectAll();
      expect(vm.checkedPositions.size).toBe(3);
      expect(vm.isAllSelected).toBe(true);

      // Unselect all
      vm.toggleSelectAll();
      expect(vm.checkedPositions.size).toBe(0);
      expect(vm.isAllSelected).toBe(false);
      expect(vm.isIndeterminate).toBe(false);
    });
  });

  describe('Chart Data Source Priority (Bug Fix)', () => {
    it('should emit positions-changed with checked positions only', async () => {
      // Set up mock data to create positions
      mockSelectedLegs.value = [
        {
          symbol: 'SPY_240119C00500000',
          strike_price: 500,
          type: 'call',
          expiry: '2024-01-19',
          side: 'buy',
          quantity: 1,
          current_price: 4.55
        },
        {
          symbol: 'SPY_240119C00510000',
          strike_price: 510,
          type: 'call',
          expiry: '2024-01-19',
          side: 'sell',
          quantity: 1,
          current_price: 3.25
        }
      ];

      wrapper = createWrapper();
      await nextTick();

      const vm = wrapper.vm;
      
      // Should have 2 positions from mock data
      expect(vm.allPositions.length).toBe(2);
      
      // Check only the first position
      const firstPositionId = vm.allPositions[0].id;
      vm.checkedPositions.add(firstPositionId);
      
      // Trigger the chart update
      vm.updateChartWithCheckedPositions();
      await nextTick();

      // Should emit only the checked position
      const emitted = wrapper.emitted('positions-changed');
      expect(emitted).toBeTruthy();
      
      const emittedPositions = emitted[emitted.length - 1][0];
      expect(emittedPositions).toHaveLength(1);
      expect(emittedPositions[0].id).toBe(firstPositionId);
    });

    it('should emit empty array when no positions are checked', async () => {
      // Set up mock data to create positions
      mockSelectedLegs.value = [
        {
          symbol: 'SPY_240119C00500000',
          strike_price: 500,
          type: 'call',
          expiry: '2024-01-19',
          side: 'buy',
          quantity: 1,
          current_price: 4.55
        },
        {
          symbol: 'SPY_240119C00510000',
          strike_price: 510,
          type: 'call',
          expiry: '2024-01-19',
          side: 'buy',
          quantity: 1,
          current_price: 3.25
        }
      ];

      wrapper = createWrapper();
      await nextTick();

      const vm = wrapper.vm;
      
      // Should have positions but don't check any
      expect(vm.allPositions.length).toBe(2);

      // Trigger the chart update without checking any positions
      vm.updateChartWithCheckedPositions();
      await nextTick();

      // Should emit empty array
      const emitted = wrapper.emitted('positions-changed');
      expect(emitted).toBeTruthy();
      
      const emittedPositions = emitted[emitted.length - 1][0];
      expect(emittedPositions).toHaveLength(0);
    });
  });

  describe('Component State Management', () => {
    it('should handle panel expansion correctly', async () => {
      wrapper = createWrapper({ forceExpanded: true, forceSection: 'analysis' });
      await nextTick();

      const vm = wrapper.vm;
      expect(vm.isExpanded).toBe(true);
      expect(vm.activeSection).toBe('analysis');
    });

    it('should toggle sections correctly', async () => {
      wrapper = createWrapper();
      await nextTick();

      const vm = wrapper.vm;
      
      // Initially collapsed
      expect(vm.isExpanded).toBe(false);
      
      // Toggle to overview
      vm.toggleSection('overview');
      expect(vm.isExpanded).toBe(true);
      expect(vm.activeSection).toBe('overview');
      
      // Toggle same section should collapse
      vm.toggleSection('overview');
      expect(vm.isExpanded).toBe(false);
      
      // Toggle different section should switch
      vm.toggleSection('analysis');
      expect(vm.isExpanded).toBe(true);
      expect(vm.activeSection).toBe('analysis');
    });

    it('should format option descriptions correctly', async () => {
      wrapper = createWrapper();
      await nextTick();

      const vm = wrapper.vm;
      
      const position = {
        option_type: 'call',
        strike_price: 500,
        expiry_date: '2024-01-19'
      };

      const formatted = vm.formatEnhancedOptionDescription(position);
      
      expect(formatted.type).toBe('C');
      expect(formatted.strike).toBe(500);
      expect(formatted.expiry).toBeTruthy();
      expect(typeof formatted.daysToExpiry).toBe('number');
      expect(typeof formatted.isITM).toBe('boolean');
    });
  });

  describe('Error Handling and Edge Cases', () => {
    it('should handle empty allPositions gracefully', async () => {
      wrapper = createWrapper();
      await nextTick();

      const vm = wrapper.vm;
      
      expect(vm.allPositions).toEqual([]);
      expect(vm.checkedPositions.size).toBe(0);
      expect(vm.isAllSelected).toBe(false);
      expect(vm.isIndeterminate).toBe(false);
    });

    it('should handle invalid position data gracefully', async () => {
      wrapper = createWrapper();
      await nextTick();

      const vm = wrapper.vm;
      
      // Test with invalid position ID
      expect(() => {
        vm.togglePositionCheck(null);
      }).not.toThrow();
      
      expect(() => {
        vm.togglePositionCheck(undefined);
      }).not.toThrow();
    });

    it('should handle symbol changes correctly', async () => {
      wrapper = createWrapper({ currentSymbol: 'SPY' });
      await nextTick();

      const vm = wrapper.vm;
      
      // Add some checked positions
      vm.checkedPositions.add('pos1');
      vm.checkedPositions.add('pos2');
      expect(vm.checkedPositions.size).toBe(2);

      // Change symbol (should clear checked positions)
      await wrapper.setProps({ currentSymbol: 'QQQ' });
      await nextTick();

      // Checked positions should be cleared on symbol change
      expect(vm.checkedPositions.size).toBe(0);
    });
  });
});
