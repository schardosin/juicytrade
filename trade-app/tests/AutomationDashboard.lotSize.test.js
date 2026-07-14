import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { mount } from '@vue/test-utils';
import { ref, nextTick } from 'vue';
import AutomationDashboard from '../src/components/automation/AutomationDashboard.vue';

vi.mock('vue-router', () => ({
  useRouter: () => ({ push: vi.fn() }),
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
  },
}));

vi.mock('../src/services/webSocketClient.js', () => ({
  default: { addCallback: vi.fn(), removeCallback: vi.fn() },
}));

vi.mock('../src/composables/useMobileDetection.js', () => ({
  useMobileDetection: () => ({ isMobile: ref(false) }),
}));

describe('AutomationDashboard — Lot Size progress', () => {
  let wrapper;

  beforeEach(async () => {
    vi.useFakeTimers();
    wrapper = mount(AutomationDashboard);
    await nextTick();
    await nextTick();
    vi.useRealTimers();
  });

  afterEach(() => {
    if (wrapper) wrapper.unmount();
  });

  describe('isMultiLot (legacy guard)', () => {
    it('returns false when no status is present', () => {
      expect(wrapper.vm.isMultiLot('cfg-x')).toBe(false);
    });

    it('returns false for a single-order run (order_plan length 1)', async () => {
      wrapper.vm.statuses = { 'cfg-1': { state: 'monitoring', order_plan: [6], current_lot_index: 0 } };
      await nextTick();
      expect(wrapper.vm.isMultiLot('cfg-1')).toBe(false);
    });

    it('returns false for a legacy run with no order_plan', async () => {
      wrapper.vm.statuses = { 'cfg-1': { state: 'monitoring' } };
      await nextTick();
      expect(wrapper.vm.isMultiLot('cfg-1')).toBe(false);
    });

    it('returns true for a multi-lot run (order_plan length > 1)', async () => {
      wrapper.vm.statuses = { 'cfg-1': { state: 'monitoring', order_plan: [2, 2, 2], current_lot_index: 1 } };
      await nextTick();
      expect(wrapper.vm.isMultiLot('cfg-1')).toBe(true);
    });
  });

  describe('lot counters', () => {
    beforeEach(async () => {
      wrapper.vm.statuses = { 'cfg-1': { state: 'monitoring', order_plan: [2, 2, 2, 1], current_lot_index: 1 } };
      await nextTick();
    });

    it('totalLots reflects the plan length', () => {
      expect(wrapper.vm.totalLots('cfg-1')).toBe(4);
    });

    it('currentLotNumber is 1-based', () => {
      expect(wrapper.vm.currentLotNumber('cfg-1')).toBe(2);
    });

    it('lotDots marks filled / active / pending correctly', () => {
      expect(wrapper.vm.lotDots('cfg-1')).toEqual(['filled', 'active', 'pending', 'pending']);
    });
  });

  describe('lot counters defaults', () => {
    it('currentLotNumber defaults to 1 when current_lot_index is absent', async () => {
      wrapper.vm.statuses = { 'cfg-1': { state: 'trading', order_plan: [2, 2] } };
      await nextTick();
      expect(wrapper.vm.currentLotNumber('cfg-1')).toBe(1);
      expect(wrapper.vm.lotDots('cfg-1')).toEqual(['active', 'pending']);
    });

    it('totalLots returns 0 for legacy runs (no plan)', () => {
      expect(wrapper.vm.totalLots('cfg-missing')).toBe(0);
      expect(wrapper.vm.lotDots('cfg-missing')).toEqual([]);
    });

    it('all dots are filled once the last lot fills', async () => {
      wrapper.vm.statuses = { 'cfg-1': { state: 'monitoring', order_plan: [2, 2, 2], current_lot_index: 2 } };
      await nextTick();
      expect(wrapper.vm.lotDots('cfg-1')).toEqual(['filled', 'filled', 'active']);
    });
  });

  describe('rendered progress row', () => {
    it('renders "Lot X of N" only for multi-lot runs', async () => {
      wrapper.vm.configs = [{ id: 'cfg-1', name: 'Multi', enabled: true, recurrence: 'once' }];
      wrapper.vm.statuses = { 'cfg-1': { state: 'monitoring', status: 'monitoring', order_plan: [2, 2, 2], current_lot_index: 1 } };
      await nextTick();
      expect(wrapper.find('.lot-progress-row').exists()).toBe(true);
      expect(wrapper.find('.lot-progress-text').text()).toContain('Lot 2 of 3');
    });

    it('hides the progress row for single-order runs', async () => {
      wrapper.vm.configs = [{ id: 'cfg-1', name: 'Single', enabled: true, recurrence: 'once' }];
      wrapper.vm.statuses = { 'cfg-1': { state: 'monitoring', status: 'monitoring', order_plan: [6], current_lot_index: 0 } };
      await nextTick();
      expect(wrapper.find('.lot-progress-row').exists()).toBe(false);
    });
  });
});
