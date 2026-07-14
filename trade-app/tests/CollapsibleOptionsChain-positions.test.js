import { describe, it, expect, beforeEach, vi, afterEach } from 'vitest';
import { mount } from '@vue/test-utils';
import { nextTick, ref, computed } from 'vue';
import CollapsibleOptionsChain from '../src/components/CollapsibleOptionsChain.vue';

// Mock IntersectionObserver
let intersectionObserverCallback = null;
const mockIntersectionObserver = vi.fn((callback) => {
  intersectionObserverCallback = callback;
  return {
    observe: vi.fn(),
    unobserve: vi.fn(),
    disconnect: vi.fn()
  };
});
global.IntersectionObserver = mockIntersectionObserver;

// Positions mock data (controlled per test)
let mockPositionsData = { positions: [] };

vi.mock('../src/composables/useMarketData.js', () => ({
  useMarketData: () => ({
    getOptionPrice: vi.fn(() => ({ value: null })),
    getOptionGreeks: vi.fn(() => ({ value: null })),
    getPositionsForSymbol: vi.fn(() => computed(() => mockPositionsData))
  })
}));

vi.mock('../src/composables/useSelectedLegs.js', () => ({
  useSelectedLegs: () => ({
    isSelected: vi.fn(() => false),
    addFromOptionsChain: vi.fn(),
    removeLeg: vi.fn(),
    getSelectionClass: vi.fn(() => '')
  })
}));

vi.mock('../src/composables/useMobileDetection.js', () => ({
  useMobileDetection: () => ({
    isMobile: ref(false),
    isTablet: ref(false),
    isDesktop: ref(true)
  })
}));

vi.mock('../src/services/smartMarketDataStore.js', () => ({
  smartMarketDataStore: {
    registerSymbolUsage: vi.fn(),
    unregisterSymbolUsage: vi.fn()
  }
}));

