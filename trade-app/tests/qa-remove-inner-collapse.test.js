import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { mount } from '@vue/test-utils';
import { ref, nextTick } from 'vue';
import fs from 'fs';
import path from 'path';
import AutomationDashboard from '../src/components/automation/AutomationDashboard.vue';

// Mock dependencies — identical to qa-dashboard-compact.test.js
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

describe('QA — Remove Inner Group-Level Collapse', () => {
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
  // Step 2 — Code Removal Verification (AC-4)
  // =========================================================================

  describe('Step 2 — Code Removal Verification (AC-4)', () => {
    const componentPath = path.resolve(__dirname, '../src/components/automation/AutomationDashboard.vue');
    const source = fs.readFileSync(componentPath, 'utf-8');

    it('2.1: component source contains no "expandedGroups"', () => {
      expect(source).not.toContain('expandedGroups');
    });

    it('2.2: component source contains no "toggleGroup"', () => {
      expect(source).not.toContain('toggleGroup');
    });

    it('2.3: component source contains no "isGroupExpanded"', () => {
      expect(source).not.toContain('isGroupExpanded');
    });

    it('2.4: component source contains no "group-collapse-header" CSS class', () => {
      expect(source).not.toContain('group-collapse-header');
    });

    it('2.5: component source contains no "collapse-chevron-sm" CSS class', () => {
      expect(source).not.toContain('collapse-chevron-sm');
    });

    it('2.6: component source contains "group-header-static" class (replacement)', () => {
      expect(source).toContain('group-header-static');
    });
  });

  // =========================================================================
  // Step 3 — Entry Criteria: All Groups Visible on Expand (AC-1, AC-3)
  // =========================================================================

  describe('Step 3 — Entry Criteria: All Groups Visible on Expand (AC-1, AC-3)', () => {

    it('3.1: 2-group config: expand entry → both groups indicator chips rendered immediately', async () => {
      wrapper.vm.configs = [makeConfig({
        indicator_groups: [
          { id: 'grp_1', name: 'Group A', indicators: [
            { id: 'ind_1', type: 'vix', enabled: true, operator: 'lt', threshold: 20 },
            { id: 'ind_2', type: 'gap', enabled: true, operator: 'lt', threshold: 1.0 },
          ]},
          { id: 'grp_2', name: 'Group B', indicators: [
            { id: 'ind_3', type: 'rsi', enabled: true, operator: 'gt', threshold: 70 },
          ]},
        ],
      })];
      await nextTick();

      // Expand entry section
      await wrapper.find('.section-collapse-header').trigger('click');
      await nextTick();

      // Both groups' indicators-grids rendered immediately
      const grids = wrapper.findAll('.section-collapse-body .indicators-grid');
      expect(grids.length).toBe(2);

      // Total chip count = 2 (Group A) + 1 (Group B) = 3
      const totalChips = wrapper.findAll('.section-collapse-body .indicator-chip');
      expect(totalChips.length).toBe(3);
    });

    it('3.2: 3-group config: expand entry → all 3 groups rendered with correct per-group chip counts', async () => {
      wrapper.vm.configs = [makeConfig({
        indicator_groups: [
          { id: 'grp_1', name: 'Group A', indicators: [
            { id: 'ind_1', type: 'vix', enabled: true, operator: 'lt', threshold: 20 },
          ]},
          { id: 'grp_2', name: 'Group B', indicators: [
            { id: 'ind_2', type: 'rsi', enabled: true, operator: 'gt', threshold: 70 },
            { id: 'ind_3', type: 'macd', enabled: true, operator: 'gt', threshold: 0 },
          ]},
          { id: 'grp_3', name: 'Group C', indicators: [
            { id: 'ind_4', type: 'sma', enabled: true, operator: 'gt', threshold: 50 },
            { id: 'ind_5', type: 'ema', enabled: true, operator: 'gt', threshold: 30 },
            { id: 'ind_6', type: 'adx', enabled: true, operator: 'gt', threshold: 25 },
          ]},
        ],
      })];
      await nextTick();

      // Expand entry section
      await wrapper.find('.section-collapse-header').trigger('click');
      await nextTick();

      // 3 indicator-group-dashboard containers
      const groups = wrapper.findAll('.indicator-group-dashboard');
      expect(groups.length).toBe(3);

      // Per-group chip counts: Group A=1, Group B=2, Group C=3
      expect(groups[0].findAll('.indicator-chip').length).toBe(1);
      expect(groups[1].findAll('.indicator-chip').length).toBe(2);
      expect(groups[2].findAll('.indicator-chip').length).toBe(3);
    });

    it('3.3: 5-group (mixed sizes: 1, 3, 0, 2, 4 indicators): expand entry → 5 groups, correct per-group chip counts, total = 10', async () => {
      wrapper.vm.configs = [makeConfig({
        indicator_groups: [
          // Group 1: 1 indicator
          { id: 'grp_1', name: 'G1', indicators: [
            { id: 'i1', type: 'vix', enabled: true, operator: 'lt', threshold: 20 },
          ]},
          // Group 2: 3 indicators
          { id: 'grp_2', name: 'G2', indicators: [
            { id: 'i2', type: 'rsi', enabled: true, operator: 'gt', threshold: 70 },
            { id: 'i3', type: 'macd', enabled: true, operator: 'gt', threshold: 0 },
            { id: 'i4', type: 'momentum', enabled: true, operator: 'gt', threshold: 5 },
          ]},
          // Group 3: 0 indicators (empty)
          { id: 'grp_3', name: 'G3', indicators: [] },
          // Group 4: 2 indicators
          { id: 'grp_4', name: 'G4', indicators: [
            { id: 'i5', type: 'sma', enabled: true, operator: 'gt', threshold: 50 },
            { id: 'i6', type: 'ema', enabled: true, operator: 'gt', threshold: 30 },
          ]},
          // Group 5: 4 indicators
          { id: 'grp_5', name: 'G5', indicators: [
            { id: 'i7', type: 'adx', enabled: true, operator: 'gt', threshold: 25 },
            { id: 'i8', type: 'cci', enabled: true, operator: 'gt', threshold: 100 },
            { id: 'i9', type: 'atr', enabled: true, operator: 'lt', threshold: 10 },
            { id: 'i10', type: 'stoch', enabled: true, operator: 'lt', threshold: 80 },
          ]},
        ],
      })];
      await nextTick();

      // Expand entry section
      await wrapper.find('.section-collapse-header').trigger('click');
      await nextTick();

      // 5 indicator-group-dashboard containers
      const groups = wrapper.findAll('.indicator-group-dashboard');
      expect(groups.length).toBe(5);

      // Per-group chip counts: [1, 3, 0, 2, 4]
      expect(groups[0].findAll('.indicator-chip').length).toBe(1);
      expect(groups[1].findAll('.indicator-chip').length).toBe(3);
      expect(groups[2].findAll('.indicator-chip').length).toBe(0);
      expect(groups[3].findAll('.indicator-chip').length).toBe(2);
      expect(groups[4].findAll('.indicator-chip').length).toBe(4);

      // Total chips = 10
      const totalChips = wrapper.findAll('.section-collapse-body .indicator-chip');
      expect(totalChips.length).toBe(10);
    });

    it('3.4: 3-group: expand entry → chip text content matches formatted types and conditions', async () => {
      wrapper.vm.configs = [makeConfig({
        indicator_groups: [
          { id: 'grp_1', name: 'Group A', indicators: [
            { id: 'ind_1', type: 'vix', enabled: true, operator: 'lt', threshold: 20 },
          ]},
          { id: 'grp_2', name: 'Group B', indicators: [
            { id: 'ind_2', type: 'rsi', enabled: true, operator: 'gt', threshold: 70 },
            { id: 'ind_3', type: 'macd', enabled: true, operator: 'gt', threshold: 0 },
          ]},
          { id: 'grp_3', name: 'Group C', indicators: [
            { id: 'ind_4', type: 'momentum', enabled: true, operator: 'lt', threshold: 5 },
          ]},
        ],
      })];
      await nextTick();

      // Expand entry section
      await wrapper.find('.section-collapse-header').trigger('click');
      await nextTick();

      const chips = wrapper.findAll('.section-collapse-body .indicator-chip');
      expect(chips.length).toBe(4);

      // Chip 0: VIX < 20
      expect(chips[0].find('.indicator-name').text()).toBe('VIX');
      expect(chips[0].find('.indicator-value').text()).toBe('< 20');

      // Chip 1: RSI > 70
      expect(chips[1].find('.indicator-name').text()).toBe('RSI');
      expect(chips[1].find('.indicator-value').text()).toBe('> 70');

      // Chip 2: MACD > 0
      expect(chips[2].find('.indicator-name').text()).toBe('MACD');
      expect(chips[2].find('.indicator-value').text()).toBe('> 0');

      // Chip 3: Momentum < 5
      expect(chips[3].find('.indicator-name').text()).toBe('Momentum');
      expect(chips[3].find('.indicator-value').text()).toBe('< 5');
    });

    it('3.5: 3-group: expand entry → OR dividers count = groups - 1, label text = "OR"', async () => {
      wrapper.vm.configs = [makeConfig({
        indicator_groups: [
          { id: 'grp_1', name: 'Group A', indicators: [
            { id: 'ind_1', type: 'vix', enabled: true, operator: 'lt', threshold: 20 },
          ]},
          { id: 'grp_2', name: 'Group B', indicators: [
            { id: 'ind_2', type: 'rsi', enabled: true, operator: 'gt', threshold: 70 },
          ]},
          { id: 'grp_3', name: 'Group C', indicators: [
            { id: 'ind_3', type: 'macd', enabled: true, operator: 'gt', threshold: 0 },
          ]},
        ],
      })];
      await nextTick();

      // Expand entry section
      await wrapper.find('.section-collapse-header').trigger('click');
      await nextTick();

      // OR dividers: 3 groups - 1 = 2
      const dividers = wrapper.findAll('.section-collapse-body .or-divider-compact');
      expect(dividers.length).toBe(2);

      // Each divider label text = "OR"
      const labels = wrapper.findAll('.section-collapse-body .or-divider-compact-label');
      expect(labels.length).toBe(2);
      expect(labels[0].text()).toBe('OR');
      expect(labels[1].text()).toBe('OR');
    });

  });

  // =========================================================================
  // Step 4 — Last Evaluation: All Groups Visible on Expand (AC-2, AC-3)
  // =========================================================================

  describe('Step 4 — Last Evaluation: All Groups Visible on Expand (AC-2, AC-3)', () => {

    it('4.1: 2-group running config: expand eval → both groups result chips rendered immediately', async () => {
      wrapper.vm.configs = [makeConfig({
        indicator_groups: [
          { id: 'grp_1', name: 'Group A', indicators: [
            { id: 'ind_1', type: 'vix', enabled: true, operator: 'lt', threshold: 20 },
            { id: 'ind_2', type: 'gap', enabled: true, operator: 'lt', threshold: 1.0 },
          ]},
          { id: 'grp_2', name: 'Group B', indicators: [
            { id: 'ind_3', type: 'rsi', enabled: true, operator: 'gt', threshold: 70 },
          ]},
        ],
      })];
      wrapper.vm.statuses = {
        'cfg-1': {
          state: 'waiting',
          is_running: true,
          all_indicators_pass: true,
          group_results: [
            { group_id: 'grp_1', group_name: 'Group A', pass: true, indicator_results: [
              { type: 'vix', value: 15.5, pass: true },
              { type: 'gap', value: 0.5, pass: true },
            ]},
            { group_id: 'grp_2', group_name: 'Group B', pass: false, indicator_results: [
              { type: 'rsi', value: 40.25, pass: false },
            ]},
          ],
        }
      };
      await nextTick();

      // Expand eval section
      await wrapper.find('.indicator-results .section-collapse-header').trigger('click');
      await nextTick();

      // Both groups' results-grids rendered immediately
      const grids = wrapper.findAll('.indicator-results .section-collapse-body .results-grid');
      expect(grids.length).toBe(2);

      // Total result-chip count = 2 (Group A) + 1 (Group B) = 3
      const totalChips = wrapper.findAll('.indicator-results .section-collapse-body .result-chip');
      expect(totalChips.length).toBe(3);
    });

    it('4.2: 3-group (mixed: 2 pass, 1 fail, includes stale): expand eval → correct pass/fail badges and stale styling', async () => {
      wrapper.vm.configs = [makeConfig({
        indicator_groups: [
          { id: 'grp_1', name: 'Alpha', indicators: [{ id: 'i1', type: 'vix', enabled: true, operator: 'lt', threshold: 20 }] },
          { id: 'grp_2', name: 'Beta', indicators: [{ id: 'i2', type: 'rsi', enabled: true, operator: 'gt', threshold: 70 }] },
          { id: 'grp_3', name: 'Gamma', indicators: [{ id: 'i3', type: 'macd', enabled: true, operator: 'gt', threshold: 0 }] },
        ],
      })];
      wrapper.vm.statuses = {
        'cfg-1': {
          state: 'waiting',
          is_running: true,
          all_indicators_pass: true,
          group_results: [
            { group_id: 'grp_1', group_name: 'Alpha', pass: true, indicator_results: [
              { type: 'vix', value: 15.0, pass: true },
            ]},
            { group_id: 'grp_2', group_name: 'Beta', pass: false, indicator_results: [
              { type: 'rsi', value: 40.0, pass: false, stale: true, error: 'Fetch timeout' },
            ]},
            { group_id: 'grp_3', group_name: 'Gamma', pass: true, indicator_results: [
              { type: 'macd', value: 1.5, pass: true },
            ]},
          ],
        }
      };
      await nextTick();

      // Expand eval section
      await wrapper.find('.indicator-results .section-collapse-header').trigger('click');
      await nextTick();

      // 3 groups rendered
      const groups = wrapper.findAll('.indicator-results .indicator-group-dashboard');
      expect(groups.length).toBe(3);

      // Pass/fail badges: 2 passed, 1 failed
      const passedBadges = wrapper.findAll('.indicator-results .group-result-badge.passed');
      const failedBadges = wrapper.findAll('.indicator-results .group-result-badge.failed');
      expect(passedBadges.length).toBe(2);
      expect(failedBadges.length).toBe(1);

      // Stale chip: 1 result-chip with .stale class and .stale-icon present
      const staleChips = wrapper.findAll('.indicator-results .result-chip.stale');
      expect(staleChips.length).toBe(1);
      expect(staleChips[0].find('.stale-icon').exists()).toBe(true);
    });

    it('4.3: 3-group: expand eval → result chip values match formatIndicatorValue() output (toFixed(2))', async () => {
      wrapper.vm.configs = [makeConfig({
        indicator_groups: [
          { id: 'grp_1', name: 'Group A', indicators: [{ id: 'i1', type: 'vix', enabled: true, operator: 'lt', threshold: 20 }] },
          { id: 'grp_2', name: 'Group B', indicators: [{ id: 'i2', type: 'rsi', enabled: true, operator: 'gt', threshold: 70 }] },
          { id: 'grp_3', name: 'Group C', indicators: [{ id: 'i3', type: 'macd', enabled: true, operator: 'gt', threshold: 0 }] },
        ],
      })];
      wrapper.vm.statuses = {
        'cfg-1': {
          state: 'waiting',
          is_running: true,
          all_indicators_pass: true,
          group_results: [
            { group_id: 'grp_1', group_name: 'Group A', pass: true, indicator_results: [
              { type: 'vix', value: 15.5, pass: true },
            ]},
            { group_id: 'grp_2', group_name: 'Group B', pass: true, indicator_results: [
              { type: 'rsi', value: 72.333, pass: true },
            ]},
            { group_id: 'grp_3', group_name: 'Group C', pass: false, indicator_results: [
              { type: 'macd', value: -0.1, pass: false },
            ]},
          ],
        }
      };
      await nextTick();

      // Expand eval section
      await wrapper.find('.indicator-results .section-collapse-header').trigger('click');
      await nextTick();

      const chips = wrapper.findAll('.indicator-results .section-collapse-body .result-chip');
      expect(chips.length).toBe(3);

      // Values should be formatted with toFixed(2)
      expect(chips[0].text()).toContain('15.50');
      expect(chips[1].text()).toContain('72.33');
      expect(chips[2].text()).toContain('-0.10');
    });

    it('4.4: 3-group: expand eval → overall status line with correct passing/failing class and text', async () => {
      wrapper.vm.configs = [makeConfig({
        indicator_groups: [
          { id: 'grp_1', name: 'Alpha', indicators: [{ id: 'i1', type: 'vix', enabled: true, operator: 'lt', threshold: 20 }] },
          { id: 'grp_2', name: 'Beta', indicators: [{ id: 'i2', type: 'rsi', enabled: true, operator: 'gt', threshold: 70 }] },
          { id: 'grp_3', name: 'Gamma', indicators: [{ id: 'i3', type: 'macd', enabled: true, operator: 'gt', threshold: 0 }] },
        ],
      })];
      wrapper.vm.statuses = {
        'cfg-1': {
          state: 'waiting',
          is_running: true,
          all_indicators_pass: true,
          group_results: [
            { group_id: 'grp_1', group_name: 'Alpha', pass: true, indicator_results: [{ type: 'vix', value: 15.0, pass: true }] },
            { group_id: 'grp_2', group_name: 'Beta', pass: false, indicator_results: [{ type: 'rsi', value: 40.0, pass: false }] },
            { group_id: 'grp_3', group_name: 'Gamma', pass: false, indicator_results: [{ type: 'macd', value: -1.0, pass: false }] },
          ],
        }
      };
      await nextTick();

      // Expand eval section
      await wrapper.find('.indicator-results .section-collapse-header').trigger('click');
      await nextTick();

      // Overall status line exists with .passing class (all_indicators_pass = true)
      const overallStatus = wrapper.find('.indicator-results .overall-status');
      expect(overallStatus.exists()).toBe(true);
      expect(overallStatus.classes()).toContain('passing');
      // Text should contain the first passing group name
      expect(overallStatus.text()).toContain('Passing');
      expect(overallStatus.text()).toContain('Alpha');

      // Now test failing case
      wrapper.vm.statuses = {
        'cfg-1': {
          state: 'waiting',
          is_running: true,
          all_indicators_pass: false,
          group_results: [
            { group_id: 'grp_1', group_name: 'Alpha', pass: false, indicator_results: [{ type: 'vix', value: 25.0, pass: false }] },
            { group_id: 'grp_2', group_name: 'Beta', pass: false, indicator_results: [{ type: 'rsi', value: 40.0, pass: false }] },
            { group_id: 'grp_3', group_name: 'Gamma', pass: false, indicator_results: [{ type: 'macd', value: -1.0, pass: false }] },
          ],
        }
      };
      await nextTick();

      const failingStatus = wrapper.find('.indicator-results .overall-status');
      expect(failingStatus.exists()).toBe(true);
      expect(failingStatus.classes()).toContain('failing');
      expect(failingStatus.text()).toContain('Not passing');
    });

    it('4.5: 2-group: expand eval → OR dividers between groups (count = 1)', async () => {
      wrapper.vm.configs = [makeConfig({
        indicator_groups: [
          { id: 'grp_1', name: 'Group A', indicators: [{ id: 'i1', type: 'vix', enabled: true, operator: 'lt', threshold: 20 }] },
          { id: 'grp_2', name: 'Group B', indicators: [{ id: 'i2', type: 'rsi', enabled: true, operator: 'gt', threshold: 70 }] },
        ],
      })];
      wrapper.vm.statuses = {
        'cfg-1': {
          state: 'waiting',
          is_running: true,
          all_indicators_pass: false,
          group_results: [
            { group_id: 'grp_1', group_name: 'Group A', pass: true, indicator_results: [{ type: 'vix', value: 15.0, pass: true }] },
            { group_id: 'grp_2', group_name: 'Group B', pass: false, indicator_results: [{ type: 'rsi', value: 40.0, pass: false }] },
          ],
        }
      };
      await nextTick();

      // Expand eval section
      await wrapper.find('.indicator-results .section-collapse-header').trigger('click');
      await nextTick();

      // OR dividers: 2 groups - 1 = 1
      const dividers = wrapper.findAll('.indicator-results .section-collapse-body .or-divider-compact');
      expect(dividers.length).toBe(1);

      const label = wrapper.find('.indicator-results .section-collapse-body .or-divider-compact-label');
      expect(label.text()).toBe('OR');
    });

  });

  // =========================================================================
  // Step 5 — Static Group Headers: Not Clickable, No Chevrons (AC-3)
  // =========================================================================

  describe('Step 5 — Static Group Headers: Not Clickable, No Chevrons (AC-3)', () => {

    // Shared config and statuses for Step 5 tests
    const step5Config = makeConfig({
      indicator_groups: [
        { id: 'grp_1', name: 'Group A', indicators: [
          { id: 'ind_1', type: 'vix', enabled: true, operator: 'lt', threshold: 20 },
          { id: 'ind_2', type: 'gap', enabled: true, operator: 'lt', threshold: 1.0 },
        ]},
        { id: 'grp_2', name: 'Group B', indicators: [
          { id: 'ind_3', type: 'rsi', enabled: true, operator: 'gt', threshold: 70 },
        ]},
      ],
    });

    const step5Statuses = {
      'cfg-1': {
        state: 'waiting',
        is_running: true,
        all_indicators_pass: true,
        group_results: [
          { group_id: 'grp_1', group_name: 'Group A', pass: true, indicator_results: [
            { type: 'vix', value: 15.0, pass: true },
            { type: 'gap', value: 0.5, pass: true },
          ]},
          { group_id: 'grp_2', group_name: 'Group B', pass: false, indicator_results: [
            { type: 'rsi', value: 40.0, pass: false },
          ]},
        ],
      }
    };

    it('5.1: expanded entry section: .group-header-static elements exist, .group-collapse-header does NOT exist', async () => {
      wrapper.vm.configs = [step5Config];
      await nextTick();

      // Expand entry section
      await wrapper.find('.indicators-section .section-collapse-header').trigger('click');
      await nextTick();

      // .group-header-static should exist (count = number of groups)
      const staticHeaders = wrapper.findAll('.indicators-section .group-header-static');
      expect(staticHeaders.length).toBe(2);

      // .group-collapse-header should NOT exist anywhere
      expect(wrapper.find('.indicators-section .group-collapse-header').exists()).toBe(false);
    });

    it('5.2: expanded eval section: .group-header-static elements exist, .group-collapse-header does NOT exist', async () => {
      wrapper.vm.configs = [step5Config];
      wrapper.vm.statuses = step5Statuses;
      await nextTick();

      // Expand eval section
      await wrapper.find('.indicator-results .section-collapse-header').trigger('click');
      await nextTick();

      // .group-header-static should exist in eval section
      const staticHeaders = wrapper.findAll('.indicator-results .group-header-static');
      expect(staticHeaders.length).toBe(2);

      // .group-collapse-header should NOT exist
      expect(wrapper.find('.indicator-results .group-collapse-header').exists()).toBe(false);
    });

    it('5.3: entry .group-header-static has NO .collapse-chevron-sm or .collapse-chevron child elements', async () => {
      wrapper.vm.configs = [step5Config];
      await nextTick();

      // Expand entry section
      await wrapper.find('.indicators-section .section-collapse-header').trigger('click');
      await nextTick();

      const staticHeaders = wrapper.findAll('.indicators-section .group-header-static');
      expect(staticHeaders.length).toBe(2);

      // Each static header should have no chevron children
      for (const header of staticHeaders) {
        expect(header.find('.collapse-chevron-sm').exists()).toBe(false);
        expect(header.find('.collapse-chevron').exists()).toBe(false);
      }
    });

    it('5.4: eval .group-header-static has NO .collapse-chevron-sm or .collapse-chevron child elements', async () => {
      wrapper.vm.configs = [step5Config];
      wrapper.vm.statuses = step5Statuses;
      await nextTick();

      // Expand eval section
      await wrapper.find('.indicator-results .section-collapse-header').trigger('click');
      await nextTick();

      const staticHeaders = wrapper.findAll('.indicator-results .group-header-static');
      expect(staticHeaders.length).toBe(2);

      // Each static header should have no chevron children
      for (const header of staticHeaders) {
        expect(header.find('.collapse-chevron-sm').exists()).toBe(false);
        expect(header.find('.collapse-chevron').exists()).toBe(false);
      }
    });

    it('5.5: clicking .group-header-static in entry section does NOT change DOM (no toggle behavior)', async () => {
      wrapper.vm.configs = [step5Config];
      await nextTick();

      // Expand entry section
      await wrapper.find('.indicators-section .section-collapse-header').trigger('click');
      await nextTick();

      // Count indicators-grid before click
      const gridsBefore = wrapper.findAll('.indicators-section .indicators-grid').length;
      expect(gridsBefore).toBe(2);

      // Click the first group-header-static
      const staticHeader = wrapper.find('.indicators-section .group-header-static');
      await staticHeader.trigger('click');
      await nextTick();

      // Count indicators-grid after click — should be unchanged
      const gridsAfter = wrapper.findAll('.indicators-section .indicators-grid').length;
      expect(gridsAfter).toBe(gridsBefore);

      // Total indicator chips should also be unchanged
      const chipsBefore = 3; // 2 in Group A + 1 in Group B
      const chipsAfter = wrapper.findAll('.indicators-section .indicator-chip').length;
      expect(chipsAfter).toBe(chipsBefore);
    });

    it('5.6: clicking .group-header-static in eval section does NOT change DOM (no toggle behavior)', async () => {
      wrapper.vm.configs = [step5Config];
      wrapper.vm.statuses = step5Statuses;
      await nextTick();

      // Expand eval section
      await wrapper.find('.indicator-results .section-collapse-header').trigger('click');
      await nextTick();

      // Count results-grid before click
      const gridsBefore = wrapper.findAll('.indicator-results .results-grid').length;
      expect(gridsBefore).toBe(2);

      // Click the first group-header-static in eval
      const staticHeader = wrapper.find('.indicator-results .group-header-static');
      await staticHeader.trigger('click');
      await nextTick();

      // Count results-grid after click — should be unchanged
      const gridsAfter = wrapper.findAll('.indicator-results .results-grid').length;
      expect(gridsAfter).toBe(gridsBefore);

      // Total result chips should also be unchanged
      const chipsBefore = 3; // 2 in Group A + 1 in Group B
      const chipsAfter = wrapper.findAll('.indicator-results .result-chip').length;
      expect(chipsAfter).toBe(chipsBefore);
    });

  });

  // =========================================================================
  // Step 6 — Section-Level Collapse Still Works (AC-5)
  // =========================================================================

  describe('Step 6 — Section-Level Collapse Still Works (AC-5)', () => {

    const step6Config = makeConfig({
      indicator_groups: [
        { id: 'grp_1', name: 'Group A', indicators: [
          { id: 'ind_1', type: 'vix', enabled: true, operator: 'lt', threshold: 20 },
          { id: 'ind_2', type: 'gap', enabled: true, operator: 'lt', threshold: 1.0 },
        ]},
        { id: 'grp_2', name: 'Group B', indicators: [
          { id: 'ind_3', type: 'rsi', enabled: true, operator: 'gt', threshold: 70 },
        ]},
      ],
    });

    const step6Statuses = {
      'cfg-1': {
        state: 'waiting',
        is_running: true,
        all_indicators_pass: true,
        group_results: [
          { group_id: 'grp_1', group_name: 'Group A', pass: true, indicator_results: [
            { type: 'vix', value: 15.0, pass: true },
            { type: 'gap', value: 0.5, pass: true },
          ]},
          { group_id: 'grp_2', group_name: 'Group B', pass: false, indicator_results: [
            { type: 'rsi', value: 40.0, pass: false },
          ]},
        ],
      }
    };

    it('6.1: multi-group entry: collapsed by default → expand → all groups visible → collapse → summary returns, no groups in DOM', async () => {
      wrapper.vm.configs = [step6Config];
      await nextTick();

      // Collapsed by default: no section-collapse-body, no indicator-group-dashboard
      expect(wrapper.find('.indicators-section .section-collapse-body').exists()).toBe(false);
      expect(wrapper.findAll('.indicators-section .indicator-group-dashboard').length).toBe(0);
      // Summary text visible in collapsed header
      expect(wrapper.find('.indicators-section .collapse-summary-text').text()).toBe('2 groups · 3 indicators');

      // Expand entry section
      await wrapper.find('.indicators-section .section-collapse-header').trigger('click');
      await nextTick();

      // All groups visible
      expect(wrapper.find('.indicators-section .section-collapse-body').exists()).toBe(true);
      expect(wrapper.findAll('.indicators-section .indicator-group-dashboard').length).toBe(2);
      expect(wrapper.findAll('.indicators-section .indicator-chip').length).toBe(3);

      // Collapse entry section
      await wrapper.find('.indicators-section .section-collapse-header').trigger('click');
      await nextTick();

      // Back to collapsed: no body, no groups
      expect(wrapper.find('.indicators-section .section-collapse-body').exists()).toBe(false);
      expect(wrapper.findAll('.indicators-section .indicator-group-dashboard').length).toBe(0);
      // Summary text still correct
      expect(wrapper.find('.indicators-section .collapse-summary-text').text()).toBe('2 groups · 3 indicators');
    });

    it('6.2: multi-group eval: collapsed by default → expand → all results visible → collapse → summary returns, no result chips in DOM', async () => {
      wrapper.vm.configs = [step6Config];
      wrapper.vm.statuses = step6Statuses;
      await nextTick();

      // Collapsed by default: no section-collapse-body in eval, no results-grid
      expect(wrapper.find('.indicator-results .section-collapse-body').exists()).toBe(false);
      expect(wrapper.findAll('.indicator-results .results-grid').length).toBe(0);
      // Collapsed summary text visible
      const summaryText = wrapper.find('.indicator-results .collapse-summary-text').text();
      expect(summaryText).toContain('1 of 2 groups passing');

      // Expand eval section
      await wrapper.find('.indicator-results .section-collapse-header').trigger('click');
      await nextTick();

      // All results visible
      expect(wrapper.find('.indicator-results .section-collapse-body').exists()).toBe(true);
      expect(wrapper.findAll('.indicator-results .results-grid').length).toBe(2);
      expect(wrapper.findAll('.indicator-results .result-chip').length).toBe(3);

      // Collapse eval section
      await wrapper.find('.indicator-results .section-collapse-header').trigger('click');
      await nextTick();

      // Back to collapsed: no body, no result chips
      expect(wrapper.find('.indicator-results .section-collapse-body').exists()).toBe(false);
      expect(wrapper.findAll('.indicator-results .results-grid').length).toBe(0);
      expect(wrapper.findAll('.indicator-results .result-chip').length).toBe(0);
      // Summary text still correct
      const summaryAfter = wrapper.find('.indicator-results .collapse-summary-text').text();
      expect(summaryAfter).toContain('1 of 2 groups passing');
    });

    it('6.3: expand entry, then expand eval on same card → both sections expanded simultaneously, all groups visible in both', async () => {
      wrapper.vm.configs = [step6Config];
      wrapper.vm.statuses = step6Statuses;
      await nextTick();

      // Expand entry section
      await wrapper.find('.indicators-section .section-collapse-header').trigger('click');
      await nextTick();

      // Expand eval section
      await wrapper.find('.indicator-results .section-collapse-header').trigger('click');
      await nextTick();

      // Entry: section-collapse-body exists with 2 indicators-grids
      expect(wrapper.find('.indicators-section .section-collapse-body').exists()).toBe(true);
      expect(wrapper.findAll('.indicators-section .indicators-grid').length).toBe(2);

      // Eval: section-collapse-body exists with 2 results-grids
      expect(wrapper.find('.indicator-results .section-collapse-body').exists()).toBe(true);
      expect(wrapper.findAll('.indicator-results .results-grid').length).toBe(2);
    });

    it('6.4: collapse entry while eval stays expanded → entry groups gone, eval groups still visible', async () => {
      wrapper.vm.configs = [step6Config];
      wrapper.vm.statuses = step6Statuses;
      await nextTick();

      // Expand both sections
      await wrapper.find('.indicators-section .section-collapse-header').trigger('click');
      await nextTick();
      await wrapper.find('.indicator-results .section-collapse-header').trigger('click');
      await nextTick();

      // Both expanded
      expect(wrapper.find('.indicators-section .section-collapse-body').exists()).toBe(true);
      expect(wrapper.find('.indicator-results .section-collapse-body').exists()).toBe(true);

      // Collapse entry only
      await wrapper.find('.indicators-section .section-collapse-header').trigger('click');
      await nextTick();

      // Entry: collapsed — no body, no groups
      expect(wrapper.find('.indicators-section .section-collapse-body').exists()).toBe(false);
      expect(wrapper.findAll('.indicators-section .indicator-group-dashboard').length).toBe(0);

      // Eval: still expanded with all results-grids visible
      expect(wrapper.find('.indicator-results .section-collapse-body').exists()).toBe(true);
      expect(wrapper.findAll('.indicator-results .results-grid').length).toBe(2);
      expect(wrapper.findAll('.indicator-results .result-chip').length).toBe(3);
    });

  });

  // =========================================================================
  // Step 7 — Single-Group Cards Unchanged (AC-6)
  // =========================================================================

  describe('Step 7 — Single-Group Cards Unchanged (AC-6)', () => {

    it('7.1: single-group config: no .section-collapse-header, no .group-header-static, no .indicator-group-dashboard in entry section', async () => {
      wrapper.vm.configs = [makeConfig({
        indicator_groups: [
          { id: 'grp_1', name: 'Default', indicators: [
            { id: 'ind_1', type: 'vix', enabled: true, operator: 'lt', threshold: 20 },
            { id: 'ind_2', type: 'rsi', enabled: true, operator: 'gt', threshold: 70 },
          ]},
        ],
      })];
      await nextTick();

      const indicatorsSection = wrapper.find('.indicators-section');

      // No section-collapse-header (single group → flat path)
      expect(indicatorsSection.find('.section-collapse-header').exists()).toBe(false);
      // No group-header-static
      expect(indicatorsSection.find('.group-header-static').exists()).toBe(false);
      // No indicator-group-dashboard
      expect(indicatorsSection.find('.indicator-group-dashboard').exists()).toBe(false);

      // Flat indicator chips should render directly
      expect(indicatorsSection.findAll('.indicator-chip').length).toBe(2);
    });

    it('7.2: single-group running config: flat .result-chip elements rendered, no .group-header-static, no .section-collapse-header in eval', async () => {
      wrapper.vm.configs = [makeConfig({
        indicator_groups: [
          { id: 'grp_1', name: 'Default', indicators: [
            { id: 'ind_1', type: 'vix', enabled: true, operator: 'lt', threshold: 20 },
            { id: 'ind_2', type: 'rsi', enabled: true, operator: 'gt', threshold: 70 },
          ]},
        ],
      })];
      wrapper.vm.statuses = {
        'cfg-1': {
          state: 'waiting',
          is_running: true,
          all_indicators_pass: true,
          // Single group → flat indicator_results (only 1 group_result, which is < 2)
          group_results: [
            { group_id: 'grp_1', group_name: 'Default', pass: true, indicator_results: [
              { type: 'vix', value: 15.0, pass: true },
              { type: 'rsi', value: 80.0, pass: true },
            ]},
          ],
          indicator_results: [
            { type: 'vix', value: 15.0, pass: true },
            { type: 'rsi', value: 80.0, pass: true },
          ],
        }
      };
      await nextTick();

      const statusDetails = wrapper.find('.status-details');
      expect(statusDetails.exists()).toBe(true);

      // Flat result chips rendered (via v-else-if indicator_results path)
      const resultChips = statusDetails.findAll('.result-chip');
      expect(resultChips.length).toBeGreaterThanOrEqual(1);

      // No group infrastructure in eval area
      expect(statusDetails.find('.indicator-results .section-collapse-header').exists()).toBe(false);
      expect(statusDetails.find('.indicator-results .group-header-static').exists()).toBe(false);
    });

    it('7.3: legacy config (no indicator_groups): renders flat indicator chips, no group infrastructure', async () => {
      wrapper.vm.configs = [makeConfig({
        indicator_groups: [],
        indicators: [
          { id: 'ind_1', type: 'vix', enabled: true, operator: 'lt', threshold: 20 },
          { id: 'ind_2', type: 'gap', enabled: true, operator: 'lt', threshold: 1.0 },
          { id: 'ind_3', type: 'rsi', enabled: true, operator: 'gt', threshold: 70 },
        ],
      })];
      await nextTick();

      const indicatorsSection = wrapper.find('.indicators-section');

      // Flat indicator chips rendered
      expect(indicatorsSection.findAll('.indicator-chip').length).toBe(3);

      // No group infrastructure
      expect(indicatorsSection.find('.section-collapse-header').exists()).toBe(false);
      expect(indicatorsSection.find('.group-header-static').exists()).toBe(false);
      expect(indicatorsSection.find('.indicator-group-dashboard').exists()).toBe(false);
    });

  });
});
