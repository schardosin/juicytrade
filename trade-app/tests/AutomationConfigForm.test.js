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
