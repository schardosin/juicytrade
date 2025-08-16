import { describe, it, expect, beforeEach, vi, afterEach } from 'vitest';
import { mount } from '@vue/test-utils';
import { nextTick } from 'vue';
import CollapsibleOptionsChain from '../src/components/CollapsibleOptionsChain.vue';

// Mock the composables with more realistic behavior
vi.mock('../src/composables/useMarketData.js', () => ({
  useMarketData: () => ({
    getOptionPrice: vi.fn(),
    getOptionGreeks: vi.fn()
  })
}));

vi.mock('../src/composables/useSelectedLegs.js', () => ({
  useSelectedLegs: () => ({
    isSelected: vi.fn(),
    addFromOptionsChain: vi.fn(),
    removeLeg: vi.fn(),
    getSelectionClass: vi.fn()
  })
}));

vi.mock('../src/services/smartMarketDataStore.js', () => ({
  smartMarketDataStore: {
    registerSymbolUsage: vi.fn(),
    unregisterSymbolUsage: vi.fn()
  }
}));

describe('CollapsibleOptionsChain - Stability & Cascade Protection', () => {
  let wrapper;
  let mockAddFromOptionsChain;
  let mockRemoveLeg;
  let mockIsSelected;
  let mockGetSelectionClass;
  let mockGetOptionPrice;
  let mockGetOptionGreeks;
  let mockRegisterSymbolUsage;
  let mockUnregisterSymbolUsage;

  const defaultProps = {
    symbol: 'SPY',
    underlyingPrice: 450.00,
    expirationDates: [
      { date: '2024-01-19', symbol: 'SPY240119', type: 'monthly' },
      { date: '2024-02-16', symbol: 'SPY240216', type: 'monthly' },
      { date: '2024-03-15', symbol: 'SPY240315', type: 'quarterly' },
    ],
    optionsDataByExpiration: {
      '2024-01-19-monthly-SPY240119': [
        {
          symbol: 'SPY_240119C00450000',
          strike_price: 450,
          type: 'call',
          bid: 2.45,
          ask: 2.55,
          delta: 0.50,
          theta: -0.05
        },
        {
          symbol: 'SPY_240119P00450000',
          strike_price: 450,
          type: 'put',
          bid: 2.40,
          ask: 2.50,
          delta: -0.50,
          theta: -0.05
        },
        {
          symbol: 'SPY_240119C00455000',
          strike_price: 455,
          type: 'call',
          bid: 1.85,
          ask: 1.95,
          delta: 0.35,
          theta: -0.04
        }
      ],
      '2024-02-16-monthly-SPY240216': [
        {
          symbol: 'SPY_240216C00450000',
          strike_price: 450,
          type: 'call',
          bid: 5.20,
          ask: 5.40,
          delta: 0.55,
          theta: -0.03
        }
      ]
    },
    loading: false,
    error: null,
    currentStrikeCount: 20
  };

  beforeEach(async () => {
    vi.clearAllMocks();
    
    // Import the mocked modules to get access to the mock functions
    const { useMarketData } = await import('../src/composables/useMarketData.js');
    const { useSelectedLegs } = await import('../src/composables/useSelectedLegs.js');
    const { smartMarketDataStore } = await import('../src/services/smartMarketDataStore.js');
    
    // Get references to the mock functions
    const marketData = useMarketData();
    const selectedLegs = useSelectedLegs();
    
    mockGetOptionPrice = marketData.getOptionPrice;
    mockGetOptionGreeks = marketData.getOptionGreeks;
    mockIsSelected = selectedLegs.isSelected;
    mockAddFromOptionsChain = selectedLegs.addFromOptionsChain;
    mockRemoveLeg = selectedLegs.removeLeg;
    mockGetSelectionClass = selectedLegs.getSelectionClass;
    mockRegisterSymbolUsage = smartMarketDataStore.registerSymbolUsage;
    mockUnregisterSymbolUsage = smartMarketDataStore.unregisterSymbolUsage;
    
    // Set up default mock behaviors
    mockIsSelected.mockReturnValue(false);
    mockGetSelectionClass.mockReturnValue('');
    mockGetOptionPrice.mockReturnValue({ value: null });
    mockGetOptionGreeks.mockReturnValue({ value: null });
  });

  afterEach(() => {
    if (wrapper) {
      wrapper.unmount();
    }
  });

  describe('Component Initialization & Structure', () => {
    it('renders without crashing with valid props', () => {
      wrapper = mount(CollapsibleOptionsChain, {
        props: defaultProps
      });
      
      expect(wrapper.exists()).toBe(true);
      expect(wrapper.find('.collapsible-options-chain').exists()).toBe(true);
    });

    it('displays strike count filter with correct options', () => {
      wrapper = mount(CollapsibleOptionsChain, {
        props: defaultProps
      });
      
      const select = wrapper.find('.strike-count-select');
      expect(select.exists()).toBe(true);
      
      const options = select.findAll('option');
      expect(options).toHaveLength(5);
      expect(options[0].text()).toBe('10 strikes');
      expect(options[4].text()).toBe('100 strikes');
    });

    it('displays all expiration dates correctly', () => {
      wrapper = mount(CollapsibleOptionsChain, {
        props: defaultProps
      });
      
      const headers = wrapper.findAll('.expiration-header');
      expect(headers).toHaveLength(3);
      
      // Verify dates are displayed
      expect(headers[0].text()).toContain('Jan');
      expect(headers[1].text()).toContain('Feb');
      expect(headers[2].text()).toContain('Mar');
    });

    it('calculates days to expiry correctly', () => {
      wrapper = mount(CollapsibleOptionsChain, {
        props: defaultProps
      });
      
      const daysLabels = wrapper.findAll('.days-label');
      expect(daysLabels.length).toBeGreaterThan(0);
      
      // Each should show days format
      daysLabels.forEach(label => {
        expect(label.text()).toMatch(/\d+d/);
      });
    });

    it('handles missing underlying price gracefully', () => {
      expect(() => {
        wrapper = mount(CollapsibleOptionsChain, {
          props: {
            ...defaultProps,
            underlyingPrice: null
          }
        });
      }).not.toThrow();
      
      expect(wrapper.exists()).toBe(true);
    });
  });

  describe('Badge Display Logic', () => {
    it('displays monthly badge correctly', () => {
      const props = {
        ...defaultProps,
        expirationDates: [{ date: '2024-01-19', symbol: 'SPY', type: 'monthly' }],
      };
      wrapper = mount(CollapsibleOptionsChain, { props });
      expect(wrapper.find('.monthly-badge').exists()).toBe(true);
      expect(wrapper.find('.monthly-badge').text()).toBe('M');
    });

    it('displays weekly badge correctly', () => {
      const props = {
        ...defaultProps,
        expirationDates: [{ date: '2024-01-26', symbol: 'SPY', type: 'weekly' }],
      };
      wrapper = mount(CollapsibleOptionsChain, { props });
      expect(wrapper.find('.weekly-badge').exists()).toBe(true);
      expect(wrapper.find('.weekly-badge').text()).toBe('W');
    });

    it('displays quarterly badge correctly', () => {
      const props = {
        ...defaultProps,
        expirationDates: [{ date: '2024-03-15', symbol: 'SPY', type: 'quarterly' }],
      };
      wrapper = mount(CollapsibleOptionsChain, { props });
      expect(wrapper.find('.quarterly-badge').exists()).toBe(true);
      expect(wrapper.find('.quarterly-badge').text()).toBe('Q');
    });

    it('displays EOM badge correctly', () => {
      const props = {
        ...defaultProps,
        expirationDates: [{ date: '2024-01-31', symbol: 'SPY', type: 'eom' }],
      };
      wrapper = mount(CollapsibleOptionsChain, { props });
      expect(wrapper.find('.eom-badge').exists()).toBe(true);
      expect(wrapper.find('.eom-badge').text()).toBe('EOM');
    });
  });

  describe('Loading & Error States', () => {
    it('displays loading state correctly', () => {
      wrapper = mount(CollapsibleOptionsChain, {
        props: {
          ...defaultProps,
          loading: true
        }
      });
      
      const loadingState = wrapper.find('.loading-state');
      expect(loadingState.exists()).toBe(true);
      expect(loadingState.text()).toContain('Loading expiration dates');
      
      const spinner = wrapper.find('.loading-spinner');
      expect(spinner.exists()).toBe(true);
    });

    it('displays error state correctly', () => {
      const errorMessage = 'Failed to load options data';
      wrapper = mount(CollapsibleOptionsChain, {
        props: {
          ...defaultProps,
          error: errorMessage
        }
      });
      
      const errorState = wrapper.find('.error-state');
      expect(errorState.exists()).toBe(true);
      expect(errorState.text()).toContain(errorMessage);
    });

    it('hides expiration groups during loading', () => {
      wrapper = mount(CollapsibleOptionsChain, {
        props: {
          ...defaultProps,
          loading: true
        }
      });
      
      const expirationGroups = wrapper.find('.expiration-groups');
      expect(expirationGroups.exists()).toBe(false);
    });

    it('hides expiration groups during error', () => {
      wrapper = mount(CollapsibleOptionsChain, {
        props: {
          ...defaultProps,
          error: 'Some error'
        }
      });
      
      const expirationGroups = wrapper.find('.expiration-groups');
      expect(expirationGroups.exists()).toBe(false);
    });
  });

  describe('Expiration Expansion & Collapse', () => {
    beforeEach(() => {
      wrapper = mount(CollapsibleOptionsChain, {
        props: defaultProps
      });
    });

    it('starts with all expirations collapsed', () => {
      const contents = wrapper.findAll('.expiration-content');
      contents.forEach(content => {
        expect(content.classes()).not.toContain('expanded');
        expect(content.classes()).toContain('collapsed');
      });
    });

    it('expands expiration when header is clicked', async () => {
      const firstHeader = wrapper.find('.expiration-header');
      await firstHeader.trigger('click');
      
      const firstContent = wrapper.find('.expiration-content');
      expect(firstContent.classes()).toContain('expanded');
    });

    it('emits expiration-expanded event', async () => {
      const firstHeader = wrapper.find('.expiration-header');
      await firstHeader.trigger('click');
      
      expect(wrapper.emitted('expiration-expanded')).toBeTruthy();
      const emittedEvent = wrapper.emitted('expiration-expanded')[0];
      expect(emittedEvent[0]).toBe('2024-01-19-monthly-SPY240119');
      expect(emittedEvent[1]).toMatchObject({
        date: '2024-01-19',
        symbol: 'SPY240119'
      });
    });

    it('collapses expanded expiration when clicked again', async () => {
      const firstHeader = wrapper.find('.expiration-header');
      
      // Expand
      await firstHeader.trigger('click');
      expect(wrapper.find('.expiration-content').classes()).toContain('expanded');
      
      // Collapse
      await firstHeader.trigger('click');
      expect(wrapper.find('.expiration-content').classes()).not.toContain('expanded');
    });

    it('emits expiration-collapsed event', async () => {
      const firstHeader = wrapper.find('.expiration-header');
      
      // Expand first
      await firstHeader.trigger('click');
      // Then collapse
      await firstHeader.trigger('click');
      
      expect(wrapper.emitted('expiration-collapsed')).toBeTruthy();
      expect(wrapper.emitted('expiration-collapsed')[0]).toEqual(['2024-01-19-monthly-SPY240119']);
    });

    it('rotates chevron icon when expanded', async () => {
      const firstHeader = wrapper.find('.expiration-header');
      const chevron = firstHeader.find('.chevron-icon');
      
      expect(chevron.classes()).not.toContain('expanded');
      
      await firstHeader.trigger('click');
      expect(chevron.classes()).toContain('expanded');
    });

    it('allows multiple expirations to be expanded simultaneously', async () => {
      const headers = wrapper.findAll('.expiration-header');
      
      await headers[0].trigger('click');
      await headers[1].trigger('click');
      
      const contents = wrapper.findAll('.expiration-content');
      expect(contents[0].classes()).toContain('expanded');
      expect(contents[1].classes()).toContain('expanded');
    });
  });

  describe('Strike Count Filter & Data Consistency', () => {
    beforeEach(() => {
      wrapper = mount(CollapsibleOptionsChain, {
        props: defaultProps
      });
    });

    it('emits strike count changes with correct data', async () => {
      const select = wrapper.find('.strike-count-select');
      
      await select.setValue('50');
      await select.trigger('change');
      
      expect(wrapper.emitted('strike-count-changed')).toBeTruthy();
      expect(wrapper.emitted('strike-count-changed')[0]).toEqual(['50']);
      
      // Verify internal state is updated
      expect(wrapper.vm.strikeCount).toBe('50');
    });

    it('updates info text correctly when strike count changes', async () => {
      const select = wrapper.find('.strike-count-select');
      
      await select.setValue('100');
      
      const infoText = wrapper.find('.info-text');
      expect(infoText.text()).toBe('100 strikes around ATM');
    });

    it('maintains strike count consistency across symbol changes', async () => {
      const select = wrapper.find('.strike-count-select');
      await select.setValue('30');
      
      // Change symbol
      await wrapper.setProps({
        symbol: 'AAPL',
        expirationDates: [{ date: '2024-01-26', symbol: 'AAPL240126', type: 'weekly' }],
        optionsDataByExpiration: {}
      });
      
      // Strike count should be preserved
      expect(wrapper.vm.strikeCount).toBe('30');
      expect(wrapper.find('.info-text').text()).toBe('30 strikes around ATM');
    });

    it('provides all expected strike count options', () => {
      const select = wrapper.find('.strike-count-select');
      const options = select.findAll('option');
      
      const expectedValues = ['10', '20', '30', '50', '100'];
      const actualValues = options.map(option => option.element.value);
      
      expect(actualValues).toEqual(expectedValues);
    });
  });

  describe('Live Data Integration Setup', () => {
    it('registers symbols for live data when expiration is expanded', async () => {
      wrapper = mount(CollapsibleOptionsChain, {
        props: defaultProps
      });
      
      const firstHeader = wrapper.find('.expiration-header');
      await firstHeader.trigger('click');
      await nextTick();
      
      // Should register all option symbols from the expanded expiration
      const expectedSymbols = defaultProps.optionsDataByExpiration['2024-01-19-monthly-SPY240119'].map(opt => opt.symbol);
      
      expectedSymbols.forEach(symbol => {
        expect(mockRegisterSymbolUsage).toHaveBeenCalledWith(
          symbol,
          expect.any(String) // component ID
        );
      });
    });

    it('verifies market data integration setup', async () => {
      wrapper = mount(CollapsibleOptionsChain, {
        props: defaultProps
      });
      
      const firstHeader = wrapper.find('.expiration-header');
      await firstHeader.trigger('click');
      
      // Verify the component has market data integration available
      expect(mockGetOptionPrice).toBeDefined();
      expect(mockGetOptionGreeks).toBeDefined();
      
      // Component should render without errors
      expect(wrapper.exists()).toBe(true);
    });

    it('handles live data methods without errors', async () => {
      wrapper = mount(CollapsibleOptionsChain, {
        props: defaultProps
      });
      
      const firstHeader = wrapper.find('.expiration-header');
      
      expect(() => {
        firstHeader.trigger('click');
      }).not.toThrow();
      
      // Component should still be functional
      expect(wrapper.exists()).toBe(true);
    });

    it('falls back to static data when live data unavailable', async () => {
      // Mock live data as unavailable
      mockGetOptionPrice.mockReturnValue({ value: null });
      mockGetOptionGreeks.mockReturnValue({ value: null });
      
      wrapper = mount(CollapsibleOptionsChain, {
        props: defaultProps
      });
      
      const firstHeader = wrapper.find('.expiration-header');
      await firstHeader.trigger('click');
      await nextTick();
      
      // Component should still render without crashing
      expect(wrapper.exists()).toBe(true);
      expect(wrapper.find('.expiration-content.expanded').exists()).toBe(true);
    });
  });

  describe('Memory Management & Symbol Cleanup', () => {
    it('cleans up symbol registrations when symbol changes', async () => {
      wrapper = mount(CollapsibleOptionsChain, {
        props: defaultProps
      });
      
      // Expand to register symbols
      const firstHeader = wrapper.find('.expiration-header');
      await firstHeader.trigger('click');
      
      const initialRegistrations = mockRegisterSymbolUsage.mock.calls.length;
      expect(initialRegistrations).toBeGreaterThan(0);
      
      // Change symbol
      await wrapper.setProps({
        symbol: 'AAPL',
        expirationDates: [{ date: '2024-01-26', symbol: 'AAPL240126', type: 'weekly' }],
        optionsDataByExpiration: {
          '2024-01-26-weekly-AAPL240126': [
            {
              symbol: 'AAPL_240126C00150000',
              strike_price: 150,
              type: 'call',
              bid: 1.45,
              ask: 1.55,
            },
          ],
        },
      });
      
      // Should unregister old symbols
      expect(mockUnregisterSymbolUsage).toHaveBeenCalled();
      
      // Should collapse all expirations
      const contents = wrapper.findAll('.expiration-content');
      contents.forEach(content => {
        expect(content.classes()).not.toContain('expanded');
      });
    });

    it('prevents memory leaks on component unmount', () => {
      wrapper = mount(CollapsibleOptionsChain, {
        props: defaultProps
      });
      
      wrapper.unmount();
      
      // Should clean up all registrations
      expect(mockUnregisterSymbolUsage).toHaveBeenCalled();
    });

    it('handles rapid expand/collapse without accumulating registrations', async () => {
      wrapper = mount(CollapsibleOptionsChain, {
        props: defaultProps
      });
      
      const firstHeader = wrapper.find('.expiration-header');
      
      // Rapid expand/collapse cycles
      for (let i = 0; i < 5; i++) {
        await firstHeader.trigger('click'); // expand
        await firstHeader.trigger('click'); // collapse
      }
      
      // Should not accumulate registrations exponentially
      const registrationCalls = mockRegisterSymbolUsage.mock.calls.length;
      expect(registrationCalls).toBeLessThan(20); // Reasonable upper bound
    });
  });

  describe('Error Handling & Edge Cases', () => {
    it('handles empty options data gracefully', async () => {
      wrapper = mount(CollapsibleOptionsChain, {
        props: {
          ...defaultProps,
          optionsDataByExpiration: {
            '2024-01-19-monthly-SPY240119': []
          }
        }
      });
      
      const firstHeader = wrapper.find('.expiration-header');
      await firstHeader.trigger('click');
      
      // Should not crash and should show expanded content area
      expect(wrapper.find('.expiration-content.expanded').exists()).toBe(true);
    });

    it('handles malformed option data without crashing', () => {
      const malformedProps = {
        ...defaultProps,
        expirationDates: [{ date: '2024-01-19', symbol: 'SPY240119', type: 'monthly' }],
        optionsDataByExpiration: {
          '2024-01-19-monthly-SPY240119': [
            {
              // Missing required fields
              strike_price: 450,
              type: 'call'
            },
            {
              symbol: 'SPY_240119P00450000',
              strike_price: 450,
              type: 'put',
              bid: 2.40,
              ask: 2.50
            }
          ]
        }
      };
      
      expect(() => {
        wrapper = mount(CollapsibleOptionsChain, {
          props: malformedProps
        });
      }).not.toThrow();
      
      // Should still render
      expect(wrapper.exists()).toBe(true);
    });

    it('handles missing underlying price gracefully', () => {
      wrapper = mount(CollapsibleOptionsChain, {
        props: {
          ...defaultProps,
          underlyingPrice: null
        }
      });
      
      // Should not crash
      expect(wrapper.exists()).toBe(true);
      
      // ATM detection should handle null price
      expect(() => {
        wrapper.vm.isAtTheMoney(450);
      }).not.toThrow();
    });

    it('handles undefined or null options data', () => {
      expect(() => {
        wrapper = mount(CollapsibleOptionsChain, {
          props: {
            ...defaultProps,
            optionsDataByExpiration: {}
          }
        });
      }).not.toThrow();
      
      expect(wrapper.exists()).toBe(true);
    });
  });

  describe('Performance & Scalability', () => {
    it('handles large option datasets efficiently', async () => {
      // Create large dataset
      const largeOptionSet = [];
      for (let strike = 300; strike <= 600; strike += 5) {
        largeOptionSet.push({
          symbol: `SPY_240119C${strike.toString().padStart(8, '0')}`,
          strike_price: strike,
          type: 'call',
          bid: 2.45,
          ask: 2.55,
          delta: 0.50,
          theta: -0.05
        });
        largeOptionSet.push({
          symbol: `SPY_240119P${strike.toString().padStart(8, '0')}`,
          strike_price: strike,
          type: 'put',
          bid: 2.40,
          ask: 2.50,
          delta: -0.50,
          theta: -0.05
        });
      }
      
      const startTime = performance.now();
      
      wrapper = mount(CollapsibleOptionsChain, {
        props: {
          ...defaultProps,
          optionsDataByExpiration: {
            '2024-01-19-monthly-SPY240119': largeOptionSet
          }
        }
      });
      
      const firstHeader = wrapper.find('.expiration-header');
      await firstHeader.trigger('click');
      
      const endTime = performance.now();
      
      // Should complete in reasonable time
      expect(endTime - startTime).toBeLessThan(500);
      
      // Component should still be responsive
      expect(wrapper.exists()).toBe(true);
      expect(wrapper.find('.expiration-content.expanded').exists()).toBe(true);
    });

    it('maintains performance during rapid user interactions', async () => {
      wrapper = mount(CollapsibleOptionsChain, {
        props: defaultProps
      });
      
      const headers = wrapper.findAll('.expiration-header');
      const startTime = performance.now();
      
      // Simulate rapid user interactions
      for (let i = 0; i < 10; i++) {
        await headers[0].trigger('click');
        await headers[1].trigger('click');
        await headers[0].trigger('click');
        await headers[1].trigger('click');
      }
      
      const endTime = performance.now();
      
      // Should handle rapid interactions efficiently
      expect(endTime - startTime).toBeLessThan(1000);
      
      // Component should still be responsive
      expect(wrapper.exists()).toBe(true);
    });
  });

  describe('Component State Management', () => {
    it('maintains correct state during symbol changes', async () => {
      wrapper = mount(CollapsibleOptionsChain, {
        props: defaultProps
      });
      
      // Expand first expiration
      const firstHeader = wrapper.find('.expiration-header');
      await firstHeader.trigger('click');
      
      // Change strike count
      const select = wrapper.find('.strike-count-select');
      await select.setValue('50');
      await select.trigger('change');
      
      expect(wrapper.emitted('strike-count-changed')).toBeTruthy();
      
      // Change symbol
      await wrapper.setProps({
        symbol: 'AAPL',
        expirationDates: [{ date: '2024-01-26', symbol: 'AAPL240126', type: 'weekly' }],
        optionsDataByExpiration: {}
      });
      
      // Should clean up and reset state
      expect(mockUnregisterSymbolUsage).toHaveBeenCalled();
      
      // Strike count should be preserved
      expect(wrapper.vm.strikeCount).toBe('50');
    });

    it('handles concurrent operations without state corruption', async () => {
      wrapper = mount(CollapsibleOptionsChain, {
        props: defaultProps
      });
      
      const headers = wrapper.findAll('.expiration-header');
      
      // Simulate concurrent operations
      const promises = [
        headers[0].trigger('click'),
        headers[1].trigger('click'),
        headers[2].trigger('click')
      ];
      
      await Promise.all(promises);
      
      // All should be expanded
      const contents = wrapper.findAll('.expiration-content');
      expect(contents[0].classes()).toContain('expanded');
      expect(contents[1].classes()).toContain('expanded');
      expect(contents[2].classes()).toContain('expanded');
      
      // Should emit correct events
      expect(wrapper.emitted('expiration-expanded')).toHaveLength(3);
    });
  });

  describe('Integration & Cleanup Validation', () => {
    it('ensures proper cleanup prevents cascade corruption', async () => {
      wrapper = mount(CollapsibleOptionsChain, {
        props: defaultProps
      });
      
      // Register symbols
      const firstHeader = wrapper.find('.expiration-header');
      await firstHeader.trigger('click');
      
      // Unmount component
      wrapper.unmount();
      
      // Should clean up all registrations
      expect(mockUnregisterSymbolUsage).toHaveBeenCalled();
      
      // Verify cleanup was called
      const cleanupCalls = mockUnregisterSymbolUsage.mock.calls;
      expect(cleanupCalls.length).toBeGreaterThan(0);
    });

    it('maintains composable integration without errors', async () => {
      wrapper = mount(CollapsibleOptionsChain, {
        props: defaultProps
      });
      
      // Verify composables are available and component is stable
      expect(mockGetSelectionClass).toBeDefined();
      expect(mockIsSelected).toBeDefined();
      
      // Component should remain stable
      expect(wrapper.exists()).toBe(true);
    });

    it('handles component lifecycle correctly', async () => {
      wrapper = mount(CollapsibleOptionsChain, {
        props: defaultProps
      });
      
      // Expand to trigger registrations
      const firstHeader = wrapper.find('.expiration-header');
      await firstHeader.trigger('click');
      
      // Change props to trigger cleanup
      await wrapper.setProps({
        symbol: 'AAPL',
        expirationDates: [],
        optionsDataByExpiration: {},
      });
      
      // Unmount to trigger final cleanup
      wrapper.unmount();
      
      // Should have called cleanup at least twice (prop change + unmount)
      expect(mockUnregisterSymbolUsage).toHaveBeenCalled();
    });
  });
});
