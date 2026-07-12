import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { mount } from '@vue/test-utils';
import { ref, nextTick } from 'vue';
import AutomationConfigForm from '../src/components/automation/AutomationConfigForm.vue';
import AutomationDashboard from '../src/components/automation/AutomationDashboard.vue';

// ── Shared mocks ──────────────────────────────────────────────────────

vi.mock('vue-router', () => ({
  useRouter: () => ({ push: vi.fn() }),
  useRoute: () => ({ params: {} }),
}));

vi.mock('../src/services/api.js', () => ({
  api: {
    // ConfigForm
    getAutomationConfig: vi.fn().mockResolvedValue({ data: { config: null } }),
    createAutomationConfig: vi.fn().mockResolvedValue({ data: { id: 'new-id' } }),
    updateAutomationConfig: vi.fn().mockResolvedValue({}),
    previewAutomationIndicators: vi.fn().mockResolvedValue({ data: { indicators: [], group_results: [], all_pass: false } }),
    getIndicatorMetadata: vi.fn().mockResolvedValue({ data: { indicators: [] } }),
    previewStrikes: vi.fn().mockResolvedValue({ success: true, data: {} }),
    // Dashboard
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

// ── Config form: capital mode ─────────────────────────────────────────

describe('AutomationConfigForm — capital mode', () => {
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

  it('defaults to fixed mode with sensible defaults', () => {
    const tc = wrapper.vm.config.trade_config;
    expect(tc.capital_mode).toBe('fixed');
    expect(tc.max_capital).toBe(5000);
    expect(tc.max_capital_percent).toBe(60);
  });

  it('exposes both capital mode options', () => {
    expect(wrapper.vm.capitalModeOptions).toEqual([
      { label: 'Fixed Amount ($)', value: 'fixed' },
      { label: 'Percentage (%)', value: 'percent' },
    ]);
  });

  it('renders the currency input in fixed mode and no percent input', async () => {
    wrapper.vm.config.trade_config.capital_mode = 'fixed';
    await nextTick();
    expect(wrapper.find('#maxCapital').exists()).toBe(true);
    expect(wrapper.find('#maxCapitalPercent').exists()).toBe(false);
  });

  it('renders the percent input in percent mode and hides currency input', async () => {
    wrapper.vm.config.trade_config.capital_mode = 'percent';
    await nextTick();
    expect(wrapper.find('#maxCapitalPercent').exists()).toBe(true);
    expect(wrapper.find('#maxCapital').exists()).toBe(false);
  });

  it('preserves each mode value when toggling back and forth', async () => {
    const tc = wrapper.vm.config.trade_config;
    tc.max_capital = 8000;
    tc.max_capital_percent = 25;
    tc.capital_mode = 'percent';
    await nextTick();
    tc.capital_mode = 'fixed';
    await nextTick();
    // Both values retained independently (D3 / FR-2)
    expect(tc.max_capital).toBe(8000);
    expect(tc.max_capital_percent).toBe(25);
  });

  it('validates percent within 1..100 (valid passes)', () => {
    const tc = wrapper.vm.config.trade_config;
    wrapper.vm.config.name = 'X';
    wrapper.vm.config.symbol = 'NDX';
    wrapper.vm.config.entry_time = '12:25';
    tc.capital_mode = 'percent';
    tc.max_capital_percent = 60;
    expect(wrapper.vm.validateConfig()).toBe(true);
    expect(wrapper.vm.errors.max_capital_percent).toBeUndefined();
  });

  it('rejects percent below 1 with a message', () => {
    const tc = wrapper.vm.config.trade_config;
    wrapper.vm.config.name = 'X';
    wrapper.vm.config.symbol = 'NDX';
    wrapper.vm.config.entry_time = '12:25';
    tc.capital_mode = 'percent';
    tc.max_capital_percent = 0;
    expect(wrapper.vm.validateConfig()).toBe(false);
    expect(wrapper.vm.errors.max_capital_percent).toBe('Enter a value between 1 and 100');
  });

  it('rejects percent above 100 with a message', () => {
    const tc = wrapper.vm.config.trade_config;
    wrapper.vm.config.name = 'X';
    wrapper.vm.config.symbol = 'NDX';
    wrapper.vm.config.entry_time = '12:25';
    tc.capital_mode = 'percent';
    tc.max_capital_percent = 150;
    expect(wrapper.vm.validateConfig()).toBe(false);
    expect(wrapper.vm.errors.max_capital_percent).toBe('Enter a value between 1 and 100');
  });

  it('does not validate percent when mode is fixed', () => {
    const tc = wrapper.vm.config.trade_config;
    wrapper.vm.config.name = 'X';
    wrapper.vm.config.symbol = 'NDX';
    wrapper.vm.config.entry_time = '12:25';
    tc.capital_mode = 'fixed';
    tc.max_capital_percent = 9999; // stale, should be ignored
    expect(wrapper.vm.validateConfig()).toBe(true);
    expect(wrapper.vm.errors.max_capital_percent).toBeUndefined();
  });
});

// ── Dashboard: formatMaxCapital ───────────────────────────────────────

describe('AutomationDashboard — formatMaxCapital', () => {
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

  it('formats fixed mode as a dollar amount', () => {
    const config = { id: 'c1', trade_config: { capital_mode: 'fixed', max_capital: 5000 } };
    expect(wrapper.vm.formatMaxCapital(config)).toBe('$5,000');
  });

  it('treats legacy (missing mode) config as fixed', () => {
    const config = { id: 'c1', trade_config: { max_capital: 1234 } };
    expect(wrapper.vm.formatMaxCapital(config)).toBe('$1,234');
  });

  it('formats percent mode as a percentage before resolution', () => {
    const config = { id: 'c2', trade_config: { capital_mode: 'percent', max_capital_percent: 60 } };
    expect(wrapper.vm.formatMaxCapital(config)).toBe('60%');
  });

  it('appends the resolved dollar amount once status carries it', () => {
    const config = { id: 'c3', trade_config: { capital_mode: 'percent', max_capital_percent: 60 } };
    wrapper.vm.statuses = { c3: { resolved_max_capital: 6000 } };
    expect(wrapper.vm.formatMaxCapital(config)).toBe('60% (≈ $6,000)');
  });

  it('rounds the resolved dollar amount to the nearest dollar', () => {
    const config = { id: 'c4', trade_config: { capital_mode: 'percent', max_capital_percent: 33 } };
    wrapper.vm.statuses = { c4: { resolved_max_capital: 6666.66 } };
    expect(wrapper.vm.formatMaxCapital(config)).toBe('33% (≈ $6,667)');
  });

  it('does not append resolved dollars when value is zero/absent', () => {
    const config = { id: 'c5', trade_config: { capital_mode: 'percent', max_capital_percent: 10 } };
    wrapper.vm.statuses = { c5: { resolved_max_capital: 0 } };
    expect(wrapper.vm.formatMaxCapital(config)).toBe('10%');
  });
});
