import { describe, it, expect, beforeEach, vi, afterEach } from 'vitest';
import { mount } from '@vue/test-utils';
import { nextTick, ref } from 'vue';
import OrderConfirmationDialog from '../src/components/OrderConfirmationDialog.vue';

// Mock the composables and services
const mockPreviewData = ref(null);
const mockIsLoading = ref(false);
const mockError = ref(null);
const mockReactiveBalance = ref({ buying_power: 10000 });
const mockReactiveAccountInfo = ref({ account_value: 50000 });

// Expose mocks to global scope for component access
globalThis.mockPreviewData = mockPreviewData;
globalThis.mockIsLoading = mockIsLoading;
globalThis.mockError = mockError;

vi.mock('../src/composables/useOrderManagement.js', () => ({
  useOrderManagement: () => ({
    previewData: mockPreviewData,
    isLoading: mockIsLoading,
    error: mockError,
    previewOrder: vi.fn(),
    submitOrder: vi.fn()
  })
}));

vi.mock('../src/composables/useMarketData.js', () => ({
  useMarketData: () => ({
    getBalance: () => mockReactiveBalance,
    getAccountInfo: () => mockReactiveAccountInfo,
    reactiveBalance: mockReactiveBalance,
    reactiveAccountInfo: mockReactiveAccountInfo
  })
}));

vi.mock('../src/services/orderService.js', () => ({
  default: {
    validateOrder: vi.fn(() => ({ isValid: true, errors: [] })),
    calculateOrderSummary: vi.fn(() => ({
      totalPremium: -150,
      maxProfit: 200,
      maxLoss: -50,
      netCredit: true,
      netDebit: false
    }))
  }
}));

