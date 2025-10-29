<template>
  <div class="login-container">
    <div class="login-card">
      <div class="login-header">
        <div class="login-logo">
          <img src="/logos/juicytrade-logo.svg" alt="juicytrade" class="logo-svg" />
        </div>
        <p class="login-subtitle">Professional Options Trading Platform</p>
        <div class="auth-method-info">
          Authentication: {{ authMethodDisplay }}
        </div>
      </div>

      <!-- Simple/Token Authentication Form -->
      <div v-if="showLoginForm" class="login-form">
        <form @submit.prevent="handleLogin">
          <div class="form-group">
            <label for="username" class="form-label">Username</label>
            <InputText
              id="username"
              v-model="credentials.username"
              class="form-input"
              placeholder="Enter username"
              :disabled="isLoading"
              required
            />
          </div>

          <div class="form-group">
            <label for="password" class="form-label">Password</label>
            <InputText
              id="password"
              v-model="credentials.password"
              type="password"
              class="form-input"
              placeholder="Enter password"
              :disabled="isLoading"
              required
            />
          </div>

          <Button
            type="submit"
            class="login-button"
            :loading="isLoading"
            :disabled="!canSubmit"
          >
            <i class="pi pi-sign-in" style="margin-right: 8px;"></i>
            Sign In
          </Button>
        </form>

        <div v-if="errorMessage" class="error-message">
          <i class="pi pi-exclamation-triangle"></i>
          {{ errorMessage }}
        </div>
      </div>

      <!-- OAuth Authentication -->
      <div v-if="showOAuthLogin" class="oauth-login">
        <Button
          @click="handleOAuthLogin"
          class="oauth-button"
          :class="`oauth-${authConfig.oauth_provider}`"
          :loading="isLoading"
        >
          <i :class="getOAuthIcon()" style="margin-right: 8px;"></i>
          Sign in with {{ getOAuthProviderName() }}
        </Button>
      </div>

      <!-- Authentication Disabled -->
      <div v-if="!authConfig.enabled" class="auth-disabled">
        <div class="auth-disabled-icon">
          <i class="pi pi-unlock"></i>
        </div>
        <p class="auth-disabled-text">
          Authentication is currently disabled. You have full access to the platform.
        </p>
        <Button @click="proceedWithoutAuth" class="proceed-button">
          <i class="pi pi-arrow-right" style="margin-right: 8px;"></i>
          Continue to Platform
        </Button>
      </div>

      <!-- Loading State -->
      <div v-if="isInitializing" class="loading-state">
        <ProgressSpinner class="loading-spinner" />
        <p class="loading-text">Initializing authentication...</p>
      </div>
    </div>

    <!-- Footer -->
    <div class="login-footer">
      <p class="footer-text">
        Secure • Professional • Reliable
      </p>
    </div>
  </div>
</template>

<script>
import { ref, computed, onMounted } from 'vue';
import { useRouter } from 'vue-router';
import authService from '../../services/authService.js';

export default {
  name: 'LoginPage',
  setup() {
    const router = useRouter();
    
    // Reactive state
    const isInitializing = ref(true);
    const isLoading = ref(false);
    const errorMessage = ref('');
    const authConfig = ref({
      method: 'disabled',
      enabled: false,
      oauth_provider: null
    });
    
    const credentials = ref({
      username: '',
      password: ''
    });

    // Computed properties
    const authMethodDisplay = computed(() => {
      const method = authConfig.value.method;
      switch (method) {
        case 'simple': return 'Simple (Username/Password)';
        case 'oauth': return `OAuth (${authConfig.value.oauth_provider || 'Unknown'})`;
        case 'token': return 'Token-based';
        case 'header': return 'Header-based';
        case 'disabled': return 'Disabled';
        default: return method;
      }
    });

    const showLoginForm = computed(() => {
      return authConfig.value.enabled && 
             (authConfig.value.method === 'simple' || authConfig.value.method === 'token');
    });

    const showOAuthLogin = computed(() => {
      return authConfig.value.enabled && 
             authConfig.value.method === 'oauth' && 
             authConfig.value.oauth_provider;
    });

    const canSubmit = computed(() => {
      return credentials.value.username.trim() && 
             credentials.value.password.trim() && 
             !isLoading.value;
    });

    // Methods
    const initializeAuth = async () => {
      try {
        isInitializing.value = true;
        
        // Initialize auth service
        await authService.init();
        
        // Get auth configuration
        const config = await authService.getAuthConfig();
        authConfig.value = config;
        
        // Check if already authenticated
        if (authService.isAuthenticated()) {
          // Redirect to main app
          const next = new URLSearchParams(window.location.search).get('next') || '/';
          router.push(next);
          return;
        }
        
      } catch (error) {
        console.error('Failed to initialize authentication:', error);
        errorMessage.value = 'Failed to initialize authentication system';
      } finally {
        isInitializing.value = false;
      }
    };

    const handleLogin = async () => {
      if (!canSubmit.value) return;
      
      try {
        isLoading.value = true;
        errorMessage.value = '';
        
        const result = await authService.login(
          credentials.value.username,
          credentials.value.password
        );
        
        if (result.success) {
          // Redirect to main app
          const next = new URLSearchParams(window.location.search).get('next') || '/';
          router.push(next);
        } else {
          errorMessage.value = result.error || 'Login failed';
        }
        
      } catch (error) {
        console.error('Login error:', error);
        errorMessage.value = 'An unexpected error occurred';
      } finally {
        isLoading.value = false;
      }
    };

    const handleOAuthLogin = () => {
      const next = new URLSearchParams(window.location.search).get('next');
      const oauthUrl = authService.getOAuthLoginUrl(next);
      window.location.href = oauthUrl;
    };

    const proceedWithoutAuth = () => {
      const next = new URLSearchParams(window.location.search).get('next') || '/';
      router.push(next);
    };

    const getOAuthIcon = () => {
      const provider = authConfig.value.oauth_provider;
      switch (provider) {
        case 'google': return 'pi pi-google';
        case 'github': return 'pi pi-github';
        case 'microsoft': return 'pi pi-microsoft';
        default: return 'pi pi-sign-in';
      }
    };

    const getOAuthProviderName = () => {
      const provider = authConfig.value.oauth_provider;
      switch (provider) {
        case 'google': return 'Google';
        case 'github': return 'GitHub';
        case 'microsoft': return 'Microsoft';
        default: return provider;
      }
    };

    // Lifecycle
    onMounted(() => {
      initializeAuth();
    });

    return {
      // State
      isInitializing,
      isLoading,
      errorMessage,
      authConfig,
      credentials,
      
      // Computed
      authMethodDisplay,
      showLoginForm,
      showOAuthLogin,
      canSubmit,
      
      // Methods
      handleLogin,
      handleOAuthLogin,
      proceedWithoutAuth,
      getOAuthIcon,
      getOAuthProviderName
    };
  }
};
</script>

