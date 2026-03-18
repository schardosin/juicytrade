<template>
  <div class="oauth-callback-page">
    <div class="oauth-card">
      <!-- Loading -->
      <template v-if="status === 'loading'">
        <div class="oauth-icon spinner">
          <i class="pi pi-spin pi-spinner"></i>
        </div>
        <h1>Completing Authorization</h1>
        <p>Exchanging authorization code with Schwab...</p>
      </template>

      <!-- Success -->
      <template v-else-if="status === 'success'">
        <div class="oauth-icon success">&#10004;</div>
        <h1>Authorization Successful</h1>
        <p>This window will close automatically.</p>
      </template>

      <!-- Cancelled -->
      <template v-else-if="status === 'cancelled'">
        <div class="oauth-icon cancelled">&#9888;</div>
        <h1>Authorization Cancelled</h1>
        <p>The authorization was cancelled. You can close this window.</p>
      </template>

      <!-- Error -->
      <template v-else-if="status === 'error'">
        <div class="oauth-icon error">&#10008;</div>
        <h1>Authorization Failed</h1>
        <p>{{ errorMessage }}</p>
        <button class="close-btn" @click="closeWindow">Close</button>
      </template>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue';
import { useRoute } from 'vue-router';
import { api } from '@/services/api.js';

const route = useRoute();
const status = ref('loading');
const errorMessage = ref('');

function notifyOpener(callbackStatus, error) {
  if (window.opener) {
    try {
      window.opener.postMessage(
        { type: 'schwab-oauth-callback', status: callbackStatus, error: error || undefined },
        window.location.origin
      );
    } catch (e) {
      console.warn('Failed to send postMessage to opener:', e);
    }
  }
}

function closeWindow() {
  try {
    window.close();
  } catch (e) {
    // Some browsers block window.close() if the window wasn't opened by script
    console.warn('Could not close window:', e);
  }
}

function autoClose(delayMs = 2000) {
  setTimeout(() => closeWindow(), delayMs);
}

onMounted(async () => {
  const code = route.query.code;
  const state = route.query.state;
  const errorParam = route.query.error;

  // User cancelled on Schwab
  if (errorParam) {
    try {
      await api.relaySchwabOAuthCallbackError(errorParam, state);
    } catch (e) {
      // Best-effort — the backend will also detect the cancellation via polling timeout
      console.warn('Failed to relay cancellation to backend:', e);
    }
    status.value = 'cancelled';
    notifyOpener('cancelled');
    autoClose();
    return;
  }

  // Normal flow — relay code and state to backend
  if (code && state) {
    try {
      const result = await api.relaySchwabOAuthCallback(code, state);
      if (result.status === 'success') {
        status.value = 'success';
        notifyOpener('success');
        autoClose();
      } else {
        status.value = 'error';
        errorMessage.value = result.error || 'Authorization failed';
        notifyOpener('error', errorMessage.value);
      }
    } catch (e) {
      status.value = 'error';
      errorMessage.value = e.response?.data?.error || e.message || 'Failed to complete authorization';
      notifyOpener('error', errorMessage.value);
    }
    return;
  }

  // Missing parameters
  status.value = 'error';
  errorMessage.value = 'Missing authorization parameters. Please try the authorization flow again.';
  notifyOpener('error', errorMessage.value);
});
</script>

<style scoped>
.oauth-callback-page {
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 100vh;
  background: #1a1a2e;
  color: #ffffff;
  font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
  margin: 0;
  padding: 16px;
}

.oauth-card {
  text-align: center;
  background: #16213e;
  border-radius: 12px;
  padding: 48px 40px;
  max-width: 480px;
  width: 100%;
  box-shadow: 0 4px 24px rgba(0, 0, 0, 0.3);
}

.oauth-icon {
  font-size: 64px;
  margin-bottom: 16px;
  line-height: 1;
}

.oauth-icon.spinner {
  color: #64b5f6;
}

.oauth-icon.spinner i {
  font-size: 64px;
}

.oauth-icon.success {
  color: #00c853;
}

.oauth-icon.cancelled {
  color: #ffc107;
}

.oauth-icon.error {
  color: #ff1744;
}

h1 {
  font-size: 22px;
  margin-bottom: 12px;
  font-weight: 600;
  color: #ffffff;
}

p {
  font-size: 15px;
  color: #b0b0b0;
  line-height: 1.5;
  margin: 0;
}

.close-btn {
  margin-top: 24px;
  padding: 10px 32px;
  background: #2a2a4a;
  color: #ffffff;
  border: 1px solid #3a3a5a;
  border-radius: 6px;
  font-size: 14px;
  cursor: pointer;
  transition: background 0.2s;
}

.close-btn:hover {
  background: #3a3a5a;
}
</style>
