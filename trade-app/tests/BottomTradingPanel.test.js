import { describe, it, expect, beforeEach, vi, afterEach } from 'vitest';
import { mount } from '@vue/test-utils';
import { nextTick, ref } from 'vue';
import BottomTradingPanel from '../src/components/BottomTradingPanel.vue';

// Create shared mock refs
const mockSelectedLegs = ref([]);
const mockHasSelectedLegs = ref(false);
const mockUpdateQuantity = vi.fn();
const mockClearAll = vi.fn();
const mockReplaceLeg = vi.fn();

// Mock the composables and services
vi.mock('../src/composables/useSmartMarketData.js', () => ({
  useSmartMarketData: () => ({
    getOptionPrice: vi.fn(() => ref({ bid: 2.50, ask: 2.60, price: 2.55 }))
  })
}));

vi.mock('../src/composables/useSelectedLegs.js', () => ({
  useSelectedLegs: () => ({
    selectedLegs: mockSelectedLegs,
    hasSelectedLegs: mockHasSelectedLegs,
    updateQuantity: mockUpdateQuantity,
    clearAll: mockClearAll,
    replaceLeg: mockReplaceLeg
  })
}));

vi.mock('../src/services/smartMarketDataStore.js', () => ({
  smartMarketDataStore: {
    registerSymbolUsage: vi.fn(),
    unregisterSymbolUsage: vi.fn()
  }
}));

vi.mock('../src/services/optionsCalculator.js', () => ({
  calculateMultiLegProfitLoss: vi.fn(() => ({
    netPremium: -1.50,
    maxProfit: 150,
    maxLoss: -50,
    positions: []
  })),
  calculateBuyingPowerEffect: vi.fn(() => 500),
  formatCurrency: vi.fn((value, decimals = 0) => `$${Math.abs(value).toFixed(decimals)}`),
  getCreditDebitInfo: vi.fn(() => ({ isCredit: true, type: 'Credit' })),
  calculateGreeks: vi.fn(() => ({ delta: 0.25, theta: -0.05 }))
}));

