import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import OAuthCallback from '../src/views/OAuthCallback.vue';

// Use vi.hoisted so variables are available inside vi.mock factories (which are hoisted)
const { mockRoute, mockApi } = vi.hoisted(() => ({
  mockRoute: { query: {} },
  mockApi: {
    relaySchwabOAuthCallback: vi.fn(),
    relaySchwabOAuthCallbackError: vi.fn(),
  },
}));

// Mock vue-router
vi.mock('vue-router', () => ({
  useRoute: () => mockRoute,
}));

// Mock api service
vi.mock('../src/services/api.js', () => ({
  api: mockApi,
}));

describe('OAuthCallback', () => {
  let originalOpener;
  let originalClose;

  beforeEach(() => {
    // Reset mocks
    vi.restoreAllMocks();
    mockApi.relaySchwabOAuthCallback.mockReset();
    mockApi.relaySchwabOAuthCallbackError.mockReset();

    // Save originals
    originalOpener = window.opener;
    originalClose = window.close;

    // Default: no opener, stub close
    window.opener = null;
    window.close = vi.fn();
  });

  afterEach(() => {
    window.opener = originalOpener;
    window.close = originalClose;
  });

  function mountWithQuery(query) {
    mockRoute.query = query;
    return mount(OAuthCallback);
  }

  it('renders loading state and calls API with code and state params', async () => {
    mockApi.relaySchwabOAuthCallback.mockResolvedValue({ status: 'success' });

    const wrapper = mountWithQuery({ code: 'test-code', state: 'test-state' });

    // Initially shows loading
    expect(wrapper.text()).toContain('Completing Authorization');

    await flushPromises();

    // API was called with correct params
    expect(mockApi.relaySchwabOAuthCallback).toHaveBeenCalledWith('test-code', 'test-state');

    // Shows success state
    expect(wrapper.text()).toContain('Authorization Successful');
    expect(wrapper.text()).toContain('close automatically');
  });

  it('shows error when API returns error response', async () => {
    mockApi.relaySchwabOAuthCallback.mockRejectedValue({
      response: { data: { error: 'Token exchange failed' } },
    });

    const wrapper = mountWithQuery({ code: 'test-code', state: 'test-state' });
    await flushPromises();

    expect(mockApi.relaySchwabOAuthCallback).toHaveBeenCalledWith('test-code', 'test-state');
    expect(wrapper.text()).toContain('Authorization Failed');
    expect(wrapper.text()).toContain('Token exchange failed');
  });

  it('handles user cancellation (error param)', async () => {
    mockApi.relaySchwabOAuthCallbackError.mockResolvedValue({ status: 'cancelled' });

    const wrapper = mountWithQuery({ error: 'access_denied', state: 'test-state' });
    await flushPromises();

    expect(mockApi.relaySchwabOAuthCallbackError).toHaveBeenCalledWith('access_denied', 'test-state');
    expect(mockApi.relaySchwabOAuthCallback).not.toHaveBeenCalled();
    expect(wrapper.text()).toContain('Authorization Cancelled');
  });

  it('sends postMessage to opener on success', async () => {
    mockApi.relaySchwabOAuthCallback.mockResolvedValue({ status: 'success' });

    const mockPostMessage = vi.fn();
    window.opener = { postMessage: mockPostMessage };

    mountWithQuery({ code: 'test-code', state: 'test-state' });
    await flushPromises();

    expect(mockPostMessage).toHaveBeenCalledWith(
      { type: 'schwab-oauth-callback', status: 'success', error: undefined },
      window.location.origin
    );
  });

  it('shows error when no code or error params', async () => {
    const wrapper = mountWithQuery({});
    await flushPromises();

    expect(wrapper.text()).toContain('Missing authorization parameters');
    expect(mockApi.relaySchwabOAuthCallback).not.toHaveBeenCalled();
    expect(mockApi.relaySchwabOAuthCallbackError).not.toHaveBeenCalled();
  });

  it('calls window.close after success', async () => {
    vi.useFakeTimers();
    mockApi.relaySchwabOAuthCallback.mockResolvedValue({ status: 'success' });

    mountWithQuery({ code: 'test-code', state: 'test-state' });
    await flushPromises();

    // window.close should not have been called yet
    expect(window.close).not.toHaveBeenCalled();

    // Advance past the 2-second auto-close delay
    vi.advanceTimersByTime(2500);

    expect(window.close).toHaveBeenCalled();

    vi.useRealTimers();
  });
});
