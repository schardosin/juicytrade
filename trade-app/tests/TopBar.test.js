import { describe, it, expect, beforeEach, vi } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import { ref, nextTick } from 'vue';

// === Controllable mock refs ===
const mockBalance = ref(null);
const mockAccountInfo = ref(null);
const mockAvailableProviders = ref(null);
const mockProviderConfig = ref(null);
const mockIsLoadingMap = {};
const mockErrorMap = {};
const mockUpdateProviderConfig = vi.fn();
const mockRefreshBalance = vi.fn();
const mockLookupSymbols = vi.fn().mockResolvedValue([]);

// Mock useMarketData composable
vi.mock('../src/composables/useMarketData.js', () => ({
  useMarketData: () => ({
    lookupSymbols: mockLookupSymbols,
    getBalance: () => mockBalance,
    getAccountInfo: () => mockAccountInfo,
    getAvailableProviders: () => mockAvailableProviders,
    getProviderConfig: () => mockProviderConfig,
    updateProviderConfig: mockUpdateProviderConfig,
    refreshBalance: mockRefreshBalance,
    isLoading: (key) => {
      if (!mockIsLoadingMap[key]) mockIsLoadingMap[key] = ref(false);
      return mockIsLoadingMap[key];
    },
    getError: (key) => {
      if (!mockErrorMap[key]) mockErrorMap[key] = ref(null);
      return mockErrorMap[key];
    }
  })
}));

// Mock useMobileDetection
vi.mock('../src/composables/useMobileDetection.js', () => ({
  useMobileDetection: () => ({
    isMobile: ref(false),
    isTablet: ref(false),
    isDesktop: ref(true)
  })
}));

// Mock vue-router
const mockPush = vi.fn();
vi.mock('vue-router', () => ({
  useRouter: () => ({ push: mockPush }),
  useRoute: () => ({ path: '/trade' })
}));

// Mock webSocketClient
vi.mock('../src/services/webSocketClient', () => ({
  default: {
    isConnected: ref(false),
    onPriceUpdate: vi.fn(),
    onGreeksUpdate: vi.fn()
  }
}));

// Mock axios
vi.mock('axios', () => ({
  default: {
    get: vi.fn().mockResolvedValue({ data: { success: true, data: { strategies_connected: false } } })
  }
}));

// Mock child components to avoid complex dependencies
vi.mock('../src/components/SettingsDialog.vue', () => ({
  default: {
    name: 'SettingsDialog',
    template: '<div class="settings-dialog-mock"></div>',
    props: ['visible']
  }
}));

vi.mock('../src/components/MobileSearchOverlay.vue', () => ({
  default: {
    name: 'MobileSearchOverlay',
    template: '<div class="mobile-search-mock"></div>',
    props: ['visible'],
    emits: ['close', 'symbol-selected']
  }
}));

import TopBar from '../src/components/TopBar.vue';

// === Test Helpers ===

// Two providers with trade_account capability, one without
const twoTradeProviders = {
  'alpaca_paper': {
    display_name: 'Alpaca',
    paper: true,
    capabilities: { rest: ['trade_account', 'stock_quotes', 'options_chain'] }
  },
  'tastytrade_live': {
    display_name: 'TastyTrade',
    paper: false,
    capabilities: { rest: ['trade_account', 'stock_quotes'] }
  },
  'polygon_data': {
    display_name: 'Polygon',
    paper: false,
    capabilities: { rest: ['stock_quotes', 'historical_data'] }
  }
};

const baseConfig = {
  trade_account: 'alpaca_paper',
  stock_quotes: 'polygon_data',
  options_chain: 'alpaca_paper',
  historical_data: 'polygon_data'
};

const createWrapper = () => {
  return mount(TopBar, {
    global: {
      stubs: {
        InputText: { template: '<input />', props: ['modelValue'] },
        Button: { template: '<button><slot /></button>', props: ['icon'] },
        Menu: { template: '<div />', props: ['model', 'popup'], methods: { toggle: vi.fn() } },
        Dropdown: {
          template: '<div class="p-dropdown-mock"><slot /></div>',
          props: ['modelValue', 'options', 'optionLabel', 'optionValue', 'loading', 'disabled', 'placeholder', 'appendTo'],
          emits: ['update:modelValue', 'change', 'show', 'hide']
        }
      }
    }
  });
};

