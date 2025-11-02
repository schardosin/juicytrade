/**
 * Authentication service for juicytrade frontend
 */

class AuthService {
  constructor() {
    this.baseURL = '/api';  // Use /api prefix to match Vite proxy configuration
    this.user = null;
    this.authenticated = false;
    this.authMethod = 'disabled';
    this.listeners = [];
  }

  /**
   * Initialize authentication service
   */
  async init() {
    try {
      // Get auth configuration
      const config = await this.getAuthConfig();
      this.authMethod = config.method;
      
      if (config.enabled) {
        // Check current auth status
        const status = await this.getAuthStatus();
        this.authenticated = status.authenticated;
        this.user = status.user;
      }
      
      this.notifyListeners();
      return true;
    } catch (error) {
      console.error('Failed to initialize auth service:', error);
      return false;
    }
  }

  /**
   * Get authentication configuration
   */
  async getAuthConfig() {
    const response = await fetch(`${this.baseURL}/auth/config`, {
      credentials: 'include'  // Include cookies
    });
    const result = await response.json();
    
    if (!result.success) {
      throw new Error(result.message || 'Failed to get auth config');
    }
    
    return result.data;
  }

  /**
   * Get current authentication status
   */
  async getAuthStatus() {
    const response = await fetch(`${this.baseURL}/auth/status`, {
      credentials: 'include'  // Include cookies
    });
    
    const result = await response.json();
    
    if (!result.success) {
      throw new Error(result.message || 'Failed to get auth status');
    }
    
    return result.data;
  }

  /**
   * Login with username and password
   */
  async login(username, password) {
    try {
      const response = await fetch(`${this.baseURL}/auth/login`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        credentials: 'include',  // Include cookies
        body: JSON.stringify({ username, password }),
      });

      const result = await response.json();
      
      if (result.success) {
        this.authenticated = true;
        this.user = result.data.user;
        this.notifyListeners();
        return { success: true, user: this.user };
      } else {
        return { success: false, error: result.message };
      }
    } catch (error) {
      console.error('Login error:', error);
      return { success: false, error: 'Login failed' };
    }
  }

  /**
   * Logout current user
   */
  async logout() {
    try {
      // CRITICAL FIX: Immediately disconnect WebSocket BEFORE invalidating token
      // This prevents the WebSocket from continuing to try to reconnect with invalid credentials
      console.log("🔌 Proactively disconnecting WebSocket before logout...");
      try {
        // Import webSocketClient dynamically to avoid circular dependency
        const webSocketClientModule = await import('./webSocketClient.js');
        webSocketClientModule.default.disconnect();
        console.log("✅ WebSocket disconnected successfully during logout");
      } catch (wsError) {
        console.warn("⚠️ Failed to disconnect WebSocket during logout:", wsError);
        // Continue with logout even if WebSocket disconnect fails
      }

      const response = await fetch(`${this.baseURL}/auth/logout`, {
        method: 'POST',
        credentials: 'include',  // Include cookies
      });

      const result = await response.json();
      
      // Always clear local state, even if backend call fails
      this.authenticated = false;
      this.user = null;
      
      // Also manually clear the cookie as a fallback
      document.cookie = 'juicytrade_session=; path=/; expires=Thu, 01 Jan 1970 00:00:00 GMT; samesite=lax';
      
      this.notifyListeners();
      
      if (result.success) {
        return { success: true };
      } else {
        console.warn('Backend logout failed, but local state cleared:', result.message);
        return { success: true }; // Still return success since we cleared local state
      }
    } catch (error) {
      console.error('Logout error:', error);
      
      // Even if the request fails, clear local state
      this.authenticated = false;
      this.user = null;
      
      // Manually clear the cookie
      document.cookie = 'juicytrade_session=; path=/; expires=Thu, 01 Jan 1970 00:00:00 GMT; samesite=lax';
      
      this.notifyListeners();
      return { success: true }; // Return success since we cleared local state
    }
  }

  /**
   * Get current user information
   */
  async getCurrentUser() {
    if (!this.authenticated) {
      return null;
    }

    try {
      const response = await fetch(`${this.baseURL}/auth/user`, {
        credentials: 'include'  // Include cookies
      });
      const result = await response.json();
      
      if (result.success) {
        this.user = result.data;
        this.notifyListeners();
        return this.user;
      } else {
        // User might not be authenticated anymore
        this.authenticated = false;
        this.user = null;
        this.notifyListeners();
        return null;
      }
    } catch (error) {
      console.error('Get user error:', error);
      return null;
    }
  }

  /**
   * Check if user is authenticated
   */
  isAuthenticated() {
    return this.authenticated;
  }

  /**
   * Get current user
   */
  getUser() {
    return this.user;
  }

  /**
   * Get authentication method
   */
  getAuthMethod() {
    return this.authMethod;
  }

  /**
   * Check if authentication is enabled
   */
  isAuthEnabled() {
    return this.authMethod !== 'disabled';
  }

  /**
   * Get OAuth login URL
   */
  getOAuthLoginUrl(next = null) {
    let url = `${this.baseURL}/auth/oauth/authorize`;
    if (next) {
      url += `?next=${encodeURIComponent(next)}`;
    }
    return url;
  }

  /**
   * Redirect to login page
   */
  redirectToLogin(next = null) {
    let url = '/auth/login';
    if (next) {
      url += `?next=${encodeURIComponent(next)}`;
    }
    window.location.href = url;
  }

  /**
   * Add authentication state listener
   */
  addListener(callback) {
    this.listeners.push(callback);
    return () => {
      this.listeners = this.listeners.filter(listener => listener !== callback);
    };
  }

  /**
   * Notify all listeners of auth state change
   */
  notifyListeners() {
    this.listeners.forEach(callback => {
      try {
        callback({
          authenticated: this.authenticated,
          user: this.user,
          authMethod: this.authMethod
        });
      } catch (error) {
        console.error('Auth listener error:', error);
      }
    });
  }

  /**
   * Handle API response for authentication errors
   */
  handleApiResponse(response) {
    if (response.status === 401) {
      // User is not authenticated
      this.authenticated = false;
      this.user = null;
      this.notifyListeners();
      
      if (this.isAuthEnabled()) {
        // Redirect to login if auth is enabled
        this.redirectToLogin(window.location.pathname);
      }
    }
    return response;
  }

  /**
   * Create authenticated fetch wrapper
   */
  createAuthenticatedFetch() {
    const originalFetch = window.fetch;
    
    return async (url, options = {}) => {
      const response = await originalFetch(url, options);
      return this.handleApiResponse(response);
    };
  }
}

// Create singleton instance
const authService = new AuthService();

export default authService;