<style scoped>
.login-container {
  min-height: 100vh;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  background: linear-gradient(135deg, #141519 0%, #1a1d23 100%);
  padding: 20px;
}

.login-card {
  background: var(--surface-card);
  border: 1px solid var(--surface-border);
  border-radius: 12px;
  padding: 40px;
  width: 100%;
  max-width: 400px;
  box-shadow: 0 8px 32px rgba(0, 0, 0, 0.3);
}

.login-header {
  text-align: center;
  margin-bottom: 32px;
}

.login-logo {
  display: flex;
  justify-content: center;
  align-items: center;
  margin-bottom: 16px;
}

.logo-svg {
  height: 80px;
  width: auto;
  display: block;
  transition: all 0.2s ease;
}

.logo-svg:hover {
  opacity: 0.8;
}

.login-subtitle {
  font-size: 1rem;
  color: var(--text-color-secondary);
  margin: 0 0 16px 0;
}

.auth-method-info {
  font-size: 0.875rem;
  color: var(--text-color-secondary);
  background: var(--surface-ground);
  padding: 8px 16px;
  border-radius: 6px;
  border: 1px solid var(--surface-border);
}

.login-form {
  margin-bottom: 24px;
}

.form-group {
  margin-bottom: 20px;
}

.form-label {
  display: block;
  font-size: 0.875rem;
  font-weight: 600;
  color: var(--text-color);
  margin-bottom: 6px;
}

.form-input {
  width: 100%;
  background: var(--surface-ground);
  border: 1px solid var(--surface-border);
  color: var(--text-color);
}

.form-input:focus {
  border-color: var(--primary-color);
  box-shadow: 0 0 0 2px rgba(34, 197, 94, 0.2);
}

.login-button {
  width: 100%;
  background: var(--primary-color);
  border: none;
  padding: 12px;
  font-size: 1rem;
  font-weight: 600;
  border-radius: 8px;
  transition: all 0.2s ease;
}

.login-button:hover:not(:disabled) {
  background: var(--primary-color-dark);
  transform: translateY(-1px);
}

.oauth-login {
  margin-bottom: 24px;
}

.oauth-button {
  width: 100%;
  padding: 12px;
  font-size: 1rem;
  font-weight: 600;
  border-radius: 8px;
  transition: all 0.2s ease;
}

.oauth-google {
  background: #4285f4;
  border: none;
  color: white;
}

.oauth-github {
  background: #333;
  border: none;
  color: white;
}

.oauth-microsoft {
  background: #0078d4;
  border: none;
  color: white;
}

.oauth-button:hover:not(:disabled) {
  transform: translateY(-1px);
  opacity: 0.9;
}

.auth-disabled {
  text-align: center;
  padding: 20px 0;
}

.auth-disabled-icon {
  font-size: 3rem;
  color: var(--text-color-secondary);
  margin-bottom: 16px;
}

.auth-disabled-text {
  color: var(--text-color-secondary);
  margin-bottom: 24px;
  line-height: 1.5;
}

.proceed-button {
  background: var(--surface-ground);
  border: 1px solid var(--surface-border);
  color: var(--text-color);
  padding: 12px 24px;
  border-radius: 8px;
  transition: all 0.2s ease;
}

.proceed-button:hover {
  background: var(--surface-hover);
  transform: translateY(-1px);
}

.loading-state {
  text-align: center;
  padding: 40px 0;
}

.loading-spinner {
  margin-bottom: 16px;
}

.loading-text {
  color: var(--text-color-secondary);
  font-size: 0.875rem;
}

.error-message {
  background: rgba(239, 68, 68, 0.1);
  border: 1px solid rgba(239, 68, 68, 0.3);
  color: #ef4444;
  padding: 12px;
  border-radius: 6px;
  margin-top: 16px;
  font-size: 0.875rem;
  display: flex;
  align-items: center;
  gap: 8px;
}

.login-footer {
  margin-top: 32px;
  text-align: center;
}

.footer-text {
  color: var(--text-color-secondary);
  font-size: 0.875rem;
  margin: 0;
}

/* Responsive design */
@media (max-width: 480px) {
  .login-card {
    padding: 24px;
    margin: 16px;
  }
  
  .login-title {
    font-size: 2rem;
  }
}
</style>
