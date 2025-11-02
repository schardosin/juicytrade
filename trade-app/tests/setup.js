// This file is used to set up the test environment for Vitest.
import { vi } from 'vitest';

// Mock the authentication service
vi.mock('../src/services/authService.js', () => ({
  default: {
    init: vi.fn().mockResolvedValue(true),
    getAuthConfig: vi.fn().mockResolvedValue({
      method: 'disabled',
      enabled: false,
      supports_methods: ['simple', 'oauth', 'token', 'header', 'disabled'],
      session_cookie_name: 'juicytrade_session'
    }),
    getAuthStatus: vi.fn().mockResolvedValue({
      authenticated: false,
      method: 'disabled',
      user: null,
      expires_at: null
    }),
    login: vi.fn().mockResolvedValue({ success: true, user: { username: 'test' } }),
    logout: vi.fn().mockResolvedValue({ success: true }),
    getCurrentUser: vi.fn().mockResolvedValue(null),
    isAuthenticated: vi.fn().mockReturnValue(false),
    getUser: vi.fn().mockReturnValue(null),
    getAuthMethod: vi.fn().mockReturnValue('disabled'),
    isAuthEnabled: vi.fn().mockReturnValue(false),
    getOAuthLoginUrl: vi.fn().mockReturnValue('/auth/oauth/authorize'),
    redirectToLogin: vi.fn(),
    addListener: vi.fn().mockReturnValue(() => {}),
    notifyListeners: vi.fn(),
    handleApiResponse: vi.fn().mockImplementation(response => response),
    createAuthenticatedFetch: vi.fn().mockReturnValue(fetch),
    baseURL: '/api',
    user: null,
    authenticated: false,
    authMethod: 'disabled',
    listeners: []
  }
}));

// Mock fetch for any remaining HTTP requests
global.fetch = vi.fn().mockImplementation((url) => {
  // Mock auth endpoints
  if (url.includes('/api/auth/config')) {
    return Promise.resolve({
      ok: true,
      json: () => Promise.resolve({
        success: true,
        data: {
          method: 'disabled',
          enabled: false,
          supports_methods: ['simple', 'oauth', 'token', 'header', 'disabled'],
          session_cookie_name: 'juicytrade_session'
        }
      })
    });
  }
  
  if (url.includes('/api/auth/status')) {
    return Promise.resolve({
      ok: true,
      json: () => Promise.resolve({
        success: true,
        data: {
          authenticated: false,
          method: 'disabled',
          user: null,
          expires_at: null
        }
      })
    });
  }
  
  // Default mock response for other endpoints
  return Promise.resolve({
    ok: true,
    json: () => Promise.resolve({ success: true, data: {} })
  });
});
