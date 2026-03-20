import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { mount } from '@vue/test-utils';
import { ref, nextTick } from 'vue';
import AutomationConfigForm from '../src/components/automation/AutomationConfigForm.vue';
import { api } from '../src/services/api.js';

// Mock dependencies — identical to existing test file
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
  }
}));

vi.mock('../src/composables/useMobileDetection.js', () => ({
  useMobileDetection: () => ({ isMobile: ref(false) })
}));

describe('QA — AutomationConfigForm', () => {
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

  // =========================================================================
  // 5.1 Group CRUD (AC-5, AC-8)
  // =========================================================================

  describe('5.1 Group CRUD', () => {
    it('addGroup increments group count and assigns sequential names', () => {
      // Start with 1 group ("Default")
      expect(wrapper.vm.config.indicator_groups.length).toBe(1);

      wrapper.vm.addGroup();
      wrapper.vm.addGroup();
      wrapper.vm.addGroup();

      expect(wrapper.vm.config.indicator_groups.length).toBe(4);
      expect(wrapper.vm.config.indicator_groups[1].name).toBe('Group 2');
      expect(wrapper.vm.config.indicator_groups[2].name).toBe('Group 3');
      expect(wrapper.vm.config.indicator_groups[3].name).toBe('Group 4');

      // All should have unique grp_ IDs
      const ids = wrapper.vm.config.indicator_groups.map(g => g.id);
      ids.forEach(id => expect(id).toMatch(/^grp_/));
      expect(new Set(ids).size).toBe(4);
    });

    it('addGroup assigns unique IDs even when called rapidly', () => {
      // Call twice without await between
      wrapper.vm.addGroup();
      wrapper.vm.addGroup();

      const id1 = wrapper.vm.config.indicator_groups[1].id;
      const id2 = wrapper.vm.config.indicator_groups[2].id;
      expect(id1).not.toBe(id2);
      expect(id1).toMatch(/^grp_/);
      expect(id2).toMatch(/^grp_/);
    });

    it('removeGroup with empty group skips confirmation', () => {
      wrapper.vm.addGroup(); // Group 2 with 0 indicators
      expect(wrapper.vm.config.indicator_groups.length).toBe(2);

      const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(false);
      wrapper.vm.removeGroup(1); // Remove the empty group

      expect(confirmSpy).not.toHaveBeenCalled();
      expect(wrapper.vm.config.indicator_groups.length).toBe(1);
      confirmSpy.mockRestore();
    });

    it('removeGroup with indicators prompts confirmation and respects accept', () => {
      // Add an indicator to group 0
      wrapper.vm.config.indicator_groups[0].indicators.push({
        id: 'ind_test_1', type: 'vix', enabled: true, operator: 'lt', threshold: 20
      });
      wrapper.vm.addGroup(); // Need 2 groups to delete

      // Set an indicator result to verify cleanup
      wrapper.vm.indicatorResults['ind_test_1'] = { value: 18, passed: true };

      const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(true);
      wrapper.vm.removeGroup(0);

      expect(confirmSpy).toHaveBeenCalled();
      expect(wrapper.vm.config.indicator_groups.length).toBe(1);
      // Indicator result should be cleaned up
      expect(wrapper.vm.indicatorResults['ind_test_1']).toBeUndefined();
      confirmSpy.mockRestore();
    });

    it('removeGroup cleans up groupTestResults', () => {
      wrapper.vm.addGroup();
      const groupId = wrapper.vm.config.indicator_groups[1].id;

      // Set group test result
      wrapper.vm.groupTestResults[groupId] = { pass: true };
      expect(wrapper.vm.groupTestResults[groupId]).toBeDefined();

      // Remove the group (it's empty, so no confirm)
      wrapper.vm.removeGroup(1);
      expect(wrapper.vm.groupTestResults[groupId]).toBeUndefined();
    });

    it('renaming group updates group.name', () => {
      wrapper.vm.config.indicator_groups[0].name = 'Custom Name';
      expect(wrapper.vm.config.indicator_groups[0].name).toBe('Custom Name');
    });

    it('cannot delete last remaining group', async () => {
      // With only 1 group, the delete button should not be rendered
      // The template has: v-if="config.indicator_groups.length > 1"
      expect(wrapper.vm.config.indicator_groups.length).toBe(1);
      await nextTick();

      // The delete button (pi-trash) inside group-header-actions should not exist
      const deleteButtons = wrapper.findAll('.group-header-actions .p-button-danger');
      expect(deleteButtons.length).toBe(0);
    });
  });

  // =========================================================================
  // 5.2 OR Divider (AC-5, AC-6)
  // =========================================================================

  describe('5.2 OR Divider', () => {
    it('no OR divider with exactly 1 group', async () => {
      await nextTick();
      expect(wrapper.find('.or-divider').exists()).toBe(false);
    });

    it('one OR divider with exactly 2 groups', async () => {
      wrapper.vm.addGroup();
      await nextTick();
      const dividers = wrapper.findAll('.or-divider');
      expect(dividers.length).toBe(1);
    });

    it('two OR dividers with 3 groups', async () => {
      wrapper.vm.addGroup();
      wrapper.vm.addGroup();
      await nextTick();
      const dividers = wrapper.findAll('.or-divider');
      expect(dividers.length).toBe(2);
    });

    it('OR divider disappears when group removed back to 1', async () => {
      wrapper.vm.addGroup();
      await nextTick();
      expect(wrapper.findAll('.or-divider').length).toBe(1);

      // Remove second group (empty, no confirm)
      wrapper.vm.removeGroup(1);
      await nextTick();
      expect(wrapper.find('.or-divider').exists()).toBe(false);
    });
  });

  // =========================================================================
  // 5.3 Test All — Per-Group Results (AC-6, AC-8)
  // =========================================================================

  describe('5.3 Test All — Per-Group Results', () => {
    it('testAllIndicators sends indicator_groups in API payload', async () => {
      // Set up 2 groups with indicators
      wrapper.vm.config.indicator_groups[0].indicators.push(
        { id: 'ind_1', type: 'vix', enabled: true, operator: 'lt', threshold: 20 }
      );
      wrapper.vm.addGroup();
      wrapper.vm.config.indicator_groups[1].indicators.push(
        { id: 'ind_2', type: 'rsi', enabled: true, operator: 'gt', threshold: 30, params: { period: 14 } }
      );

      api.previewAutomationIndicators.mockResolvedValueOnce({
        data: { group_results: [], all_pass: false }
      });

      await wrapper.vm.testAllIndicators();

      expect(api.previewAutomationIndicators).toHaveBeenCalledTimes(1);
      const payload = api.previewAutomationIndicators.mock.calls[0][0];
      expect(payload.indicator_groups).toBeDefined();
      expect(payload.indicator_groups.length).toBe(2);
      // Should not send flat indicators
      expect(payload.indicators).toBeUndefined();
    });

    it('testAllIndicators maps group_results to groupTestResults by group_id', async () => {
      const grpId0 = wrapper.vm.config.indicator_groups[0].id;
      wrapper.vm.config.indicator_groups[0].indicators.push(
        { id: 'ind_1', type: 'vix', enabled: true, operator: 'lt', threshold: 20 }
      );
      wrapper.vm.addGroup();
      const grpId1 = wrapper.vm.config.indicator_groups[1].id;
      wrapper.vm.config.indicator_groups[1].indicators.push(
        { id: 'ind_2', type: 'rsi', enabled: true, operator: 'gt', threshold: 30 }
      );

      api.previewAutomationIndicators.mockResolvedValueOnce({
        data: {
          group_results: [
            { group_id: grpId0, pass: true, indicator_results: [{ type: 'vix', value: 18, pass: true }] },
            { group_id: grpId1, pass: false, indicator_results: [{ type: 'rsi', value: 25, pass: false }] },
          ],
          all_pass: true
        }
      });

      await wrapper.vm.testAllIndicators();

      expect(wrapper.vm.groupTestResults[grpId0]).toEqual({ pass: true });
      expect(wrapper.vm.groupTestResults[grpId1]).toEqual({ pass: false });
    });

    it('testAllIndicators maps individual indicator results by indicator id', async () => {
      const grpId0 = wrapper.vm.config.indicator_groups[0].id;
      wrapper.vm.config.indicator_groups[0].indicators.push(
        { id: 'ind_vix_1', type: 'vix', enabled: true, operator: 'lt', threshold: 20 }
      );

      api.previewAutomationIndicators.mockResolvedValueOnce({
        data: {
          group_results: [
            {
              group_id: grpId0,
              pass: true,
              indicator_results: [
                { type: 'vix', value: 18.5, pass: true, operator: 'lt', threshold: 20, stale: false }
              ]
            }
          ],
          all_pass: true
        }
      });

      await wrapper.vm.testAllIndicators();

      const result = wrapper.vm.indicatorResults['ind_vix_1'];
      expect(result).toBeDefined();
      expect(result.value).toBe(18.5);
      expect(result.passed).toBe(true);
      expect(result.stale).toBe(false);
    });

    it('overall result shows group count for multi-group', async () => {
      // 2 groups, API returns 1 passing
      wrapper.vm.config.indicator_groups[0].indicators.push(
        { id: 'ind_1', type: 'vix', enabled: true, operator: 'lt', threshold: 20 }
      );
      wrapper.vm.addGroup();
      const grpId0 = wrapper.vm.config.indicator_groups[0].id;
      const grpId1 = wrapper.vm.config.indicator_groups[1].id;
      wrapper.vm.config.indicator_groups[1].indicators.push(
        { id: 'ind_2', type: 'rsi', enabled: true, operator: 'gt', threshold: 30 }
      );

      api.previewAutomationIndicators.mockResolvedValueOnce({
        data: {
          group_results: [
            { group_id: grpId0, pass: true, indicator_results: [] },
            { group_id: grpId1, pass: false, indicator_results: [] },
          ],
          all_pass: true
        }
      });

      await wrapper.vm.testAllIndicators();
      await nextTick();

      const allResult = wrapper.find('.all-result');
      expect(allResult.exists()).toBe(true);
      expect(allResult.text()).toContain('1 of 2 groups passing');
    });

    it('overall result shows "No groups passing" when all fail', async () => {
      wrapper.vm.config.indicator_groups[0].indicators.push(
        { id: 'ind_1', type: 'vix', enabled: true, operator: 'lt', threshold: 20 }
      );
      wrapper.vm.addGroup();
      const grpId0 = wrapper.vm.config.indicator_groups[0].id;
      const grpId1 = wrapper.vm.config.indicator_groups[1].id;
      wrapper.vm.config.indicator_groups[1].indicators.push(
        { id: 'ind_2', type: 'rsi', enabled: true, operator: 'gt', threshold: 30 }
      );

      api.previewAutomationIndicators.mockResolvedValueOnce({
        data: {
          group_results: [
            { group_id: grpId0, pass: false, indicator_results: [] },
            { group_id: grpId1, pass: false, indicator_results: [] },
          ],
          all_pass: false
        }
      });

      await wrapper.vm.testAllIndicators();
      await nextTick();

      const allResult = wrapper.find('.all-result');
      expect(allResult.exists()).toBe(true);
      expect(allResult.text()).toContain('No groups passing');
    });

    it('overall result shows simple text for single group', async () => {
      // 1 group only
      wrapper.vm.config.indicator_groups[0].indicators.push(
        { id: 'ind_1', type: 'vix', enabled: true, operator: 'lt', threshold: 20 }
      );
      const grpId0 = wrapper.vm.config.indicator_groups[0].id;

      api.previewAutomationIndicators.mockResolvedValueOnce({
        data: {
          group_results: [
            { group_id: grpId0, pass: true, indicator_results: [{ type: 'vix', value: 18, pass: true }] },
          ],
          all_pass: true
        }
      });

      await wrapper.vm.testAllIndicators();
      await nextTick();

      const allResult = wrapper.find('.all-result');
      expect(allResult.exists()).toBe(true);
      // Single group uses "All Passed" or "Some Failed", not group-counting language
      expect(allResult.text()).toContain('All Passed');
      expect(allResult.text()).not.toContain('groups passing');
    });
  });

  // =========================================================================
  // 5.4 Single-Group Lightweight UX (AC-9)
  // =========================================================================

  describe('5.4 Single-Group Lightweight UX', () => {
    it('single group has "lightweight" CSS class on container', async () => {
      await nextTick();
      const group = wrapper.find('.indicator-group');
      expect(group.exists()).toBe(true);
      expect(group.classes()).toContain('lightweight');
    });

    it('single group name input has "lightweight" CSS class', async () => {
      await nextTick();
      const nameInput = wrapper.find('.group-name-input');
      expect(nameInput.exists()).toBe(true);
      expect(nameInput.classes()).toContain('lightweight');
    });

    it('"Add Group" button visible with single group', async () => {
      await nextTick();
      // When there's 1 group, the "Add Group" button is rendered via v-if="config.indicator_groups.length <= 1"
      // in section-actions-left. PrimeVue Button is unresolved, so check the wrapper's HTML.
      const sectionLeft = wrapper.find('.section-actions-left');
      expect(sectionLeft.exists()).toBe(true);
      // The unresolved Button component renders with its attributes in the HTML
      expect(sectionLeft.html()).toContain('Add Group');
    });

    it('multi-group removes "lightweight" class', async () => {
      wrapper.vm.addGroup();
      await nextTick();

      const groups = wrapper.findAll('.indicator-group');
      expect(groups.length).toBe(2);
      // Neither group should have lightweight class
      groups.forEach(g => {
        expect(g.classes()).not.toContain('lightweight');
      });
    });
  });

  // =========================================================================
  // 5.5 Save Payload Structure (AC-1)
  // =========================================================================

  describe('5.5 Save Payload Structure', () => {
    it('save sends indicator_groups with correct nested structure', async () => {
      // Set required fields for validation
      wrapper.vm.config.name = 'Test Config';
      wrapper.vm.config.symbol = 'NDX';

      // Set up 2 groups with 1 indicator each
      wrapper.vm.config.indicator_groups[0].indicators.push({
        id: 'ind_1', type: 'vix', enabled: true, operator: 'lt', threshold: 20
      });
      wrapper.vm.addGroup();
      wrapper.vm.config.indicator_groups[1].indicators.push({
        id: 'ind_2', type: 'gap', enabled: true, operator: 'lt', threshold: 1.5
      });

      await wrapper.vm.saveConfig();

      expect(api.createAutomationConfig).toHaveBeenCalledTimes(1);
      const payload = api.createAutomationConfig.mock.calls[0][0];

      // (a) indicators is []
      expect(payload.indicators).toEqual([]);

      // (b) indicator_groups has 2 entries
      expect(payload.indicator_groups).toBeDefined();
      expect(payload.indicator_groups.length).toBe(2);

      // (c) each entry has id, name, indicators array with full indicator objects
      const g1 = payload.indicator_groups[0];
      expect(g1.id).toBeTruthy();
      expect(g1.name).toBeTruthy();
      expect(g1.indicators.length).toBe(1);
      expect(g1.indicators[0].id).toBe('ind_1');
      expect(g1.indicators[0].type).toBe('vix');
      expect(g1.indicators[0].enabled).toBe(true);
      expect(g1.indicators[0].operator).toBe('lt');
      expect(g1.indicators[0].threshold).toBe(20);

      const g2 = payload.indicator_groups[1];
      expect(g2.indicators[0].type).toBe('gap');
    });

    it('save preserves indicator params in groups', async () => {
      wrapper.vm.config.name = 'Params Test';
      wrapper.vm.config.symbol = 'NDX';

      wrapper.vm.config.indicator_groups[0].indicators.push({
        id: 'ind_rsi', type: 'rsi', enabled: true, operator: 'gt', threshold: 30,
        params: { period: 21 }
      });

      await wrapper.vm.saveConfig();

      const payload = api.createAutomationConfig.mock.calls[0][0];
      const ind = payload.indicator_groups[0].indicators[0];
      expect(ind.params).toEqual({ period: 21 });
    });

    it('save preserves indicator symbol in groups', async () => {
      wrapper.vm.config.name = 'Symbol Test';
      wrapper.vm.config.symbol = 'NDX';

      wrapper.vm.config.indicator_groups[0].indicators.push({
        id: 'ind_vix', type: 'vix', enabled: true, operator: 'lt', threshold: 25,
        symbol: 'UVXY'
      });

      await wrapper.vm.saveConfig();

      const payload = api.createAutomationConfig.mock.calls[0][0];
      const ind = payload.indicator_groups[0].indicators[0];
      expect(ind.symbol).toBe('UVXY');
    });
  });
});
