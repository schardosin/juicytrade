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

  it('current_lot_index beyond plan length marks all real lots filled (no crash)', async () => {
    // Corrupted / stale index after a restart: index 9 on a 3-lot plan.
    wrapper.vm.statuses = { 'cfg-1': { state: 'monitoring', order_plan: [2, 2, 2], current_lot_index: 9 } };
    await nextTick();
    // Every lot index < 9 => all 'filled', none 'active'/'pending'. Must not throw.
    expect(wrapper.vm.lotDots('cfg-1')).toEqual(['filled', 'filled', 'filled']);
    // currentLotNumber is still computed (1-based) — documents that the UI would
    // display "Lot 10 of 3" for a corrupted index. This is a display oddity, not
    // a crash, worth noting to @dev.
    expect(wrapper.vm.currentLotNumber('cfg-1')).toBe(10);
  });

  it('negative current_lot_index falls back to lot 0 (|| 0 short-circuit only guards falsy)', async () => {
    // NOTE: `status.current_lot_index || 0` does NOT sanitize a negative index
    // (-1 is truthy). This test documents the actual behavior of the real code.
    wrapper.vm.statuses = { 'cfg-1': { state: 'monitoring', order_plan: [2, 2], current_lot_index: -1 } };
    await nextTick();
    // With idx = -1: no lot index < -1, none === -1 => every lot is 'pending'.
    expect(wrapper.vm.lotDots('cfg-1')).toEqual(['pending', 'pending']);
    // currentLotNumber = -1 + 1 = 0 -> UI would show "Lot 0 of 2".
    expect(wrapper.vm.currentLotNumber('cfg-1')).toBe(0);
  });

  it('single-lot plan with a valid index stays hidden (legacy parity)', async () => {
    wrapper.vm.configs = [{ id: 'cfg-1', name: 'Single', enabled: true, recurrence: 'daily' }];
    wrapper.vm.statuses = { 'cfg-1': { state: 'monitoring', status: 'monitoring', order_plan: [4], current_lot_index: 0 } };
    await nextTick();
    expect(wrapper.vm.isMultiLot('cfg-1')).toBe(false);
    expect(wrapper.find('.lot-progress-row').exists()).toBe(false);
  });
});
