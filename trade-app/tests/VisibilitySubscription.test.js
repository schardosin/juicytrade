import { describe, it, expect, beforeEach, vi, afterEach } from 'vitest';
import { mount } from '@vue/test-utils';
import { nextTick } from 'vue';
import CollapsibleOptionsChain from '../src/components/CollapsibleOptionsChain.vue';
import { useOptionsChainManager } from '../src/composables/useOptionsChainManager.js';

// Mock IntersectionObserver
const mockObserve = vi.fn();
const mockDisconnect = vi.fn();
window.IntersectionObserver = vi.fn().mockImplementation((callback) => ({
  observe: mockObserve,
  disconnect: mockDisconnect,
  trigger: (entries) => callback(entries)
}));

// Mock composables
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

import { smartMarketDataStore } from '../src/services/smartMarketDataStore.js';

describe('Visibility-Based Subscription', () => {
  let wrapper;
  
  const defaultProps = {
    symbol: 'SPY',
    underlyingPrice: 450.00,
    expirationDates: [
      { date: '2024-01-19', symbol: 'SPY240119', type: 'monthly' }
    ],
    optionsDataByExpiration: {
      '2024-01-19-monthly-SPY240119': [
        {
          symbol: 'SPY_240119C00450000',
          strike_price: 450,
          type: 'call',
          bid: 2.45,
          ask: 2.55
        },
        {
          symbol: 'SPY_240119P00450000',
          strike_price: 450,
          type: 'put',
          bid: 2.40,
          ask: 2.50
        }
      ]
    },
    expandedExpirations: new Set(['2024-01-19-monthly-SPY240119'])
  };

  beforeEach(() => {
    vi.clearAllMocks();
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
    if (wrapper) wrapper.unmount();
  });

  it('initializes IntersectionObserver on mount', () => {
    wrapper = mount(CollapsibleOptionsChain, {
      props: defaultProps
    });
    
    expect(window.IntersectionObserver).toHaveBeenCalled();
  });

  it('observes option rows', async () => {
    wrapper = mount(CollapsibleOptionsChain, {
      props: defaultProps
    });
    
    // Wait for DOM update and observation
    await nextTick();
    vi.advanceTimersByTime(100); // Wait for observation timeout
    
    expect(mockObserve).toHaveBeenCalled();
  });

  it('emits visible-symbols-changed when elements intersect', async () => {
    wrapper = mount(CollapsibleOptionsChain, {
      props: defaultProps
    });
    
    // Get the observer instance
    const observerInstance = window.IntersectionObserver.mock.results[0].value;
    
    // Simulate intersection with data-symbols
    observerInstance.trigger([
      {
        isIntersecting: true,
        target: { dataset: { symbols: 'SPY_240119C00450000,SPY_240119P00450000' } }
      }
    ]);
    
    // Wait for debounce
    vi.advanceTimersByTime(200);
    
    expect(wrapper.emitted('visible-symbols-changed')).toBeTruthy();
    expect(wrapper.emitted('visible-symbols-changed')[0][0]).toContain('SPY_240119C00450000');
    expect(wrapper.emitted('visible-symbols-changed')[0][0]).toContain('SPY_240119P00450000');
  });

  it('updates visible symbols when scrolling (new intersection)', async () => {
    wrapper = mount(CollapsibleOptionsChain, {
      props: defaultProps
    });
    
    const observerInstance = window.IntersectionObserver.mock.results[0].value;
    
    // First intersection
    observerInstance.trigger([
      {
        isIntersecting: true,
        target: { dataset: { symbols: 'SPY_240119C00450000,SPY_240119P00450000' } }
      }
    ]);
    
    vi.advanceTimersByTime(200);
    
    // Second intersection (scroll)
    observerInstance.trigger([
      {
        isIntersecting: true,
        target: { dataset: { symbols: 'SPY_240119C00455000,SPY_240119P00455000' } }
      }
    ]);
    
    vi.advanceTimersByTime(200);
    
    // Should emit with all symbols
    const events = wrapper.emitted('visible-symbols-changed');
    expect(events.length).toBe(2);
    expect(events[1][0]).toContain('SPY_240119C00450000');
    expect(events[1][0]).toContain('SPY_240119P00450000');
    expect(events[1][0]).toContain('SPY_240119C00455000');
    expect(events[1][0]).toContain('SPY_240119P00455000');
  });

  it('removes symbols when they become invisible', async () => {
    wrapper = mount(CollapsibleOptionsChain, {
      props: defaultProps
    });
    
    const observerInstance = window.IntersectionObserver.mock.results[0].value;
    
    // Intersect
    observerInstance.trigger([
      {
        isIntersecting: true,
        target: { dataset: { symbols: 'SPY_240119C00450000,SPY_240119P00450000' } }
      }
    ]);
    
    vi.advanceTimersByTime(200);
    
    // Un-intersect
    observerInstance.trigger([
      {
        isIntersecting: false,
        target: { dataset: { symbols: 'SPY_240119C00450000,SPY_240119P00450000' } }
      }
    ]);
    
    vi.advanceTimersByTime(200);
    
    const events = wrapper.emitted('visible-symbols-changed');
    expect(events.length).toBe(2);
    expect(events[1][0]).toEqual([]);
  });

  it('cleans up visible symbols when expiration is collapsed', async () => {
    wrapper = mount(CollapsibleOptionsChain, {
      props: defaultProps
    });
    
    const observerInstance = window.IntersectionObserver.mock.results[0].value;
    
    // Intersect
    observerInstance.trigger([
      {
        isIntersecting: true,
        target: { dataset: { symbols: 'SPY_240119C00450000,SPY_240119P00450000' } }
      }
    ]);
    
    vi.advanceTimersByTime(200);
    
    // Collapse expiration
    const header = wrapper.find('.expiration-header');
    await header.trigger('click'); // Collapse
    
    vi.advanceTimersByTime(200);
    
    const events = wrapper.emitted('visible-symbols-changed');
    // Last event should have empty array or at least not contain the collapsed symbol
    const lastEvent = events[events.length - 1];
    expect(lastEvent[0]).not.toContain('SPY_240119C00450000');
  });

  it('does not register symbols before they are visible', async () => {
    wrapper = mount(CollapsibleOptionsChain, {
      props: defaultProps
    });
    
    // Wait for mount
    await nextTick();
    
    // Check that smartMarketDataStore.registerSymbolUsage was NOT called yet
    const registerCalls = smartMarketDataStore.registerSymbolUsage.mock.calls;
    const optionRegisterCalls = registerCalls.filter(call => call[0].includes('SPY_240119'));
    
    expect(optionRegisterCalls.length).toBe(0);
    
    // Now make it visible
    const observerInstance = window.IntersectionObserver.mock.results[0].value;
    observerInstance.trigger([
      {
        isIntersecting: true,
        target: { dataset: { symbols: 'SPY_240119C00450000,SPY_240119P00450000' } }
      }
    ]);
    
    vi.advanceTimersByTime(200);
    
    // Now it should be registered
    expect(smartMarketDataStore.registerSymbolUsage).toHaveBeenCalledWith(
      'SPY_240119C00450000', 
      expect.stringContaining('CollapsibleOptionsChain')
    );
    expect(smartMarketDataStore.registerSymbolUsage).toHaveBeenCalledWith(
      'SPY_240119P00450000', 
      expect.stringContaining('CollapsibleOptionsChain')
    );
  });
});
