import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { mount } from '@vue/test-utils';
import { ref, nextTick } from 'vue';
import AutomationConfigForm from '../src/components/automation/AutomationConfigForm.vue';
import { api } from '../src/services/api.js';

vi.mock('vue-router', () => ({
  useRouter: () => ({ push: vi.fn() }),
  useRoute: () => ({ params: {} }),
}));

vi.mock('../src/services/api.js', () => ({
  api: {
    getAutomationConfig: vi.fn().mockResolvedValue({ data: { config: null } }),
    createAutomationConfig: vi.fn().mockResolvedValue({ data: { id: 'new-id' } }),
    updateAutomationConfig: vi.fn().mockResolvedValue({}),
    previewAutomationIndicators: vi.fn().mockResolvedValue({ data: { indicators: [], group_results: [], all_pass: false } }),
    getIndicatorMetadata: vi.fn().mockResolvedValue({ data: { indicators: [] } }),
    previewStrikes: vi.fn().mockResolvedValue({ success: true, data: {} }),
  },
}));

vi.mock('../src/composables/useMobileDetection.js', () => ({
  useMobileDetection: () => ({ isMobile: ref(false) }),
}));

describe('AutomationConfigForm — Lot Size (multi-order)', () => {
  let wrapper;

  beforeEach(async () => {
    vi.clearAllMocks();
    wrapper = mount(AutomationConfigForm);
    await nextTick();
    await nextTick();
  });

  afterEach(() => {
    if (wrapper) wrapper.unmount();
  });

  it('defaults lot_size to 1 (single order / legacy behavior)', () => {
    expect(wrapper.vm.config.trade_config.lot_size).toBe(1);
  });

  it('defaults legs_drift to false', () => {
    expect(wrapper.vm.config.trade_config.legs_drift).toBe(false);
  });

  it('renders a Lot Size input and a Legs Drift toggle', () => {
    expect(wrapper.find('#lotSize').exists()).toBe(true);
    expect(wrapper.find('#legsDrift').exists()).toBe(true);
  });

  it('does not move existing trade-parameter fields (delta drift still present)', () => {
    expect(wrapper.find('#deltaDriftLimit').exists()).toBe(true);
    expect(wrapper.find('#maxAttempts').exists() || wrapper.find('#attemptInterval').exists()).toBe(true);
  });

  it('includes lot_size and legs_drift in the saved payload', async () => {
    wrapper.vm.config.name = 'Lot Config';
    wrapper.vm.config.symbol = 'NDX';
    wrapper.vm.config.trade_config.lot_size = 2;
    wrapper.vm.config.trade_config.legs_drift = true;

    await wrapper.vm.saveConfig();

    expect(api.createAutomationConfig).toHaveBeenCalledTimes(1);
    const payload = api.createAutomationConfig.mock.calls[0][0];
    expect(payload.trade_config.lot_size).toBe(2);
    expect(payload.trade_config.legs_drift).toBe(true);
  });

  it('preserves single-order defaults in the payload when untouched (backward compatible)', async () => {
    wrapper.vm.config.name = 'Default Config';
    wrapper.vm.config.symbol = 'NDX';

    await wrapper.vm.saveConfig();

    const payload = api.createAutomationConfig.mock.calls[0][0];
    expect(payload.trade_config.lot_size).toBe(1);
    expect(payload.trade_config.legs_drift).toBe(false);
  });
});
