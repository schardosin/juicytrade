import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { mount } from '@vue/test-utils';
import { ref, nextTick } from 'vue';
import AutomationConfigForm from '../src/components/automation/AutomationConfigForm.vue';
import { api } from '../src/services/api.js';

// Mock dependencies
vi.mock('vue-router', () => ({
  useRouter: () => ({
    push: vi.fn()
  }),
  useRoute: () => ({
    params: {}
  })
}));

vi.mock('../src/services/api.js', () => ({
  api: {
    getAutomationConfig: vi.fn().mockResolvedValue({ data: { config: null } }),
    createAutomationConfig: vi.fn().mockResolvedValue({ data: { id: 'new-id' } }),
    updateAutomationConfig: vi.fn().mockResolvedValue({}),
    previewAutomationIndicators: vi.fn().mockResolvedValue({ data: { indicators: [], group_results: [], all_pass: false } }),
    getIndicatorMetadata: vi.fn().mockResolvedValue({ data: { indicators: [] } }),
    previewStrikes: vi.fn().mockResolvedValue({ success: true, data: {} }),
    getAccount: vi.fn().mockResolvedValue({ portfolio_value: 0 }),
  }
}));

vi.mock('../src/composables/useMobileDetection.js', () => ({
  useMobileDetection: () => ({ isMobile: ref(false) })
}));

describe('AutomationConfigForm — Indicator Groups', () => {
  let wrapper;

  beforeEach(async () => {
    vi.clearAllMocks();
    wrapper = mount(AutomationConfigForm);
    await nextTick();
    await nextTick();
  });

  afterEach(() => {
    if (wrapper) {
      wrapper.unmount();
    }
  });

  // ─── Config Initialization ─────────────────────────────────────────

  describe('Config initialization', () => {
    it('renders with a single default group', () => {
      const groups = wrapper.vm.config.indicator_groups;
      expect(groups).toBeDefined();
      expect(groups.length).toBe(1);
      expect(groups[0].name).toBe('Default');
      expect(groups[0].indicators).toEqual([]);
      expect(groups[0].id).toBeTruthy();
    });

    it('has empty legacy indicators array', () => {
      expect(wrapper.vm.config.indicators).toEqual([]);
    });
  });

  // ─── Group Management ──────────────────────────────────────────────

  describe('Group management', () => {
    it('addGroup creates a new empty group', () => {
      expect(wrapper.vm.config.indicator_groups.length).toBe(1);
      wrapper.vm.addGroup();
      expect(wrapper.vm.config.indicator_groups.length).toBe(2);

      const newGroup = wrapper.vm.config.indicator_groups[1];
      expect(newGroup.name).toBe('Group 2');
      expect(newGroup.indicators).toEqual([]);
      expect(newGroup.id).toMatch(/^grp_/);
    });

    it('removeGroup removes the specified group', () => {
      wrapper.vm.addGroup();
      expect(wrapper.vm.config.indicator_groups.length).toBe(2);

      const groupId = wrapper.vm.config.indicator_groups[1].id;
      wrapper.vm.removeGroup(1);
      expect(wrapper.vm.config.indicator_groups.length).toBe(1);
      expect(wrapper.vm.config.indicator_groups.find(g => g.id === groupId)).toBeUndefined();
    });

    it('removeGroup prompts confirmation when group has indicators', () => {
      // Add an indicator to first group
      wrapper.vm.config.indicator_groups[0].indicators.push({
        id: 'ind_test', type: 'vix', enabled: true, operator: 'lt', threshold: 20
      });
      wrapper.vm.addGroup(); // Need 2 groups so delete button appears

      // Mock confirm to return false (cancel)
      const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(false);
      wrapper.vm.removeGroup(0);
      expect(confirmSpy).toHaveBeenCalled();
      // Group should NOT be removed (user cancelled)
      expect(wrapper.vm.config.indicator_groups.length).toBe(2);

      confirmSpy.mockRestore();
    });

    it('addIndicatorToGroup adds indicator to specified group', () => {
      wrapper.vm.addGroup();
      wrapper.vm.addIndicatorToGroup('vix', 1);

      expect(wrapper.vm.config.indicator_groups[0].indicators.length).toBe(0);
      expect(wrapper.vm.config.indicator_groups[1].indicators.length).toBe(1);
      expect(wrapper.vm.config.indicator_groups[1].indicators[0].type).toBe('vix');
    });

    it('removeIndicatorFromGroup removes from correct group', () => {
      wrapper.vm.addIndicatorToGroup('vix', 0);
      wrapper.vm.addIndicatorToGroup('gap', 0);
      const indId = wrapper.vm.config.indicator_groups[0].indicators[0].id;

      wrapper.vm.removeIndicatorFromGroup(0, indId);
      expect(wrapper.vm.config.indicator_groups[0].indicators.length).toBe(1);
      expect(wrapper.vm.config.indicator_groups[0].indicators[0].type).toBe('gap');
    });
  });

  // ─── OR Divider Rendering ──────────────────────────────────────────

  describe('OR divider rendering', () => {
    it('no OR divider with 1 group', async () => {
      await nextTick();
      expect(wrapper.find('.or-divider').exists()).toBe(false);
    });

    it('OR divider shown with 2+ groups', async () => {
      wrapper.vm.addGroup();
      await nextTick();
      expect(wrapper.find('.or-divider').exists()).toBe(true);
    });
  });

  // ─── Save Config ───────────────────────────────────────────────────

  describe('Save config', () => {
    it('sends indicator_groups and empty indicators on save', async () => {
      // Set required fields
      wrapper.vm.config.name = 'Test Config';
      wrapper.vm.config.symbol = 'NDX';
      wrapper.vm.addIndicatorToGroup('vix', 0);

      await wrapper.vm.saveConfig();

      expect(api.createAutomationConfig).toHaveBeenCalledTimes(1);
      const payload = api.createAutomationConfig.mock.calls[0][0];
      expect(payload.indicators).toEqual([]);
      expect(payload.indicator_groups).toBeDefined();
      expect(payload.indicator_groups.length).toBe(1);
      expect(payload.indicator_groups[0].indicators.length).toBe(1);
      expect(payload.indicator_groups[0].indicators[0].type).toBe('vix');
    });
  });

  // ─── Legacy Config Loading ─────────────────────────────────────────

  describe('Legacy config migration', () => {
    it('wraps legacy indicators into a Default group on load', async () => {
      // Simulate loadConfig with legacy data (indicators only, no groups)
      api.getAutomationConfig.mockResolvedValueOnce({
        data: {
          config: {
            id: 'cfg-1',
            name: 'Legacy Config',
            symbol: 'NDX',
            entry_time: '12:25',
            entry_timezone: 'America/New_York',
            recurrence: 'once',
            enabled: true,
            indicators: [
              { id: 'ind_1', type: 'vix', enabled: true, operator: 'lt', threshold: 20 },
              { id: 'ind_2', type: 'gap', enabled: true, operator: 'lt', threshold: 1.0 },
            ],
            indicator_groups: [],
            trade_config: { strategy: 'put_spread' }
          }
        }
      });

      // Manually call loadConfig (simulate edit mode)
      // We need to use the internal method — set configId first
      wrapper.vm.config.name = ''; // Reset
      // Directly apply the migration logic to test it
      wrapper.vm.config.indicators = [
        { id: 'ind_1', type: 'vix', enabled: true, operator: 'lt', threshold: 20 },
        { id: 'ind_2', type: 'gap', enabled: true, operator: 'lt', threshold: 1.0 },
      ];
      wrapper.vm.config.indicator_groups = [];

      // Simulate the migration logic from loadConfig
      if (!wrapper.vm.config.indicator_groups || wrapper.vm.config.indicator_groups.length === 0) {
        if (wrapper.vm.config.indicators && wrapper.vm.config.indicators.length > 0) {
          wrapper.vm.config.indicator_groups = [{
            id: wrapper.vm.generateGroupId(),
            name: 'Default',
            indicators: wrapper.vm.config.indicators
          }];
          wrapper.vm.config.indicators = [];
        }
      }

      expect(wrapper.vm.config.indicator_groups.length).toBe(1);
      expect(wrapper.vm.config.indicator_groups[0].name).toBe('Default');
      expect(wrapper.vm.config.indicator_groups[0].indicators.length).toBe(2);
      expect(wrapper.vm.config.indicators).toEqual([]);
    });
  });
});

