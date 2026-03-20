import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { mount } from '@vue/test-utils';
import { ref, nextTick } from 'vue';
import AutomationDashboard from '../src/components/automation/AutomationDashboard.vue';

// Mock dependencies
vi.mock('vue-router', () => ({
  useRouter: () => ({
    push: vi.fn()
  })
}));

vi.mock('../src/services/api.js', () => ({
  api: {
    getAutomationConfigs: vi.fn().mockResolvedValue({ data: { configs: [] } }),
    getAllAutomationStatuses: vi.fn().mockResolvedValue({ data: { automations: [] } }),
    startAutomation: vi.fn().mockResolvedValue({}),
    stopAutomation: vi.fn().mockResolvedValue({}),
    evaluateAutomationConfig: vi.fn().mockResolvedValue({}),
    toggleAutomationConfig: vi.fn().mockResolvedValue({}),
    createAutomationConfig: vi.fn().mockResolvedValue({ data: { id: 'new-id' } }),
    deleteAutomationConfig: vi.fn().mockResolvedValue({}),
    getAutomationLogs: vi.fn().mockResolvedValue({ data: { logs: [] } }),
    resetAutomationTradedToday: vi.fn().mockResolvedValue({}),
  }
}));

vi.mock('../src/services/webSocketClient.js', () => ({
  default: {
    addCallback: vi.fn(),
    removeCallback: vi.fn(),
  }
}));

vi.mock('../src/composables/useMobileDetection.js', () => ({
  useMobileDetection: () => ({ isMobile: ref(false) })
}));