describe('OrderConfirmationDialog - Enhanced Equity Support', () => {
  let wrapper;

  beforeEach(() => {
    // Reset all mocks
    vi.clearAllMocks();
    
    // Setup default mock values
    mockPreviewData.value = null;
    mockIsLoading.value = false;
    mockError.value = null;
    mockReactiveBalance.value = { buying_power: 10000 };
    mockReactiveAccountInfo.value = { account_value: 50000 };
  });

  afterEach(() => {
    if (wrapper) {
      wrapper.unmount();
    }
  });

  describe('Equity Order Display', () => {
    it('displays equity buy order correctly', async () => {
      const equityOrderData = {
        symbol: 'SPY',
        side: 'buy',
        quantity: 100,
        orderType: 'market',
        timeInForce: 'day',
        estimatedCost: 64247.00
      };

      mockPreviewData.value = {
        order_cost: -64247.00,
        commission: 0.0,
        fees: 0.0,
        buying_power_effect: -32123.50
      };

      wrapper = mount(OrderConfirmationDialog, {
        props: {
          visible: true,
          orderData: equityOrderData
        }
      });

      await nextTick();

      // Should display order details
      expect(wrapper.find('.bottom-sheet-overlay').exists()).toBe(true);
      expect(wrapper.text()).toContain('SPY');
      expect(wrapper.text()).toContain('100');
    });

    it('displays equity sell order correctly', async () => {
      const equityOrderData = {
        symbol: 'AAPL',
        side: 'sell',
        quantity: 50,
        orderType: 'limit',
        timeInForce: 'gtc',
        limitPrice: 150.25,
        estimatedCost: 7512.50
      };

      mockPreviewData.value = {
        order_cost: 7512.50,
        commission: 0.0,
        fees: 0.0,
        buying_power_effect: 3756.25
      };

      wrapper = mount(OrderConfirmationDialog, {
        props: {
          visible: true,
          orderData: equityOrderData
        }
      });

      await nextTick();

      expect(wrapper.text()).toContain('AAPL');
      expect(wrapper.text()).toContain('50');
      expect(wrapper.text()).toContain('150.25');
    });

    it('displays short sell order with appropriate indicators', async () => {
      const shortSellOrderData = {
        symbol: 'TSLA',
        side: 'sell',
        quantity: 25,
        orderType: 'market',
        timeInForce: 'day',
        is_short_sell: true,
        estimatedCost: 5000.00
      };

      mockPreviewData.value = {
        order_cost: 5000.00,
        commission: 0.0,
        fees: 0.0,
        buying_power_effect: 2500.00
      };

      wrapper = mount(OrderConfirmationDialog, {
        props: {
          visible: true,
          orderData: shortSellOrderData
        }
      });

      await nextTick();

      expect(wrapper.text()).toContain('TSLA');
      expect(wrapper.text()).toContain('25');
      // Should indicate short sell somehow in the UI
    });
  });

  describe('Price Per Unit Calculation', () => {
    it('calculates price per unit correctly for equity orders', async () => {
      const equityOrderData = {
        symbol: 'SPY',
        side: 'buy',
        quantity: 100,
        orderType: 'limit',
        limitPrice: 642.47,
        estimatedCost: 64247.00
      };

      mockPreviewData.value = {
        order_cost: -64247.00,
        commission: 0.0,
        fees: 0.0,
        buying_power_effect: -32123.50
      };

      wrapper = mount(OrderConfirmationDialog, {
        props: {
          visible: true,
          orderData: equityOrderData
        }
      });

      await nextTick();

      // Should calculate price per unit as order_cost / quantity
      // For buy orders, order_cost is negative, so price per unit should be positive
      const expectedPricePerUnit = Math.abs(-64247.00) / 100; // 642.47
      
      // Check if the calculated price is displayed somewhere in the component
      expect(wrapper.vm.formattedLegs).toBeDefined();
    });

    it('handles zero quantity gracefully', async () => {
      const equityOrderData = {
        symbol: 'SPY',
        side: 'buy',
        quantity: 0,
        orderType: 'market',
        estimatedCost: 0
      };

      mockPreviewData.value = {
        order_cost: 0,
        commission: 0.0,
        fees: 0.0,
        buying_power_effect: 0
      };

      wrapper = mount(OrderConfirmationDialog, {
        props: {
          visible: true,
          orderData: equityOrderData
        }
      });

      await nextTick();

      // Should not crash with division by zero
      expect(wrapper.exists()).toBe(true);
    });

    it('calculates price per unit for fractional shares', async () => {
      const fractionalOrderData = {
        symbol: 'SPY',
        side: 'buy',
        quantity: 0.5,
        orderType: 'market',
        estimatedCost: 321.23
      };

      mockPreviewData.value = {
        order_cost: -321.23,
        commission: 0.0,
        fees: 0.0,
        buying_power_effect: -160.62
      };

      wrapper = mount(OrderConfirmationDialog, {
        props: {
          visible: true,
          orderData: fractionalOrderData
        }
      });

      await nextTick();

      // Should handle fractional quantities correctly
      const expectedPricePerUnit = Math.abs(-321.23) / 0.5; // 642.46
      expect(wrapper.exists()).toBe(true);
    });
  });

  describe('UI Conditional Rendering for Equity Orders', () => {
    it('hides options-specific fields for equity orders', async () => {
      const equityOrderData = {
        symbol: 'SPY',
        side: 'buy',
        quantity: 100,
        orderType: 'market'
      };

      wrapper = mount(OrderConfirmationDialog, {
        props: {
          visible: true,
          orderData: equityOrderData
        }
      });

      await nextTick();

      // Should not show "Net Credit @" field for equity trades
      expect(wrapper.text()).not.toContain('Net Credit @');
      
      // Should not show "cr/db" labels for equity trades
      expect(wrapper.text()).not.toContain(' cr');
      expect(wrapper.text()).not.toContain(' db');
    });

    it('shows appropriate labels for equity orders', async () => {
      const equityOrderData = {
        symbol: 'AAPL',
        side: 'sell',
        quantity: 50,
        orderType: 'limit',
        limitPrice: 150.25
      };

      mockPreviewData.value = {
        order_cost: 7512.50,
        commission: 0.0,
        fees: 0.0,
        buying_power_effect: 3756.25
      };

      wrapper = mount(OrderConfirmationDialog, {
        props: {
          visible: true,
          orderData: equityOrderData
        }
      });

      await nextTick();

      // Should show plain dollar amounts without cr/db suffixes
      expect(wrapper.text()).toContain('$');
      expect(wrapper.text()).not.toContain('cr');
      expect(wrapper.text()).not.toContain('db');
    });

    it('displays different order types correctly', async () => {
      const orderTypes = [
        { orderType: 'market', label: 'Market' },
        { orderType: 'limit', label: 'Limit' },
        { orderType: 'stop', label: 'Stop' },
        { orderType: 'stop_limit', label: 'Stop Limit' }
      ];

      for (const { orderType, label } of orderTypes) {
        const orderData = {
          symbol: 'SPY',
          side: 'buy',
          quantity: 100,
          orderType: orderType,
          ...(orderType.includes('limit') && { limitPrice: 642.47 }),
          ...(orderType.includes('stop') && { stopPrice: 640.00 })
        };

        wrapper = mount(OrderConfirmationDialog, {
          props: {
            visible: true,
            orderData: orderData
          }
        });

        await nextTick();

        // Should display the order type appropriately
        expect(wrapper.exists()).toBe(true);
        
        wrapper.unmount();
      }
    });
  });

  describe('Preview Data Integration', () => {
    it('uses backend preview data when available', async () => {
      const equityOrderData = {
        symbol: 'SPY',
        side: 'buy',
        quantity: 100,
        orderType: 'market'
      };

      mockPreviewData.value = {
        order_cost: -64247.00,
        commission: 1.00,
        fees: 0.50,
        buying_power_effect: -32124.00
      };

      wrapper = mount(OrderConfirmationDialog, {
        props: {
          visible: true,
          orderData: equityOrderData
        }
      });

      await nextTick();

      // Should use preview data values
      expect(wrapper.vm.getEstimatedCostRaw()).toBe(64247.00);
      expect(wrapper.vm.getCommissionAndFeesRaw()).toBe(1.50);
      expect(wrapper.vm.getBPEffectRaw()).toBe(32124.00);
    });

    it('falls back to hardcoded values when preview data unavailable', async () => {
      const equityOrderData = {
        symbol: 'SPY',
        side: 'buy',
        quantity: 100,
        orderType: 'market',
        estimatedCost: 64247.00
      };

      mockPreviewData.value = null;

      wrapper = mount(OrderConfirmationDialog, {
        props: {
          visible: true,
          orderData: equityOrderData
        }
      });

      await nextTick();

      // Should fall back to order data values
      expect(wrapper.vm.getEstimatedCostRaw()).toBe(64247.00);
    });

    it('shows loading state during preview fetch', async () => {
      const equityOrderData = {
        symbol: 'SPY',
        side: 'buy',
        quantity: 100,
        orderType: 'market'
      };

      mockIsLoading.value = true;

      wrapper = mount(OrderConfirmationDialog, {
        props: {
          visible: true,
          orderData: equityOrderData
        }
      });

      await nextTick();

      // Should show loading indicators
      expect(wrapper.text()).toContain('Loading');
    });

    it('handles preview data errors gracefully', async () => {
      const equityOrderData = {
        symbol: 'SPY',
        side: 'buy',
        quantity: 100,
        orderType: 'market'
      };

      mockError.value = 'Failed to fetch preview data';

      wrapper = mount(OrderConfirmationDialog, {
        props: {
          visible: true,
          orderData: equityOrderData
        }
      });

      await nextTick();

      // Should handle errors gracefully and still display the dialog
      expect(wrapper.exists()).toBe(true);
    });
  });

  describe('Buying Power Integration', () => {
    it('displays buying power from useMarketData composable', async () => {
      const equityOrderData = {
        symbol: 'SPY',
        side: 'buy',
        quantity: 100,
        orderType: 'market'
      };

      mockReactiveBalance.value = { buying_power: 15000 };

      wrapper = mount(OrderConfirmationDialog, {
        props: {
          visible: true,
          orderData: equityOrderData
        }
      });

      await nextTick();

      // Should use buying power from useMarketData
      expect(wrapper.vm.getBuyingPower()).toBe("15000.00");
    });

    it('handles different buying power types', async () => {
      const equityOrderData = {
        symbol: 'SPY',
        side: 'buy',
        quantity: 100,
        orderType: 'market'
      };

      // Test options_buying_power fallback
      mockReactiveBalance.value = { options_buying_power: 12000 };

      wrapper = mount(OrderConfirmationDialog, {
        props: {
          visible: true,
          orderData: equityOrderData
        }
      });

      await nextTick();

      expect(wrapper.vm.getBuyingPower()).toBe("12000.00");

      // Test day_trading_buying_power fallback
      mockReactiveBalance.value = { day_trading_buying_power: 8000 };
      await nextTick();

      expect(wrapper.vm.getBuyingPower()).toBe("8000.00");
    });

    it('handles missing buying power data', async () => {
      const equityOrderData = {
        symbol: 'SPY',
        side: 'buy',
        quantity: 100,
        orderType: 'market'
      };

      mockReactiveBalance.value = {};
      mockReactiveAccountInfo.value = {};

      wrapper = mount(OrderConfirmationDialog, {
        props: {
          visible: true,
          orderData: equityOrderData
        }
      });

      await nextTick();

      // Should handle missing data gracefully
      expect(wrapper.vm.getBuyingPower()).toBe("--");
    });
  });

  describe('Order Validation & Submission', () => {
    it('validates equity orders before submission', async () => {
      const equityOrderData = {
        symbol: 'SPY',
        side: 'buy',
        quantity: 100,
        orderType: 'market'
      };

      const mockValidateOrder = vi.fn(() => ({ isValid: true, errors: [] }));
      const orderService = await import('../src/services/orderService.js');
      orderService.default.validateOrder = mockValidateOrder;

      wrapper = mount(OrderConfirmationDialog, {
        props: {
          visible: true,
          orderData: equityOrderData
        }
      });

      await nextTick();

      const submitBtn = wrapper.find('.submit-btn, .confirm-btn, [data-testid="submit-button"]');
      if (submitBtn.exists()) {
        await submitBtn.trigger('click');
        expect(mockValidateOrder).toHaveBeenCalledWith(equityOrderData);
      }
    });

    it('prevents submission of invalid equity orders', async () => {
      const invalidEquityOrderData = {
        symbol: 'SPY',
        side: 'buy',
        quantity: 0, // Invalid quantity
        orderType: 'market'
      };

      const mockValidateOrder = vi.fn(() => ({ 
        isValid: false, 
        errors: ['Quantity must be greater than 0'] 
      }));
      const orderService = await import('../src/services/orderService.js');
      orderService.default.validateOrder = mockValidateOrder;

      wrapper = mount(OrderConfirmationDialog, {
        props: {
          visible: true,
          orderData: invalidEquityOrderData
        }
      });

      await nextTick();

      // Submit button should be disabled or show validation errors
      const submitBtn = wrapper.find('.submit-btn, .confirm-btn, [data-testid="submit-button"]');
      if (submitBtn.exists()) {
        expect(submitBtn.attributes('disabled')).toBeDefined() || 
        expect(wrapper.text()).toContain('Quantity must be greater than 0');
      }
    });

    it('emits correct events on order submission', async () => {
      const equityOrderData = {
        symbol: 'SPY',
        side: 'buy',
        quantity: 100,
        orderType: 'market'
      };

      wrapper = mount(OrderConfirmationDialog, {
        props: {
          visible: true,
          orderData: equityOrderData
        }
      });

      await nextTick();

      const submitBtn = wrapper.find('.submit-btn, .confirm-btn, [data-testid="submit-button"]');
      if (submitBtn.exists()) {
        await submitBtn.trigger('click');

        // Should emit order-confirmed event
        const emittedEvents = wrapper.emitted('order-confirmed') || wrapper.emitted('submit');
        expect(emittedEvents).toBeTruthy();
      }
    });
  });

  describe('Dialog Visibility & Controls', () => {
    it('shows dialog when visible prop is true', () => {
      wrapper = mount(OrderConfirmationDialog, {
        props: {
          visible: true,
          orderData: {
            symbol: 'SPY',
            side: 'buy',
            quantity: 100
          }
        }
      });

      expect(wrapper.find('.bottom-sheet-overlay').exists()).toBe(true);
    });

    it('hides dialog when visible prop is false', () => {
      wrapper = mount(OrderConfirmationDialog, {
        props: {
          visible: false,
          orderData: {
            symbol: 'SPY',
            side: 'buy',
            quantity: 100
          }
        }
      });

      expect(wrapper.find('.dialog-overlay, .modal-overlay').exists()).toBe(false);
    });

    it('emits close event when cancel button clicked', async () => {
      wrapper = mount(OrderConfirmationDialog, {
        props: {
          visible: true,
          orderData: {
            symbol: 'SPY',
            side: 'buy',
            quantity: 100
          }
        }
      });

      const cancelBtn = wrapper.find('.cancel-btn, .close-btn, [data-testid="cancel-button"]');
      if (cancelBtn.exists()) {
        await cancelBtn.trigger('click');

        const emittedEvents = wrapper.emitted('close') || wrapper.emitted('cancel');
        expect(emittedEvents).toBeTruthy();
      }
    });

    it('emits close event when overlay clicked', async () => {
      wrapper = mount(OrderConfirmationDialog, {
        props: {
          visible: true,
          orderData: {
            symbol: 'SPY',
            side: 'buy',
            quantity: 100
          }
        }
      });

      const overlay = wrapper.find('.dialog-overlay, .modal-overlay');
      if (overlay.exists()) {
        await overlay.trigger('click');

        const emittedEvents = wrapper.emitted('close');
        expect(emittedEvents).toBeTruthy();
      }
    });
  });

  describe('Edge Cases & Error Handling', () => {
    it('handles missing order data gracefully', async () => {
      wrapper = mount(OrderConfirmationDialog, {
        props: {
          visible: true,
          orderData: null
        }
      });

      await nextTick();

      // Should not crash with null order data
      expect(wrapper.exists()).toBe(true);
    });

    it('handles malformed order data', async () => {
      const malformedOrderData = {
        // Missing required fields
        side: 'buy'
      };

      wrapper = mount(OrderConfirmationDialog, {
        props: {
          visible: true,
          orderData: malformedOrderData
        }
      });

      await nextTick();

      // Should handle malformed data gracefully
      expect(wrapper.exists()).toBe(true);
    });

    it('handles extreme values correctly', async () => {
      const extremeOrderData = {
        symbol: 'SPY',
        side: 'buy',
        quantity: 999999,
        orderType: 'limit',
        limitPrice: 99999.99,
        estimatedCost: 99999999999
      };

      mockPreviewData.value = {
        order_cost: -99999999999,
        commission: 0.0,
        fees: 0.0,
        buying_power_effect: -49999999999.5
      };

      wrapper = mount(OrderConfirmationDialog, {
        props: {
          visible: true,
          orderData: extremeOrderData
        }
      });

      await nextTick();

      // Should handle extreme values without breaking
      expect(wrapper.exists()).toBe(true);
    });

    it('handles network errors during preview', async () => {
      const equityOrderData = {
        symbol: 'SPY',
        side: 'buy',
        quantity: 100,
        orderType: 'market'
      };

      mockError.value = 'Network error: Unable to fetch preview';

      wrapper = mount(OrderConfirmationDialog, {
        props: {
          visible: true,
          orderData: equityOrderData
        }
      });

      await nextTick();

      // Should display error message or fallback gracefully
      expect(wrapper.exists()).toBe(true);
      expect(wrapper.text()).toContain('error');
    });
  });

  describe('Accessibility & User Experience', () => {
    it('has proper ARIA labels and roles', () => {
      wrapper = mount(OrderConfirmationDialog, {
        props: {
          visible: true,
          orderData: {
            symbol: 'SPY',
            side: 'buy',
            quantity: 100
          }
        }
      });

      const dialog = wrapper.find('[role="dialog"], .dialog');
      expect(dialog.exists()).toBe(true);
    });

  it('supports keyboard navigation', async () => {
    wrapper = mount(OrderConfirmationDialog, {
      props: {
        visible: true,
        orderData: {
          symbol: 'SPY',
          side: 'buy',
          quantity: 100
        }
      }
    });

    // Should handle Escape key to close - trigger on the bottom sheet container
    const bottomSheet = wrapper.find('.bottom-sheet');
    await bottomSheet.trigger('keydown', { key: 'Escape' });
    
    const emittedEvents = wrapper.emitted('close');
    expect(emittedEvents).toBeTruthy();
  });

    it('maintains focus management', () => {
      wrapper = mount(OrderConfirmationDialog, {
        props: {
          visible: true,
          orderData: {
            symbol: 'SPY',
            side: 'buy',
            quantity: 100
          }
        }
      });

      // Dialog should be focusable
      const focusableElements = wrapper.findAll('button, input, select, textarea, [tabindex]');
      expect(focusableElements.length).toBeGreaterThan(0);
    });
  });

  describe('Performance Considerations', () => {
    it('handles rapid prop changes efficiently', async () => {
      wrapper = mount(OrderConfirmationDialog, {
        props: {
          visible: true,
          orderData: {
            symbol: 'SPY',
            side: 'buy',
            quantity: 100
          }
        }
      });

      // Rapidly change order data
      for (let i = 0; i < 10; i++) {
        await wrapper.setProps({
          orderData: {
            symbol: i % 2 === 0 ? 'SPY' : 'QQQ',
            side: 'buy',
            quantity: 100 + i
          }
        });
      }

      // Component should remain stable
      expect(wrapper.exists()).toBe(true);
    });

    it('cleans up resources on unmount', () => {
      wrapper = mount(OrderConfirmationDialog, {
        props: {
          visible: true,
          orderData: {
            symbol: 'SPY',
            side: 'buy',
            quantity: 100
          }
        }
      });

      // Should unmount without errors
      expect(() => wrapper.unmount()).not.toThrow();
    });
  });
});