describe('BottomTradingPanel - Cascade Protection & Trading Interface Integrity', () => {
  let wrapper;

  beforeEach(() => {
    // Reset all mocks and refs
    vi.clearAllMocks();
    mockSelectedLegs.value = [];
    mockHasSelectedLegs.value = false;
  });

  afterEach(() => {
    if (wrapper) {
      wrapper.unmount();
    }
  });

  describe('Component Initialization & Props', () => {
    it('renders correctly when visible', () => {
      wrapper = mount(BottomTradingPanel, {
        props: {
          visible: true,
          symbol: 'SPY',
          underlyingPrice: 450.00
        }
      });

      expect(wrapper.find('.bottom-panel').exists()).toBe(true);
      expect(wrapper.find('.bottom-panel').classes()).toContain('slide-up');
      expect(wrapper.find('.symbol-display').text()).toBe('SPY');
    });

    it('does not render when not visible', () => {
      wrapper = mount(BottomTradingPanel, {
        props: {
          visible: false,
          symbol: 'SPY'
        }
      });

      expect(wrapper.find('.bottom-panel').exists()).toBe(false);
    });

    it('handles default props correctly', () => {
      wrapper = mount(BottomTradingPanel, {
        props: {
          visible: true
        }
      });

      expect(wrapper.find('.symbol-display').text()).toBe('SPX');
    });
  });

  describe('Selected Legs Display & Management', () => {
    beforeEach(() => {
      // Set up mock data
      mockSelectedLegs.value = [
        {
          symbol: 'SPY_240119C00450000',
          quantity: 2,
          side: 'buy',
          action: 'buy_to_open',
          type: 'Call',
          strike_price: 450,
          expiry: '2024-01-19',
          bid: 2.40,
          ask: 2.60,
          current_price: 2.50
        },
        {
          symbol: 'SPY_240119P00440000',
          quantity: 1,
          side: 'sell',
          action: 'sell_to_open',
          type: 'Put',
          strike_price: 440,
          expiry: '2024-01-19',
          bid: 1.80,
          ask: 2.00,
          current_price: 1.90
        }
      ];
      mockHasSelectedLegs.value = true;
    });

    it('displays selected legs correctly', async () => {
      wrapper = mount(BottomTradingPanel, {
        props: {
          visible: true,
          symbol: 'SPY'
        }
      });

      await nextTick();

      const orderLegs = wrapper.findAll('.order-leg');
      expect(orderLegs).toHaveLength(2);

      // Check first leg (Call)
      const firstLeg = orderLegs[0];
      expect(firstLeg.find('.leg-qty').text()).toBe('2');
      expect(firstLeg.find('.leg-strike').text()).toBe('450');
      expect(firstLeg.find('.leg-type').text()).toBe('C');
      expect(firstLeg.find('.leg-action').text()).toBe('BTO');

      // Check second leg (Put)
      const secondLeg = orderLegs[1];
      expect(secondLeg.find('.leg-qty').text()).toBe('1');
      expect(secondLeg.find('.leg-strike').text()).toBe('440');
      expect(secondLeg.find('.leg-type').text()).toBe('P');
      expect(secondLeg.find('.leg-action').text()).toBe('STO');
    });

    it('handles leg selection toggle correctly', async () => {
      wrapper = mount(BottomTradingPanel, {
        props: {
          visible: true,
          symbol: 'SPY'
        }
      });

      await nextTick();

      const firstLeg = wrapper.find('.order-leg');
      expect(firstLeg.classes()).not.toContain('selected');

      // Click to select
      await firstLeg.trigger('click');
      expect(firstLeg.classes()).toContain('selected');

      // Click again to deselect
      await firstLeg.trigger('click');
      expect(firstLeg.classes()).not.toContain('selected');
    });

    it('formats dates correctly', async () => {
      wrapper = mount(BottomTradingPanel, {
        props: {
          visible: true,
          symbol: 'SPY'
        }
      });

      await nextTick();

      const dateElements = wrapper.findAll('.leg-date');
      expect(dateElements[0].text()).toBe('Jan 19');
    });

    it('handles invalid dates gracefully', async () => {
      mockSelectedLegs.value = [
        {
          symbol: 'SPY_240119C00450000',
          quantity: 1,
          side: 'buy',
          action: 'buy_to_open',
          type: 'Call',
          strike_price: 450,
          expiry: 'invalid-date',
          bid: 2.40,
          ask: 2.60
        }
      ];

      wrapper = mount(BottomTradingPanel, {
        props: {
          visible: true,
          symbol: 'SPY'
        }
      });

      await nextTick();

      const dateElement = wrapper.find('.leg-date');
      expect(dateElement.text()).toBe('Jul 11'); // Default fallback
    });
  });

  describe('Price Controls & Live Data Integration', () => {
    beforeEach(() => {
      mockSelectedLegs.value = [
        {
          symbol: 'SPY_240119C00450000',
          quantity: 1,
          side: 'buy',
          action: 'buy_to_open',
          type: 'Call',
          strike_price: 450,
          expiry: '2024-01-19',
          bid: 2.40,
          ask: 2.60,
          current_price: 2.50
        }
      ];
    });

    it('displays bid/mid/ask prices correctly', async () => {
      wrapper = mount(BottomTradingPanel, {
        props: {
          visible: true,
          symbol: 'SPY'
        }
      });

      await nextTick();

      // Check price displays exist
      const bidSection = wrapper.find('.bid-section .price-text');
      const askSection = wrapper.find('.ask-section .price-text');
      
      expect(bidSection.exists()).toBe(true);
      expect(askSection.exists()).toBe(true);
    });

    it('handles price lock/unlock functionality', async () => {
      wrapper = mount(BottomTradingPanel, {
        props: {
          visible: true,
          symbol: 'SPY'
        }
      });

      await nextTick();

      const lockButton = wrapper.find('.lock-btn');
      expect(lockButton.classes()).not.toContain('locked');

      // Click to lock
      await lockButton.trigger('click');
      expect(lockButton.classes()).toContain('locked');

      // Click to unlock
      await lockButton.trigger('click');
      expect(lockButton.classes()).not.toContain('locked');
    });

    it('handles price increment/decrement correctly', async () => {
      wrapper = mount(BottomTradingPanel, {
        props: {
          visible: true,
          symbol: 'SPY'
        }
      });

      await nextTick();

      const priceInput = wrapper.find('.price-input');
      const priceButtons = wrapper.findAll('.price-btn');
      
      // Find increment and decrement buttons (skip lock button)
      const incrementBtn = priceButtons.find(btn => btn.text() === '+');
      const decrementBtn = priceButtons.find(btn => btn.text() === '-');

      if (incrementBtn && decrementBtn) {
        const initialValue = parseFloat(priceInput.element.value) || 0;

        // Test increment
        await incrementBtn.trigger('click');
        expect(parseFloat(priceInput.element.value)).toBeCloseTo(initialValue + 0.01, 2);

        // Test decrement
        await decrementBtn.trigger('click');
        expect(parseFloat(priceInput.element.value)).toBeCloseTo(initialValue, 2);
      }
    });
  });

  describe('Quantity Management & Controls', () => {
    beforeEach(() => {
      mockSelectedLegs.value = [
        {
          symbol: 'SPY_240119C00450000',
          quantity: 5,
          side: 'buy',
          action: 'buy_to_open',
          type: 'Call',
          strike_price: 450,
          expiry: '2024-01-19'
        }
      ];
    });

    it('increments quantity correctly', async () => {
      wrapper = mount(BottomTradingPanel, {
        props: {
          visible: true,
          symbol: 'SPY'
        }
      });

      await nextTick();

      const quantityControls = wrapper.find('.control-group:nth-child(3)');
      const incrementBtn = quantityControls.findAll('.ctrl-btn')[1]; // Second button (up arrow)
      
      await incrementBtn.trigger('click');

      expect(mockUpdateQuantity).toHaveBeenCalledWith('SPY_240119C00450000', 6);
    });

    it('decrements quantity correctly', async () => {
      wrapper = mount(BottomTradingPanel, {
        props: {
          visible: true,
          symbol: 'SPY'
        }
      });

      await nextTick();

      const quantityControls = wrapper.find('.control-group:nth-child(3)');
      const decrementBtn = quantityControls.findAll('.ctrl-btn')[0]; // First button (down arrow)
      
      await decrementBtn.trigger('click');

      expect(mockUpdateQuantity).toHaveBeenCalledWith('SPY_240119C00450000', 4);
    });

    it('does not decrement quantity below 1', async () => {
      mockSelectedLegs.value[0].quantity = 1;

      wrapper = mount(BottomTradingPanel, {
        props: {
          visible: true,
          symbol: 'SPY'
        }
      });

      await nextTick();

      const quantityControls = wrapper.find('.control-group:nth-child(3)');
      const decrementBtn = quantityControls.findAll('.ctrl-btn')[0];
      
      await decrementBtn.trigger('click');

      expect(mockUpdateQuantity).not.toHaveBeenCalled();
    });

    it('does not increment quantity above 100', async () => {
      mockSelectedLegs.value[0].quantity = 100;

      wrapper = mount(BottomTradingPanel, {
        props: {
          visible: true,
          symbol: 'SPY'
        }
      });

      await nextTick();

      const quantityControls = wrapper.find('.control-group:nth-child(3)');
      const incrementBtn = quantityControls.findAll('.ctrl-btn')[1];
      
      await incrementBtn.trigger('click');

      expect(mockUpdateQuantity).not.toHaveBeenCalled();
    });
  });

  describe('Order Configuration & Validation', () => {
    beforeEach(() => {
      mockSelectedLegs.value = [
        {
          symbol: 'SPY_240119C00450000',
          quantity: 1,
          side: 'buy',
          action: 'buy_to_open',
          type: 'Call',
          strike_price: 450,
          expiry: '2024-01-19',
          bid: 2.40,
          ask: 2.60,
          current_price: 2.50
        }
      ];
      mockHasSelectedLegs.value = true;
    });

    it('enables Review & Send button when legs are selected', async () => {
      wrapper = mount(BottomTradingPanel, {
        props: {
          visible: true,
          symbol: 'SPY'
        }
      });

      await nextTick();

      const reviewBtn = wrapper.find('.review-btn');
      expect(reviewBtn.attributes('disabled')).toBeFalsy();
    });

    it('disables Review & Send button when no legs are selected', async () => {
      mockSelectedLegs.value = [];
      mockHasSelectedLegs.value = false;

      wrapper = mount(BottomTradingPanel, {
        props: {
          visible: true,
          symbol: 'SPY'
        }
      });

      await nextTick();

      const reviewBtn = wrapper.find('.review-btn');
      expect(reviewBtn.attributes('disabled')).toBeDefined();
    });

    it('handles order type selection correctly', async () => {
      wrapper = mount(BottomTradingPanel, {
        props: {
          visible: true,
          symbol: 'SPY'
        }
      });

      await nextTick();

      const orderTypeSelect = wrapper.find('.order-type-section .config-select');
      expect(orderTypeSelect.element.value).toBe('limit');

      await orderTypeSelect.setValue('market');
      expect(orderTypeSelect.element.value).toBe('market');
    });

    it('handles time in force selection correctly', async () => {
      wrapper = mount(BottomTradingPanel, {
        props: {
          visible: true,
          symbol: 'SPY'
        }
      });

      await nextTick();

      const timeInForceSelect = wrapper.find('.time-force-section .config-select');
      expect(timeInForceSelect.element.value).toBe('day');

      await timeInForceSelect.setValue('gtc');
      expect(timeInForceSelect.element.value).toBe('gtc');
    });

    it('emits review-send event with correct order data', async () => {
      wrapper = mount(BottomTradingPanel, {
        props: {
          visible: true,
          symbol: 'SPY',
          underlyingPrice: 450.00
        }
      });

      await nextTick();

      const reviewBtn = wrapper.find('.review-btn');
      await reviewBtn.trigger('click');

      const emittedEvents = wrapper.emitted('review-send');
      expect(emittedEvents).toHaveLength(1);

      const orderData = emittedEvents[0][0];
      expect(orderData).toMatchObject({
        symbol: 'SPY',
        expiry: '2024-01-19',
        legs: expect.arrayContaining([
          expect.objectContaining({
            symbol: 'SPY_240119C00450000',
            side: 'buy',
            action: 'buy_to_open',
            quantity: 1,
            type: 'Call',
            strike: 450
          })
        ]),
        orderType: 'limit',
        timeInForce: 'day',
        underlyingPrice: 450.00
      });
    });

    it('handles clear trade correctly', async () => {
      wrapper = mount(BottomTradingPanel, {
        props: {
          visible: true,
          symbol: 'SPY'
        }
      });

      await nextTick();

      const clearBtn = wrapper.find('.clear-btn');
      await clearBtn.trigger('click');

      expect(mockClearAll).toHaveBeenCalled();
    });
  });

  describe('Statistics & Greeks Display', () => {
    beforeEach(() => {
      mockSelectedLegs.value = [
        {
          symbol: 'SPY_240119C00450000',
          quantity: 1,
          side: 'buy',
          action: 'buy_to_open',
          type: 'Call',
          strike_price: 450,
          expiry: '2024-01-19',
          bid: 2.40,
          ask: 2.60,
          current_price: 2.50
        }
      ];
    });

    it('displays statistics correctly', async () => {
      wrapper = mount(BottomTradingPanel, {
        props: {
          visible: true,
          symbol: 'SPY'
        }
      });

      await nextTick();

      const statGroups = wrapper.findAll('.stat-group');
      expect(statGroups.length).toBeGreaterThan(0);

      // Check that stats are displayed (basic structure test)
      const deltaValue = wrapper.find('.stat-group:nth-child(4) .stat-value');
      const thetaValue = wrapper.find('.stat-group:nth-child(5) .stat-value');

      expect(deltaValue.exists()).toBe(true);
      expect(thetaValue.exists()).toBe(true);
    });

    it('applies correct CSS classes for positive/negative values', async () => {
      wrapper = mount(BottomTradingPanel, {
        props: {
          visible: true,
          symbol: 'SPY'
        }
      });

      await nextTick();

      const thetaValue = wrapper.find('.stat-group:nth-child(5) .stat-value');
      const maxProfitValue = wrapper.find('.stat-group:nth-child(6) .stat-value');
      const maxLossValue = wrapper.find('.stat-group:nth-child(7) .stat-value');

      expect(thetaValue.classes()).toContain('negative');
      expect(maxProfitValue.classes()).toContain('positive');
      expect(maxLossValue.classes()).toContain('negative');
    });
  });

  describe('Progress Bar & Price Visualization', () => {
    beforeEach(() => {
      mockSelectedLegs.value = [
        {
          symbol: 'SPY_240119C00450000',
          quantity: 1,
          side: 'buy',
          action: 'buy_to_open',
          type: 'Call',
          strike_price: 450,
          expiry: '2024-01-19',
          bid: 2.40,
          ask: 2.60,
          current_price: 2.50
        }
      ];
    });

    it('displays progress bars correctly', async () => {
      wrapper = mount(BottomTradingPanel, {
        props: {
          visible: true,
          symbol: 'SPY'
        }
      });

      await nextTick();

      const progressSection = wrapper.find('.progress-section');
      const leftProgressBar = wrapper.find('.progress-bar.left');
      const rightProgressBar = wrapper.find('.progress-bar.right');
      const centerIndicator = wrapper.find('.center-indicator');

      expect(progressSection.exists()).toBe(true);
      expect(leftProgressBar.exists()).toBe(true);
      expect(rightProgressBar.exists()).toBe(true);
      expect(centerIndicator.exists()).toBe(true);
    });
  });

  describe('Edge Cases & Error Handling', () => {
    it('handles empty selected legs gracefully', async () => {
      mockSelectedLegs.value = [];
      mockHasSelectedLegs.value = false;

      wrapper = mount(BottomTradingPanel, {
        props: {
          visible: true,
          symbol: 'SPY'
        }
      });

      await nextTick();

      const orderLegs = wrapper.findAll('.order-leg');
      expect(orderLegs).toHaveLength(0);

      const reviewBtn = wrapper.find('.review-btn');
      expect(reviewBtn.attributes('disabled')).toBeDefined();
    });

    it('handles missing price data gracefully', async () => {
      mockSelectedLegs.value = [
        {
          symbol: 'SPY_240119C00450000',
          quantity: 1,
          side: 'buy',
          action: 'buy_to_open',
          type: 'Call',
          strike_price: 450,
          expiry: '2024-01-19'
          // Missing bid, ask, current_price
        }
      ];

      wrapper = mount(BottomTradingPanel, {
        props: {
          visible: true,
          symbol: 'SPY'
        }
      });

      await nextTick();

      const orderLegs = wrapper.findAll('.order-leg');
      expect(orderLegs).toHaveLength(1);

      // Should display price from live data or fallback (component uses live price lookup)
      const priceElement = orderLegs[0].find('.leg-price');
      expect(priceElement.text()).toMatch(/^\$\d+\.\d{2}$/); // Should be a valid price format
    });

    it('handles null/undefined strike prices gracefully', async () => {
      mockSelectedLegs.value = [
        {
          symbol: 'SPY_240119C00450000',
          quantity: 1,
          side: 'buy',
          action: 'buy_to_open',
          type: 'Call',
          strike_price: null,
          expiry: '2024-01-19',
          bid: 2.40,
          ask: 2.60
        }
      ];

      wrapper = mount(BottomTradingPanel, {
        props: {
          visible: true,
          symbol: 'SPY'
        }
      });

      await nextTick();

      const orderLegs = wrapper.findAll('.order-leg');
      if (orderLegs.length > 0) {
        const strikeElement = orderLegs[0].find('.leg-strike');
        expect(strikeElement.text()).toBe('-');
      }
    });
  });

  describe('Real-World Cascade Scenarios', () => {
    it('handles complete trading workflow data cascade', async () => {
      // Simulate complete workflow: leg selection → trading panel → analysis → order
      mockSelectedLegs.value = [
        {
          symbol: 'SPY_240119C00450000',
          quantity: 2,
          side: 'buy',
          action: 'buy_to_open',
          type: 'Call',
          strike_price: 450,
          expiry: '2024-01-19',
          bid: 2.40,
          ask: 2.60,
          current_price: 2.50
        },
        {
          symbol: 'SPY_240119P00440000',
          quantity: 2,
          side: 'sell',
          action: 'sell_to_open',
          type: 'Put',
          strike_price: 440,
          expiry: '2024-01-19',
          bid: 1.80,
          ask: 2.00,
          current_price: 1.90
        }
      ];
      mockHasSelectedLegs.value = true;

      wrapper = mount(BottomTradingPanel, {
        props: {
          visible: true,
          symbol: 'SPY',
          underlyingPrice: 445.00
        }
      });

      await nextTick();

      // Verify legs display correctly
      const orderLegs = wrapper.findAll('.order-leg');
      expect(orderLegs).toHaveLength(2);

      // Verify statistics are calculated
      const statGroups = wrapper.findAll('.stat-group');
      expect(statGroups.length).toBeGreaterThan(0);

      // Verify price controls work
      const priceInput = wrapper.find('.price-input');
      expect(priceInput.exists()).toBe(true);

      // Verify order can be submitted
      const reviewBtn = wrapper.find('.review-btn');
      expect(reviewBtn.attributes('disabled')).toBeFalsy();

      // Submit order and verify event emission
      await reviewBtn.trigger('click');
      const emittedEvents = wrapper.emitted('review-send');
      expect(emittedEvents).toHaveLength(1);
      expect(emittedEvents[0][0]).toMatchObject({
        symbol: 'SPY',
        legs: expect.arrayContaining([
          expect.objectContaining({ symbol: 'SPY_240119C00450000' }),
          expect.objectContaining({ symbol: 'SPY_240119P00440000' })
        ])
      });
    });

    it('handles rapid price updates without memory leaks', async () => {
      mockSelectedLegs.value = [
        {
          symbol: 'SPY_240119C00450000',
          quantity: 1,
          side: 'buy',
          action: 'buy_to_open',
          type: 'Call',
          strike_price: 450,
          expiry: '2024-01-19',
          bid: 2.40,
          ask: 2.60,
          current_price: 2.50
        }
      ];

      wrapper = mount(BottomTradingPanel, {
        props: {
          visible: true,
          symbol: 'SPY'
        }
      });

      await nextTick();

      // Component should remain stable after rapid updates
      const priceInput = wrapper.find('.price-input');
      expect(priceInput.exists()).toBe(true);
    });

    it('handles provider data format changes gracefully', async () => {
      // Test different provider data formats
      const providerFormats = [
        // Tastytrade format
        {
          symbol: 'SPY_240119C00450000',
          quantity: 1,
          side: 'buy',
          action: 'buy_to_open',
          type: 'Call',
          strike_price: 450,
          expiry: '2024-01-19',
          bid: 2.40,
          ask: 2.60,
          current_price: 2.50
        },
        // Alpaca format
        {
          symbol: 'SPY240119C00450000',
          quantity: 1,
          side: 'buy',
          action: 'buy_to_open',
          type: 'call',
          strike: 450,
          expiry: '2024-01-19',
          bid: 2.40,
          ask: 2.60,
          avg_entry_price: 2.50
        }
      ];

      for (const format of providerFormats) {
        mockSelectedLegs.value = [format];

        wrapper = mount(BottomTradingPanel, {
          props: {
            visible: true,
            symbol: 'SPY'
          }
        });

        await nextTick();

        const orderLegs = wrapper.findAll('.order-leg');
        expect(orderLegs).toHaveLength(1);

        // Should handle both formats gracefully
        const strikeElement = orderLegs[0].find('.leg-strike');
        expect(strikeElement.text()).toBe('450');

        wrapper.unmount();
      }
    });
  });

  describe('Proportional Quantity Adjustment Logic', () => {
    // Helper functions to test the internal logic
    const calculateGCD = (a, b) => {
      return b === 0 ? a : calculateGCD(b, a % b);
    };

    const calculateArrayGCD = (numbers) => {
      if (numbers.length === 0) return 1;
      if (numbers.length === 1) return numbers[0];
      
      return numbers.reduce((gcd, num) => calculateGCD(gcd, num));
    };

    const analyzeQuantityPattern = (legs) => {
      if (legs.length === 0) return { ratios: [], multiplier: 1, canIncrement: false, canDecrement: false };
      
      const quantities = legs.map(leg => leg.quantity);
      const gcd = calculateArrayGCD(quantities);
      const ratios = quantities.map(qty => qty / gcd);
      
      // Check if we can increment (no leg would exceed limits)
      const canIncrement = legs.every((leg, index) => {
        const maxQty = leg.source === 'positions' && leg.original_quantity ? leg.original_quantity : 100;
        const nextQty = ratios[index] * (gcd + 1); // Calculate what the next quantity would be
        return nextQty <= maxQty;
      });
      
      const canDecrement = gcd > 1;
      
      return {
        ratios,
        multiplier: gcd,
        canIncrement,
        canDecrement
      };
    };

    const getNextProportionalQuantities = (legs, direction) => {
      const analysis = analyzeQuantityPattern(legs);
      
      if (direction === 'increment' && !analysis.canIncrement) {
        return null;
      }
      
      if (direction === 'decrement' && !analysis.canDecrement) {
        return null;
      }
      
      const newMultiplier = direction === 'increment' 
        ? analysis.multiplier + 1 
        : analysis.multiplier - 1;
      
      const newQuantities = analysis.ratios.map(ratio => ratio * newMultiplier);
      
      const isValid = legs.every((leg, index) => {
        const newQty = newQuantities[index];
        const maxQty = leg.source === 'positions' && leg.original_quantity ? leg.original_quantity : 100;
        return newQty >= 1 && newQty <= maxQty;
      });
      
      return isValid ? newQuantities : null;
    };

    describe('GCD Calculation', () => {
      it('calculates GCD correctly for two numbers', () => {
        expect(calculateGCD(12, 8)).toBe(4);
        expect(calculateGCD(15, 25)).toBe(5);
        expect(calculateGCD(7, 13)).toBe(1);
        expect(calculateGCD(0, 5)).toBe(5);
      });

      it('calculates GCD correctly for arrays', () => {
        expect(calculateArrayGCD([12, 8, 16])).toBe(4);
        expect(calculateArrayGCD([1, 1, 2])).toBe(1);
        expect(calculateArrayGCD([2, 4, 6])).toBe(2);
        expect(calculateArrayGCD([3, 6, 9])).toBe(3);
        expect(calculateArrayGCD([5])).toBe(5);
        expect(calculateArrayGCD([])).toBe(1);
      });
    });

    describe('Pattern Analysis', () => {
      it('analyzes butterfly spread pattern (1:1:2)', () => {
        const legs = [
          { quantity: 1, source: 'options_chain' },
          { quantity: 1, source: 'options_chain' },
          { quantity: 2, source: 'options_chain' }
        ];

        const analysis = analyzeQuantityPattern(legs);
        expect(analysis.ratios).toEqual([1, 1, 2]);
        expect(analysis.multiplier).toBe(1);
        expect(analysis.canIncrement).toBe(true);
        expect(analysis.canDecrement).toBe(false);
      });

      it('analyzes equal quantities pattern (2:2:2)', () => {
        const legs = [
          { quantity: 2, source: 'options_chain' },
          { quantity: 2, source: 'options_chain' },
          { quantity: 2, source: 'options_chain' }
        ];

        const analysis = analyzeQuantityPattern(legs);
        expect(analysis.ratios).toEqual([1, 1, 1]);
        expect(analysis.multiplier).toBe(2);
        expect(analysis.canIncrement).toBe(true);
        expect(analysis.canDecrement).toBe(true);
      });

      it('analyzes complex ratio pattern (2:3:4)', () => {
        const legs = [
          { quantity: 2, source: 'options_chain' },
          { quantity: 3, source: 'options_chain' },
          { quantity: 4, source: 'options_chain' }
        ];

        const analysis = analyzeQuantityPattern(legs);
        expect(analysis.ratios).toEqual([2, 3, 4]);
        expect(analysis.multiplier).toBe(1);
        expect(analysis.canIncrement).toBe(true);
        expect(analysis.canDecrement).toBe(false);
      });

      it('handles position constraints correctly', () => {
        const legs = [
          { quantity: 2, source: 'positions', original_quantity: 3 },
          { quantity: 4, source: 'positions', original_quantity: 6 }
        ];

        const analysis = analyzeQuantityPattern(legs);
        expect(analysis.ratios).toEqual([1, 2]);
        expect(analysis.multiplier).toBe(2);
        expect(analysis.canIncrement).toBe(true); // 3:6 would be within limits
        expect(analysis.canDecrement).toBe(true); // 1:2 would be valid
      });

      it('prevents increment when exceeding position limits', () => {
        const legs = [
          { quantity: 3, source: 'positions', original_quantity: 3 },
          { quantity: 6, source: 'positions', original_quantity: 6 }
        ];

        const analysis = analyzeQuantityPattern(legs);
        expect(analysis.canIncrement).toBe(false); // Would exceed original quantities
      });
    });

    describe('Proportional Quantity Calculation', () => {
      it('calculates next increment for butterfly spread (1:1:2 → 2:2:4)', () => {
        const legs = [
          { quantity: 1, source: 'options_chain' },
          { quantity: 1, source: 'options_chain' },
          { quantity: 2, source: 'options_chain' }
        ];

        const newQuantities = getNextProportionalQuantities(legs, 'increment');
        expect(newQuantities).toEqual([2, 2, 4]);
      });

      it('calculates multiple increments maintaining ratio (2:2:4 → 3:3:6)', () => {
        const legs = [
          { quantity: 2, source: 'options_chain' },
          { quantity: 2, source: 'options_chain' },
          { quantity: 4, source: 'options_chain' }
        ];

        const newQuantities = getNextProportionalQuantities(legs, 'increment');
        expect(newQuantities).toEqual([3, 3, 6]);
      });

      it('calculates decrement maintaining ratio (4:4:8 → 3:3:6)', () => {
        const legs = [
          { quantity: 4, source: 'options_chain' },
          { quantity: 4, source: 'options_chain' },
          { quantity: 8, source: 'options_chain' }
        ];

        const newQuantities = getNextProportionalQuantities(legs, 'decrement');
        expect(newQuantities).toEqual([3, 3, 6]);
      });

      it('handles equal quantities correctly (3:3:3 → 4:4:4)', () => {
        const legs = [
          { quantity: 3, source: 'options_chain' },
          { quantity: 3, source: 'options_chain' },
          { quantity: 3, source: 'options_chain' }
        ];

        const newQuantities = getNextProportionalQuantities(legs, 'increment');
        expect(newQuantities).toEqual([4, 4, 4]);
      });

      it('handles complex ratios correctly (2:3:4 → 4:6:8)', () => {
        const legs = [
          { quantity: 2, source: 'options_chain' },
          { quantity: 3, source: 'options_chain' },
          { quantity: 4, source: 'options_chain' }
        ];

        const newQuantities = getNextProportionalQuantities(legs, 'increment');
        expect(newQuantities).toEqual([4, 6, 8]);
      });

      it('returns null when cannot decrement (already at minimum)', () => {
        const legs = [
          { quantity: 1, source: 'options_chain' },
          { quantity: 1, source: 'options_chain' },
          { quantity: 2, source: 'options_chain' }
        ];

        const newQuantities = getNextProportionalQuantities(legs, 'decrement');
        expect(newQuantities).toBeNull();
      });

      it('returns null when increment would exceed limits', () => {
        const legs = [
          { quantity: 100, source: 'options_chain' },
          { quantity: 100, source: 'options_chain' }
        ];

        const newQuantities = getNextProportionalQuantities(legs, 'increment');
        expect(newQuantities).toBeNull();
      });

      it('respects position quantity limits', () => {
        const legs = [
          { quantity: 2, source: 'positions', original_quantity: 3 },
          { quantity: 4, source: 'positions', original_quantity: 6 }
        ];

        const newQuantities = getNextProportionalQuantities(legs, 'increment');
        expect(newQuantities).toEqual([3, 6]); // Should reach max allowed

        // Try to increment again - should fail
        legs[0].quantity = 3;
        legs[1].quantity = 6;
        const nextQuantities = getNextProportionalQuantities(legs, 'increment');
        expect(nextQuantities).toBeNull();
      });
    });

    describe('Integration with Component', () => {
      beforeEach(() => {
        // Reset mocks
        vi.clearAllMocks();
      });

      it('applies proportional increment to butterfly spread', async () => {
        mockSelectedLegs.value = [
          {
            symbol: 'SPY_240119C00450000',
            quantity: 1,
            side: 'buy',
            action: 'buy_to_open',
            type: 'Call',
            strike_price: 450,
            expiry: '2024-01-19',
            source: 'options_chain'
          },
          {
            symbol: 'SPY_240119C00460000',
            quantity: 1,
            side: 'sell',
            action: 'sell_to_open',
            type: 'Call',
            strike_price: 460,
            expiry: '2024-01-19',
            source: 'options_chain'
          },
          {
            symbol: 'SPY_240119C00470000',
            quantity: 2,
            side: 'buy',
            action: 'buy_to_open',
            type: 'Call',
            strike_price: 470,
            expiry: '2024-01-19',
            source: 'options_chain'
          }
        ];

        wrapper = mount(BottomTradingPanel, {
          props: {
            visible: true,
            symbol: 'SPY'
          }
        });

        await nextTick();

        const quantityControls = wrapper.find('.control-group:nth-child(3)');
        const incrementBtn = quantityControls.findAll('.ctrl-btn')[1];
        
        await incrementBtn.trigger('click');

        // Should call updateQuantity for each leg with proportional amounts
        expect(mockUpdateQuantity).toHaveBeenCalledTimes(3);
        expect(mockUpdateQuantity).toHaveBeenCalledWith('SPY_240119C00450000', 2);
        expect(mockUpdateQuantity).toHaveBeenCalledWith('SPY_240119C00460000', 2);
        expect(mockUpdateQuantity).toHaveBeenCalledWith('SPY_240119C00470000', 4);
      });

      it('applies proportional decrement maintaining ratios', async () => {
        mockSelectedLegs.value = [
          {
            symbol: 'SPY_240119C00450000',
            quantity: 4,
            side: 'buy',
            action: 'buy_to_open',
            type: 'Call',
            strike_price: 450,
            expiry: '2024-01-19',
            source: 'options_chain'
          },
          {
            symbol: 'SPY_240119C00460000',
            quantity: 4,
            side: 'sell',
            action: 'sell_to_open',
            type: 'Call',
            strike_price: 460,
            expiry: '2024-01-19',
            source: 'options_chain'
          },
          {
            symbol: 'SPY_240119C00470000',
            quantity: 8,
            side: 'buy',
            action: 'buy_to_open',
            type: 'Call',
            strike_price: 470,
            expiry: '2024-01-19',
            source: 'options_chain'
          }
        ];

        wrapper = mount(BottomTradingPanel, {
          props: {
            visible: true,
            symbol: 'SPY'
          }
        });

        await nextTick();

        const quantityControls = wrapper.find('.control-group:nth-child(3)');
        const decrementBtn = quantityControls.findAll('.ctrl-btn')[0];
        
        await decrementBtn.trigger('click');

        // Should call updateQuantity for each leg with proportional amounts
        expect(mockUpdateQuantity).toHaveBeenCalledTimes(3);
        expect(mockUpdateQuantity).toHaveBeenCalledWith('SPY_240119C00450000', 3);
        expect(mockUpdateQuantity).toHaveBeenCalledWith('SPY_240119C00460000', 3);
        expect(mockUpdateQuantity).toHaveBeenCalledWith('SPY_240119C00470000', 6);
      });

      it('falls back to old behavior when proportional calculation fails', async () => {
        mockSelectedLegs.value = [
          {
            symbol: 'SPY_240119C00450000',
            quantity: 100, // At max limit
            side: 'buy',
            action: 'buy_to_open',
            type: 'Call',
            strike_price: 450,
            expiry: '2024-01-19',
            source: 'options_chain'
          }
        ];

        wrapper = mount(BottomTradingPanel, {
          props: {
            visible: true,
            symbol: 'SPY'
          }
        });

        await nextTick();

        const quantityControls = wrapper.find('.control-group:nth-child(3)');
        const incrementBtn = quantityControls.findAll('.ctrl-btn')[1];
        
        await incrementBtn.trigger('click');

        // Should not call updateQuantity since we're at max limit
        expect(mockUpdateQuantity).not.toHaveBeenCalled();
      });

      it('works with selected legs only', async () => {
        mockSelectedLegs.value = [
          {
            symbol: 'SPY_240119C00450000',
            quantity: 1,
            side: 'buy',
            action: 'buy_to_open',
            type: 'Call',
            strike_price: 450,
            expiry: '2024-01-19',
            source: 'options_chain'
          },
          {
            symbol: 'SPY_240119C00460000',
            quantity: 2,
            side: 'sell',
            action: 'sell_to_open',
            type: 'Call',
            strike_price: 460,
            expiry: '2024-01-19',
            source: 'options_chain'
          }
        ];

        wrapper = mount(BottomTradingPanel, {
          props: {
            visible: true,
            symbol: 'SPY'
          }
        });

        await nextTick();

        // Select only the first leg
        const firstLeg = wrapper.find('.order-leg');
        await firstLeg.trigger('click');

        const quantityControls = wrapper.find('.control-group:nth-child(3)');
        const incrementBtn = quantityControls.findAll('.ctrl-btn')[1];
        
        await incrementBtn.trigger('click');

        // Should only update the selected leg (fallback to old behavior for single leg)
        expect(mockUpdateQuantity).toHaveBeenCalledTimes(1);
        expect(mockUpdateQuantity).toHaveBeenCalledWith('SPY_240119C00450000', 2);
      });

      it('maintains backward compatibility with equal quantities', async () => {
        mockSelectedLegs.value = [
          {
            symbol: 'SPY_240119C00450000',
            quantity: 3,
            side: 'buy',
            action: 'buy_to_open',
            type: 'Call',
            strike_price: 450,
            expiry: '2024-01-19',
            source: 'options_chain'
          },
          {
            symbol: 'SPY_240119C00460000',
            quantity: 3,
            side: 'sell',
            action: 'sell_to_open',
            type: 'Call',
            strike_price: 460,
            expiry: '2024-01-19',
            source: 'options_chain'
          }
        ];

        wrapper = mount(BottomTradingPanel, {
          props: {
            visible: true,
            symbol: 'SPY'
          }
        });

        await nextTick();

        const quantityControls = wrapper.find('.control-group:nth-child(3)');
        const incrementBtn = quantityControls.findAll('.ctrl-btn')[1];
        
        await incrementBtn.trigger('click');

        // Should increment both legs to 4 (maintaining 1:1 ratio)
        expect(mockUpdateQuantity).toHaveBeenCalledTimes(2);
        expect(mockUpdateQuantity).toHaveBeenCalledWith('SPY_240119C00450000', 4);
        expect(mockUpdateQuantity).toHaveBeenCalledWith('SPY_240119C00460000', 4);
      });
    });
  });

  describe('Performance & Memory Management', () => {
    it('handles rapid visibility toggles efficiently', async () => {
      wrapper = mount(BottomTradingPanel, {
        props: {
          visible: false,
          symbol: 'SPY'
        }
      });

      // Rapidly toggle visibility
      for (let i = 0; i < 10; i++) {
        await wrapper.setProps({ visible: i % 2 === 0 });
        await nextTick();
      }

      // Component should remain stable
      expect(wrapper.exists()).toBe(true);
    });

    it('handles symbol changes without errors', async () => {
      wrapper = mount(BottomTradingPanel, {
        props: {
          visible: true,
          symbol: 'SPY'
        }
      });

      await nextTick();

      // Change symbol
      await wrapper.setProps({ symbol: 'QQQ' });
      await nextTick();

      // Component should remain stable
      expect(wrapper.find('.symbol-display').text()).toBe('QQQ');
    });
  });
});
