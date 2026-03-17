import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { mount } from '@vue/test-utils';
import { ref, nextTick } from 'vue';
import AutomationDashboard from '../src/components/automation/AutomationDashboard.vue';

// Mock dependencies
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

describe('AutomationDashboard', () => {
  let wrapper;

  beforeEach(async () => {
    vi.useFakeTimers();
    wrapper = mount(AutomationDashboard);
    // Allow onMounted async work (loadConfigs + loadStatuses) to complete
    await nextTick();
    await nextTick();
    vi.useRealTimers();
  });

  afterEach(() => {
    if (wrapper) {
      wrapper.unmount();
    }
  });

  // ─── getStatusClass ─────────────────────────────────────────────────

  describe('getStatusClass', () => {
    it('returns "disabled" when config.enabled is false', () => {
      const config = { id: 'cfg-1', enabled: false };
      expect(wrapper.vm.getStatusClass(config)).toBe('disabled');
    });

    it('returns "idle" when config is enabled but not running', () => {
      const config = { id: 'cfg-1', enabled: true };
      // No status entry → isConfigRunning returns false
      expect(wrapper.vm.getStatusClass(config)).toBe('idle');
    });

    it('returns "running-ready" when running and all_indicators_pass is true', async () => {
      const config = { id: 'cfg-1', enabled: true };
      wrapper.vm.statuses = {
        'cfg-1': { is_running: true, state: 'waiting', all_indicators_pass: true }
      };
      await nextTick();
      expect(wrapper.vm.getStatusClass(config)).toBe('running-ready');
    });

    it('returns "running-not-ready" when running and all_indicators_pass is false', async () => {
      const config = { id: 'cfg-1', enabled: true };
      wrapper.vm.statuses = {
        'cfg-1': { is_running: true, state: 'waiting', all_indicators_pass: false }
      };
      await nextTick();
      expect(wrapper.vm.getStatusClass(config)).toBe('running-not-ready');
    });

    it('returns "running-not-ready" when running and all_indicators_pass is undefined', async () => {
      const config = { id: 'cfg-1', enabled: true };
      wrapper.vm.statuses = {
        'cfg-1': { is_running: true, state: 'evaluating' }
      };
      await nextTick();
      expect(wrapper.vm.getStatusClass(config)).toBe('running-not-ready');
    });

    it('returns "running-not-ready" when running and all_indicators_pass is null', async () => {
      const config = { id: 'cfg-1', enabled: true };
      wrapper.vm.statuses = {
        'cfg-1': { is_running: true, state: 'trading', all_indicators_pass: null }
      };
      await nextTick();
      expect(wrapper.vm.getStatusClass(config)).toBe('running-not-ready');
    });

    it('returns "running-not-ready" when running via active state and no all_indicators_pass', async () => {
      const config = { id: 'cfg-1', enabled: true };
      // is_running is false but state is an active state → isConfigRunning returns true
      wrapper.vm.statuses = {
        'cfg-1': { is_running: false, state: 'monitoring' }
      };
      await nextTick();
      expect(wrapper.vm.getStatusClass(config)).toBe('running-not-ready');
    });
  });

  // ─── getRunningStatusClass ──────────────────────────────────────────

  describe('getRunningStatusClass', () => {
    it('returns "status-disabled" when config.enabled is false', () => {
      const config = { id: 'cfg-1', enabled: false };
      expect(wrapper.vm.getRunningStatusClass(config)).toBe('status-disabled');
    });

    it('returns "status-idle" when no status exists', () => {
      const config = { id: 'cfg-1', enabled: true };
      // No status entry for cfg-1
      expect(wrapper.vm.getRunningStatusClass(config)).toBe('status-idle');
    });

    it('returns "status-waiting" for state=waiting with all_indicators_pass=false', async () => {
      const config = { id: 'cfg-1', enabled: true };
      wrapper.vm.statuses = {
        'cfg-1': { state: 'waiting', all_indicators_pass: false }
      };
      await nextTick();
      expect(wrapper.vm.getRunningStatusClass(config)).toBe('status-waiting');
    });

    it('returns "status-running" for state=waiting with all_indicators_pass=true', async () => {
      const config = { id: 'cfg-1', enabled: true };
      wrapper.vm.statuses = {
        'cfg-1': { state: 'waiting', all_indicators_pass: true }
      };
      await nextTick();
      expect(wrapper.vm.getRunningStatusClass(config)).toBe('status-running');
    });

    it('returns "status-waiting" for state=evaluating with all_indicators_pass=false', async () => {
      const config = { id: 'cfg-1', enabled: true };
      wrapper.vm.statuses = {
        'cfg-1': { state: 'evaluating', all_indicators_pass: false }
      };
      await nextTick();
      expect(wrapper.vm.getRunningStatusClass(config)).toBe('status-waiting');
    });

    it('returns "status-running" for state=evaluating with all_indicators_pass=true', async () => {
      const config = { id: 'cfg-1', enabled: true };
      wrapper.vm.statuses = {
        'cfg-1': { state: 'evaluating', all_indicators_pass: true }
      };
      await nextTick();
      expect(wrapper.vm.getRunningStatusClass(config)).toBe('status-running');
    });

    it('returns "status-running" for state=trading regardless of all_indicators_pass', async () => {
      const config = { id: 'cfg-1', enabled: true };
      wrapper.vm.statuses = {
        'cfg-1': { state: 'trading', all_indicators_pass: false }
      };
      await nextTick();
      expect(wrapper.vm.getRunningStatusClass(config)).toBe('status-running');

      wrapper.vm.statuses = {
        'cfg-1': { state: 'trading', all_indicators_pass: true }
      };
      await nextTick();
      expect(wrapper.vm.getRunningStatusClass(config)).toBe('status-running');
    });

    it('returns "status-running" for state=monitoring regardless of all_indicators_pass', async () => {
      const config = { id: 'cfg-1', enabled: true };
      wrapper.vm.statuses = {
        'cfg-1': { state: 'monitoring', all_indicators_pass: false }
      };
      await nextTick();
      expect(wrapper.vm.getRunningStatusClass(config)).toBe('status-running');

      wrapper.vm.statuses = {
        'cfg-1': { state: 'monitoring', all_indicators_pass: true }
      };
      await nextTick();
      expect(wrapper.vm.getRunningStatusClass(config)).toBe('status-running');
    });

    it('returns "status-completed" for state=completed', async () => {
      const config = { id: 'cfg-1', enabled: true };
      wrapper.vm.statuses = {
        'cfg-1': { state: 'completed' }
      };
      await nextTick();
      expect(wrapper.vm.getRunningStatusClass(config)).toBe('status-completed');
    });

    it('returns "status-failed" for state=failed', async () => {
      const config = { id: 'cfg-1', enabled: true };
      wrapper.vm.statuses = {
        'cfg-1': { state: 'failed' }
      };
      await nextTick();
      expect(wrapper.vm.getRunningStatusClass(config)).toBe('status-failed');
    });
  });
});