describe('AutomationDashboard', () => {
  let wrapper;

  beforeEach(async () => {
    vi.useFakeTimers();
    wrapper = mount(AutomationDashboard);
    // Allow onMounted async work (loadConfigs + loadStatuses) to complete
    await nextTick();
    await nextTick();
    vi.useRealTimers();
  });

  afterEach(() => {
    if (wrapper) {
      wrapper.unmount();
    }
  });

  // ─── getStatusClass ─────────────────────────────────────────────────

  describe('getStatusClass', () => {
    it('returns "disabled" when config.enabled is false', () => {
      const config = { id: 'cfg-1', enabled: false };
      expect(wrapper.vm.getStatusClass(config)).toBe('disabled');
    });

    it('returns "idle" when config is enabled but not running', () => {
      const config = { id: 'cfg-1', enabled: true };
      // No status entry → isConfigRunning returns false
      expect(wrapper.vm.getStatusClass(config)).toBe('idle');
    });

    it('returns "running-ready" when running and all_indicators_pass is true', async () => {
      const config = { id: 'cfg-1', enabled: true };
      wrapper.vm.statuses = {
        'cfg-1': { is_running: true, state: 'waiting', all_indicators_pass: true }
      };
      await nextTick();
      expect(wrapper.vm.getStatusClass(config)).toBe('running-ready');
    });

    it('returns "running-not-ready" when running and all_indicators_pass is false', async () => {
      const config = { id: 'cfg-1', enabled: true };
      wrapper.vm.statuses = {
        'cfg-1': { is_running: true, state: 'waiting', all_indicators_pass: false }
      };
      await nextTick();
      expect(wrapper.vm.getStatusClass(config)).toBe('running-not-ready');
    });

    it('returns "running-not-ready" when running and all_indicators_pass is undefined', async () => {
      const config = { id: 'cfg-1', enabled: true };
      wrapper.vm.statuses = {
        'cfg-1': { is_running: true, state: 'evaluating' }
      };
      await nextTick();
      expect(wrapper.vm.getStatusClass(config)).toBe('running-not-ready');
    });

    it('returns "running-not-ready" when running and all_indicators_pass is null', async () => {
      const config = { id: 'cfg-1', enabled: true };
      wrapper.vm.statuses = {
        'cfg-1': { is_running: true, state: 'trading', all_indicators_pass: null }
      };
      await nextTick();
      expect(wrapper.vm.getStatusClass(config)).toBe('running-not-ready');
    });

    it('returns "running-not-ready" when running via active state and no all_indicators_pass', async () => {
      const config = { id: 'cfg-1', enabled: true };
      // is_running is false but state is an active state → isConfigRunning returns true
      wrapper.vm.statuses = {
        'cfg-1': { is_running: false, state: 'monitoring' }
      };
      await nextTick();
      expect(wrapper.vm.getStatusClass(config)).toBe('running-not-ready');
    });
  });

  // ─── getRunningStatusClass ──────────────────────────────────────────

  describe('getRunningStatusClass', () => {
    it('returns "status-disabled" when config.enabled is false', () => {
      const config = { id: 'cfg-1', enabled: false };
      expect(wrapper.vm.getRunningStatusClass(config)).toBe('status-disabled');
    });

    it('returns "status-idle" when no status exists', () => {
      const config = { id: 'cfg-1', enabled: true };
      // No status entry for cfg-1
      expect(wrapper.vm.getRunningStatusClass(config)).toBe('status-idle');
    });

    it('returns "status-waiting" for state=waiting with all_indicators_pass=false', async () => {
      const config = { id: 'cfg-1', enabled: true };
      wrapper.vm.statuses = {
        'cfg-1': { state: 'waiting', all_indicators_pass: false }
      };
      await nextTick();
      expect(wrapper.vm.getRunningStatusClass(config)).toBe('status-waiting');
    });

    it('returns "status-running" for state=waiting with all_indicators_pass=true', async () => {
      const config = { id: 'cfg-1', enabled: true };
      wrapper.vm.statuses = {
        'cfg-1': { state: 'waiting', all_indicators_pass: true }
      };
      await nextTick();
      expect(wrapper.vm.getRunningStatusClass(config)).toBe('status-running');
    });

    it('returns "status-waiting" for state=evaluating with all_indicators_pass=false', async () => {
      const config = { id: 'cfg-1', enabled: true };
      wrapper.vm.statuses = {
        'cfg-1': { state: 'evaluating', all_indicators_pass: false }
      };
      await nextTick();
      expect(wrapper.vm.getRunningStatusClass(config)).toBe('status-waiting');
    });

    it('returns "status-running" for state=evaluating with all_indicators_pass=true', async () => {
      const config = { id: 'cfg-1', enabled: true };
      wrapper.vm.statuses = {
        'cfg-1': { state: 'evaluating', all_indicators_pass: true }
      };
      await nextTick();
      expect(wrapper.vm.getRunningStatusClass(config)).toBe('status-running');
    });

    it('returns "status-running" for state=trading regardless of all_indicators_pass', async () => {
      const config = { id: 'cfg-1', enabled: true };
      wrapper.vm.statuses = {
        'cfg-1': { state: 'trading', all_indicators_pass: false }
      };
      await nextTick();
      expect(wrapper.vm.getRunningStatusClass(config)).toBe('status-running');

      wrapper.vm.statuses = {
        'cfg-1': { state: 'trading', all_indicators_pass: true }
      };
      await nextTick();
      expect(wrapper.vm.getRunningStatusClass(config)).toBe('status-running');
    });

    it('returns "status-running" for state=monitoring regardless of all_indicators_pass', async () => {
      const config = { id: 'cfg-1', enabled: true };
      wrapper.vm.statuses = {
        'cfg-1': { state: 'monitoring', all_indicators_pass: false }
      };
      await nextTick();
      expect(wrapper.vm.getRunningStatusClass(config)).toBe('status-running');

      wrapper.vm.statuses = {
        'cfg-1': { state: 'monitoring', all_indicators_pass: true }
      };
      await nextTick();
      expect(wrapper.vm.getRunningStatusClass(config)).toBe('status-running');
    });

    it('returns "status-completed" for state=completed', async () => {
      const config = { id: 'cfg-1', enabled: true };
      wrapper.vm.statuses = {
        'cfg-1': { state: 'completed' }
      };
      await nextTick();
      expect(wrapper.vm.getRunningStatusClass(config)).toBe('status-completed');
    });

    it('returns "status-failed" for state=failed', async () => {
      const config = { id: 'cfg-1', enabled: true };
      wrapper.vm.statuses = {
        'cfg-1': { state: 'failed' }
      };
      await nextTick();
      expect(wrapper.vm.getRunningStatusClass(config)).toBe('status-failed');
    });
  });

  // ─── getEnabledIndicators (Indicator Groups) ───────────────────────

  describe('getEnabledIndicators', () => {
    it('reads from indicator_groups when available', () => {
      const config = {
        indicator_groups: [
          {
            id: 'grp_1', name: 'Low Vol',
            indicators: [
              { id: 'ind_1', type: 'vix', enabled: true, operator: 'lt', threshold: 20 },
              { id: 'ind_2', type: 'gap', enabled: false, operator: 'lt', threshold: 1 },
            ]
          },
          {
            id: 'grp_2', name: 'High Vol',
            indicators: [
              { id: 'ind_3', type: 'rsi', enabled: true, operator: 'gt', threshold: 70 },
            ]
          },
        ],
        indicators: []
      };

      const enabled = wrapper.vm.getEnabledIndicators(config);
      expect(enabled.length).toBe(2);
      expect(enabled[0].type).toBe('vix');
      expect(enabled[1].type).toBe('rsi');
    });

    it('falls back to legacy indicators when no groups', () => {
      const config = {
        indicators: [
          { id: 'ind_1', type: 'vix', enabled: true },
          { id: 'ind_2', type: 'gap', enabled: false },
          { id: 'ind_3', type: 'range', enabled: true },
        ]
      };

      const enabled = wrapper.vm.getEnabledIndicators(config);
      expect(enabled.length).toBe(2);
      expect(enabled[0].type).toBe('vix');
      expect(enabled[1].type).toBe('range');
    });

    it('returns empty array when no indicators or groups', () => {
      const config = {};
      const enabled = wrapper.vm.getEnabledIndicators(config);
      expect(enabled).toEqual([]);
    });
  });

  // ─── formatIndicatorType (Extended types) ──────────────────────────

  describe('formatIndicatorType', () => {
    it('formats original indicator types', () => {
      expect(wrapper.vm.formatIndicatorType('vix')).toBe('VIX');
      expect(wrapper.vm.formatIndicatorType('gap')).toBe('Gap');
      expect(wrapper.vm.formatIndicatorType('range')).toBe('Range');
      expect(wrapper.vm.formatIndicatorType('trend')).toBe('Trend');
      expect(wrapper.vm.formatIndicatorType('calendar')).toBe('FOMC');
    });

    it('formats new indicator types', () => {
      expect(wrapper.vm.formatIndicatorType('rsi')).toBe('RSI');
      expect(wrapper.vm.formatIndicatorType('macd')).toBe('MACD');
      expect(wrapper.vm.formatIndicatorType('momentum')).toBe('Momentum');
      expect(wrapper.vm.formatIndicatorType('cmo')).toBe('CMO');
      expect(wrapper.vm.formatIndicatorType('stoch')).toBe('Stoch');
      expect(wrapper.vm.formatIndicatorType('stoch_rsi')).toBe('StochRSI');
      expect(wrapper.vm.formatIndicatorType('adx')).toBe('ADX');
      expect(wrapper.vm.formatIndicatorType('cci')).toBe('CCI');
      expect(wrapper.vm.formatIndicatorType('sma')).toBe('SMA');
      expect(wrapper.vm.formatIndicatorType('ema')).toBe('EMA');
      expect(wrapper.vm.formatIndicatorType('atr')).toBe('ATR');
      expect(wrapper.vm.formatIndicatorType('bb_percent')).toBe('BB%B');
    });

    it('returns raw type for unknown types', () => {
      expect(wrapper.vm.formatIndicatorType('unknown_type')).toBe('unknown_type');
    });
  });

  // ─── Grouped Rendering ─────────────────────────────────────────────

  describe('Grouped indicator rendering', () => {
    it('renders grouped indicators when config has 2+ indicator_groups', async () => {
      wrapper.vm.configs = [{
        id: 'cfg-1',
        name: 'Test Config',
        symbol: 'NDX',
        enabled: true,
        indicator_groups: [
          { id: 'grp_1', name: 'Low Vol', indicators: [{ id: 'ind_1', type: 'vix', enabled: true, operator: 'lt', threshold: 20 }] },
          { id: 'grp_2', name: 'High Vol', indicators: [{ id: 'ind_2', type: 'rsi', enabled: true, operator: 'gt', threshold: 70 }] },
        ],
        indicators: [],
        trade_config: { strategy: 'put_spread', max_capital: 5000 },
      }];
      await nextTick();

      expect(wrapper.findAll('.indicator-group-dashboard').length).toBe(2);
      expect(wrapper.findAll('.or-divider-compact').length).toBe(1);
    });

    it('renders flat indicators for single group config', async () => {
      wrapper.vm.configs = [{
        id: 'cfg-1',
        name: 'Test Config',
        symbol: 'NDX',
        enabled: true,
        indicator_groups: [
          { id: 'grp_1', name: 'Default', indicators: [{ id: 'ind_1', type: 'vix', enabled: true, operator: 'lt', threshold: 20 }] },
        ],
        indicators: [],
        trade_config: { strategy: 'put_spread', max_capital: 5000 },
      }];
      await nextTick();

      // Single group renders flat — no group-dashboard containers
      expect(wrapper.findAll('.indicator-group-dashboard').length).toBe(0);
      expect(wrapper.findAll('.or-divider-compact').length).toBe(0);
      // Should still have indicator chips
      expect(wrapper.findAll('.indicator-chip').length).toBe(1);
    });

    it('renders flat indicators for legacy config (backward compat)', async () => {
      wrapper.vm.configs = [{
        id: 'cfg-1',
        name: 'Legacy Config',
        symbol: 'NDX',
        enabled: true,
        indicators: [
          { id: 'ind_1', type: 'vix', enabled: true, operator: 'lt', threshold: 20 },
          { id: 'ind_2', type: 'gap', enabled: true, operator: 'lt', threshold: 1.0 },
        ],
        trade_config: { strategy: 'put_spread', max_capital: 5000 },
      }];
      await nextTick();

      expect(wrapper.findAll('.indicator-group-dashboard').length).toBe(0);
      expect(wrapper.findAll('.indicator-chip').length).toBe(2);
    });
  });
});
