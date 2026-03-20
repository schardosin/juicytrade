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

      // Multi-group now renders collapsed by default — expand the section first
      expect(wrapper.find('.section-collapse-header').exists()).toBe(true);
      await wrapper.find('.section-collapse-header').trigger('click');
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

  // ─── Compact mode — Entry Criteria ───────────────────────────────────

  describe('Compact mode — Entry Criteria', () => {
    const multiGroupConfig = {
      id: 'cfg-multi',
      name: 'Multi Group Config',
      symbol: 'NDX',
      enabled: true,
      indicator_groups: [
        { id: 'grp_1', name: 'Low Vol', indicators: [
          { id: 'ind_1', type: 'vix', enabled: true, operator: 'lt', threshold: 20 },
          { id: 'ind_2', type: 'gap', enabled: true, operator: 'lt', threshold: 1 },
          { id: 'ind_3', type: 'range', enabled: false, operator: 'gt', threshold: 5 },
        ]},
        { id: 'grp_2', name: 'High Vol', indicators: [
          { id: 'ind_4', type: 'rsi', enabled: true, operator: 'gt', threshold: 70 },
          { id: 'ind_5', type: 'macd', enabled: true, operator: 'gt', threshold: 0 },
        ]},
        { id: 'grp_3', name: 'Trend Up', indicators: [
          { id: 'ind_6', type: 'sma', enabled: true, operator: 'gt', threshold: 100 },
          { id: 'ind_7', type: 'ema', enabled: true, operator: 'gt', threshold: 50 },
          { id: 'ind_8', type: 'adx', enabled: true, operator: 'gt', threshold: 25 },
        ]},
      ],
      indicators: [],
      trade_config: { strategy: 'put_spread', max_capital: 5000 },
    };

    it('renders collapsed by default for multi-group cards', async () => {
      wrapper.vm.configs = [multiGroupConfig];
      await nextTick();

      // Section collapse header should exist
      expect(wrapper.find('.section-collapse-header').exists()).toBe(true);
      // Summary text should show correct counts (3 groups, 7 enabled indicators)
      expect(wrapper.find('.collapse-summary-text').text()).toBe('3 groups · 7 indicators');
      // Section body should NOT exist (collapsed by default)
      expect(wrapper.find('.section-collapse-body').exists()).toBe(false);
      // No group containers visible when collapsed
      expect(wrapper.findAll('.indicator-group-dashboard').length).toBe(0);
    });

    it('clicking expands the Entry Criteria section', async () => {
      wrapper.vm.configs = [multiGroupConfig];
      await nextTick();

      // Click the section collapse header
      await wrapper.find('.section-collapse-header').trigger('click');
      await nextTick();

      // Section body should now be rendered
      expect(wrapper.find('.section-collapse-body').exists()).toBe(true);
      // Should have 3 group containers
      expect(wrapper.findAll('.indicator-group-dashboard').length).toBe(3);
      // Should have OR dividers between groups (2 dividers for 3 groups)
      expect(wrapper.findAll('.section-collapse-body .or-divider-compact').length).toBe(2);
    });

    it('groups are collapsed by default within expanded section', async () => {
      wrapper.vm.configs = [multiGroupConfig];
      await nextTick();

      // Expand the section
      await wrapper.find('.section-collapse-header').trigger('click');
      await nextTick();

      // Group collapse headers should exist for each group
      expect(wrapper.findAll('.group-collapse-header').length).toBe(3);
      // But no indicator grids should be visible (groups collapsed)
      expect(wrapper.findAll('.section-collapse-body .indicators-grid').length).toBe(0);
      // Group indicator counts should be visible
      const counts = wrapper.findAll('.group-indicator-count');
      expect(counts.length).toBe(3);
      expect(counts[0].text()).toBe('2 indicators'); // grp_1: 2 enabled out of 3
      expect(counts[1].text()).toBe('2 indicators'); // grp_2: 2 enabled
      expect(counts[2].text()).toBe('3 indicators'); // grp_3: 3 enabled
    });

    it('clicking a group header expands that group', async () => {
      wrapper.vm.configs = [multiGroupConfig];
      await nextTick();

      // Expand the section first
      await wrapper.find('.section-collapse-header').trigger('click');
      await nextTick();

      // Click the first group header
      const groupHeaders = wrapper.findAll('.group-collapse-header');
      await groupHeaders[0].trigger('click');
      await nextTick();

      // First group should now show its indicators grid
      const grids = wrapper.findAll('.section-collapse-body .indicators-grid');
      expect(grids.length).toBe(1);
      // Should have 2 indicator chips (2 enabled in grp_1)
      expect(grids[0].findAll('.indicator-chip').length).toBe(2);
      // Other groups should remain collapsed (no additional grids)
    });

    it('single-group configs render flat without collapse', async () => {
      wrapper.vm.configs = [{
        id: 'cfg-single',
        name: 'Single Group Config',
        symbol: 'NDX',
        enabled: true,
        indicator_groups: [
          { id: 'grp_1', name: 'Default', indicators: [
            { id: 'ind_1', type: 'vix', enabled: true, operator: 'lt', threshold: 20 },
          ]},
        ],
        indicators: [],
        trade_config: { strategy: 'put_spread', max_capital: 5000 },
      }];
      await nextTick();

      // No section collapse header for single group
      expect(wrapper.find('.section-collapse-header').exists()).toBe(false);
      // Indicator chips should render flat
      expect(wrapper.findAll('.indicator-chip').length).toBe(1);
      // No group collapse headers
      expect(wrapper.find('.group-collapse-header').exists()).toBe(false);
    });

    it('legacy configs (no groups) render flat without collapse', async () => {
      wrapper.vm.configs = [{
        id: 'cfg-legacy',
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

      // No section collapse header
      expect(wrapper.find('.section-collapse-header').exists()).toBe(false);
      // Flat indicator chips
      expect(wrapper.findAll('.indicator-chip').length).toBe(2);
    });
  });

  // ─── Compact mode helpers ────────────────────────────────────────────

  describe('Compact mode helpers', () => {

    // ─── isMultiGroup ───────────────────────────────────────────────

    describe('isMultiGroup', () => {
      it('returns true for config with 2+ indicator_groups', () => {
        const config = {
          indicator_groups: [
            { id: 'grp_1', name: 'Group 1', indicators: [] },
            { id: 'grp_2', name: 'Group 2', indicators: [] },
          ]
        };
        expect(wrapper.vm.isMultiGroup(config)).toBe(true);
      });

      it('returns false for config with 1 group', () => {
        const config = {
          indicator_groups: [
            { id: 'grp_1', name: 'Group 1', indicators: [] },
          ]
        };
        expect(wrapper.vm.isMultiGroup(config)).toBe(false);
      });

      it('returns false for config with no groups (legacy)', () => {
        const config = {
          indicators: [{ id: 'ind_1', type: 'vix', enabled: true }]
        };
        expect(wrapper.vm.isMultiGroup(config)).toBe(false);
      });

      it('returns false for config with empty indicator_groups array', () => {
        const config = { indicator_groups: [] };
        expect(wrapper.vm.isMultiGroup(config)).toBe(false);
      });
    });

    // ─── getEntrySummary ────────────────────────────────────────────

    describe('getEntrySummary', () => {
      it('returns correct summary for 3 groups with 3 enabled indicators each', () => {
        const config = {
          indicator_groups: [
            { id: 'grp_1', indicators: [
              { type: 'vix', enabled: true }, { type: 'gap', enabled: true }, { type: 'rsi', enabled: true }
            ]},
            { id: 'grp_2', indicators: [
              { type: 'macd', enabled: true }, { type: 'cci', enabled: true }, { type: 'adx', enabled: true }
            ]},
            { id: 'grp_3', indicators: [
              { type: 'sma', enabled: true }, { type: 'ema', enabled: true }, { type: 'atr', enabled: true }
            ]},
          ]
        };
        expect(wrapper.vm.getEntrySummary(config)).toBe('3 groups · 9 indicators');
      });

      it('counts only enabled indicators (skips enabled: false)', () => {
        const config = {
          indicator_groups: [
            { id: 'grp_1', indicators: [
              { type: 'vix', enabled: true }, { type: 'gap', enabled: false }, { type: 'rsi', enabled: true }
            ]},
            { id: 'grp_2', indicators: [
              { type: 'macd', enabled: false }, { type: 'cci', enabled: true }
            ]},
          ]
        };
        expect(wrapper.vm.getEntrySummary(config)).toBe('2 groups · 3 indicators');
      });

      it('returns "2 groups · 0 indicators" when groups exist but all indicators are disabled', () => {
        const config = {
          indicator_groups: [
            { id: 'grp_1', indicators: [
              { type: 'vix', enabled: false }, { type: 'gap', enabled: false }
            ]},
            { id: 'grp_2', indicators: [
              { type: 'rsi', enabled: false }
            ]},
          ]
        };
        expect(wrapper.vm.getEntrySummary(config)).toBe('2 groups · 0 indicators');
      });
    });

    // ─── getEvalSummary ─────────────────────────────────────────────

    describe('getEvalSummary', () => {
      it('returns null when no status exists for the config', () => {
        expect(wrapper.vm.getEvalSummary('nonexistent-id')).toBeNull();
      });

      it('returns null when status has no group_results', async () => {
        wrapper.vm.statuses = {
          'cfg-1': { state: 'waiting', indicator_results: [] }
        };
        await nextTick();
        expect(wrapper.vm.getEvalSummary('cfg-1')).toBeNull();
      });

      it('returns correct summary when 1 of 3 groups passes', async () => {
        wrapper.vm.statuses = {
          'cfg-eval': {
            state: 'waiting',
            group_results: [
              { group_id: 'g1', group_name: 'Low VIX', pass: true, indicator_results: [] },
              { group_id: 'g2', group_name: 'High Vol', pass: false, indicator_results: [] },
              { group_id: 'g3', group_name: 'Trend Up', pass: false, indicator_results: [] },
            ]
          }
        };
        await nextTick();
        const result = wrapper.vm.getEvalSummary('cfg-eval');
        expect(result).toEqual({
          passing: true,
          passingGroupName: 'Low VIX',
          summary: '1 of 3 groups passing'
        });
      });

      it('returns correct summary when no groups pass', async () => {
        wrapper.vm.statuses = {
          'cfg-eval': {
            state: 'waiting',
            group_results: [
              { group_id: 'g1', group_name: 'Group A', pass: false, indicator_results: [] },
              { group_id: 'g2', group_name: 'Group B', pass: false, indicator_results: [] },
            ]
          }
        };
        await nextTick();
        const result = wrapper.vm.getEvalSummary('cfg-eval');
        expect(result).toEqual({
          passing: false,
          passingGroupName: null,
          summary: '0 of 2 groups passing'
        });
      });
    });

    // ─── Toggle behavior ────────────────────────────────────────────

    describe('Toggle behavior', () => {
      it('toggleEntrySection flips from collapsed (default) to expanded', () => {
        expect(wrapper.vm.expandedEntrySections['cfg-toggle']).toBeFalsy();
        wrapper.vm.toggleEntrySection('cfg-toggle');
        expect(wrapper.vm.expandedEntrySections['cfg-toggle']).toBe(true);
      });

      it('calling toggleEntrySection twice returns to collapsed', () => {
        wrapper.vm.toggleEntrySection('cfg-toggle2');
        wrapper.vm.toggleEntrySection('cfg-toggle2');
        expect(wrapper.vm.expandedEntrySections['cfg-toggle2']).toBe(false);
      });

      it('toggleEvalSection works independently from toggleEntrySection on the same card', () => {
        wrapper.vm.toggleEntrySection('cfg-independent');
        expect(wrapper.vm.expandedEntrySections['cfg-independent']).toBe(true);
        expect(wrapper.vm.expandedEvalSections['cfg-independent']).toBeFalsy();

        wrapper.vm.toggleEvalSection('cfg-independent');
        expect(wrapper.vm.expandedEvalSections['cfg-independent']).toBe(true);
        // Entry should still be expanded
        expect(wrapper.vm.expandedEntrySections['cfg-independent']).toBe(true);
      });

      it('toggleGroup expands group 0 without affecting group 1', () => {
        wrapper.vm.toggleGroup('cfg-grp', 'entry', 0);
        expect(wrapper.vm.isGroupExpanded('cfg-grp', 'entry', 0)).toBe(true);
        expect(wrapper.vm.isGroupExpanded('cfg-grp', 'entry', 1)).toBe(false);
      });

      it('isGroupExpanded returns false by default, true after toggle', () => {
        expect(wrapper.vm.isGroupExpanded('cfg-new', 'eval', 0)).toBe(false);
        wrapper.vm.toggleGroup('cfg-new', 'eval', 0);
        expect(wrapper.vm.isGroupExpanded('cfg-new', 'eval', 0)).toBe(true);
      });
    });

    // ─── State independence ─────────────────────────────────────────

    describe('State independence', () => {
      it('toggling card A does not affect card B', () => {
        wrapper.vm.toggleEntrySection('card-A');
        wrapper.vm.toggleEvalSection('card-A');
        wrapper.vm.toggleGroup('card-A', 'entry', 0);

        // Card B should all be default (collapsed)
        expect(wrapper.vm.expandedEntrySections['card-B']).toBeFalsy();
        expect(wrapper.vm.expandedEvalSections['card-B']).toBeFalsy();
        expect(wrapper.vm.isGroupExpanded('card-B', 'entry', 0)).toBe(false);

        // Card A should be expanded
        expect(wrapper.vm.expandedEntrySections['card-A']).toBe(true);
        expect(wrapper.vm.expandedEvalSections['card-A']).toBe(true);
        expect(wrapper.vm.isGroupExpanded('card-A', 'entry', 0)).toBe(true);
      });
    });
  });
});
