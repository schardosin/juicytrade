import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { mount } from '@vue/test-utils';
import { ref, nextTick } from 'vue';
import AutomationDashboard from '../src/components/automation/AutomationDashboard.vue';
import { api } from '../src/services/api.js';
import webSocketClient from '../src/services/webSocketClient.js';

// Mock dependencies — identical to existing test file
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

// Helper: base config object to reduce boilerplate
const makeConfig = (overrides = {}) => ({
  id: 'cfg-1',
  name: 'Test Config',
  symbol: 'NDX',
  enabled: true,
  indicators: [],
  indicator_groups: [],
  entry_time: '12:25',
  entry_timezone: 'America/New_York',
  recurrence: 'once',
  trade_config: { strategy: 'put_spread', max_capital: 5000 },
  ...overrides,
});

describe('QA — AutomationDashboard', () => {
  let wrapper;

  beforeEach(async () => {
    vi.useFakeTimers();
    vi.clearAllMocks();
    wrapper = mount(AutomationDashboard);
    await nextTick();
    await nextTick();
    vi.useRealTimers();
  });

  afterEach(() => {
    if (wrapper) {
      wrapper.unmount();
    }
  });

  // =========================================================================
  // 6.1 Group Results Rendering (AC-7)
  // =========================================================================

  describe('6.1 Group Results Rendering', () => {
    it('renders grouped indicator results when group_results has 2+ entries', async () => {
      wrapper.vm.configs = [makeConfig({
        indicator_groups: [
          { id: 'grp_1', name: 'Low Vol', indicators: [{ id: 'ind_1', type: 'vix', enabled: true, operator: 'lt', threshold: 20 }] },
          { id: 'grp_2', name: 'High Vol', indicators: [{ id: 'ind_2', type: 'rsi', enabled: true, operator: 'gt', threshold: 70 }] },
        ],
      })];
      wrapper.vm.statuses = {
        'cfg-1': {
          state: 'waiting',
          is_running: true,
          group_results: [
            { group_id: 'grp_1', group_name: 'Low Vol', pass: true, indicator_results: [{ type: 'vix', value: 18, pass: true, stale: false }] },
            { group_id: 'grp_2', group_name: 'High Vol', pass: false, indicator_results: [{ type: 'rsi', value: 25, pass: false, stale: false }] },
          ],
          indicator_results: [{ type: 'vix', value: 18, pass: true }, { type: 'rsi', value: 25, pass: false }],
          all_indicators_pass: true,
        }
      };
      await nextTick();

      // Expand the collapsed eval section first
      const statusDetails = wrapper.find('.status-details');
      expect(statusDetails.exists()).toBe(true);
      await statusDetails.find('.indicator-results .section-collapse-header').trigger('click');
      await nextTick();

      // status-details section should contain indicator-group-dashboard containers
      const groupDashboards = statusDetails.findAll('.indicator-group-dashboard');
      expect(groupDashboards.length).toBe(2);

      // group-result-badge elements
      const badges = statusDetails.findAll('.group-result-badge');
      expect(badges.length).toBe(2);
    });

    it('renders per-group indicator chips inside each group', async () => {
      wrapper.vm.configs = [makeConfig({
        indicator_groups: [
          { id: 'grp_1', name: 'Group A', indicators: [{ id: 'ind_1', type: 'vix', enabled: true, operator: 'lt', threshold: 20 }] },
          { id: 'grp_2', name: 'Group B', indicators: [{ id: 'ind_2', type: 'rsi', enabled: true, operator: 'gt', threshold: 30 }] },
        ],
      })];
      wrapper.vm.statuses = {
        'cfg-1': {
          state: 'waiting',
          is_running: true,
          group_results: [
            { group_id: 'grp_1', group_name: 'Group A', pass: true, indicator_results: [{ type: 'vix', value: 18, pass: true, stale: false }] },
            { group_id: 'grp_2', group_name: 'Group B', pass: false, indicator_results: [{ type: 'rsi', value: 25, pass: false, stale: false }] },
          ],
          all_indicators_pass: true,
        }
      };
      await nextTick();

      // Expand the eval section
      await wrapper.find('.status-details .indicator-results .section-collapse-header').trigger('click');
      await nextTick();

      // Expand both groups to reveal result chips
      const groupHeaders = wrapper.findAll('.status-details .indicator-results .group-collapse-header');
      await groupHeaders[0].trigger('click');
      await groupHeaders[1].trigger('click');
      await nextTick();

      const groups = wrapper.findAll('.status-details .indicator-group-dashboard');
      expect(groups.length).toBe(2);

      // Each group should have result-chip elements
      const chips = wrapper.findAll('.status-details .result-chip');
      expect(chips.length).toBe(2);
    });

    it('renders overall status line with passing group name', async () => {
      wrapper.vm.configs = [makeConfig({
        indicator_groups: [
          { id: 'grp_1', name: 'Low Vol', indicators: [{ id: 'ind_1', type: 'vix', enabled: true, operator: 'lt', threshold: 20 }] },
          { id: 'grp_2', name: 'High Vol', indicators: [{ id: 'ind_2', type: 'rsi', enabled: true, operator: 'gt', threshold: 70 }] },
        ],
      })];
      wrapper.vm.statuses = {
        'cfg-1': {
          state: 'waiting',
          is_running: true,
          group_results: [
            { group_id: 'grp_1', group_name: 'Low Vol', pass: true, indicator_results: [] },
            { group_id: 'grp_2', group_name: 'High Vol', pass: false, indicator_results: [] },
          ],
          all_indicators_pass: true,
        }
      };
      await nextTick();

      // Expand the eval section to reveal overall-status
      await wrapper.find('.indicator-results .section-collapse-header').trigger('click');
      await nextTick();

      const overall = wrapper.find('.overall-status');
      expect(overall.exists()).toBe(true);
      expect(overall.classes()).toContain('passing');
      expect(overall.text()).toContain('Low Vol');
    });

    it('renders overall "Not passing" status when no groups pass', async () => {
      wrapper.vm.configs = [makeConfig({
        indicator_groups: [
          { id: 'grp_1', name: 'Group A', indicators: [{ id: 'ind_1', type: 'vix', enabled: true, operator: 'lt', threshold: 20 }] },
          { id: 'grp_2', name: 'Group B', indicators: [{ id: 'ind_2', type: 'rsi', enabled: true, operator: 'gt', threshold: 70 }] },
        ],
      })];
      wrapper.vm.statuses = {
        'cfg-1': {
          state: 'waiting',
          is_running: true,
          group_results: [
            { group_id: 'grp_1', group_name: 'Group A', pass: false, indicator_results: [] },
            { group_id: 'grp_2', group_name: 'Group B', pass: false, indicator_results: [] },
          ],
          all_indicators_pass: false,
        }
      };
      await nextTick();

      // Expand the eval section to reveal overall-status
      await wrapper.find('.indicator-results .section-collapse-header').trigger('click');
      await nextTick();

      const overall = wrapper.find('.overall-status');
      expect(overall.exists()).toBe(true);
      expect(overall.classes()).toContain('failing');
      expect(overall.text()).toContain('Not passing');
    });

    it('stale indicator in group_results shows stale chip styling', async () => {
      wrapper.vm.configs = [makeConfig({
        indicator_groups: [
          { id: 'grp_1', name: 'Group A', indicators: [{ id: 'ind_1', type: 'vix', enabled: true, operator: 'lt', threshold: 20 }] },
          { id: 'grp_2', name: 'Group B', indicators: [{ id: 'ind_2', type: 'rsi', enabled: true, operator: 'gt', threshold: 30 }] },
        ],
      })];
      wrapper.vm.statuses = {
        'cfg-1': {
          state: 'waiting',
          is_running: true,
          group_results: [
            { group_id: 'grp_1', group_name: 'Group A', pass: false, indicator_results: [
              { type: 'vix', value: 0, pass: false, stale: true, error: 'data fetch timeout' }
            ]},
            { group_id: 'grp_2', group_name: 'Group B', pass: true, indicator_results: [
              { type: 'rsi', value: 45, pass: true, stale: false }
            ]},
          ],
          all_indicators_pass: true,
        }
      };
      await nextTick();

      // Expand the eval section
      await wrapper.find('.indicator-results .section-collapse-header').trigger('click');
      await nextTick();

      // Expand both groups to reveal result chips
      const groupHeaders = wrapper.findAll('.indicator-results .group-collapse-header');
      await groupHeaders[0].trigger('click');
      await groupHeaders[1].trigger('click');
      await nextTick();

      const staleChips = wrapper.findAll('.result-chip.stale');
      expect(staleChips.length).toBe(1);

      // Stale icon should be rendered
      const staleIcon = staleChips[0].find('.stale-icon');
      expect(staleIcon.exists()).toBe(true);
    });
  });

  // =========================================================================
  // 6.2 OR Divider Between Groups (AC-7)
  // =========================================================================

  describe('6.2 OR Divider Between Groups', () => {
    it('OR divider rendered between group results in status details', async () => {
      wrapper.vm.configs = [makeConfig({
        indicator_groups: [
          { id: 'grp_1', name: 'Group A', indicators: [{ id: 'ind_1', type: 'vix', enabled: true, operator: 'lt', threshold: 20 }] },
          { id: 'grp_2', name: 'Group B', indicators: [{ id: 'ind_2', type: 'rsi', enabled: true, operator: 'gt', threshold: 30 }] },
        ],
      })];
      wrapper.vm.statuses = {
        'cfg-1': {
          state: 'waiting',
          is_running: true,
          group_results: [
            { group_id: 'grp_1', group_name: 'Group A', pass: true, indicator_results: [{ type: 'vix', value: 18, pass: true, stale: false }] },
            { group_id: 'grp_2', group_name: 'Group B', pass: false, indicator_results: [{ type: 'rsi', value: 25, pass: false, stale: false }] },
          ],
          all_indicators_pass: true,
        }
      };
      await nextTick();

      // Expand the eval section to reveal OR dividers
      const statusDetails = wrapper.find('.status-details');
      await statusDetails.find('.indicator-results .section-collapse-header').trigger('click');
      await nextTick();

      const dividers = statusDetails.findAll('.or-divider-compact');
      expect(dividers.length).toBe(1);
    });

    it('OR divider rendered between groups in config indicators section', async () => {
      wrapper.vm.configs = [makeConfig({
        indicator_groups: [
          { id: 'grp_1', name: 'Group A', indicators: [{ id: 'ind_1', type: 'vix', enabled: true, operator: 'lt', threshold: 20 }] },
          { id: 'grp_2', name: 'Group B', indicators: [{ id: 'ind_2', type: 'rsi', enabled: true, operator: 'gt', threshold: 30 }] },
        ],
      })];
      await nextTick();

      // Expand the collapsed entry section to reveal OR dividers
      await wrapper.find('.indicators-section .section-collapse-header').trigger('click');
      await nextTick();

      const indicatorsSection = wrapper.find('.indicators-section');
      const dividers = indicatorsSection.findAll('.or-divider-compact');
      expect(dividers.length).toBe(1);
    });

    it('no OR divider with 1 group result', async () => {
      wrapper.vm.configs = [makeConfig({
        indicator_groups: [
          { id: 'grp_1', name: 'Default', indicators: [{ id: 'ind_1', type: 'vix', enabled: true, operator: 'lt', threshold: 20 }] },
        ],
      })];
      wrapper.vm.statuses = {
        'cfg-1': {
          state: 'waiting',
          is_running: true,
          group_results: [
            { group_id: 'grp_1', group_name: 'Default', pass: true, indicator_results: [{ type: 'vix', value: 18, pass: true, stale: false }] },
          ],
          indicator_results: [{ type: 'vix', value: 18, pass: true }],
          all_indicators_pass: true,
        }
      };
      await nextTick();

      // Single group_results entry → template uses flat rendering path (group_results.length > 1 is false)
      const statusDetails = wrapper.find('.status-details');
      expect(statusDetails.exists()).toBe(true);
      const dividers = statusDetails.findAll('.or-divider-compact');
      expect(dividers.length).toBe(0);
    });
  });

  // =========================================================================
  // 6.3 Single-Group Flat Display (AC-9)
  // =========================================================================

  describe('6.3 Single-Group Flat Display', () => {
    it('single group config renders flat indicator chips (no group container)', async () => {
      wrapper.vm.configs = [makeConfig({
        indicator_groups: [
          { id: 'grp_1', name: 'Default', indicators: [
            { id: 'ind_1', type: 'vix', enabled: true, operator: 'lt', threshold: 20 },
            { id: 'ind_2', type: 'gap', enabled: true, operator: 'lt', threshold: 1.0 },
          ]},
        ],
      })];
      await nextTick();

      const indicatorsSection = wrapper.find('.indicators-section');
      // Should NOT have indicator-group-dashboard (single group uses flat path)
      expect(indicatorsSection.findAll('.indicator-group-dashboard').length).toBe(0);
      // Should have flat indicator-chip elements
      expect(indicatorsSection.findAll('.indicator-chip').length).toBe(2);
    });

    it('single group status renders flat results (no group header)', async () => {
      wrapper.vm.configs = [makeConfig({
        indicator_groups: [
          { id: 'grp_1', name: 'Default', indicators: [{ id: 'ind_1', type: 'vix', enabled: true, operator: 'lt', threshold: 20 }] },
        ],
      })];
      wrapper.vm.statuses = {
        'cfg-1': {
          state: 'waiting',
          is_running: true,
          // Single group → group_results has 1 entry, template checks length > 1
          group_results: [
            { group_id: 'grp_1', group_name: 'Default', pass: true, indicator_results: [{ type: 'vix', value: 18, pass: true, stale: false }] },
          ],
          indicator_results: [{ type: 'vix', value: 18, pass: true }],
          all_indicators_pass: true,
        }
      };
      await nextTick();

      const statusDetails = wrapper.find('.status-details');
      // With group_results.length === 1, the template uses the flat rendering path
      expect(statusDetails.findAll('.indicator-group-dashboard').length).toBe(0);
      // Should have flat result-chip elements
      expect(statusDetails.findAll('.result-chip').length).toBeGreaterThanOrEqual(1);
    });

    it('single group evaluation dialog renders flat results', async () => {
      wrapper.vm.configs = [makeConfig({
        indicator_groups: [
          { id: 'grp_1', name: 'Default', indicators: [{ id: 'ind_1', type: 'vix', enabled: true, operator: 'lt', threshold: 20 }] },
        ],
      })];

      api.evaluateAutomationConfig.mockResolvedValueOnce({
        data: {
          all_pass: true,
          indicators: [{ type: 'vix', value: 18, pass: true, operator: 'lt', threshold: 20, symbol: 'VIX' }],
          group_results: [
            { group_id: 'grp_1', group_name: 'Default', pass: true, indicator_results: [{ type: 'vix', value: 18, pass: true }] },
          ],
        }
      });

      await wrapper.vm.evaluateIndicators(wrapper.vm.configs[0]);
      await nextTick();

      // evalResult should use flat path (group_results.length === 1)
      expect(wrapper.vm.evalResult).toBeDefined();
      expect(wrapper.vm.evalResult.group_results.length).toBe(1);
      // The flat template uses evalResult.results
      expect(wrapper.vm.evalResult.results.length).toBe(1);
    });
  });

  // =========================================================================
  // 6.4 Backward Compatibility with Legacy Data (AC-4)
  // =========================================================================

  describe('6.4 Backward Compatibility with Legacy Data', () => {
    it('config with only legacy indicators renders flat chips', async () => {
      wrapper.vm.configs = [makeConfig({
        indicators: [
          { id: 'ind_1', type: 'vix', enabled: true, operator: 'lt', threshold: 20 },
          { id: 'ind_2', type: 'gap', enabled: true, operator: 'lt', threshold: 1.0 },
        ],
        indicator_groups: undefined, // No groups at all
      })];
      await nextTick();

      // getEnabledIndicators falls back to legacy indicators
      const enabled = wrapper.vm.getEnabledIndicators(wrapper.vm.configs[0]);
      expect(enabled.length).toBe(2);

      // Flat indicator-chip elements
      const indicatorsSection = wrapper.find('.indicators-section');
      expect(indicatorsSection.findAll('.indicator-chip').length).toBe(2);
      expect(indicatorsSection.findAll('.indicator-group-dashboard').length).toBe(0);
    });

    it('status update without group_results renders flat indicator_results', async () => {
      wrapper.vm.configs = [makeConfig({
        indicator_groups: [
          { id: 'grp_1', name: 'Default', indicators: [{ id: 'ind_1', type: 'vix', enabled: true, operator: 'lt', threshold: 20 }] },
        ],
      })];
      wrapper.vm.statuses = {
        'cfg-1': {
          state: 'waiting',
          is_running: true,
          // No group_results — backward compat path
          indicator_results: [
            { type: 'vix', value: 18, pass: true, stale: false },
            { type: 'gap', value: 0.5, pass: true, stale: false },
          ],
          all_indicators_pass: true,
        }
      };
      await nextTick();

      const statusDetails = wrapper.find('.status-details');
      // Should use the flat rendering path (no group_results or length <= 1)
      expect(statusDetails.findAll('.indicator-group-dashboard').length).toBe(0);
      expect(statusDetails.findAll('.result-chip').length).toBe(2);
    });

    it('handleAutomationUpdate merges group_results into statuses', async () => {
      wrapper.vm.configs = [makeConfig()];

      // Get the callback that was registered with addCallback
      const addCallbackCalls = webSocketClient.addCallback.mock.calls;
      const automationUpdateCall = addCallbackCalls.find(call => call[0] === 'automation_update');
      expect(automationUpdateCall).toBeDefined();
      const handler = automationUpdateCall[1];

      // Simulate a WebSocket message with group_results
      handler({
        automation_id: 'cfg-1',
        data: {
          status: 'evaluating',
          group_results: [
            { group_id: 'grp_1', group_name: 'Low Vol', pass: true, indicator_results: [{ type: 'vix', value: 18, pass: true }] },
            { group_id: 'grp_2', group_name: 'High Vol', pass: false, indicator_results: [{ type: 'rsi', value: 25, pass: false }] },
          ],
          indicator_results: [
            { type: 'vix', value: 18, pass: true },
            { type: 'rsi', value: 25, pass: false },
          ],
          all_indicators_pass: true,
        }
      });
      await nextTick();

      const status = wrapper.vm.statuses['cfg-1'];
      expect(status).toBeDefined();
      expect(status.group_results).toBeDefined();
      expect(status.group_results.length).toBe(2);
      expect(status.group_results[0].group_name).toBe('Low Vol');
      expect(status.all_indicators_pass).toBe(true);
    });

    it('evaluation dialog handles multi-group response', async () => {
      wrapper.vm.configs = [makeConfig({
        indicator_groups: [
          { id: 'grp_1', name: 'Group A', indicators: [{ id: 'ind_1', type: 'vix', enabled: true, operator: 'lt', threshold: 20 }] },
          { id: 'grp_2', name: 'Group B', indicators: [{ id: 'ind_2', type: 'rsi', enabled: true, operator: 'gt', threshold: 30 }] },
        ],
      })];

      api.evaluateAutomationConfig.mockResolvedValueOnce({
        data: {
          all_pass: true,
          indicators: [
            { type: 'vix', value: 18, pass: true, operator: 'lt', threshold: 20 },
            { type: 'rsi', value: 45, pass: true, operator: 'gt', threshold: 30 },
          ],
          group_results: [
            { group_id: 'grp_1', group_name: 'Group A', pass: true, indicator_results: [{ type: 'vix', value: 18, pass: true }] },
            { group_id: 'grp_2', group_name: 'Group B', pass: true, indicator_results: [{ type: 'rsi', value: 45, pass: true }] },
          ],
        }
      });

      await wrapper.vm.evaluateIndicators(wrapper.vm.configs[0]);
      await nextTick();

      expect(wrapper.vm.evalResult).toBeDefined();
      expect(wrapper.vm.evalResult.group_results.length).toBe(2);
      expect(wrapper.vm.evalResult.all_passed).toBe(true);
      expect(wrapper.vm.showEvalDialog).toBe(true);
    });
  });
});
