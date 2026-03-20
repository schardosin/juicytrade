import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { mount } from '@vue/test-utils';
import { ref, nextTick } from 'vue';
import AutomationDashboard from '../src/components/automation/AutomationDashboard.vue';
import webSocketClient from '../src/services/webSocketClient.js';
import { api } from '../src/services/api.js';

// Mock dependencies — identical to qa-AutomationDashboard.test.js
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

describe('QA — Dashboard Compact Mode (Steps 2–3)', () => {
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
  // Step 2 — Boundary: Exactly 2 Groups (Minimum Multi-Group Threshold)
  // =========================================================================

  describe('Step 2 — Boundary: Exactly 2 Groups', () => {

    it('2.1: config with exactly 2 groups renders collapsed summary header in entry section', async () => {
      wrapper.vm.configs = [makeConfig({
        indicator_groups: [
          { id: 'grp_1', name: 'Group A', indicators: [{ id: 'ind_1', type: 'vix', enabled: true, operator: 'lt', threshold: 20 }] },
          { id: 'grp_2', name: 'Group B', indicators: [{ id: 'ind_2', type: 'rsi', enabled: true, operator: 'gt', threshold: 70 }] },
        ],
      })];
      await nextTick();

      // Entry section should have a section-collapse-header (collapsed summary)
      const indicatorsSection = wrapper.find('.indicators-section');
      expect(indicatorsSection.find('.section-collapse-header').exists()).toBe(true);
      expect(indicatorsSection.find('.collapse-summary-text').text()).toBe('2 groups · 2 indicators');

      // Should NOT render flat indicator chips (collapsed by default)
      expect(indicatorsSection.find('.section-collapse-body').exists()).toBe(false);
      expect(indicatorsSection.findAll('.indicator-chip').length).toBe(0);
    });

    it('2.2: config with exactly 2 group_results renders collapsed eval summary', async () => {
      wrapper.vm.configs = [makeConfig({
        indicator_groups: [
          { id: 'grp_1', name: 'Alpha', indicators: [{ id: 'ind_1', type: 'vix', enabled: true, operator: 'lt', threshold: 20 }] },
          { id: 'grp_2', name: 'Beta', indicators: [{ id: 'ind_2', type: 'rsi', enabled: true, operator: 'gt', threshold: 70 }] },
        ],
      })];
      wrapper.vm.statuses = {
        'cfg-1': {
          state: 'waiting',
          is_running: true,
          all_indicators_pass: true,
          group_results: [
            { group_id: 'grp_1', group_name: 'Alpha', pass: true, indicator_results: [{ type: 'vix', value: 15, pass: true }] },
            { group_id: 'grp_2', group_name: 'Beta', pass: false, indicator_results: [{ type: 'rsi', value: 40, pass: false }] },
          ],
        }
      };
      await nextTick();

      // Eval section should have a collapsed summary header
      const statusDetails = wrapper.find('.status-details');
      expect(statusDetails.exists()).toBe(true);
      const evalHeader = statusDetails.find('.indicator-results .section-collapse-header');
      expect(evalHeader.exists()).toBe(true);

      const summaryText = evalHeader.find('.collapse-summary-text').text();
      expect(summaryText).toContain('Passing');
      expect(summaryText).toContain('Alpha');
      expect(summaryText).toContain('1 of 2 groups passing');

      // Body should NOT exist (collapsed by default)
      expect(statusDetails.find('.indicator-results .section-collapse-body').exists()).toBe(false);
    });

    it('2.3: config transitions from 1 group to 2 groups — collapse header appears', async () => {
      // Start with 1 group (flat path)
      wrapper.vm.configs = [makeConfig({
        indicator_groups: [
          { id: 'grp_1', name: 'Only Group', indicators: [{ id: 'ind_1', type: 'vix', enabled: true, operator: 'lt', threshold: 20 }] },
        ],
      })];
      await nextTick();

      // No collapse header for single group
      expect(wrapper.find('.indicators-section .section-collapse-header').exists()).toBe(false);
      // Flat indicator chips should render
      expect(wrapper.findAll('.indicator-chip').length).toBe(1);

      // Transition to 2 groups by replacing configs
      wrapper.vm.configs = [makeConfig({
        indicator_groups: [
          { id: 'grp_1', name: 'Group A', indicators: [{ id: 'ind_1', type: 'vix', enabled: true, operator: 'lt', threshold: 20 }] },
          { id: 'grp_2', name: 'Group B', indicators: [{ id: 'ind_2', type: 'rsi', enabled: true, operator: 'gt', threshold: 70 }] },
        ],
      })];
      await nextTick();

      // Collapse header should now appear
      expect(wrapper.find('.indicators-section .section-collapse-header').exists()).toBe(true);
      expect(wrapper.find('.collapse-summary-text').text()).toBe('2 groups · 2 indicators');
    });

    it('2.4: config with exactly 1 group renders flat — no .section-collapse-header', async () => {
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
      // No section collapse header
      expect(indicatorsSection.find('.section-collapse-header').exists()).toBe(false);
      // No group collapse header
      expect(indicatorsSection.find('.group-collapse-header').exists()).toBe(false);
      // Flat indicator chips should render directly
      expect(indicatorsSection.findAll('.indicator-chip').length).toBe(2);
    });

    it('2.5: config with 0 groups and legacy indicators renders flat', async () => {
      wrapper.vm.configs = [makeConfig({
        indicator_groups: [],
        indicators: [
          { id: 'ind_1', type: 'vix', enabled: true, operator: 'lt', threshold: 20 },
          { id: 'ind_2', type: 'gap', enabled: true, operator: 'lt', threshold: 1.0 },
        ],
      })];
      await nextTick();

      const indicatorsSection = wrapper.find('.indicators-section');
      // No section collapse header (0 groups → isMultiGroup false)
      expect(indicatorsSection.find('.section-collapse-header').exists()).toBe(false);
      // Flat indicator chips from legacy indicators array
      expect(indicatorsSection.findAll('.indicator-chip').length).toBe(2);
      // No group containers
      expect(indicatorsSection.findAll('.indicator-group-dashboard').length).toBe(0);
    });

  });

  // =========================================================================
  // Step 3 — Adversarial / Unusual Data Shapes
  // =========================================================================

  describe('Step 3 — Adversarial / Unusual Data Shapes', () => {

    it('3.1: multi-group config where every group has empty indicators → summary shows "0 indicators"', async () => {
      wrapper.vm.configs = [makeConfig({
        indicator_groups: [
          { id: 'grp_1', name: 'Empty A', indicators: [] },
          { id: 'grp_2', name: 'Empty B', indicators: [] },
        ],
      })];
      await nextTick();

      const summaryText = wrapper.find('.collapse-summary-text').text();
      expect(summaryText).toBe('2 groups · 0 indicators');
    });

    it('3.2: group with missing indicators property (undefined) — getEntrySummary does not throw', async () => {
      wrapper.vm.configs = [makeConfig({
        indicator_groups: [
          { id: 'grp_1', name: 'Has indicators', indicators: [{ id: 'ind_1', type: 'vix', enabled: true, operator: 'lt', threshold: 20 }] },
          { id: 'grp_2', name: 'Missing indicators' },  // indicators is undefined
        ],
      })];
      await nextTick();

      // Should not crash — getEntrySummary guards with (g.indicators || [])
      const summaryText = wrapper.find('.collapse-summary-text').text();
      expect(summaryText).toBe('2 groups · 1 indicators');

      // Expanding should not crash either
      await wrapper.find('.section-collapse-header').trigger('click');
      await nextTick();

      // Both groups should render
      expect(wrapper.findAll('.indicator-group-dashboard').length).toBe(2);

      // Group with undefined indicators should show "0 indicators"
      const counts = wrapper.findAll('.group-indicator-count');
      expect(counts[0].text()).toBe('1 indicators');
      expect(counts[1].text()).toBe('0 indicators');
    });

    it('3.3: group with indicators: null — no crash on .filter()', async () => {
      wrapper.vm.configs = [makeConfig({
        indicator_groups: [
          { id: 'grp_1', name: 'Group A', indicators: [{ id: 'ind_1', type: 'vix', enabled: true, operator: 'lt', threshold: 20 }] },
          { id: 'grp_2', name: 'Null indicators', indicators: null },
        ],
      })];
      await nextTick();

      // Should not crash — (g.indicators || []) guards against null
      const summaryText = wrapper.find('.collapse-summary-text').text();
      expect(summaryText).toBe('2 groups · 1 indicators');

      // Expand section — groups are always visible (no group-level collapse)
      await wrapper.find('.section-collapse-header').trigger('click');
      await nextTick();

      const groupHeaders = wrapper.findAll('.group-header-static');
      expect(groupHeaders.length).toBe(2);

      // Both indicators-grids should be visible immediately
      const grids = wrapper.findAll('.section-collapse-body .indicators-grid');
      expect(grids.length).toBe(2);
      // Group A has 1 enabled indicator chip
      expect(grids[0].findAll('.indicator-chip').length).toBe(1);
      // Group with null indicators has 0 chips (no crash)
      expect(grids[1].findAll('.indicator-chip').length).toBe(0);
    });

    it('3.4: status with group_results: [] — getEvalSummary returns null, flat path, no crash', async () => {
      wrapper.vm.configs = [makeConfig({
        indicator_groups: [
          { id: 'grp_1', name: 'A', indicators: [{ id: 'ind_1', type: 'vix', enabled: true, operator: 'lt', threshold: 20 }] },
          { id: 'grp_2', name: 'B', indicators: [{ id: 'ind_2', type: 'rsi', enabled: true, operator: 'gt', threshold: 70 }] },
        ],
      })];
      wrapper.vm.statuses = {
        'cfg-1': {
          state: 'waiting',
          is_running: true,
          group_results: [],  // empty array
          indicator_results: [{ type: 'vix', value: 18, pass: true }],
          all_indicators_pass: true,
        }
      };
      await nextTick();

      // getEvalSummary should return null because group_results.length is 0 (falsy)
      expect(wrapper.vm.getEvalSummary('cfg-1')).toBeNull();

      // Template should use the flat path (v-else-if with indicator_results)
      const statusDetails = wrapper.find('.status-details');
      expect(statusDetails.exists()).toBe(true);

      // No multi-group collapse header in eval area
      expect(statusDetails.find('.indicator-results .section-collapse-header').exists()).toBe(false);

      // Flat result chips should render
      expect(statusDetails.findAll('.result-chip').length).toBe(1);
    });

    it('3.5: group with indicator_results: [] — expand shows no result chips, no error', async () => {
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
          all_indicators_pass: false,
          group_results: [
            { group_id: 'grp_1', group_name: 'Group A', pass: false, indicator_results: [] },
            { group_id: 'grp_2', group_name: 'Group B', pass: false, indicator_results: [{ type: 'rsi', value: 40, pass: false }] },
          ],
        }
      };
      await nextTick();

      // Expand eval section — results grids are visible immediately
      await wrapper.find('.indicator-results .section-collapse-header').trigger('click');
      await nextTick();

      // Both groups' results-grids should be visible (no group-level collapse)
      const grids = wrapper.findAll('.indicator-results .section-collapse-body .results-grid');
      expect(grids.length).toBe(2);
      expect(grids[0].findAll('.result-chip').length).toBe(0);
      // Group B should have 1 result chip
      expect(grids[1].findAll('.result-chip').length).toBe(1);
    });

    it('3.6: group with indicator_results: undefined — expand should not throw', async () => {
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
          all_indicators_pass: false,
          group_results: [
            { group_id: 'grp_1', group_name: 'Group A', pass: false },  // indicator_results is undefined
            { group_id: 'grp_2', group_name: 'Group B', pass: true, indicator_results: [{ type: 'rsi', value: 80, pass: true }] },
          ],
        }
      };
      await nextTick();

      // Expand eval section — groups are always visible (no group-level collapse)
      await wrapper.find('.indicator-results .section-collapse-header').trigger('click');
      await nextTick();

      // The results-grid should render but with 0 result chips for undefined indicator_results
      // (v-for on undefined renders nothing — no crash)
      const groupDashboards = wrapper.findAll('.indicator-results .indicator-group-dashboard');
      expect(groupDashboards.length).toBe(2);
    });

    it('3.7: config where indicator_groups is null — isMultiGroup returns false, flat display, no crash', async () => {
      wrapper.vm.configs = [makeConfig({
        indicator_groups: null,
        indicators: [
          { id: 'ind_1', type: 'vix', enabled: true, operator: 'lt', threshold: 20 },
        ],
      })];
      await nextTick();

      // isMultiGroup should return false (null?.length is undefined, || 0 → 0, > 1 is false)
      expect(wrapper.vm.isMultiGroup(wrapper.vm.configs[0])).toBe(false);

      // No section collapse header
      expect(wrapper.find('.indicators-section .section-collapse-header').exists()).toBe(false);
      // Flat indicator chips from legacy indicators fallback
      expect(wrapper.findAll('.indicator-chip').length).toBe(1);
    });

    it('3.8: group_results with pass: undefined — passing resolves to false', async () => {
      wrapper.vm.statuses = {
        'cfg-1': {
          state: 'waiting',
          is_running: true,
          group_results: [
            { group_id: 'grp_1', group_name: 'Group A', pass: undefined, indicator_results: [] },
            { group_id: 'grp_2', group_name: 'Group B', pass: undefined, indicator_results: [] },
          ],
        }
      };
      await nextTick();

      const result = wrapper.vm.getEvalSummary('cfg-1');
      expect(result).not.toBeNull();
      // pass: undefined is falsy → filter(g => g.pass) yields 0 groups
      expect(result.passing).toBe(false);
      expect(result.passingGroupName).toBeNull();
      expect(result.summary).toBe('0 of 2 groups passing');
    });

    it('3.9: group with group_name: null — falls back to "Group N"', async () => {
      wrapper.vm.configs = [makeConfig({
        indicator_groups: [
          { id: 'grp_1', name: null, indicators: [{ id: 'ind_1', type: 'vix', enabled: true, operator: 'lt', threshold: 20 }] },
          { id: 'grp_2', name: 'Valid Name', indicators: [{ id: 'ind_2', type: 'rsi', enabled: true, operator: 'gt', threshold: 70 }] },
        ],
      })];
      await nextTick();

      // Expand entry section
      await wrapper.find('.section-collapse-header').trigger('click');
      await nextTick();

      const groupLabels = wrapper.findAll('.group-label');
      expect(groupLabels.length).toBe(2);
      // group.name is null → template uses: group.name || 'Group ' + (gIdx + 1)
      expect(groupLabels[0].text()).toBe('Group 1');
      expect(groupLabels[1].text()).toBe('Valid Name');
    });

    it('3.10: group with group_name: "" (empty string) — falls back to "Group N"', async () => {
      wrapper.vm.configs = [makeConfig({
        indicator_groups: [
          { id: 'grp_1', name: '', indicators: [{ id: 'ind_1', type: 'vix', enabled: true, operator: 'lt', threshold: 20 }] },
          { id: 'grp_2', name: 'Second Group', indicators: [{ id: 'ind_2', type: 'rsi', enabled: true, operator: 'gt', threshold: 70 }] },
        ],
      })];
      await nextTick();

      // Expand entry section
      await wrapper.find('.section-collapse-header').trigger('click');
      await nextTick();

      const groupLabels = wrapper.findAll('.group-label');
      expect(groupLabels.length).toBe(2);
      // group.name is '' (falsy) → template uses: group.name || 'Group ' + (gIdx + 1)
      expect(groupLabels[0].text()).toBe('Group 1');
      expect(groupLabels[1].text()).toBe('Second Group');
    });

  });

  // =========================================================================
  // Step 4 — Large / Extreme Configs
  // =========================================================================

  describe('Step 4 — Large / Extreme Configs', () => {

    // Helper: generate N groups, each with M enabled indicators
    const makeLargeConfig = (numGroups, indicatorsPerGroup) => {
      const groups = [];
      const indicatorTypes = ['vix', 'rsi', 'macd', 'momentum', 'cmo', 'stoch', 'adx', 'cci', 'sma', 'ema'];
      for (let g = 0; g < numGroups; g++) {
        const indicators = [];
        for (let i = 0; i < indicatorsPerGroup; i++) {
          indicators.push({
            id: `ind_${g}_${i}`,
            type: indicatorTypes[i % indicatorTypes.length],
            enabled: true,
            operator: 'gt',
            threshold: (g + 1) * 10 + i,
          });
        }
        groups.push({ id: `grp_${g}`, name: `Group ${g + 1}`, indicators });
      }
      return makeConfig({ id: 'cfg-large', indicator_groups: groups });
    };

    it('4.1: 20 groups × 10 indicators each → summary reads "20 groups · 200 indicators"', async () => {
      wrapper.vm.configs = [makeLargeConfig(20, 10)];
      await nextTick();

      const summaryText = wrapper.find('.collapse-summary-text').text();
      expect(summaryText).toBe('20 groups · 200 indicators');
    });

    it('4.2: expand section → 20 group-header-static, 19 OR dividers', async () => {
      wrapper.vm.configs = [makeLargeConfig(20, 10)];
      await nextTick();

      // Expand the entry section
      await wrapper.find('.section-collapse-header').trigger('click');
      await nextTick();

      // 20 static group headers
      expect(wrapper.findAll('.group-header-static').length).toBe(20);
      // 19 OR dividers (between 20 groups)
      expect(wrapper.findAll('.or-divider-compact').length).toBe(19);
      // 20 indicator-group-dashboard containers
      expect(wrapper.findAll('.indicator-group-dashboard').length).toBe(20);
    });

    it('4.3: status with 20 group_results → eval summary shows correct count', async () => {
      wrapper.vm.configs = [makeLargeConfig(20, 10)];

      // Build 20 group_results: groups 0, 4, 9 pass (3 passing)
      const groupResults = [];
      for (let g = 0; g < 20; g++) {
        const pass = (g === 0 || g === 4 || g === 9);
        groupResults.push({
          group_id: `grp_${g}`,
          group_name: `Group ${g + 1}`,
          pass,
          indicator_results: [{ type: 'vix', value: pass ? 15 : 25, pass }],
        });
      }
      wrapper.vm.statuses = {
        'cfg-large': {
          state: 'waiting',
          is_running: true,
          all_indicators_pass: true,
          group_results: groupResults,
        }
      };
      await nextTick();

      const evalSummary = wrapper.vm.getEvalSummary('cfg-large');
      expect(evalSummary.passing).toBe(true);
      expect(evalSummary.passingGroupName).toBe('Group 1'); // first passing group
      expect(evalSummary.summary).toBe('3 of 20 groups passing');

      // Verify in the DOM
      const summaryText = wrapper.find('.indicator-results .collapse-summary-text').text();
      expect(summaryText).toContain('3 of 20 groups passing');
    });

    it('4.4: 2 groups, one has 50 enabled indicators → group header shows "50 indicators"', async () => {
      const config = makeConfig({
        indicator_groups: [
          { id: 'grp_1', name: 'Small Group', indicators: [
            { id: 'ind_s1', type: 'vix', enabled: true, operator: 'lt', threshold: 20 },
          ]},
          { id: 'grp_2', name: 'Large Group', indicators: Array.from({ length: 50 }, (_, i) => ({
            id: `ind_l${i}`,
            type: ['vix', 'rsi', 'macd', 'sma', 'ema'][i % 5],
            enabled: true,
            operator: 'gt',
            threshold: i + 1,
          }))},
        ],
      });
      wrapper.vm.configs = [config];
      await nextTick();

      // Summary should show total: 2 groups · 51 indicators
      expect(wrapper.find('.collapse-summary-text').text()).toBe('2 groups · 51 indicators');

      // Expand entry section to see group headers
      await wrapper.find('.section-collapse-header').trigger('click');
      await nextTick();

      const counts = wrapper.findAll('.group-indicator-count');
      expect(counts.length).toBe(2);
      expect(counts[0].text()).toBe('1 indicators');
      expect(counts[1].text()).toBe('50 indicators');
    });

  });

  // =========================================================================
  // Step 5 — Multi-Card Independence
  // =========================================================================

  describe('Step 5 — Multi-Card Independence', () => {

    // Card A: multi-group
    const cardA = makeConfig({
      id: 'card-A',
      name: 'Card A',
      indicator_groups: [
        { id: 'grp_a1', name: 'A-Group1', indicators: [{ id: 'a1', type: 'vix', enabled: true, operator: 'lt', threshold: 20 }] },
        { id: 'grp_a2', name: 'A-Group2', indicators: [{ id: 'a2', type: 'rsi', enabled: true, operator: 'gt', threshold: 70 }] },
      ],
    });

    // Card B: multi-group
    const cardB = makeConfig({
      id: 'card-B',
      name: 'Card B',
      indicator_groups: [
        { id: 'grp_b1', name: 'B-Group1', indicators: [{ id: 'b1', type: 'macd', enabled: true, operator: 'gt', threshold: 0 }] },
        { id: 'grp_b2', name: 'B-Group2', indicators: [{ id: 'b2', type: 'sma', enabled: true, operator: 'gt', threshold: 50 }] },
      ],
    });

    // Card C: single-group (flat rendering)
    const cardC = makeConfig({
      id: 'card-C',
      name: 'Card C',
      indicator_groups: [
        { id: 'grp_c1', name: 'Default', indicators: [{ id: 'c1', type: 'ema', enabled: true, operator: 'gt', threshold: 30 }] },
      ],
    });

    it('5.1: expand card A entry → card B stays collapsed, card C has no collapse header', async () => {
      wrapper.vm.configs = [cardA, cardB, cardC];
      await nextTick();

      const cards = wrapper.findAll('.config-card');
      expect(cards.length).toBe(3);

      // Card A: has collapse header (multi-group)
      expect(cards[0].find('.indicators-section .section-collapse-header').exists()).toBe(true);
      // Card B: has collapse header (multi-group)
      expect(cards[1].find('.indicators-section .section-collapse-header').exists()).toBe(true);
      // Card C: NO collapse header (single-group, flat)
      expect(cards[2].find('.indicators-section .section-collapse-header').exists()).toBe(false);
      // Card C should render flat indicator chip
      expect(cards[2].findAll('.indicator-chip').length).toBe(1);

      // Expand card A entry section
      await cards[0].find('.indicators-section .section-collapse-header').trigger('click');
      await nextTick();

      // Card A: expanded — section-collapse-body should exist
      expect(cards[0].find('.indicators-section .section-collapse-body').exists()).toBe(true);
      expect(cards[0].findAll('.indicator-group-dashboard').length).toBe(2);

      // Card B: still collapsed — NO section-collapse-body
      expect(cards[1].find('.indicators-section .section-collapse-body').exists()).toBe(false);

      // Card C: still flat, no collapse infrastructure
      expect(cards[2].find('.indicators-section .section-collapse-header').exists()).toBe(false);
      expect(cards[2].findAll('.indicator-chip').length).toBe(1);
    });

    it('5.2: expand card A eval, then expand card B eval → both show all results-grids independently', async () => {
      wrapper.vm.configs = [cardA, cardB];

      // Set up running statuses for both cards
      wrapper.vm.statuses = {
        'card-A': {
          state: 'waiting', is_running: true, all_indicators_pass: true,
          group_results: [
            { group_id: 'grp_a1', group_name: 'A-Group1', pass: true, indicator_results: [{ type: 'vix', value: 15, pass: true }] },
            { group_id: 'grp_a2', group_name: 'A-Group2', pass: false, indicator_results: [{ type: 'rsi', value: 40, pass: false }] },
          ],
        },
        'card-B': {
          state: 'waiting', is_running: true, all_indicators_pass: false,
          group_results: [
            { group_id: 'grp_b1', group_name: 'B-Group1', pass: false, indicator_results: [{ type: 'macd', value: -1, pass: false }] },
            { group_id: 'grp_b2', group_name: 'B-Group2', pass: false, indicator_results: [{ type: 'sma', value: 45, pass: false }] },
          ],
        },
      };
      await nextTick();

      const cards = wrapper.findAll('.config-card');

      // Expand card A eval section
      await cards[0].find('.indicator-results .section-collapse-header').trigger('click');
      await nextTick();

      // Card A: all results-grids visible immediately (no group-level collapse)
      expect(cards[0].findAll('.indicator-results .results-grid').length).toBe(2);

      // Now expand card B eval section
      await cards[1].find('.indicator-results .section-collapse-header').trigger('click');
      await nextTick();

      // Card B: all results-grids visible immediately
      expect(cards[1].find('.indicator-results .section-collapse-body').exists()).toBe(true);
      expect(cards[1].findAll('.indicator-results .results-grid').length).toBe(2);

      // Verify card B has static group headers
      const cardBGroupHeaders = cards[1].findAll('.indicator-results .group-header-static');
      expect(cardBGroupHeaders.length).toBe(2);
    });

    it('5.3: collapse card A entry back — A is collapsed while B remains unchanged', async () => {
      wrapper.vm.configs = [cardA, cardB];
      await nextTick();

      const cards = wrapper.findAll('.config-card');

      // Expand both cards' entry sections
      await cards[0].find('.indicators-section .section-collapse-header').trigger('click');
      await nextTick();
      await cards[1].find('.indicators-section .section-collapse-header').trigger('click');
      await nextTick();

      // Both should be expanded
      expect(cards[0].find('.indicators-section .section-collapse-body').exists()).toBe(true);
      expect(cards[1].find('.indicators-section .section-collapse-body').exists()).toBe(true);

      // Collapse card A entry section
      await cards[0].find('.indicators-section .section-collapse-header').trigger('click');
      await nextTick();

      // Card A should be collapsed
      expect(cards[0].find('.indicators-section .section-collapse-body').exists()).toBe(false);
      // Card B should still be expanded
      expect(cards[1].find('.indicators-section .section-collapse-body').exists()).toBe(true);
      expect(cards[1].findAll('.indicator-group-dashboard').length).toBe(2);
    });

    it('5.4: expand entry on both → all groups visible on both cards independently', async () => {
      wrapper.vm.configs = [cardA, cardB];
      await nextTick();

      const cards = wrapper.findAll('.config-card');

      // Expand both cards' entry sections
      await cards[0].find('.indicators-section .section-collapse-header').trigger('click');
      await nextTick();
      await cards[1].find('.indicators-section .section-collapse-header').trigger('click');
      await nextTick();

      // Verify static group headers exist (no clickable group-collapse-header)
      expect(cards[0].findAll('.indicators-section .group-header-static').length).toBe(2);
      expect(cards[1].findAll('.indicators-section .group-header-static').length).toBe(2);

      // All indicators-grids visible on both cards (no group-level collapse)
      expect(cards[0].findAll('.indicators-section .indicators-grid').length).toBe(2);
      expect(cards[1].findAll('.indicators-section .indicators-grid').length).toBe(2);

      // Card A shows VIX and RSI chips
      const cardAChips = cards[0].findAll('.indicators-section .indicator-chip .indicator-name');
      expect(cardAChips.length).toBe(2);
      expect(cardAChips[0].text()).toBe('VIX');
      expect(cardAChips[1].text()).toBe('RSI');

      // Card B shows MACD and SMA chips
      const cardBChips = cards[1].findAll('.indicators-section .indicator-chip .indicator-name');
      expect(cardBChips.length).toBe(2);
      expect(cardBChips[0].text()).toBe('MACD');
      expect(cardBChips[1].text()).toBe('SMA');
    });

  });

  // =========================================================================
  // Step 6 — Rapid Toggle Sequences
  // =========================================================================

  describe('Step 6 — Rapid Toggle Sequences', () => {

    const multiGroupConfig = makeConfig({
      id: 'cfg-rapid',
      indicator_groups: [
        { id: 'grp_1', name: 'Group A', indicators: [{ id: 'ind_1', type: 'vix', enabled: true, operator: 'lt', threshold: 20 }] },
        { id: 'grp_2', name: 'Group B', indicators: [{ id: 'ind_2', type: 'rsi', enabled: true, operator: 'gt', threshold: 70 }] },
      ],
    });

    const multiGroupStatuses = {
      'cfg-rapid': {
        state: 'waiting', is_running: true, all_indicators_pass: true,
        group_results: [
          { group_id: 'grp_1', group_name: 'Group A', pass: true, indicator_results: [{ type: 'vix', value: 15, pass: true }] },
          { group_id: 'grp_2', group_name: 'Group B', pass: false, indicator_results: [{ type: 'rsi', value: 40, pass: false }] },
        ],
      }
    };

    it('6.1: click entry section header 10 times rapidly — final state matches expected parity (even = collapsed)', async () => {
      wrapper.vm.configs = [multiGroupConfig];
      await nextTick();

      const header = wrapper.find('.indicators-section .section-collapse-header');

      // Click 10 times rapidly (even number → back to collapsed)
      for (let i = 0; i < 10; i++) {
        await header.trigger('click');
      }
      await nextTick();

      // 10 clicks (even) → collapsed (back to default)
      expect(wrapper.vm.expandedEntrySections['cfg-rapid']).toBe(false);
      expect(wrapper.find('.indicators-section .section-collapse-body').exists()).toBe(false);
    });

    it('6.2: click eval section header 10 times rapidly — same parity check', async () => {
      wrapper.vm.configs = [multiGroupConfig];
      wrapper.vm.statuses = multiGroupStatuses;
      await nextTick();

      const header = wrapper.find('.indicator-results .section-collapse-header');

      // Click 10 times (even → collapsed)
      for (let i = 0; i < 10; i++) {
        await header.trigger('click');
      }
      await nextTick();

      expect(wrapper.vm.expandedEvalSections['cfg-rapid']).toBe(false);
      expect(wrapper.find('.indicator-results .section-collapse-body').exists()).toBe(false);
    });

    it('6.4: expand entry, collapse section, re-expand → indicators-grids still visible', async () => {
      wrapper.vm.configs = [multiGroupConfig];
      await nextTick();

      // Expand entry section
      await wrapper.find('.indicators-section .section-collapse-header').trigger('click');
      await nextTick();

      // All indicators-grids visible (no group-level collapse)
      expect(wrapper.findAll('.indicators-section .indicators-grid').length).toBe(2);

      // Collapse the section
      await wrapper.find('.indicators-section .section-collapse-header').trigger('click');
      await nextTick();

      // Section body gone
      expect(wrapper.find('.indicators-section .section-collapse-body').exists()).toBe(false);

      // Re-expand the section
      await wrapper.find('.indicators-section .section-collapse-header').trigger('click');
      await nextTick();

      // All indicators-grids should be visible again
      expect(wrapper.findAll('.indicators-section .indicators-grid').length).toBe(2);
    });

    it('6.5: expand eval, collapse eval, WebSocket update, re-expand → results-grids visible', async () => {
      wrapper.vm.configs = [multiGroupConfig];
      wrapper.vm.statuses = multiGroupStatuses;
      await nextTick();

      // Expand eval section
      await wrapper.find('.indicator-results .section-collapse-header').trigger('click');
      await nextTick();

      // All results-grids visible immediately (no group-level collapse)
      expect(wrapper.findAll('.indicator-results .results-grid').length).toBe(2);

      // Collapse the eval section
      await wrapper.find('.indicator-results .section-collapse-header').trigger('click');
      await nextTick();
      expect(wrapper.find('.indicator-results .section-collapse-body').exists()).toBe(false);

      // Simulate WebSocket status update
      const addCallbackCalls = webSocketClient.addCallback.mock.calls;
      const automationUpdateCall = addCallbackCalls.find(call => call[0] === 'automation_update');
      const handler = automationUpdateCall[1];

      handler({
        automation_id: 'cfg-rapid',
        data: {
          status: 'evaluating',
          group_results: [
            { group_id: 'grp_1', group_name: 'Group A', pass: false, indicator_results: [{ type: 'vix', value: 22, pass: false }] },
            { group_id: 'grp_2', group_name: 'Group B', pass: true, indicator_results: [{ type: 'rsi', value: 80, pass: true }] },
          ],
          all_indicators_pass: true,
        }
      });
      await nextTick();

      // Re-expand the eval section
      await wrapper.find('.indicator-results .section-collapse-header').trigger('click');
      await nextTick();

      // All results-grids should be visible (state survived collapse + data update)
      expect(wrapper.findAll('.indicator-results .results-grid').length).toBe(2);
    });

  });

  // =========================================================================
  // Step 7 — State Persistence Across Data Updates
  // =========================================================================

  describe('Step 7 — State Persistence Across Data Updates', () => {

    it('7.1: expand entry, update configs (same ID, new data) → entry remains expanded with all groups visible', async () => {
      wrapper.vm.configs = [makeConfig({
        id: 'cfg-persist',
        indicator_groups: [
          { id: 'grp_1', name: 'Group A', indicators: [{ id: 'ind_1', type: 'vix', enabled: true, operator: 'lt', threshold: 20 }] },
          { id: 'grp_2', name: 'Group B', indicators: [{ id: 'ind_2', type: 'rsi', enabled: true, operator: 'gt', threshold: 70 }] },
        ],
      })];
      await nextTick();

      // Expand entry section
      await wrapper.find('.indicators-section .section-collapse-header').trigger('click');
      await nextTick();

      expect(wrapper.vm.expandedEntrySections['cfg-persist']).toBe(true);
      // All indicators-grids visible (no group-level collapse)
      expect(wrapper.findAll('.indicators-section .indicators-grid').length).toBe(2);

      // Update configs with new indicator data (same config ID, changed thresholds)
      wrapper.vm.configs = [makeConfig({
        id: 'cfg-persist',
        indicator_groups: [
          { id: 'grp_1', name: 'Group A', indicators: [{ id: 'ind_1', type: 'vix', enabled: true, operator: 'lt', threshold: 25 }] },
          { id: 'grp_2', name: 'Group B', indicators: [{ id: 'ind_2', type: 'rsi', enabled: true, operator: 'gt', threshold: 60 }] },
        ],
      })];
      await nextTick();

      // Entry section should still be expanded
      expect(wrapper.vm.expandedEntrySections['cfg-persist']).toBe(true);
      expect(wrapper.find('.indicators-section .section-collapse-body').exists()).toBe(true);

      // All indicators-grids should still be visible
      expect(wrapper.findAll('.indicators-section .indicators-grid').length).toBe(2);
    });

    it('7.2: expand eval, WebSocket update with new group_results → eval stays expanded', async () => {
      wrapper.vm.configs = [makeConfig({
        id: 'cfg-ws',
        indicator_groups: [
          { id: 'grp_1', name: 'Group A', indicators: [{ id: 'ind_1', type: 'vix', enabled: true, operator: 'lt', threshold: 20 }] },
          { id: 'grp_2', name: 'Group B', indicators: [{ id: 'ind_2', type: 'rsi', enabled: true, operator: 'gt', threshold: 70 }] },
        ],
      })];
      wrapper.vm.statuses = {
        'cfg-ws': {
          state: 'waiting', is_running: true, all_indicators_pass: false,
          group_results: [
            { group_id: 'grp_1', group_name: 'Group A', pass: false, indicator_results: [{ type: 'vix', value: 25, pass: false }] },
            { group_id: 'grp_2', group_name: 'Group B', pass: false, indicator_results: [{ type: 'rsi', value: 40, pass: false }] },
          ],
        }
      };
      await nextTick();

      // Expand eval section
      await wrapper.find('.indicator-results .section-collapse-header').trigger('click');
      await nextTick();
      expect(wrapper.find('.indicator-results .section-collapse-body').exists()).toBe(true);

      // Simulate handleAutomationUpdate via WebSocket
      const addCallbackCalls = webSocketClient.addCallback.mock.calls;
      const automationUpdateCall = addCallbackCalls.find(call => call[0] === 'automation_update');
      const handler = automationUpdateCall[1];

      handler({
        automation_id: 'cfg-ws',
        data: {
          status: 'evaluating',
          group_results: [
            { group_id: 'grp_1', group_name: 'Group A', pass: true, indicator_results: [{ type: 'vix', value: 15, pass: true }] },
            { group_id: 'grp_2', group_name: 'Group B', pass: true, indicator_results: [{ type: 'rsi', value: 80, pass: true }] },
          ],
          all_indicators_pass: true,
        }
      });
      await nextTick();

      // Eval section should still be expanded
      expect(wrapper.vm.expandedEvalSections['cfg-ws']).toBe(true);
      expect(wrapper.find('.indicator-results .section-collapse-body').exists()).toBe(true);
    });

    it('7.3: expand eval, WebSocket removes a group (3→2) → eval stays expanded, no crash', async () => {
      wrapper.vm.configs = [makeConfig({
        id: 'cfg-shrink',
        indicator_groups: [
          { id: 'grp_1', name: 'A', indicators: [{ id: 'i1', type: 'vix', enabled: true, operator: 'lt', threshold: 20 }] },
          { id: 'grp_2', name: 'B', indicators: [{ id: 'i2', type: 'rsi', enabled: true, operator: 'gt', threshold: 70 }] },
          { id: 'grp_3', name: 'C', indicators: [{ id: 'i3', type: 'macd', enabled: true, operator: 'gt', threshold: 0 }] },
        ],
      })];
      wrapper.vm.statuses = {
        'cfg-shrink': {
          state: 'waiting', is_running: true, all_indicators_pass: false,
          group_results: [
            { group_id: 'grp_1', group_name: 'A', pass: false, indicator_results: [{ type: 'vix', value: 25, pass: false }] },
            { group_id: 'grp_2', group_name: 'B', pass: true, indicator_results: [{ type: 'rsi', value: 80, pass: true }] },
            { group_id: 'grp_3', group_name: 'C', pass: false, indicator_results: [{ type: 'macd', value: -1, pass: false }] },
          ],
        }
      };
      await nextTick();

      // Expand eval section
      await wrapper.find('.indicator-results .section-collapse-header').trigger('click');
      await nextTick();

      // All results-grids visible (no group-level collapse)
      expect(wrapper.findAll('.indicator-results .results-grid').length).toBe(3);

      // WebSocket update: group C removed (3 → 2 groups)
      const addCallbackCalls = webSocketClient.addCallback.mock.calls;
      const automationUpdateCall = addCallbackCalls.find(call => call[0] === 'automation_update');
      const handler = automationUpdateCall[1];

      handler({
        automation_id: 'cfg-shrink',
        data: {
          status: 'evaluating',
          group_results: [
            { group_id: 'grp_1', group_name: 'A', pass: true, indicator_results: [{ type: 'vix', value: 15, pass: true }] },
            { group_id: 'grp_2', group_name: 'B', pass: true, indicator_results: [{ type: 'rsi', value: 80, pass: true }] },
          ],
          all_indicators_pass: true,
        }
      });
      await nextTick();

      // Eval section should still be expanded (no crash)
      expect(wrapper.vm.expandedEvalSections['cfg-shrink']).toBe(true);
      expect(wrapper.find('.indicator-results .section-collapse-body').exists()).toBe(true);

      // Now only 2 groups rendered
      expect(wrapper.findAll('.indicator-results .indicator-group-dashboard').length).toBe(2);
    });

    it('7.4: expand eval, WebSocket adds a group (2→3) → all groups visible including new group', async () => {
      wrapper.vm.configs = [makeConfig({
        id: 'cfg-grow',
        indicator_groups: [
          { id: 'grp_1', name: 'A', indicators: [{ id: 'i1', type: 'vix', enabled: true, operator: 'lt', threshold: 20 }] },
          { id: 'grp_2', name: 'B', indicators: [{ id: 'i2', type: 'rsi', enabled: true, operator: 'gt', threshold: 70 }] },
        ],
      })];
      wrapper.vm.statuses = {
        'cfg-grow': {
          state: 'waiting', is_running: true, all_indicators_pass: true,
          group_results: [
            { group_id: 'grp_1', group_name: 'A', pass: true, indicator_results: [{ type: 'vix', value: 15, pass: true }] },
            { group_id: 'grp_2', group_name: 'B', pass: false, indicator_results: [{ type: 'rsi', value: 40, pass: false }] },
          ],
        }
      };
      await nextTick();

      // Expand eval section
      await wrapper.find('.indicator-results .section-collapse-header').trigger('click');
      await nextTick();

      // All results-grids visible (no group-level collapse)
      expect(wrapper.findAll('.indicator-results .results-grid').length).toBe(2);

      // WebSocket update: group C added (2 → 3 groups)
      const addCallbackCalls = webSocketClient.addCallback.mock.calls;
      const automationUpdateCall = addCallbackCalls.find(call => call[0] === 'automation_update');
      const handler = automationUpdateCall[1];

      handler({
        automation_id: 'cfg-grow',
        data: {
          status: 'evaluating',
          group_results: [
            { group_id: 'grp_1', group_name: 'A', pass: true, indicator_results: [{ type: 'vix', value: 15, pass: true }] },
            { group_id: 'grp_2', group_name: 'B', pass: true, indicator_results: [{ type: 'rsi', value: 80, pass: true }] },
            { group_id: 'grp_3', group_name: 'C', pass: false, indicator_results: [{ type: 'macd', value: -1, pass: false }] },
          ],
          all_indicators_pass: true,
        }
      });
      await nextTick();

      // Eval section should still be expanded
      expect(wrapper.vm.expandedEvalSections['cfg-grow']).toBe(true);

      // Now 3 groups rendered
      expect(wrapper.findAll('.indicator-results .indicator-group-dashboard').length).toBe(3);

      // All 3 results-grids should be visible
      expect(wrapper.findAll('.indicator-results .results-grid').length).toBe(3);
    });

    it('7.5: expand entry, replace configs with same ID but different group count → entry remains expanded', async () => {
      wrapper.vm.configs = [makeConfig({
        id: 'cfg-replace',
        indicator_groups: [
          { id: 'grp_1', name: 'A', indicators: [{ id: 'i1', type: 'vix', enabled: true, operator: 'lt', threshold: 20 }] },
          { id: 'grp_2', name: 'B', indicators: [{ id: 'i2', type: 'rsi', enabled: true, operator: 'gt', threshold: 70 }] },
        ],
      })];
      await nextTick();

      // Expand entry section
      await wrapper.find('.indicators-section .section-collapse-header').trigger('click');
      await nextTick();

      expect(wrapper.vm.expandedEntrySections['cfg-replace']).toBe(true);
      expect(wrapper.findAll('.indicator-group-dashboard').length).toBe(2);

      // Replace configs entirely with same config ID but 3 groups
      wrapper.vm.configs = [makeConfig({
        id: 'cfg-replace',
        indicator_groups: [
          { id: 'grp_1', name: 'A', indicators: [{ id: 'i1', type: 'vix', enabled: true, operator: 'lt', threshold: 20 }] },
          { id: 'grp_2', name: 'B', indicators: [{ id: 'i2', type: 'rsi', enabled: true, operator: 'gt', threshold: 70 }] },
          { id: 'grp_3', name: 'C', indicators: [{ id: 'i3', type: 'macd', enabled: true, operator: 'gt', threshold: 0 }] },
        ],
      })];
      await nextTick();

      // Entry section should still be expanded (keyed by config ID)
      expect(wrapper.vm.expandedEntrySections['cfg-replace']).toBe(true);
      expect(wrapper.find('.indicators-section .section-collapse-body').exists()).toBe(true);
      // Now 3 groups rendered
      expect(wrapper.findAll('.indicator-group-dashboard').length).toBe(3);
    });

    it('7.6: status transitions from no group_results (flat) to group_results with 2+ entries → DOM switches to collapsed grouped view', async () => {
      wrapper.vm.configs = [makeConfig({
        id: 'cfg-transition',
        indicator_groups: [
          { id: 'grp_1', name: 'A', indicators: [{ id: 'i1', type: 'vix', enabled: true, operator: 'lt', threshold: 20 }] },
          { id: 'grp_2', name: 'B', indicators: [{ id: 'i2', type: 'rsi', enabled: true, operator: 'gt', threshold: 70 }] },
        ],
      })];

      // Start with flat indicator_results (no group_results)
      wrapper.vm.statuses = {
        'cfg-transition': {
          state: 'waiting', is_running: true,
          indicator_results: [
            { type: 'vix', value: 15, pass: true },
            { type: 'rsi', value: 80, pass: true },
          ],
          all_indicators_pass: true,
        }
      };
      await nextTick();

      // Should render flat path (no multi-group collapse header in eval area)
      const statusDetails = wrapper.find('.status-details');
      expect(statusDetails.exists()).toBe(true);
      expect(statusDetails.find('.indicator-results .section-collapse-header').exists()).toBe(false);
      expect(statusDetails.findAll('.result-chip').length).toBe(2);

      // Transition: WebSocket update with group_results (2+ entries)
      const addCallbackCalls = webSocketClient.addCallback.mock.calls;
      const automationUpdateCall = addCallbackCalls.find(call => call[0] === 'automation_update');
      const handler = automationUpdateCall[1];

      handler({
        automation_id: 'cfg-transition',
        data: {
          status: 'evaluating',
          group_results: [
            { group_id: 'grp_1', group_name: 'A', pass: true, indicator_results: [{ type: 'vix', value: 15, pass: true }] },
            { group_id: 'grp_2', group_name: 'B', pass: true, indicator_results: [{ type: 'rsi', value: 80, pass: true }] },
          ],
          indicator_results: [
            { type: 'vix', value: 15, pass: true },
            { type: 'rsi', value: 80, pass: true },
          ],
          all_indicators_pass: true,
        }
      });
      await nextTick();

      // DOM should now show the grouped collapsed view (section-collapse-header)
      expect(wrapper.find('.indicator-results .section-collapse-header').exists()).toBe(true);
      // Flat result-chip elements should no longer be visible (collapsed by default)
      expect(wrapper.find('.indicator-results .section-collapse-body').exists()).toBe(false);
    });

    it('7.7: status transitions from group_results with 3 entries to group_results with 1 → DOM switches back to flat', async () => {
      wrapper.vm.configs = [makeConfig({
        id: 'cfg-flatten',
        indicator_groups: [
          { id: 'grp_1', name: 'A', indicators: [{ id: 'i1', type: 'vix', enabled: true, operator: 'lt', threshold: 20 }] },
          { id: 'grp_2', name: 'B', indicators: [{ id: 'i2', type: 'rsi', enabled: true, operator: 'gt', threshold: 70 }] },
          { id: 'grp_3', name: 'C', indicators: [{ id: 'i3', type: 'macd', enabled: true, operator: 'gt', threshold: 0 }] },
        ],
      })];

      // Start with 3 group_results (multi-group collapsed view)
      wrapper.vm.statuses = {
        'cfg-flatten': {
          state: 'waiting', is_running: true, all_indicators_pass: true,
          group_results: [
            { group_id: 'grp_1', group_name: 'A', pass: true, indicator_results: [{ type: 'vix', value: 15, pass: true }] },
            { group_id: 'grp_2', group_name: 'B', pass: false, indicator_results: [{ type: 'rsi', value: 40, pass: false }] },
            { group_id: 'grp_3', group_name: 'C', pass: false, indicator_results: [{ type: 'macd', value: -1, pass: false }] },
          ],
        }
      };
      await nextTick();

      // Should render grouped collapsed view
      expect(wrapper.find('.indicator-results .section-collapse-header').exists()).toBe(true);

      // Transition: WebSocket update with only 1 group_result (falls below threshold)
      const addCallbackCalls = webSocketClient.addCallback.mock.calls;
      const automationUpdateCall = addCallbackCalls.find(call => call[0] === 'automation_update');
      const handler = automationUpdateCall[1];

      handler({
        automation_id: 'cfg-flatten',
        data: {
          status: 'evaluating',
          group_results: [
            { group_id: 'grp_1', group_name: 'A', pass: true, indicator_results: [{ type: 'vix', value: 15, pass: true }] },
          ],
          indicator_results: [
            { type: 'vix', value: 15, pass: true },
          ],
          all_indicators_pass: true,
        }
      });
      await nextTick();

      // DOM should switch back to flat rendering (group_results.length <= 1)
      // No multi-group collapse header
      expect(wrapper.find('.indicator-results .section-collapse-header').exists()).toBe(false);
      // Flat result chips should render
      expect(wrapper.findAll('.result-chip').length).toBe(1);
    });

  });

  // =========================================================================
  // Step 8 — Entry Criteria Summary Text Accuracy
  // =========================================================================

  describe('Step 8 — Entry Criteria Summary Text Accuracy', () => {

    it('8.1: 3 groups with [3 enabled/0 disabled], [2 enabled/1 disabled], [1 enabled/2 disabled] → "3 groups · 6 indicators"', () => {
      const config = {
        indicator_groups: [
          { id: 'g1', indicators: [
            { type: 'vix', enabled: true }, { type: 'gap', enabled: true }, { type: 'rsi', enabled: true },
          ]},
          { id: 'g2', indicators: [
            { type: 'macd', enabled: true }, { type: 'cci', enabled: true }, { type: 'adx', enabled: false },
          ]},
          { id: 'g3', indicators: [
            { type: 'sma', enabled: true }, { type: 'ema', enabled: false }, { type: 'atr', enabled: false },
          ]},
        ]
      };
      expect(wrapper.vm.getEntrySummary(config)).toBe('3 groups · 6 indicators');
    });

    it('8.2: 2 groups with all indicators enabled: false → "2 groups · 0 indicators"', () => {
      const config = {
        indicator_groups: [
          { id: 'g1', indicators: [
            { type: 'vix', enabled: false }, { type: 'gap', enabled: false },
          ]},
          { id: 'g2', indicators: [
            { type: 'rsi', enabled: false },
          ]},
        ]
      };
      expect(wrapper.vm.getEntrySummary(config)).toBe('2 groups · 0 indicators');
    });

    it('8.3: 4 groups with 1 indicator each, all enabled → "4 groups · 4 indicators"', () => {
      const config = {
        indicator_groups: [
          { id: 'g1', indicators: [{ type: 'vix', enabled: true }] },
          { id: 'g2', indicators: [{ type: 'rsi', enabled: true }] },
          { id: 'g3', indicators: [{ type: 'macd', enabled: true }] },
          { id: 'g4', indicators: [{ type: 'sma', enabled: true }] },
        ]
      };
      expect(wrapper.vm.getEntrySummary(config)).toBe('4 groups · 4 indicators');
    });

    it('8.4: rendered .collapse-summary-text matches getEntrySummary() return value', async () => {
      const config = makeConfig({
        indicator_groups: [
          { id: 'g1', name: 'X', indicators: [
            { id: 'i1', type: 'vix', enabled: true, operator: 'lt', threshold: 20 },
            { id: 'i2', type: 'gap', enabled: false, operator: 'lt', threshold: 1 },
          ]},
          { id: 'g2', name: 'Y', indicators: [
            { id: 'i3', type: 'rsi', enabled: true, operator: 'gt', threshold: 70 },
            { id: 'i4', type: 'macd', enabled: true, operator: 'gt', threshold: 0 },
            { id: 'i5', type: 'sma', enabled: true, operator: 'gt', threshold: 100 },
          ]},
          { id: 'g3', name: 'Z', indicators: [
            { id: 'i6', type: 'ema', enabled: true, operator: 'gt', threshold: 50 },
          ]},
        ],
      });
      wrapper.vm.configs = [config];
      await nextTick();

      const computedSummary = wrapper.vm.getEntrySummary(config);
      const renderedText = wrapper.find('.indicators-section .collapse-summary-text').text();
      expect(renderedText).toBe(computedSummary);
      expect(renderedText).toBe('3 groups · 5 indicators');
    });

  });

  // =========================================================================
  // Step 9 — Last Evaluation Summary Text Accuracy
  // =========================================================================

  describe('Step 9 — Last Evaluation Summary Text Accuracy', () => {

    it('9.1: all groups pass → { passing: true, passingGroupName: <first>, summary: "3 of 3 groups passing" }', async () => {
      wrapper.vm.statuses = {
        'cfg-all-pass': {
          state: 'waiting',
          group_results: [
            { group_id: 'g1', group_name: 'Alpha', pass: true, indicator_results: [] },
            { group_id: 'g2', group_name: 'Beta', pass: true, indicator_results: [] },
            { group_id: 'g3', group_name: 'Gamma', pass: true, indicator_results: [] },
          ],
        }
      };
      await nextTick();

      const result = wrapper.vm.getEvalSummary('cfg-all-pass');
      expect(result).toEqual({
        passing: true,
        passingGroupName: 'Alpha',
        summary: '3 of 3 groups passing',
      });
    });

    it('9.2: no groups pass → { passing: false, passingGroupName: null, summary: "0 of 3 groups passing" }', async () => {
      wrapper.vm.statuses = {
        'cfg-none-pass': {
          state: 'waiting',
          group_results: [
            { group_id: 'g1', group_name: 'Alpha', pass: false, indicator_results: [] },
            { group_id: 'g2', group_name: 'Beta', pass: false, indicator_results: [] },
            { group_id: 'g3', group_name: 'Gamma', pass: false, indicator_results: [] },
          ],
        }
      };
      await nextTick();

      const result = wrapper.vm.getEvalSummary('cfg-none-pass');
      expect(result).toEqual({
        passing: false,
        passingGroupName: null,
        summary: '0 of 3 groups passing',
      });
    });

    it('9.3: multiple groups pass → passingGroupName is the first passing group (array order)', async () => {
      wrapper.vm.statuses = {
        'cfg-multi-pass': {
          state: 'waiting',
          group_results: [
            { group_id: 'g1', group_name: 'First Fail', pass: false, indicator_results: [] },
            { group_id: 'g2', group_name: 'Second Pass', pass: true, indicator_results: [] },
            { group_id: 'g3', group_name: 'Third Pass', pass: true, indicator_results: [] },
            { group_id: 'g4', group_name: 'Fourth Fail', pass: false, indicator_results: [] },
          ],
        }
      };
      await nextTick();

      const result = wrapper.vm.getEvalSummary('cfg-multi-pass');
      expect(result.passing).toBe(true);
      // First passing group by array order is "Second Pass"
      expect(result.passingGroupName).toBe('Second Pass');
      expect(result.summary).toBe('2 of 4 groups passing');
    });

    it('9.4: rendered collapsed summary DOM includes checkmark entity for passing, cross entity for not-passing', async () => {
      // Test passing case
      wrapper.vm.configs = [makeConfig({
        id: 'cfg-pass',
        indicator_groups: [
          { id: 'g1', name: 'PassGroup', indicators: [{ id: 'i1', type: 'vix', enabled: true, operator: 'lt', threshold: 20 }] },
          { id: 'g2', name: 'FailGroup', indicators: [{ id: 'i2', type: 'rsi', enabled: true, operator: 'gt', threshold: 70 }] },
        ],
      })];
      wrapper.vm.statuses = {
        'cfg-pass': {
          state: 'waiting', is_running: true, all_indicators_pass: true,
          group_results: [
            { group_id: 'g1', group_name: 'PassGroup', pass: true, indicator_results: [{ type: 'vix', value: 15, pass: true }] },
            { group_id: 'g2', group_name: 'FailGroup', pass: false, indicator_results: [{ type: 'rsi', value: 40, pass: false }] },
          ],
        }
      };
      await nextTick();

      const passSummaryText = wrapper.find('.indicator-results .collapse-summary-text').text();
      // ✓ = \u2713 (&#10003;)
      expect(passSummaryText).toContain('\u2713');
      expect(passSummaryText).toContain('Passing');

      // Now test not-passing case
      wrapper.vm.statuses = {
        'cfg-pass': {
          state: 'waiting', is_running: true, all_indicators_pass: false,
          group_results: [
            { group_id: 'g1', group_name: 'PassGroup', pass: false, indicator_results: [{ type: 'vix', value: 25, pass: false }] },
            { group_id: 'g2', group_name: 'FailGroup', pass: false, indicator_results: [{ type: 'rsi', value: 40, pass: false }] },
          ],
        }
      };
      await nextTick();

      const failSummaryText = wrapper.find('.indicator-results .collapse-summary-text').text();
      // ✗ = \u2717 (&#10007;)
      expect(failSummaryText).toContain('\u2717');
      expect(failSummaryText).toContain('Not passing');
    });

    it('9.5: 1 of 10 groups passing → "1 of 10 groups passing"', async () => {
      const groupResults = [];
      for (let i = 0; i < 10; i++) {
        groupResults.push({
          group_id: `g${i}`,
          group_name: `Group ${i + 1}`,
          pass: i === 7, // Only group 8 (index 7) passes
          indicator_results: [],
        });
      }
      wrapper.vm.statuses = {
        'cfg-10': { state: 'waiting', group_results: groupResults }
      };
      await nextTick();

      const result = wrapper.vm.getEvalSummary('cfg-10');
      expect(result.passing).toBe(true);
      expect(result.passingGroupName).toBe('Group 8');
      expect(result.summary).toBe('1 of 10 groups passing');
    });

  });

  // =========================================================================
  // Step 10 — Chevron Icon Correctness
  // =========================================================================

  describe('Step 10 — Chevron Icon Correctness', () => {

    const chevronConfig = makeConfig({
      id: 'cfg-chevron',
      indicator_groups: [
        { id: 'g1', name: 'Group A', indicators: [{ id: 'i1', type: 'vix', enabled: true, operator: 'lt', threshold: 20 }] },
        { id: 'g2', name: 'Group B', indicators: [{ id: 'i2', type: 'rsi', enabled: true, operator: 'gt', threshold: 70 }] },
      ],
    });

    const chevronStatuses = {
      'cfg-chevron': {
        state: 'waiting', is_running: true, all_indicators_pass: true,
        group_results: [
          { group_id: 'g1', group_name: 'Group A', pass: true, indicator_results: [{ type: 'vix', value: 15, pass: true }] },
          { group_id: 'g2', group_name: 'Group B', pass: false, indicator_results: [{ type: 'rsi', value: 40, pass: false }] },
        ],
      }
    };

    it('10.1: collapsed entry section → chevron has class pi-chevron-right', async () => {
      wrapper.vm.configs = [chevronConfig];
      await nextTick();

      const chevron = wrapper.find('.indicators-section .section-collapse-header .collapse-chevron');
      expect(chevron.classes()).toContain('pi-chevron-right');
      expect(chevron.classes()).not.toContain('pi-chevron-down');
    });

    it('10.2: expanded entry section → chevron has class pi-chevron-down', async () => {
      wrapper.vm.configs = [chevronConfig];
      await nextTick();

      await wrapper.find('.indicators-section .section-collapse-header').trigger('click');
      await nextTick();

      const chevron = wrapper.find('.indicators-section .section-collapse-header .collapse-chevron');
      expect(chevron.classes()).toContain('pi-chevron-down');
      expect(chevron.classes()).not.toContain('pi-chevron-right');
    });

    it('10.3: collapsed eval section → chevron has class pi-chevron-right', async () => {
      wrapper.vm.configs = [chevronConfig];
      wrapper.vm.statuses = chevronStatuses;
      await nextTick();

      const chevron = wrapper.find('.indicator-results .section-collapse-header .collapse-chevron');
      expect(chevron.classes()).toContain('pi-chevron-right');
      expect(chevron.classes()).not.toContain('pi-chevron-down');
    });

    it('10.4: expanded eval section → chevron has class pi-chevron-down', async () => {
      wrapper.vm.configs = [chevronConfig];
      wrapper.vm.statuses = chevronStatuses;
      await nextTick();

      await wrapper.find('.indicator-results .section-collapse-header').trigger('click');
      await nextTick();

      const chevron = wrapper.find('.indicator-results .section-collapse-header .collapse-chevron');
      expect(chevron.classes()).toContain('pi-chevron-down');
      expect(chevron.classes()).not.toContain('pi-chevron-right');
    });

    it('10.7: collapse a section and re-expand → chevron returns to pi-chevron-down', async () => {
      wrapper.vm.configs = [chevronConfig];
      await nextTick();

      // Expand entry section
      await wrapper.find('.indicators-section .section-collapse-header').trigger('click');
      await nextTick();

      let chevron = wrapper.find('.indicators-section .section-collapse-header .collapse-chevron');
      expect(chevron.classes()).toContain('pi-chevron-down');

      // Collapse
      await wrapper.find('.indicators-section .section-collapse-header').trigger('click');
      await nextTick();

      chevron = wrapper.find('.indicators-section .section-collapse-header .collapse-chevron');
      expect(chevron.classes()).toContain('pi-chevron-right');

      // Re-expand
      await wrapper.find('.indicators-section .section-collapse-header').trigger('click');
      await nextTick();

      chevron = wrapper.find('.indicators-section .section-collapse-header .collapse-chevron');
      expect(chevron.classes()).toContain('pi-chevron-down');
      expect(chevron.classes()).not.toContain('pi-chevron-right');
    });

  });

  // =========================================================================
  // Step 11 — Interaction with Existing Actions (AC-8)
  // =========================================================================

  describe('Step 11 — Interaction with Existing Actions', () => {

    const actionConfig = makeConfig({
      id: 'cfg-action',
      name: 'Action Config',
      indicator_groups: [
        { id: 'g1', name: 'Group A', indicators: [{ id: 'i1', type: 'vix', enabled: true, operator: 'lt', threshold: 20 }] },
        { id: 'g2', name: 'Group B', indicators: [{ id: 'i2', type: 'rsi', enabled: true, operator: 'gt', threshold: 70 }] },
      ],
    });

    const actionStatuses = {
      'cfg-action': {
        state: 'waiting', is_running: true, all_indicators_pass: true,
        group_results: [
          { group_id: 'g1', group_name: 'Group A', pass: true, indicator_results: [{ type: 'vix', value: 15, pass: true }] },
          { group_id: 'g2', group_name: 'Group B', pass: false, indicator_results: [{ type: 'rsi', value: 40, pass: false }] },
        ],
      }
    };

    it('11.1: with entry section expanded, startAutomation calls api.startAutomation with correct config ID', async () => {
      wrapper.vm.configs = [actionConfig];
      await nextTick();

      // Expand entry section
      await wrapper.find('.indicators-section .section-collapse-header').trigger('click');
      await nextTick();
      expect(wrapper.vm.expandedEntrySections['cfg-action']).toBe(true);

      // Call startAutomation
      await wrapper.vm.startAutomation(actionConfig);
      await nextTick();

      expect(api.startAutomation).toHaveBeenCalledWith('cfg-action');

      // Entry section should still be expanded
      expect(wrapper.vm.expandedEntrySections['cfg-action']).toBe(true);
    });

    it('11.2: with eval section expanded, stopAutomation calls api.stopAutomation with correct config ID', async () => {
      wrapper.vm.configs = [actionConfig];
      wrapper.vm.statuses = actionStatuses;
      await nextTick();

      // Expand eval section
      await wrapper.find('.indicator-results .section-collapse-header').trigger('click');
      await nextTick();

      expect(wrapper.vm.expandedEvalSections['cfg-action']).toBe(true);
      // All results-grids visible immediately (no group-level collapse)
      expect(wrapper.findAll('.indicator-results .results-grid').length).toBe(2);

      // Call stopAutomation
      await wrapper.vm.stopAutomation(actionConfig);
      await nextTick();

      expect(api.stopAutomation).toHaveBeenCalledWith('cfg-action');

      // Eval section should still be expanded
      expect(wrapper.vm.expandedEvalSections['cfg-action']).toBe(true);
    });

    it('11.3: with entry section expanded, evaluateIndicators opens eval dialog, collapse state unchanged after close', async () => {
      // Mock evaluateAutomationConfig to return multi-group results
      api.evaluateAutomationConfig.mockResolvedValueOnce({
        data: {
          all_pass: false,
          group_results: [
            { group_id: 'g1', group_name: 'Group A', pass: true, indicator_results: [{ type: 'vix', value: 15, pass: true, operator: 'lt', threshold: 20 }] },
            { group_id: 'g2', group_name: 'Group B', pass: false, indicator_results: [{ type: 'rsi', value: 40, pass: false, operator: 'gt', threshold: 70 }] },
          ],
          indicators: [
            { type: 'vix', value: 15, pass: true },
            { type: 'rsi', value: 40, pass: false },
          ],
        }
      });

      wrapper.vm.configs = [actionConfig];
      await nextTick();

      // Expand entry section
      await wrapper.find('.indicators-section .section-collapse-header').trigger('click');
      await nextTick();
      expect(wrapper.vm.expandedEntrySections['cfg-action']).toBe(true);

      // Call evaluateIndicators
      await wrapper.vm.evaluateIndicators(actionConfig);
      await nextTick();

      expect(api.evaluateAutomationConfig).toHaveBeenCalledWith('cfg-action');

      // Dialog should be open with results
      expect(wrapper.vm.showEvalDialog).toBe(true);
      expect(wrapper.vm.evalResult).not.toBeNull();
      expect(wrapper.vm.evalResult.group_results.length).toBe(2);

      // Close dialog
      wrapper.vm.showEvalDialog = false;
      await nextTick();

      // Entry section should still be expanded
      expect(wrapper.vm.expandedEntrySections['cfg-action']).toBe(true);
    });

    it('11.4: with eval section expanded, editConfig navigates to edit route', async () => {
      wrapper.vm.configs = [actionConfig];
      wrapper.vm.statuses = actionStatuses;
      await nextTick();

      // Expand eval section
      await wrapper.find('.indicator-results .section-collapse-header').trigger('click');
      await nextTick();
      expect(wrapper.vm.expandedEvalSections['cfg-action']).toBe(true);

      // editConfig calls router.push internally — verify it doesn't throw
      // and that eval section state is preserved
      expect(() => wrapper.vm.editConfig(actionConfig)).not.toThrow();

      // Eval section should still be expanded (navigation doesn't reset collapse state)
      expect(wrapper.vm.expandedEvalSections['cfg-action']).toBe(true);
    });

    it('11.5: with entry section expanded, duplicateConfig calls api.createAutomationConfig, collapse state unchanged', async () => {
      api.createAutomationConfig.mockResolvedValueOnce({ data: { id: 'new-dup-id' } });

      wrapper.vm.configs = [actionConfig];
      await nextTick();

      // Expand entry section
      await wrapper.find('.indicators-section .section-collapse-header').trigger('click');
      await nextTick();

      expect(wrapper.vm.expandedEntrySections['cfg-action']).toBe(true);
      // All indicators-grids visible (no group-level collapse)
      expect(wrapper.findAll('.indicators-section .indicators-grid').length).toBe(2);

      // Call duplicateConfig
      await wrapper.vm.duplicateConfig(actionConfig);
      await nextTick();

      expect(api.createAutomationConfig).toHaveBeenCalled();
      const callArg = api.createAutomationConfig.mock.calls[0][0];
      expect(callArg.name).toBe('Action Config (Copy)');

      // Collapse state should be unchanged
      expect(wrapper.vm.expandedEntrySections['cfg-action']).toBe(true);
    });

    it('11.6: with entry section expanded, confirmDelete opens dialog, confirming deletes config', async () => {
      wrapper.vm.configs = [actionConfig];
      await nextTick();

      // Expand entry section
      await wrapper.find('.indicators-section .section-collapse-header').trigger('click');
      await nextTick();
      expect(wrapper.vm.expandedEntrySections['cfg-action']).toBe(true);

      // Call confirmDelete — should open dialog
      wrapper.vm.confirmDelete(actionConfig);
      await nextTick();

      expect(wrapper.vm.showDeleteDialog).toBe(true);
      expect(wrapper.vm.configToDelete).toEqual(actionConfig);

      // Confirm the delete
      await wrapper.vm.deleteConfig();
      await nextTick();

      expect(api.deleteAutomationConfig).toHaveBeenCalledWith('cfg-action');
      expect(wrapper.vm.showDeleteDialog).toBe(false);
      expect(wrapper.vm.configToDelete).toBeNull();
    });

    it('11.7: with eval section expanded, toggleConfig calls api.toggleAutomationConfig, collapse state unchanged', async () => {
      wrapper.vm.configs = [actionConfig];
      wrapper.vm.statuses = actionStatuses;
      await nextTick();

      // Expand eval section
      await wrapper.find('.indicator-results .section-collapse-header').trigger('click');
      await nextTick();
      expect(wrapper.vm.expandedEvalSections['cfg-action']).toBe(true);

      // Call toggleConfig
      await wrapper.vm.toggleConfig(actionConfig);
      await nextTick();

      expect(api.toggleAutomationConfig).toHaveBeenCalledWith('cfg-action');

      // Eval section should still be expanded
      expect(wrapper.vm.expandedEvalSections['cfg-action']).toBe(true);
    });

    it('11.8: click View Logs with eval section expanded → logs dialog opens, eval state unchanged', async () => {
      api.getAutomationLogs.mockResolvedValueOnce({ data: { logs: [{ timestamp: '2025-01-01T12:00:00Z', level: 'info', message: 'test log' }] } });

      wrapper.vm.configs = [actionConfig];
      wrapper.vm.statuses = actionStatuses;
      await nextTick();

      // Expand eval section
      await wrapper.find('.indicator-results .section-collapse-header').trigger('click');
      await nextTick();

      expect(wrapper.vm.expandedEvalSections['cfg-action']).toBe(true);
      // All results-grids visible immediately (no group-level collapse)
      expect(wrapper.findAll('.indicator-results .results-grid').length).toBe(2);

      // Call showLogs
      await wrapper.vm.showLogs(actionConfig);
      await nextTick();

      expect(wrapper.vm.showLogsDialog).toBe(true);
      expect(api.getAutomationLogs).toHaveBeenCalledWith('cfg-action');

      // Eval section should still be expanded
      expect(wrapper.vm.expandedEvalSections['cfg-action']).toBe(true);
    });

  });

  // =========================================================================
  // Step 12 — Evaluation Dialog Unaffected (AC-8)
  // =========================================================================

  describe('Step 12 — Evaluation Dialog Unaffected', () => {

    it('12.1: multi-group config: evaluateIndicators → dialog shows full multi-group results', async () => {
      api.evaluateAutomationConfig.mockResolvedValueOnce({
        data: {
          all_pass: true,
          group_results: [
            { group_id: 'g1', group_name: 'Alpha', pass: true, indicator_results: [{ type: 'vix', value: 15, pass: true, operator: 'lt', threshold: 20 }] },
            { group_id: 'g2', group_name: 'Beta', pass: false, indicator_results: [{ type: 'rsi', value: 40, pass: false, operator: 'gt', threshold: 70 }] },
          ],
          indicators: [
            { type: 'vix', value: 15, pass: true },
            { type: 'rsi', value: 40, pass: false },
          ],
        }
      });

      const multiConfig = makeConfig({
        id: 'cfg-eval-multi',
        indicator_groups: [
          { id: 'g1', name: 'Alpha', indicators: [{ id: 'i1', type: 'vix', enabled: true, operator: 'lt', threshold: 20 }] },
          { id: 'g2', name: 'Beta', indicators: [{ id: 'i2', type: 'rsi', enabled: true, operator: 'gt', threshold: 70 }] },
        ],
      });
      wrapper.vm.configs = [multiConfig];
      await nextTick();

      await wrapper.vm.evaluateIndicators(multiConfig);
      await nextTick();

      // Dialog should be open
      expect(wrapper.vm.showEvalDialog).toBe(true);
      expect(wrapper.vm.evalResult.group_results.length).toBe(2);

      // Multi-group rendering in dialog: eval-group-section elements
      const evalGroups = wrapper.findAll('.eval-group-section');
      expect(evalGroups.length).toBe(2);

      // Group names rendered
      const groupNames = wrapper.findAll('.eval-group-name');
      expect(groupNames[0].text()).toBe('Alpha');
      expect(groupNames[1].text()).toBe('Beta');

      // Pass/fail badges
      const badges = wrapper.findAll('.eval-group-section .group-result-badge');
      expect(badges[0].classes()).toContain('passed');
      expect(badges[1].classes()).toContain('failed');

      // OR divider between groups
      expect(wrapper.findAll('.eval-results .or-divider-compact').length).toBe(1);

      // No section-collapse-header or group-collapse-header inside the dialog
      const evalDialog = wrapper.find('.eval-results');
      expect(evalDialog.find('.section-collapse-header').exists()).toBe(false);
      expect(evalDialog.find('.group-collapse-header').exists()).toBe(false);
    });

    it('12.2: single-group config: evaluateIndicators → dialog shows flat results, no group headers', async () => {
      api.evaluateAutomationConfig.mockResolvedValueOnce({
        data: {
          all_pass: true,
          group_results: [
            { group_id: 'g1', group_name: 'Default', pass: true, indicator_results: [{ type: 'vix', value: 15, pass: true }] },
          ],
          indicators: [
            { type: 'vix', value: 15, pass: true, operator: 'lt', threshold: 20 },
          ],
        }
      });

      const singleConfig = makeConfig({
        id: 'cfg-eval-single',
        indicator_groups: [
          { id: 'g1', name: 'Default', indicators: [{ id: 'i1', type: 'vix', enabled: true, operator: 'lt', threshold: 20 }] },
        ],
      });
      wrapper.vm.configs = [singleConfig];
      await nextTick();

      await wrapper.vm.evaluateIndicators(singleConfig);
      await nextTick();

      expect(wrapper.vm.showEvalDialog).toBe(true);

      // Single group → flat rendering path (group_results.length <= 1)
      // No eval-group-section elements
      expect(wrapper.findAll('.eval-group-section').length).toBe(0);
      // No group headers
      expect(wrapper.findAll('.eval-group-name').length).toBe(0);
      // Flat eval items should render
      expect(wrapper.findAll('.eval-item').length).toBe(1);
    });

    it('12.3: close evaluation dialog → card collapse state unchanged', async () => {
      api.evaluateAutomationConfig.mockResolvedValueOnce({
        data: {
          all_pass: false,
          group_results: [
            { group_id: 'g1', group_name: 'A', pass: true, indicator_results: [{ type: 'vix', value: 15, pass: true }] },
            { group_id: 'g2', group_name: 'B', pass: false, indicator_results: [{ type: 'rsi', value: 40, pass: false }] },
          ],
          indicators: [],
        }
      });

      const config = makeConfig({
        id: 'cfg-eval-close',
        indicator_groups: [
          { id: 'g1', name: 'A', indicators: [{ id: 'i1', type: 'vix', enabled: true, operator: 'lt', threshold: 20 }] },
          { id: 'g2', name: 'B', indicators: [{ id: 'i2', type: 'rsi', enabled: true, operator: 'gt', threshold: 70 }] },
        ],
      });
      wrapper.vm.configs = [config];
      wrapper.vm.statuses = {
        'cfg-eval-close': {
          state: 'waiting', is_running: true, all_indicators_pass: false,
          group_results: [
            { group_id: 'g1', group_name: 'A', pass: true, indicator_results: [{ type: 'vix', value: 15, pass: true }] },
            { group_id: 'g2', group_name: 'B', pass: false, indicator_results: [{ type: 'rsi', value: 40, pass: false }] },
          ],
        }
      };
      await nextTick();

      // Expand both entry and eval sections
      await wrapper.find('.indicators-section .section-collapse-header').trigger('click');
      await nextTick();
      await wrapper.find('.indicator-results .section-collapse-header').trigger('click');
      await nextTick();

      // Verify expanded state
      expect(wrapper.vm.expandedEntrySections['cfg-eval-close']).toBe(true);
      expect(wrapper.vm.expandedEvalSections['cfg-eval-close']).toBe(true);

      // Open eval dialog
      await wrapper.vm.evaluateIndicators(config);
      await nextTick();
      expect(wrapper.vm.showEvalDialog).toBe(true);

      // Close the dialog
      wrapper.vm.showEvalDialog = false;
      await nextTick();

      // All card collapse states should be unchanged
      expect(wrapper.vm.expandedEntrySections['cfg-eval-close']).toBe(true);
      expect(wrapper.vm.expandedEvalSections['cfg-eval-close']).toBe(true);

      // DOM should still show expanded sections
      expect(wrapper.find('.indicators-section .section-collapse-body').exists()).toBe(true);
      expect(wrapper.find('.indicator-results .section-collapse-body').exists()).toBe(true);
    });

  });

  // =========================================================================
  // Step 13 — Mixed Card Types in Grid (AC-6, AC-9)
  // =========================================================================

  describe('Step 13 — Mixed Card Types in Grid', () => {

    // Card 1: legacy flat (no indicator_groups, uses indicators array)
    const legacyFlat = makeConfig({
      id: 'card-legacy',
      name: 'Legacy Flat',
      indicator_groups: [],
      indicators: [
        { id: 'i1', type: 'vix', enabled: true, operator: 'lt', threshold: 20 },
        { id: 'i2', type: 'gap', enabled: true, operator: 'lt', threshold: 1 },
      ],
    });

    // Card 2: single-group (flat rendering)
    const singleGroup = makeConfig({
      id: 'card-single',
      name: 'Single Group',
      indicator_groups: [
        { id: 'g1', name: 'Default', indicators: [{ id: 'i1', type: 'vix', enabled: true, operator: 'lt', threshold: 20 }] },
      ],
    });

    // Card 3: 2-group (multi-group, compact mode)
    const twoGroup = makeConfig({
      id: 'card-2grp',
      name: 'Two Groups',
      indicator_groups: [
        { id: 'g1', name: 'Group A', indicators: [{ id: 'i1', type: 'vix', enabled: true, operator: 'lt', threshold: 20 }] },
        { id: 'g2', name: 'Group B', indicators: [{ id: 'i2', type: 'rsi', enabled: true, operator: 'gt', threshold: 70 }] },
      ],
    });

    // Card 4: 3-group (multi-group, compact mode)
    const threeGroup = makeConfig({
      id: 'card-3grp',
      name: 'Three Groups',
      indicator_groups: [
        { id: 'g1', name: 'Alpha', indicators: [{ id: 'i1', type: 'vix', enabled: true, operator: 'lt', threshold: 20 }] },
        { id: 'g2', name: 'Beta', indicators: [{ id: 'i2', type: 'rsi', enabled: true, operator: 'gt', threshold: 70 }] },
        { id: 'g3', name: 'Gamma', indicators: [{ id: 'i3', type: 'macd', enabled: true, operator: 'gt', threshold: 0 }] },
      ],
    });

    // Card 5: disabled 2-group
    const disabled2Group = makeConfig({
      id: 'card-disabled',
      name: 'Disabled Two Groups',
      enabled: false,
      indicator_groups: [
        { id: 'g1', name: 'X', indicators: [{ id: 'i1', type: 'sma', enabled: true, operator: 'gt', threshold: 100 }] },
        { id: 'g2', name: 'Y', indicators: [{ id: 'i2', type: 'ema', enabled: true, operator: 'gt', threshold: 50 }] },
      ],
    });

    it('13.1: 5 cards — only 2-group and 3-group cards have .section-collapse-header in entry section', async () => {
      wrapper.vm.configs = [legacyFlat, singleGroup, twoGroup, threeGroup, disabled2Group];
      await nextTick();

      const cards = wrapper.findAll('.config-card');
      expect(cards.length).toBe(5);

      // Card 0 (legacy flat): NO collapse header
      expect(cards[0].find('.indicators-section .section-collapse-header').exists()).toBe(false);
      // Card 1 (single group): NO collapse header
      expect(cards[1].find('.indicators-section .section-collapse-header').exists()).toBe(false);
      // Card 2 (2-group): HAS collapse header
      expect(cards[2].find('.indicators-section .section-collapse-header').exists()).toBe(true);
      // Card 3 (3-group): HAS collapse header
      expect(cards[3].find('.indicators-section .section-collapse-header').exists()).toBe(true);
      // Card 4 (disabled 2-group): HAS collapse header
      expect(cards[4].find('.indicators-section .section-collapse-header').exists()).toBe(true);

      // Flat cards should have indicator chips rendered directly
      expect(cards[0].findAll('.indicator-chip').length).toBe(2);
      expect(cards[1].findAll('.indicator-chip').length).toBe(1);
    });

    it('13.2: collapsed multi-group cards should not have .indicators-grid in entry section (DOM compactness)', async () => {
      wrapper.vm.configs = [twoGroup, threeGroup];
      await nextTick();

      const cards = wrapper.findAll('.config-card');

      // Both cards are collapsed by default — no indicators-grid inside entry section
      expect(cards[0].find('.indicators-section .indicators-grid').exists()).toBe(false);
      expect(cards[1].find('.indicators-section .indicators-grid').exists()).toBe(false);

      // No section-collapse-body either
      expect(cards[0].find('.indicators-section .section-collapse-body').exists()).toBe(false);
      expect(cards[1].find('.indicators-section .section-collapse-body').exists()).toBe(false);

      // No indicator chips rendered (all behind collapsed sections)
      expect(cards[0].findAll('.indicator-chip').length).toBe(0);
      expect(cards[1].findAll('.indicator-chip').length).toBe(0);
    });

    it('13.3: running multi-group card has collapsed eval section, idle single-group card has no eval section — both render fine', async () => {
      wrapper.vm.configs = [twoGroup, singleGroup];
      wrapper.vm.statuses = {
        'card-2grp': {
          state: 'waiting', is_running: true, all_indicators_pass: true,
          group_results: [
            { group_id: 'g1', group_name: 'Group A', pass: true, indicator_results: [{ type: 'vix', value: 15, pass: true }] },
            { group_id: 'g2', group_name: 'Group B', pass: false, indicator_results: [{ type: 'rsi', value: 40, pass: false }] },
          ],
        },
        // card-single has no status entry → no eval section rendered
      };
      await nextTick();

      const cards = wrapper.findAll('.config-card');

      // Card 0 (2-group, running): has status-details and collapsed eval section
      expect(cards[0].find('.status-details').exists()).toBe(true);
      expect(cards[0].find('.indicator-results .section-collapse-header').exists()).toBe(true);
      expect(cards[0].find('.indicator-results .section-collapse-body').exists()).toBe(false); // collapsed

      // Card 1 (single-group, idle): no status-details at all
      expect(cards[1].find('.status-details').exists()).toBe(false);
      expect(cards[1].find('.indicator-results').exists()).toBe(false);
    });

    it('13.4: disabled multi-group card still renders collapsed entry section (compact mode independent of enabled/disabled)', async () => {
      wrapper.vm.configs = [disabled2Group];
      await nextTick();

      const card = wrapper.find('.config-card');
      // Should have collapse header despite being disabled
      expect(card.find('.indicators-section .section-collapse-header').exists()).toBe(true);
      expect(card.find('.collapse-summary-text').text()).toBe('2 groups · 2 indicators');

      // Expanding should still work
      await card.find('.indicators-section .section-collapse-header').trigger('click');
      await nextTick();

      expect(card.find('.indicators-section .section-collapse-body').exists()).toBe(true);
      expect(card.findAll('.indicator-group-dashboard').length).toBe(2);
    });

  });

  // =========================================================================
  // Step 14 — WebSocket Handler Edge Cases (AC-7)
  // =========================================================================

  describe('Step 14 — WebSocket Handler Edge Cases', () => {

    // Helper to get the WebSocket handler
    const getWsHandler = () => {
      const addCallbackCalls = webSocketClient.addCallback.mock.calls;
      const call = addCallbackCalls.find(c => c[0] === 'automation_update');
      return call[1];
    };

    it('14.1: WebSocket message with non-matching automation_id — statuses updated without crash', async () => {
      wrapper.vm.configs = [makeConfig({ id: 'cfg-existing' })];
      await nextTick();

      const handler = getWsHandler();

      // Send message for an ID that doesn't match any config
      expect(() => {
        handler({
          automation_id: 'cfg-nonexistent',
          data: {
            status: 'evaluating',
            group_results: [
              { group_id: 'g1', group_name: 'Ghost', pass: true, indicator_results: [] },
            ],
          }
        });
      }).not.toThrow();

      await nextTick();

      // Status should be stored even for unknown config (handler doesn't filter by configs)
      expect(wrapper.vm.statuses['cfg-nonexistent']).toBeDefined();
      expect(wrapper.vm.statuses['cfg-nonexistent'].state).toBe('evaluating');
    });

    it('14.2: WebSocket message with data: null — handler does not crash', async () => {
      wrapper.vm.configs = [makeConfig()];
      await nextTick();

      const handler = getWsHandler();

      // Guard: if (message.automation_id && message.data) — data: null is falsy
      expect(() => {
        handler({ automation_id: 'cfg-1', data: null });
      }).not.toThrow();

      await nextTick();

      // Status should NOT have been updated (guard skips)
      expect(wrapper.vm.statuses['cfg-1']).toBeUndefined();
    });

    it('14.3: WebSocket message with data: {} — statuses updated with empty fields, no crash', async () => {
      wrapper.vm.configs = [makeConfig()];
      await nextTick();

      const handler = getWsHandler();

      expect(() => {
        handler({ automation_id: 'cfg-1', data: {} });
      }).not.toThrow();

      await nextTick();

      // Status should be set with empty/undefined fields
      const status = wrapper.vm.statuses['cfg-1'];
      expect(status).toBeDefined();
      expect(status.state).toBeUndefined(); // data.status was undefined
      expect(status.group_results).toBeUndefined();
    });

    it('14.4: two rapid WebSocket messages for same ID — final state reflects last message', async () => {
      wrapper.vm.configs = [makeConfig({
        id: 'cfg-rapid-ws',
        indicator_groups: [
          { id: 'g1', name: 'A', indicators: [{ id: 'i1', type: 'vix', enabled: true, operator: 'lt', threshold: 20 }] },
          { id: 'g2', name: 'B', indicators: [{ id: 'i2', type: 'rsi', enabled: true, operator: 'gt', threshold: 70 }] },
        ],
      })];
      await nextTick();

      const handler = getWsHandler();

      // First message: group A passes
      handler({
        automation_id: 'cfg-rapid-ws',
        data: {
          status: 'evaluating',
          group_results: [
            { group_id: 'g1', group_name: 'A', pass: true, indicator_results: [{ type: 'vix', value: 15, pass: true }] },
            { group_id: 'g2', group_name: 'B', pass: false, indicator_results: [{ type: 'rsi', value: 40, pass: false }] },
          ],
          all_indicators_pass: true,
        }
      });

      // Second message immediately: group B passes instead
      handler({
        automation_id: 'cfg-rapid-ws',
        data: {
          status: 'waiting',
          group_results: [
            { group_id: 'g1', group_name: 'A', pass: false, indicator_results: [{ type: 'vix', value: 25, pass: false }] },
            { group_id: 'g2', group_name: 'B', pass: true, indicator_results: [{ type: 'rsi', value: 80, pass: true }] },
          ],
          all_indicators_pass: true,
        }
      });

      await nextTick();

      // Final state should reflect the LAST message
      const status = wrapper.vm.statuses['cfg-rapid-ws'];
      expect(status.state).toBe('waiting');
      expect(status.group_results[0].pass).toBe(false); // A now fails
      expect(status.group_results[1].pass).toBe(true);  // B now passes

      // getEvalSummary should reflect last state
      const summary = wrapper.vm.getEvalSummary('cfg-rapid-ws');
      expect(summary.passingGroupName).toBe('B');
      expect(summary.summary).toBe('1 of 2 groups passing');
    });

    it('14.5: WebSocket message arrives while entry section expanded and user mid-interaction — collapse state preserved', async () => {
      wrapper.vm.configs = [makeConfig({
        id: 'cfg-mid-interact',
        indicator_groups: [
          { id: 'g1', name: 'A', indicators: [{ id: 'i1', type: 'vix', enabled: true, operator: 'lt', threshold: 20 }] },
          { id: 'g2', name: 'B', indicators: [{ id: 'i2', type: 'rsi', enabled: true, operator: 'gt', threshold: 70 }] },
        ],
      })];
      wrapper.vm.statuses = {
        'cfg-mid-interact': {
          state: 'waiting', is_running: true, all_indicators_pass: false,
          group_results: [
            { group_id: 'g1', group_name: 'A', pass: false, indicator_results: [{ type: 'vix', value: 25, pass: false }] },
            { group_id: 'g2', group_name: 'B', pass: false, indicator_results: [{ type: 'rsi', value: 40, pass: false }] },
          ],
        }
      };
      await nextTick();

      // Expand entry section
      await wrapper.find('.indicators-section .section-collapse-header').trigger('click');
      await nextTick();

      // Expand eval section
      await wrapper.find('.indicator-results .section-collapse-header').trigger('click');
      await nextTick();

      // Verify expanded state
      expect(wrapper.vm.expandedEntrySections['cfg-mid-interact']).toBe(true);
      expect(wrapper.vm.expandedEvalSections['cfg-mid-interact']).toBe(true);

      // WebSocket update arrives mid-interaction
      const handler = getWsHandler();
      handler({
        automation_id: 'cfg-mid-interact',
        data: {
          status: 'evaluating',
          group_results: [
            { group_id: 'g1', group_name: 'A', pass: true, indicator_results: [{ type: 'vix', value: 15, pass: true }] },
            { group_id: 'g2', group_name: 'B', pass: true, indicator_results: [{ type: 'rsi', value: 80, pass: true }] },
          ],
          all_indicators_pass: true,
        }
      });
      await nextTick();

      // All collapse states should be preserved
      expect(wrapper.vm.expandedEntrySections['cfg-mid-interact']).toBe(true);
      expect(wrapper.vm.expandedEvalSections['cfg-mid-interact']).toBe(true);

      // DOM should reflect both expanded states
      expect(wrapper.find('.indicators-section .section-collapse-body').exists()).toBe(true);
      expect(wrapper.find('.indicator-results .section-collapse-body').exists()).toBe(true);
    });

  });

});