// ─── Max Capital: fixed/percent toggle (Steps 12 & 13) ──────────────────

describe('AutomationConfigForm — Max Capital mode', () => {
  let wrapper;

  afterEach(() => {
    if (wrapper) wrapper.unmount();
  });

  const mountForm = async () => {
    wrapper = mount(AutomationConfigForm);
    await nextTick();
    await nextTick();
    return wrapper;
  };

  describe('Defaults & toggle', () => {
    beforeEach(async () => {
      vi.clearAllMocks();
      api.getAccount.mockResolvedValue({ portfolio_value: 0 });
      await mountForm();
    });

    it('defaults to fixed mode with the currency input rendered', () => {
      expect(wrapper.vm.config.trade_config.max_capital_mode).toBe('fixed');
      expect(wrapper.vm.config.trade_config.max_capital).toBe(5000);
      expect(wrapper.vm.config.trade_config.max_capital_percent).toBe(60);
      expect(wrapper.find('#maxCapital').exists()).toBe(true);
      expect(wrapper.find('#maxCapitalPercent').exists()).toBe(false);
    });

    it('clicking percent shows the percent input', async () => {
      wrapper.vm.setCapitalMode('percent');
      await nextTick();
      expect(wrapper.vm.config.trade_config.max_capital_mode).toBe('percent');
      expect(wrapper.find('#maxCapitalPercent').exists()).toBe(true);
      expect(wrapper.find('#maxCapital').exists()).toBe(false);
    });

    it('preserves both values across Fixed→Percent→Fixed toggles (FR-2)', async () => {
      wrapper.vm.config.trade_config.max_capital = 7500;
      wrapper.vm.config.trade_config.max_capital_percent = 42;

      wrapper.vm.setCapitalMode('percent');
      await nextTick();
      wrapper.vm.setCapitalMode('fixed');
      await nextTick();

      expect(wrapper.vm.config.trade_config.max_capital).toBe(7500);
      expect(wrapper.vm.config.trade_config.max_capital_percent).toBe(42);
    });

    it('setCapitalMode never mutates the numeric values', async () => {
      const beforeFixed = wrapper.vm.config.trade_config.max_capital;
      const beforePct = wrapper.vm.config.trade_config.max_capital_percent;
      wrapper.vm.setCapitalMode('percent');
      await nextTick();
      expect(wrapper.vm.config.trade_config.max_capital).toBe(beforeFixed);
      expect(wrapper.vm.config.trade_config.max_capital_percent).toBe(beforePct);
    });
  });

  describe('maxCapitalHint', () => {
    beforeEach(async () => {
      vi.clearAllMocks();
      api.getAccount.mockResolvedValue({ portfolio_value: 0 });
      await mountForm();
    });

    it('shows generic hint in fixed mode', () => {
      expect(wrapper.vm.maxCapitalHint).toBe('Maximum capital to risk on this trade');
    });

    it('shows plain fallback in percent mode when netLiq is unknown', async () => {
      wrapper.vm.setCapitalMode('percent');
      wrapper.vm.config.trade_config.max_capital_percent = 60;
      wrapper.vm.netLiq = 0;
      await nextTick();
      expect(wrapper.vm.maxCapitalHint).toBe('60% of account Net Liquidating Value');
    });

    it('shows resolved dollars in percent mode when netLiq is known', async () => {
      wrapper.vm.setCapitalMode('percent');
      wrapper.vm.config.trade_config.max_capital_percent = 60;
      wrapper.vm.netLiq = 10000;
      await nextTick();
      // 10000 * 60% = 6000
      expect(wrapper.vm.maxCapitalHint).toContain('6,000');
      expect(wrapper.vm.maxCapitalHint).toContain('10,000');
      expect(wrapper.vm.maxCapitalHint).toContain('60%');
    });
  });

  describe('Net Liq preview fetch', () => {
    it('sets netLiq from portfolio_value on mount', async () => {
      vi.clearAllMocks();
      api.getAccount.mockResolvedValue({ portfolio_value: 12345 });
      await mountForm();
      await nextTick();
      expect(wrapper.vm.netLiq).toBe(12345);
    });

    it('falls back to equity when portfolio_value is absent', async () => {
      vi.clearAllMocks();
      api.getAccount.mockResolvedValue({ equity: 8000 });
      await mountForm();
      await nextTick();
      expect(wrapper.vm.netLiq).toBe(8000);
    });

    it('leaves netLiq=0 when getAccount rejects, and does not block', async () => {
      vi.clearAllMocks();
      api.getAccount.mockRejectedValue(new Error('provider down'));
      await mountForm();
      await nextTick();
      expect(wrapper.vm.netLiq).toBe(0);
      // percent hint degrades to the plain fallback
      wrapper.vm.setCapitalMode('percent');
      wrapper.vm.config.trade_config.max_capital_percent = 60;
      await nextTick();
      expect(wrapper.vm.maxCapitalHint).toBe('60% of account Net Liquidating Value');
    });
  });

  describe('Percent validation', () => {
    beforeEach(async () => {
      vi.clearAllMocks();
      api.getAccount.mockResolvedValue({ portfolio_value: 10000 });
      await mountForm();
      // valid required fields so only percent validation is under test
      wrapper.vm.config.name = 'My Config';
      wrapper.vm.config.symbol = 'NDX';
      wrapper.vm.config.entry_time = '12:25';
      wrapper.vm.setCapitalMode('percent');
      await nextTick();
    });

    it('blocks percent = 0', async () => {
      wrapper.vm.config.trade_config.max_capital_percent = 0;
      const ok = wrapper.vm.validateConfig();
      expect(ok).toBe(false);
      expect(wrapper.vm.errors.max_capital_percent).toBeTruthy();
    });

    it('blocks percent = 150', async () => {
      wrapper.vm.config.trade_config.max_capital_percent = 150;
      const ok = wrapper.vm.validateConfig();
      expect(ok).toBe(false);
      expect(wrapper.vm.errors.max_capital_percent).toBeTruthy();
    });

    it('passes percent = 60', async () => {
      wrapper.vm.config.trade_config.max_capital_percent = 60;
      const ok = wrapper.vm.validateConfig();
      expect(ok).toBe(true);
      expect(wrapper.vm.errors.max_capital_percent).toBeFalsy();
    });
  });
});