describe('TopBar - Trade Account Switching', () => {
  let wrapper;

  beforeEach(() => {
    vi.clearAllMocks();
    vi.useFakeTimers();

    // Reset mock data to good defaults
    mockBalance.value = { portfolio_value: 50000, buying_power: 25000 };
    mockAccountInfo.value = null;
    mockAvailableProviders.value = { ...twoTradeProviders };
    mockProviderConfig.value = { ...baseConfig };
    mockUpdateProviderConfig.mockResolvedValue(undefined);
    mockRefreshBalance.mockResolvedValue(undefined);

    // Reset loading/error maps
    Object.keys(mockIsLoadingMap).forEach(k => { mockIsLoadingMap[k].value = false; });
    Object.keys(mockErrorMap).forEach(k => { mockErrorMap[k].value = null; });
  });

  afterEach(() => {
    vi.useRealTimers();
    if (wrapper) {
      wrapper.unmount();
      wrapper = null;
    }
  });

  // =============================================================
  // 1. tradeAccountOptions computed property
  // =============================================================
  describe('tradeAccountOptions computed', () => {
    it('includes providers with trade_account capability', async () => {
      wrapper = createWrapper();
      await nextTick();

      const options = wrapper.vm.tradeAccountOptions;
      expect(options).toHaveLength(2);

      const values = options.map(o => o.value);
      expect(values).toContain('alpaca_paper');
      expect(values).toContain('tastytrade_live');
    });

    it('excludes providers without trade_account capability', async () => {
      wrapper = createWrapper();
      await nextTick();

      const options = wrapper.vm.tradeAccountOptions;
      const values = options.map(o => o.value);
      expect(values).not.toContain('polygon_data');
    });

    it('returns empty array when providers are null', async () => {
      mockAvailableProviders.value = null;
      wrapper = createWrapper();
      await nextTick();

      expect(wrapper.vm.tradeAccountOptions).toEqual([]);
    });

    it('builds options with correct shape (label, value, paper, displayName)', async () => {
      wrapper = createWrapper();
      await nextTick();

      const options = wrapper.vm.tradeAccountOptions;
      const alpacaOpt = options.find(o => o.value === 'alpaca_paper');
      const tastyOpt = options.find(o => o.value === 'tastytrade_live');

      expect(alpacaOpt).toEqual({
        label: 'Alpaca (Paper)',
        value: 'alpaca_paper',
        paper: true,
        displayName: 'Alpaca'
      });

      expect(tastyOpt).toEqual({
        label: 'TastyTrade (Live)',
        value: 'tastytrade_live',
        paper: false,
        displayName: 'TastyTrade'
      });
    });

    it('uses instanceId as fallback when display_name is missing', async () => {
      mockAvailableProviders.value = {
        'custom_broker': {
          paper: true,
          capabilities: { rest: ['trade_account'] }
        }
      };
      wrapper = createWrapper();
      await nextTick();

      const options = wrapper.vm.tradeAccountOptions;
      expect(options).toHaveLength(1);
      expect(options[0].label).toBe('custom_broker (Paper)');
      expect(options[0].displayName).toBe('custom_broker');
    });

    it('updates reactively when providers change after mount', async () => {
      wrapper = createWrapper();
      await nextTick();

      // Initial: 2 trade-capable providers
      expect(wrapper.vm.tradeAccountOptions).toHaveLength(2);

      // Add a third trade-capable provider after mount
      mockAvailableProviders.value = {
        ...twoTradeProviders,
        'schwab_live': {
          display_name: 'Schwab',
          paper: false,
          capabilities: { rest: ['trade_account', 'stock_quotes'] }
        }
      };
      await nextTick();

      // Computed should reactively update to include the new provider
      const options = wrapper.vm.tradeAccountOptions;
      expect(options).toHaveLength(3);

      const values = options.map(o => o.value);
      expect(values).toContain('alpaca_paper');
      expect(values).toContain('tastytrade_live');
      expect(values).toContain('schwab_live');
    });
  });

  // =============================================================
  // 2. tradeDropdownDisabled computed
  // =============================================================
  describe('tradeDropdownDisabled computed', () => {
    it('is disabled when switchingAccount is true', async () => {
      wrapper = createWrapper();
      await nextTick();

      // Multiple options exist but switching in progress
      wrapper.vm.switchingAccount = true;
      await nextTick();

      expect(wrapper.vm.tradeDropdownDisabled).toBe(true);
    });

    it('is disabled when only 1 trade-capable provider exists', async () => {
      mockAvailableProviders.value = {
        'alpaca_paper': {
          display_name: 'Alpaca',
          paper: true,
          capabilities: { rest: ['trade_account'] }
        }
      };
      wrapper = createWrapper();
      await nextTick();

      expect(wrapper.vm.tradeAccountOptions).toHaveLength(1);
      expect(wrapper.vm.tradeDropdownDisabled).toBe(true);
    });

    it('is disabled when zero trade-capable providers exist', async () => {
      mockAvailableProviders.value = {
        'polygon_data': {
          display_name: 'Polygon',
          paper: false,
          capabilities: { rest: ['stock_quotes'] }
        }
      };
      wrapper = createWrapper();
      await nextTick();

      expect(wrapper.vm.tradeAccountOptions).toHaveLength(0);
      expect(wrapper.vm.tradeDropdownDisabled).toBe(true);
    });

    it('is NOT disabled when multiple options and not switching', async () => {
      wrapper = createWrapper();
      await nextTick();

      expect(wrapper.vm.tradeAccountOptions.length).toBeGreaterThan(1);
      expect(wrapper.vm.switchingAccount).toBe(false);
      expect(wrapper.vm.tradeDropdownDisabled).toBe(false);
    });
  });

  // =============================================================
  // 3. handleTradeAccountSwitch method
  // =============================================================
  describe('handleTradeAccountSwitch', () => {
    it('calls updateProviderConfig with full config (only trade_account changed) and refreshBalance on success', async () => {
      wrapper = createWrapper();
      await nextTick();

      await wrapper.vm.handleTradeAccountSwitch({ value: 'tastytrade_live' });
      await flushPromises();

      // Verify updateProviderConfig called with full config, only trade_account changed
      expect(mockUpdateProviderConfig).toHaveBeenCalledTimes(1);
      const sentConfig = mockUpdateProviderConfig.mock.calls[0][0];
      expect(sentConfig.trade_account).toBe('tastytrade_live');
      expect(sentConfig.stock_quotes).toBe('polygon_data');
      expect(sentConfig.options_chain).toBe('alpaca_paper');
      expect(sentConfig.historical_data).toBe('polygon_data');

      // Verify refreshBalance called
      expect(mockRefreshBalance).toHaveBeenCalledTimes(1);
    });

    it('is a no-op when same account is selected', async () => {
      wrapper = createWrapper();
      await nextTick();

      await wrapper.vm.handleTradeAccountSwitch({ value: 'alpaca_paper' });
      await flushPromises();

      expect(mockUpdateProviderConfig).not.toHaveBeenCalled();
      expect(mockRefreshBalance).not.toHaveBeenCalled();
    });

    it('is a no-op when event value is null', async () => {
      wrapper = createWrapper();
      await nextTick();

      await wrapper.vm.handleTradeAccountSwitch({ value: null });
      await flushPromises();

      expect(mockUpdateProviderConfig).not.toHaveBeenCalled();
    });

    it('sets switchingAccount during the operation', async () => {
      // Make updateProviderConfig take time to resolve
      let resolveUpdate;
      mockUpdateProviderConfig.mockReturnValue(new Promise(r => { resolveUpdate = r; }));

      wrapper = createWrapper();
      await nextTick();

      const promise = wrapper.vm.handleTradeAccountSwitch({ value: 'tastytrade_live' });

      // Should be switching now
      expect(wrapper.vm.switchingAccount).toBe(true);

      resolveUpdate();
      await promise;
      await flushPromises();

      // Should be done switching
      expect(wrapper.vm.switchingAccount).toBe(false);
    });

    it('reverts selectedTradeAccount and sets switchError on failure', async () => {
      mockUpdateProviderConfig.mockRejectedValue(new Error('Network error'));

      wrapper = createWrapper();
      await nextTick();

      // The watcher should have synced this from config
      expect(wrapper.vm.selectedTradeAccount).toBe('alpaca_paper');

      await wrapper.vm.handleTradeAccountSwitch({ value: 'tastytrade_live' });
      await flushPromises();

      // Should revert to previous account
      expect(wrapper.vm.selectedTradeAccount).toBe('alpaca_paper');
      // Should set error
      expect(wrapper.vm.switchError).toBe('Failed to switch account. Please try again.');
      // switchingAccount should be cleared
      expect(wrapper.vm.switchingAccount).toBe(false);
    });

    it('auto-clears switchError after 4 seconds', async () => {
      mockUpdateProviderConfig.mockRejectedValue(new Error('API error'));

      wrapper = createWrapper();
      await nextTick();

      await wrapper.vm.handleTradeAccountSwitch({ value: 'tastytrade_live' });
      await flushPromises();

      expect(wrapper.vm.switchError).toBe('Failed to switch account. Please try again.');

      // Advance 4 seconds
      vi.advanceTimersByTime(4000);

      expect(wrapper.vm.switchError).toBeNull();
    });

    it('updates trade account indicator after successful switch', async () => {
      wrapper = createWrapper();
      await nextTick();

      // Initial state: Alpaca Paper
      expect(wrapper.vm.tradeAccountName).toBe('Alpaca');
      expect(wrapper.vm.tradeAccountType).toBe('Paper');
      expect(wrapper.vm.tradeAccountTypeClass).toBe('type-paper');

      // Trigger the switch
      await wrapper.vm.handleTradeAccountSwitch({ value: 'tastytrade_live' });
      await flushPromises();

      // Simulate the config update that the backend would cause
      mockProviderConfig.value = { ...baseConfig, trade_account: 'tastytrade_live' };
      await nextTick();

      // Computed properties should reactively update
      expect(wrapper.vm.tradeAccountName).toBe('TastyTrade');
      expect(wrapper.vm.tradeAccountType).toBe('Live');
      expect(wrapper.vm.tradeAccountTypeClass).toBe('type-live');
    });

    it('updates Net Liq and Buying Power after successful switch', async () => {
      wrapper = createWrapper();
      await nextTick();

      // Initial balance from beforeEach: 50000 / 25000
      expect(wrapper.vm.netLiquidation).toBe(50000);
      expect(wrapper.vm.buyingPower).toBe(25000);

      // Trigger the switch
      await wrapper.vm.handleTradeAccountSwitch({ value: 'tastytrade_live' });
      await flushPromises();

      // Simulate new balance data arriving after the switch
      mockBalance.value = { portfolio_value: 75000, buying_power: 40000 };
      await nextTick();

      // Balance display should reactively update
      expect(wrapper.vm.netLiquidation).toBe(75000);
      expect(wrapper.vm.buyingPower).toBe(40000);
    });

    it('allows concurrent calls since no re-entry guard exists (documents behavior)', async () => {
      // The handler checks newInstanceId against config.trade_account, NOT against
      // switchingAccount. Since config isn't updated until the call resolves, a second
      // call with the same target passes the guard and triggers another API call.
      let resolveFirst, resolveSecond;
      mockUpdateProviderConfig
        .mockReturnValueOnce(new Promise(r => { resolveFirst = r; }))
        .mockReturnValueOnce(new Promise(r => { resolveSecond = r; }));

      wrapper = createWrapper();
      await nextTick();

      // First call — sets switchingAccount=true, calls updateProviderConfig
      const promise1 = wrapper.vm.handleTradeAccountSwitch({ value: 'tastytrade_live' });
      expect(wrapper.vm.switchingAccount).toBe(true);

      // Second call while first is pending — config hasn't changed yet,
      // so 'tastytrade_live' !== 'alpaca_paper' still passes the guard
      const promise2 = wrapper.vm.handleTradeAccountSwitch({ value: 'tastytrade_live' });

      // Both calls made it through — no re-entry guard
      expect(mockUpdateProviderConfig).toHaveBeenCalledTimes(2);

      // Clean up: resolve both promises
      resolveFirst();
      resolveSecond();
      await promise1;
      await promise2;
      await flushPromises();
    });

    it('handles gracefully when provider config is null', async () => {
      mockProviderConfig.value = null;
      wrapper = createWrapper();
      await nextTick();

      await wrapper.vm.handleTradeAccountSwitch({ value: 'tastytrade_live' });
      await flushPromises();

      // Should proceed: null?.trade_account is undefined !== 'tastytrade_live'
      expect(mockUpdateProviderConfig).toHaveBeenCalledTimes(1);

      // Spreading null produces empty object, so only trade_account is set
      const sentConfig = mockUpdateProviderConfig.mock.calls[0][0];
      expect(sentConfig).toEqual({ trade_account: 'tastytrade_live' });

      // No error thrown, refreshBalance called
      expect(mockRefreshBalance).toHaveBeenCalledTimes(1);
      expect(wrapper.vm.switchError).toBeNull();
      expect(wrapper.vm.switchingAccount).toBe(false);
    });

    it('closes tooltip after 300ms on success', async () => {
      wrapper = createWrapper();
      await nextTick();

      // Open tooltip
      wrapper.vm.showProviderTooltip = true;
      wrapper.vm.dropdownOpen = true;

      await wrapper.vm.handleTradeAccountSwitch({ value: 'tastytrade_live' });
      await flushPromises();

      // Not closed yet
      expect(wrapper.vm.showProviderTooltip).toBe(true);

      // Advance 300ms
      vi.advanceTimersByTime(300);

      expect(wrapper.vm.showProviderTooltip).toBe(false);
      expect(wrapper.vm.dropdownOpen).toBe(false);
    });
  });

  // =============================================================
  // 4. onTooltipAreaLeave method
  // =============================================================
  describe('onTooltipAreaLeave', () => {
    it('keeps tooltip open when dropdownOpen is true', async () => {
      wrapper = createWrapper();
      await nextTick();

      wrapper.vm.showProviderTooltip = true;
      wrapper.vm.dropdownOpen = true;

      wrapper.vm.onTooltipAreaLeave();

      expect(wrapper.vm.showProviderTooltip).toBe(true);
    });

    it('closes tooltip when dropdownOpen is false', async () => {
      wrapper = createWrapper();
      await nextTick();

      wrapper.vm.showProviderTooltip = true;
      wrapper.vm.dropdownOpen = false;

      wrapper.vm.onTooltipAreaLeave();

      expect(wrapper.vm.showProviderTooltip).toBe(false);
    });
  });

  // =============================================================
  // 5. Watcher: selectedTradeAccount sync
  // =============================================================
  describe('selectedTradeAccount sync watcher', () => {
    it('syncs selectedTradeAccount from provider config on mount', async () => {
      wrapper = createWrapper();
      await nextTick();

      // The immediate watcher should have picked up the initial config
      expect(wrapper.vm.selectedTradeAccount).toBe('alpaca_paper');
    });

    it('updates selectedTradeAccount when config changes externally', async () => {
      wrapper = createWrapper();
      await nextTick();

      expect(wrapper.vm.selectedTradeAccount).toBe('alpaca_paper');

      // Simulate external config change (e.g., Settings dialog)
      mockProviderConfig.value = { ...baseConfig, trade_account: 'tastytrade_live' };
      await nextTick();

      expect(wrapper.vm.selectedTradeAccount).toBe('tastytrade_live');
    });

    it('does NOT update selectedTradeAccount while switching is in progress', async () => {
      wrapper = createWrapper();
      await nextTick();

      expect(wrapper.vm.selectedTradeAccount).toBe('alpaca_paper');

      // Simulate switching in progress
      wrapper.vm.switchingAccount = true;
      await nextTick();

      // External config change during switch
      mockProviderConfig.value = { ...baseConfig, trade_account: 'tastytrade_live' };
      await nextTick();

      // Should NOT update because switchingAccount is true
      expect(wrapper.vm.selectedTradeAccount).toBe('alpaca_paper');
    });
  });

  // =============================================================
  // 6. Template rendering
  // =============================================================
  describe('Template rendering', () => {
    it('renders Dropdown in tooltip when showProviderTooltip is true', async () => {
      wrapper = createWrapper();
      await nextTick();

      // Tooltip hidden initially
      expect(wrapper.find('.provider-tooltip').exists()).toBe(false);

      // Show tooltip
      wrapper.vm.showProviderTooltip = true;
      await nextTick();

      expect(wrapper.find('.provider-tooltip').exists()).toBe(true);
      expect(wrapper.find('.trade-account-dropdown-wrapper').exists()).toBe(true);
      expect(wrapper.find('.p-dropdown-mock').exists()).toBe(true);
    });

    it('renders error message when switchError is set', async () => {
      wrapper = createWrapper();
      await nextTick();

      wrapper.vm.showProviderTooltip = true;
      wrapper.vm.switchError = 'Failed to switch account. Please try again.';
      await nextTick();

      const errorEl = wrapper.find('.switch-error');
      expect(errorEl.exists()).toBe(true);
      expect(errorEl.text()).toContain('Failed to switch account');
    });

    it('does not render error message when switchError is null', async () => {
      wrapper = createWrapper();
      await nextTick();

      wrapper.vm.showProviderTooltip = true;
      await nextTick();

      expect(wrapper.find('.switch-error').exists()).toBe(false);
    });

    it('renders market data service rows as static labels (not dropdowns)', async () => {
      wrapper = createWrapper();
      await nextTick();

      wrapper.vm.showProviderTooltip = true;
      await nextTick();

      // Find the Market Data category
      const categories = wrapper.findAll('.provider-category');
      // Second category is Market Data
      const marketDataCategory = categories[1];
      expect(marketDataCategory).toBeDefined();

      // Market data items should have .provider-name spans (static), not dropdowns
      const providerNames = marketDataCategory.findAll('.provider-name');
      expect(providerNames.length).toBeGreaterThan(0);

      // No dropdown wrappers in market data section
      const dropdowns = marketDataCategory.findAll('.trade-account-dropdown-wrapper');
      expect(dropdowns).toHaveLength(0);
    });

    it('does not render tooltip when providersLoading is true', async () => {
      wrapper = createWrapper();
      await nextTick();

      wrapper.vm.showProviderTooltip = true;
      await nextTick();

      // Tooltip should be visible initially
      expect(wrapper.find('.provider-tooltip').exists()).toBe(true);

      // Simulate providers loading
      mockIsLoadingMap['providers.available'].value = true;
      await nextTick();

      // Tooltip should be hidden due to loading guard
      expect(wrapper.find('.provider-tooltip').exists()).toBe(false);
    });

    it('does not render tooltip when providersError is set', async () => {
      wrapper = createWrapper();
      await nextTick();

      wrapper.vm.showProviderTooltip = true;
      await nextTick();

      // Tooltip should be visible initially
      expect(wrapper.find('.provider-tooltip').exists()).toBe(true);

      // Simulate providers error
      mockErrorMap['providers.available'].value = 'Failed to load providers';
      await nextTick();

      // Tooltip should be hidden due to error guard
      expect(wrapper.find('.provider-tooltip').exists()).toBe(false);
    });

    it('passes correct props to Dropdown component', async () => {
      wrapper = createWrapper();
      await nextTick();

      // Show tooltip so Dropdown renders
      wrapper.vm.showProviderTooltip = true;
      await nextTick();

      const dropdown = wrapper.findComponent('.p-dropdown-mock');
      expect(dropdown.exists()).toBe(true);

      // Verify options match tradeAccountOptions
      expect(dropdown.props('options')).toEqual(wrapper.vm.tradeAccountOptions);
      expect(dropdown.props('options')).toHaveLength(2);

      // Verify disabled is false (2 providers, not switching)
      expect(dropdown.props('disabled')).toBe(false);

      // Verify loading is false (not switching)
      expect(dropdown.props('loading')).toBe(false);

      // Verify modelValue matches selected account
      expect(dropdown.props('modelValue')).toBe('alpaca_paper');
    });

    it('Dropdown shows disabled state when single trade-capable provider', async () => {
      mockAvailableProviders.value = {
        'alpaca_paper': {
          display_name: 'Alpaca',
          paper: true,
          capabilities: { rest: ['trade_account', 'stock_quotes'] }
        }
      };

      wrapper = createWrapper();
      await nextTick();

      // Show tooltip so Dropdown renders
      wrapper.vm.showProviderTooltip = true;
      await nextTick();

      const dropdown = wrapper.findComponent('.p-dropdown-mock');
      expect(dropdown.exists()).toBe(true);

      // Dropdown should be disabled with only 1 trade-capable provider
      expect(dropdown.props('disabled')).toBe(true);
    });

    it('renders trade account indicator with correct name and type', async () => {
      wrapper = createWrapper();
      await nextTick();

      const indicator = wrapper.find('.trade-account-indicator');
      expect(indicator.exists()).toBe(true);
      expect(indicator.text()).toContain('Alpaca');
      expect(indicator.text()).toContain('Paper');
    });
  });
});
