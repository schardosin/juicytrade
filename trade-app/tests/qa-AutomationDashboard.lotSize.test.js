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

// ADVERSARIAL: hostile / boundary inputs for the multi-lot dashboard guard.
// The developer covered the happy guard cases; these probe empty plans,
// corrupted indices, and malformed field types that a restart or a partial
// WebSocket payload could realistically deliver.
describe('QA — AutomationDashboard lot-size guard (adversarial)', () => {
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

  it('empty order_plan ([]) is NOT multi-lot and renders no progress row', async () => {
    wrapper.vm.configs = [{ id: 'cfg-1', name: 'Empty', enabled: true, recurrence: 'once' }];
    wrapper.vm.statuses = { 'cfg-1': { state: 'monitoring', status: 'monitoring', order_plan: [], current_lot_index: 0 } };
    await nextTick();
    expect(wrapper.vm.isMultiLot('cfg-1')).toBe(false);
    expect(wrapper.vm.totalLots('cfg-1')).toBe(0);
    expect(wrapper.find('.lot-progress-row').exists()).toBe(false);
  });

  it('order_plan present but not an array (null) does not throw and is not multi-lot', async () => {
    wrapper.vm.statuses = { 'cfg-1': { state: 'monitoring', order_plan: null, current_lot_index: 0 } };
    await nextTick();
    expect(wrapper.vm.isMultiLot('cfg-1')).toBe(false);
    expect(wrapper.vm.totalLots('cfg-1')).toBe(0);
    expect(wrapper.vm.lotDots('cfg-1')).toEqual([]);
  });

  it('current_lot_index beyond plan length is clamped to the last lot (no crash)', async () => {
    // Corrupted / stale index after a restart: index 9 on a 3-lot plan.
    wrapper.vm.statuses = { 'cfg-1': { state: 'monitoring', order_plan: [2, 2, 2], current_lot_index: 9 } };
    await nextTick();
    // QA-2 fix: the index is clamped to [0, len-1], so the last lot is 'active'
    // and earlier lots are 'filled'. Must not throw.
    expect(wrapper.vm.lotDots('cfg-1')).toEqual(['filled', 'filled', 'active']);
    // currentLotNumber is clamped so the UI shows "Lot 3 of 3" instead of the
    // pre-fix nonsense "Lot 10 of 3".
    expect(wrapper.vm.currentLotNumber('cfg-1')).toBe(3);
  });

  it('negative current_lot_index is clamped to the first lot (QA-2 fix)', async () => {
    // NOTE: pre-fix, `status.current_lot_index || 0` did NOT sanitize a negative
    // index (-1 is truthy). The QA-2 clamp now maps negatives to lot 0.
    wrapper.vm.statuses = { 'cfg-1': { state: 'monitoring', order_plan: [2, 2], current_lot_index: -1 } };
    await nextTick();
    // Clamped idx = 0: first lot 'active', rest 'pending'.
    expect(wrapper.vm.lotDots('cfg-1')).toEqual(['active', 'pending']);
    // currentLotNumber = 0 + 1 = 1 -> UI shows "Lot 1 of 2" (never "Lot 0 of 2").
    expect(wrapper.vm.currentLotNumber('cfg-1')).toBe(1);
  });

  it('single-lot plan with a valid index stays hidden (legacy parity)', async () => {
    wrapper.vm.configs = [{ id: 'cfg-1', name: 'Single', enabled: true, recurrence: 'daily' }];
    wrapper.vm.statuses = { 'cfg-1': { state: 'monitoring', status: 'monitoring', order_plan: [4], current_lot_index: 0 } };
    await nextTick();
    expect(wrapper.vm.isMultiLot('cfg-1')).toBe(false);
    expect(wrapper.find('.lot-progress-row').exists()).toBe(false);
  });
});
