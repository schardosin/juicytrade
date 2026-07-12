import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { mount } from '@vue/test-utils';
import { ref, nextTick } from 'vue';
import AutomationConfigForm from '../src/components/automation/AutomationConfigForm.vue';
import AutomationDashboard from '../src/components/automation/AutomationDashboard.vue';

// Adversarial tests for the percentage-based Max Capital feature (issue #75).
// These target edge cases the happy-path suite (AutomationCapitalMode.test.js)
// does not exercise: empty/null/NaN percent input, missing trade_config on the
// dashboard, and non-finite resolved dollar values.

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
    getAutomationConfigs: vi.fn().mockResolvedValue({ data: { configs: [] } }),
    getAllAutomationStatuses: vi.fn().mockResolvedValue({ data: { automations: [] } }),
    startAutomation: vi.fn().mockResolvedValue({}),
    stopAutomation: vi.fn().mockResolvedValue({}),
    evaluateAutomationConfig: vi.fn().mockResolvedValue({}),
    toggleAutomationConfig: vi.fn().mockResolvedValue({}),
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

describe('QA — AutomationConfigForm percent validation edge cases', () => {
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

  const seedValid = (tc) => {
    wrapper.vm.config.name = 'X';
    wrapper.vm.config.symbol = 'NDX';
    wrapper.vm.config.entry_time = '12:25';
    tc.capital_mode = 'percent';
  };

  it('rejects a null percent (empty input) in percent mode', () => {
    const tc = wrapper.vm.config.trade_config;
    seedValid(tc);
    tc.max_capital_percent = null;
    expect(wrapper.vm.validateConfig()).toBe(false);
    expect(wrapper.vm.errors.max_capital_percent).toBe('Enter a value between 1 and 100');
  });

  it('rejects an undefined percent in percent mode', () => {
    const tc = wrapper.vm.config.trade_config;
    seedValid(tc);
    tc.max_capital_percent = undefined;
    expect(wrapper.vm.validateConfig()).toBe(false);
    expect(wrapper.vm.errors.max_capital_percent).toBe('Enter a value between 1 and 100');
  });

  it('rejects a NaN percent in percent mode', () => {
    const tc = wrapper.vm.config.trade_config;
    seedValid(tc);
    tc.max_capital_percent = NaN;
    expect(wrapper.vm.validateConfig()).toBe(false);
    expect(wrapper.vm.errors.max_capital_percent).toBe('Enter a value between 1 and 100');
  });

  it('rejects a string percent that is not a number', () => {
    const tc = wrapper.vm.config.trade_config;
    seedValid(tc);
    tc.max_capital_percent = '60'; // typeof !== 'number' => rejected
    expect(wrapper.vm.validateConfig()).toBe(false);
    expect(wrapper.vm.errors.max_capital_percent).toBe('Enter a value between 1 and 100');
  });

  it('accepts the exact inclusive boundaries 1 and 100', () => {
    const tc = wrapper.vm.config.trade_config;
    seedValid(tc);
    tc.max_capital_percent = 1;
    expect(wrapper.vm.validateConfig()).toBe(true);
    tc.max_capital_percent = 100;
    expect(wrapper.vm.validateConfig()).toBe(true);
    expect(wrapper.vm.errors.max_capital_percent).toBeUndefined();
  });
});

describe('QA — AutomationDashboard formatMaxCapital robustness', () => {
  let wrapper;

  beforeEach(async () => {
    vi.clearAllMocks();
    vi.useFakeTimers();
    wrapper = mount(AutomationDashboard);
    await nextTick();
    await nextTick();
    vi.useRealTimers();
  });

  afterEach(() => {
    if (wrapper) wrapper.unmount();
  });

  it('does not throw when trade_config is entirely missing (legacy/corrupt row)', () => {
    const config = { id: 'q1' }; // no trade_config
    expect(() => wrapper.vm.formatMaxCapital(config)).not.toThrow();
  });

  it('does not throw when config itself is null', () => {
    expect(() => wrapper.vm.formatMaxCapital(null)).not.toThrow();
  });

  it('shows plain percent (no dollar suffix) when resolved value is NaN', () => {
    const config = { id: 'q2', trade_config: { capital_mode: 'percent', max_capital_percent: 60 } };
    wrapper.vm.statuses = { q2: { resolved_max_capital: NaN } };
    // NaN > 0 is false, so the resolved suffix must be omitted.
    expect(wrapper.vm.formatMaxCapital(config)).toBe('60%');
  });

  it('shows plain percent when resolved value is negative', () => {
    const config = { id: 'q3', trade_config: { capital_mode: 'percent', max_capital_percent: 60 } };
    wrapper.vm.statuses = { q3: { resolved_max_capital: -6000 } };
    expect(wrapper.vm.formatMaxCapital(config)).toBe('60%');
  });
});