describe('CollapsibleOptionsChain - Position Badges', () => {
  let wrapper;

  const defaultProps = {
    symbol: 'SPX',
    underlyingPrice: 5500,
    expirationDates: [
      { date: '2025-07-18', symbol: 'SPX250718', type: 'monthly' }
    ],
    optionsDataByExpiration: {
      '2025-07-18-monthly-SPX250718': [
        {
          symbol: 'SPX250718C05500000',
          strike_price: 5500,
          type: 'call',
          bid: 45.00,
          ask: 46.00,
          delta: 0.50,
          theta: -0.15
        },
        {
          symbol: 'SPX250718P05500000',
          strike_price: 5500,
          type: 'put',
          bid: 44.00,
          ask: 45.00,
          delta: -0.50,
          theta: -0.15
        },
        {
          symbol: 'SPX250718C05600000',
          strike_price: 5600,
          type: 'call',
          bid: 20.00,
          ask: 21.00,
          delta: 0.35,
          theta: -0.10
        },
        {
          symbol: 'SPX250718P05600000',
          strike_price: 5600,
          type: 'put',
          bid: 80.00,
          ask: 81.00,
          delta: -0.65,
          theta: -0.10
        },
        {
          symbol: 'SPX250718C05400000',
          strike_price: 5400,
          type: 'call',
          bid: 110.00,
          ask: 111.00,
          delta: 0.65,
          theta: -0.10
        },
        {
          symbol: 'SPX250718P05400000',
          strike_price: 5400,
          type: 'put',
          bid: 15.00,
          ask: 16.00,
          delta: -0.35,
          theta: -0.10
        }
      ]
    },
    loading: false,
    error: null,
    currentStrikeCount: 50
  };

  const mountAndExpand = async (props = defaultProps) => {
    wrapper = mount(CollapsibleOptionsChain, { props });
    // Click the expiration header to expand
    const header = wrapper.find('.expiration-header');
    await header.trigger('click');
    await nextTick();
    return wrapper;
  };

  beforeEach(() => {
    vi.clearAllMocks();
    mockPositionsData = { positions: [] };
  });

  afterEach(() => {
    if (wrapper) {
      wrapper.unmount();
    }
  });

  describe('Badge Rendering', () => {
    it('shows no badges when there are no positions', async () => {
      mockPositionsData = { positions: [] };
      await mountAndExpand();

      const badges = wrapper.findAll('.position-badge');
      expect(badges).toHaveLength(0);
    });

    it('renders a green badge with +N for a long call position', async () => {
      mockPositionsData = {
        positions: [
          { symbol: 'SPX250718C05500000', qty: 2 }
        ]
      };
      await mountAndExpand();

      const badges = wrapper.findAll('.position-badge');
      expect(badges.length).toBeGreaterThanOrEqual(1);

      const longBadge = badges.find(b => b.text() === '+2');
      expect(longBadge).toBeTruthy();
      expect(longBadge.classes()).toContain('position-long');
    });

    it('renders a red badge with -N for a short put position', async () => {
      mockPositionsData = {
        positions: [
          { symbol: 'SPX250718P05500000', qty: -5 }
        ]
      };
      await mountAndExpand();

      const badges = wrapper.findAll('.position-badge');
      expect(badges.length).toBeGreaterThanOrEqual(1);

      const shortBadge = badges.find(b => b.text() === '-5');
      expect(shortBadge).toBeTruthy();
      expect(shortBadge.classes()).toContain('position-short');
    });

    it('renders badge on call side for call positions', async () => {
      mockPositionsData = {
        positions: [
          { symbol: 'SPX250718C05500000', qty: 1 }
        ]
      };
      await mountAndExpand();

      const badge = wrapper.find('.call-side .position-badge');
      expect(badge.exists()).toBe(true);
      expect(badge.text()).toBe('+1');
      expect(badge.classes()).toContain('position-long');
    });

    it('renders badge on put side for put positions', async () => {
      mockPositionsData = {
        positions: [
          { symbol: 'SPX250718P05500000', qty: -3 }
        ]
      };
      await mountAndExpand();

      const badge = wrapper.find('.put-side .position-badge');
      expect(badge.exists()).toBe(true);
      expect(badge.text()).toBe('-3');
      expect(badge.classes()).toContain('position-short');
    });

    it('renders badges on both call and put side simultaneously', async () => {
      mockPositionsData = {
        positions: [
          { symbol: 'SPX250718C05500000', qty: 2 },
          { symbol: 'SPX250718P05500000', qty: -1 }
        ]
      };
      await mountAndExpand();

      const callBadge = wrapper.find('.call-side .position-badge');
      const putBadge = wrapper.find('.put-side .position-badge');
      expect(callBadge.exists()).toBe(true);
      expect(putBadge.exists()).toBe(true);
      expect(callBadge.text()).toBe('+2');
      expect(putBadge.text()).toBe('-1');
    });

    it('does not render badge on rows without positions', async () => {
      mockPositionsData = {
        positions: [
          { symbol: 'SPX250718C05500000', qty: 2 }
        ]
      };
      await mountAndExpand();

      // Find all option rows - the one at strike 5600 should not have a call badge
      const rows = wrapper.findAll('.option-row');
      // Strike 5600 row should not have a position badge on call side
      const row5600 = rows.find(r => r.find('.strike-price').text().includes('5600'));
      if (row5600) {
        const callSide = row5600.find('.call-side');
        const badge = callSide.find('.position-badge');
        expect(badge.exists()).toBe(false);
      }
    });
  });

  describe('Position Aggregation', () => {
    it('aggregates multiple legs with same symbol into net quantity', async () => {
      mockPositionsData = {
        positions: [
          { symbol: 'SPX250718C05500000', qty: 2 },
          { symbol: 'SPX250718C05500000', qty: 1 }
        ]
      };
      await mountAndExpand();

      const badge = wrapper.find('.call-side .position-badge');
      expect(badge.exists()).toBe(true);
      expect(badge.text()).toBe('+3');
      expect(badge.classes()).toContain('position-long');
    });

    it('shows no badge when positions net to zero', async () => {
      mockPositionsData = {
        positions: [
          { symbol: 'SPX250718C05500000', qty: 2 },
          { symbol: 'SPX250718C05500000', qty: -2 }
        ]
      };
      await mountAndExpand();

      const callBadges = wrapper.findAll('.call-side .position-badge');
      // The badge for SPX250718C05500000 should not appear
      const badge5500 = callBadges.find(b => b.text() === '0' || b.text() === '+0');
      expect(badge5500).toBeFalsy();
    });

    it('shows short badge when net is negative from mixed legs', async () => {
      mockPositionsData = {
        positions: [
          { symbol: 'SPX250718P05500000', qty: 1 },
          { symbol: 'SPX250718P05500000', qty: -4 }
        ]
      };
      await mountAndExpand();

      const badge = wrapper.find('.put-side .position-badge');
      expect(badge.exists()).toBe(true);
      expect(badge.text()).toBe('-3');
      expect(badge.classes()).toContain('position-short');
    });
  });

  describe('Conditional Display', () => {
    it('shows no badges when positions data is null', async () => {
      mockPositionsData = null;
      await mountAndExpand();

      const badges = wrapper.findAll('.position-badge');
      expect(badges).toHaveLength(0);
    });

    it('shows no badges when positions array is empty', async () => {
      mockPositionsData = { positions: [] };
      await mountAndExpand();

      const badges = wrapper.findAll('.position-badge');
      expect(badges).toHaveLength(0);
    });

    it('skips position legs with no symbol', async () => {
      mockPositionsData = {
        positions: [
          { symbol: null, qty: 5 },
          { symbol: 'SPX250718C05500000', qty: 1 }
        ]
      };
      await mountAndExpand();

      const badges = wrapper.findAll('.position-badge');
      expect(badges).toHaveLength(1);
      expect(badges[0].text()).toBe('+1');
    });

    it('renders badges for multiple strikes correctly', async () => {
      mockPositionsData = {
        positions: [
          { symbol: 'SPX250718C05500000', qty: 2 },
          { symbol: 'SPX250718C05600000', qty: -1 },
          { symbol: 'SPX250718P05400000', qty: 3 }
        ]
      };
      await mountAndExpand();

      const badges = wrapper.findAll('.position-badge');
      expect(badges.length).toBe(3);

      const texts = badges.map(b => b.text());
      expect(texts).toContain('+2');
      expect(texts).toContain('-1');
      expect(texts).toContain('+3');
    });
  });

  describe('Badge Styling', () => {
    it('applies position-long class for positive quantities', async () => {
      mockPositionsData = {
        positions: [
          { symbol: 'SPX250718C05500000', qty: 1 }
        ]
      };
      await mountAndExpand();

      const badge = wrapper.find('.position-badge');
      expect(badge.classes()).toContain('position-long');
      expect(badge.classes()).not.toContain('position-short');
    });

    it('applies position-short class for negative quantities', async () => {
      mockPositionsData = {
        positions: [
          { symbol: 'SPX250718C05500000', qty: -1 }
        ]
      };
      await mountAndExpand();

      const badge = wrapper.find('.position-badge');
      expect(badge.classes()).toContain('position-short');
      expect(badge.classes()).not.toContain('position-long');
    });

    it('formats positive quantity with + prefix', async () => {
      mockPositionsData = {
        positions: [
          { symbol: 'SPX250718C05500000', qty: 10 }
        ]
      };
      await mountAndExpand();

      const badge = wrapper.find('.position-badge');
      expect(badge.text()).toBe('+10');
    });

    it('formats negative quantity without extra prefix', async () => {
      mockPositionsData = {
        positions: [
          { symbol: 'SPX250718P05500000', qty: -7 }
        ]
      };
      await mountAndExpand();

      const badge = wrapper.find('.position-badge');
      expect(badge.text()).toBe('-7');
    });
  });
});
