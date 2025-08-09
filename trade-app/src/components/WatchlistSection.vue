<template>
  <div class="watchlist-section">
    <!-- Controls Section -->
    <div class="controls-section">
      <!-- Watchlist Selector Row -->
      <div class="selector-row">
        <select 
          v-model="selectedWatchlistId" 
          class="watchlist-selector"
          @change="handleWatchlistChange"
          :disabled="isLoading"
        >
          <option 
            v-for="option in watchlistOptions" 
            :key="option.value" 
            :value="option.value"
          >
            {{ option.label }}
          </option>
        </select>
        <button 
          class="settings-btn" 
          @click="showSettings = !showSettings"
          title="Watchlist Settings"
        >
          ⚙️
        </button>
      </div>

      <!-- Settings Panel -->
      <div v-if="showSettings" class="settings-panel">
        <div class="settings-row">
          <button @click="showCreateDialog = true" class="create-btn">
            + New Watchlist
          </button>
          <button 
            @click="handleRenameWatchlist" 
            class="rename-btn"
            :disabled="!activeWatchlist"
          >
            Rename
          </button>
          <button 
            @click="handleDeleteWatchlist" 
            class="delete-btn"
            :disabled="!activeWatchlist || Object.keys(watchlists).length <= 1"
          >
            Delete
          </button>
        </div>
      </div>

      <!-- Add Symbol Row -->
      <div class="symbol-row">
        <input 
          v-model="newSymbol"
          @keyup.enter="handleAddSymbol"
          @input="validateInput"
          placeholder="Add Symbol: AAPL"
          class="symbol-input"
          :disabled="isLoading"
        />
        <button 
          @click="handleAddSymbol" 
          class="add-btn"
          :disabled="!canAddSymbol"
        >
          Add
        </button>
      </div>
    </div>

    <!-- Loading State -->
    <div v-if="isLoading" class="loading-state">
      <div class="loading-spinner"></div>
      <span>Loading watchlist...</span>
    </div>

    <!-- Error State -->
    <div v-else-if="error" class="error-state">
      <span class="error-message">{{ error }}</span>
      <button @click="loadWatchlists" class="retry-btn">Retry</button>
    </div>

    <!-- Watchlist Content -->
    <div v-else class="watchlist-content">
      <div v-if="!activeWatchlist" class="no-watchlist">
        <p>No watchlist selected</p>
        <button @click="showCreateDialog = true" class="create-first-btn">
          Create Your First Watchlist
        </button>
      </div>

      <div v-else-if="activeSymbols.length === 0" class="empty-watchlist">
        <p>{{ activeWatchlist.name }} is empty</p>
        <p class="hint">Add symbols using the input above</p>
      </div>

      <div v-else class="symbols-table">
        <!-- Table Header -->
        <div class="table-header">
          <span class="symbol-col">Symbol</span>
          <span class="last-col">Last</span>
          <span class="change-col">Chg</span>
          <span class="change-percent-col">Chg%</span>
          <span class="actions-col"></span>
        </div>

        <!-- Symbol Rows -->
        <div 
          v-for="symbol in activeSymbols" 
          :key="symbol"
          class="symbol-data-row"
          @click="handleSymbolClick(symbol)"
        >
          <span class="symbol-name">{{ symbol }}</span>
          
          <span class="last-price">
            {{ formatPrice(getSymbolPrice(symbol)) }}
          </span>
          
          <span 
            class="change-amount"
            :class="getChangeClass(getSymbolChange(symbol))"
          >
            {{ formatChange(getSymbolChange(symbol)) }}
          </span>
          
          <span 
            class="change-percent"
            :class="getChangeClass(getSymbolChangePercent(symbol))"
          >
            {{ formatChangePercent(getSymbolChangePercent(symbol)) }}
          </span>
          
          <button 
            class="remove-btn"
            @click.stop="handleRemoveSymbol(symbol)"
            title="Remove symbol"
          >
            ×
          </button>
        </div>
      </div>
    </div>

    <!-- Create Watchlist Dialog -->
    <div v-if="showCreateDialog" class="dialog-overlay" @click="closeCreateDialog">
      <div class="dialog" @click.stop>
        <div class="dialog-header">
          <h4>Create New Watchlist</h4>
          <button @click="closeCreateDialog" class="close-btn">×</button>
        </div>
        <div class="dialog-body">
          <input 
            v-model="newWatchlistName"
            @keyup.enter="handleCreateWatchlist"
            placeholder="Watchlist name"
            class="dialog-input"
            ref="watchlistNameInput"
          />
        </div>
        <div class="dialog-footer">
          <button @click="closeCreateDialog" class="cancel-btn">Cancel</button>
          <button 
            @click="handleCreateWatchlist" 
            class="confirm-btn"
            :disabled="!newWatchlistName.trim()"
          >
            Create
          </button>
        </div>
      </div>
    </div>

    <!-- Rename Dialog -->
    <div v-if="showRenameDialog" class="dialog-overlay" @click="closeRenameDialog">
      <div class="dialog" @click.stop>
        <div class="dialog-header">
          <h4>Rename Watchlist</h4>
          <button @click="closeRenameDialog" class="close-btn">×</button>
        </div>
        <div class="dialog-body">
          <input 
            v-model="renameWatchlistName"
            @keyup.enter="handleConfirmRename"
            placeholder="New watchlist name"
            class="dialog-input"
            ref="renameInput"
          />
        </div>
        <div class="dialog-footer">
          <button @click="closeRenameDialog" class="cancel-btn">Cancel</button>
          <button 
            @click="handleConfirmRename" 
            class="confirm-btn"
            :disabled="!renameWatchlistName.trim()"
          >
            Rename
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<script>
import { ref, computed, nextTick, watch, onUnmounted } from "vue";
import { useWatchlist } from "../composables/useWatchlist.js";
import { smartMarketDataStore } from "../services/smartMarketDataStore.js";

export default {
  name: "WatchlistSection",
  setup() {
    // Component registration system
    const componentId = `WatchlistSection-${Math.random().toString(36).substr(2, 9)}`;
    const registeredSymbols = new Set();

    // Single registration method per component to prevent double registration
    const ensureSymbolRegistration = (symbol) => {
      if (!registeredSymbols.has(symbol)) {
        smartMarketDataStore.registerSymbolUsage(symbol, componentId);
        registeredSymbols.add(symbol);
      }
    };

    // Watchlist composable
    const {
      watchlists,
      activeWatchlistId,
      activeWatchlist,
      watchlistOptions,
      activeSymbols,
      isLoading,
      error,
      loadWatchlists,
      createWatchlist,
      deleteWatchlist,
      addSymbol,
      removeSymbol,
      setActiveWatchlist,
      renameWatchlist,
      validateSymbol,
      getSymbolData,
    } = useWatchlist();

    // Local state
    const newSymbol = ref("");
    const selectedWatchlistId = ref("");
    const showSettings = ref(false);
    const showCreateDialog = ref(false);
    const showRenameDialog = ref(false);
    const newWatchlistName = ref("");
    const renameWatchlistName = ref("");

    // Refs for dialog inputs
    const watchlistNameInput = ref(null);
    const renameInput = ref(null);

    // Sync selected watchlist with active watchlist
    watch(activeWatchlistId, (newId) => {
      selectedWatchlistId.value = newId;
    }, { immediate: true });

    // Computed properties
    const canAddSymbol = computed(() => {
      return newSymbol.value.trim() && 
             validateSymbol(newSymbol.value) && 
             selectedWatchlistId.value &&
             !isLoading.value;
    });

    // Methods
    const validateInput = () => {
      // Real-time validation feedback could be added here
    };

    const handleAddSymbol = async () => {
      if (!canAddSymbol.value) return;

      const success = await addSymbol(newSymbol.value, selectedWatchlistId.value);
      if (success) {
        newSymbol.value = "";
      }
    };

    const handleRemoveSymbol = async (symbol) => {
      await removeSymbol(symbol, selectedWatchlistId.value);
    };

    const handleWatchlistChange = async () => {
      if (selectedWatchlistId.value !== activeWatchlistId.value) {
        await setActiveWatchlist(selectedWatchlistId.value);
      }
    };

    const handleCreateWatchlist = async () => {
      if (!newWatchlistName.value.trim()) return;

      try {
        const watchlistId = await createWatchlist(newWatchlistName.value.trim());
        if (watchlistId) {
          await setActiveWatchlist(watchlistId);
        }
        closeCreateDialog();
      } catch (err) {
        // Error handled by composable
      }
    };

    const handleDeleteWatchlist = async () => {
      if (!activeWatchlist.value) return;

      const confirmed = confirm(`Are you sure you want to delete "${activeWatchlist.value.name}"?`);
      if (confirmed) {
        await deleteWatchlist(activeWatchlistId.value);
      }
    };

    const handleRenameWatchlist = () => {
      if (!activeWatchlist.value) return;
      
      renameWatchlistName.value = activeWatchlist.value.name;
      showRenameDialog.value = true;
      
      nextTick(() => {
        renameInput.value?.focus();
      });
    };

    const handleConfirmRename = async () => {
      if (!renameWatchlistName.value.trim()) return;

      try {
        await renameWatchlist(activeWatchlistId.value, renameWatchlistName.value.trim());
        closeRenameDialog();
      } catch (err) {
        // Error handled by composable
      }
    };

    const closeCreateDialog = () => {
      showCreateDialog.value = false;
      newWatchlistName.value = "";
    };

    const closeRenameDialog = () => {
      showRenameDialog.value = false;
      renameWatchlistName.value = "";
    };

    // Price formatting and data methods
    const getSymbolPrice = (symbol) => {
      const data = getSymbolData(symbol);
      return data.value?.price || 0;
    };

    const getSymbolBid = (symbol) => {
      const data = getSymbolData(symbol);
      return data.value?.bid || 0;
    };

    const getSymbolAsk = (symbol) => {
      const data = getSymbolData(symbol);
      return data.value?.ask || 0;
    };

    const getSymbolChange = (symbol) => {
      const data = getSymbolData(symbol);
      return data.value?.change || 0;
    };

    const getSymbolChangePercent = (symbol) => {
      const data = getSymbolData(symbol);
      return data.value?.changePercent || 0;
    };

    const formatPrice = (price) => {
      if (!price || price === 0) return "--";
      return price.toFixed(2);
    };

    const formatChange = (change) => {
      if (!change || change === 0) return "0.00";
      const sign = change > 0 ? "+" : "";
      return `${sign}${change.toFixed(2)}`;
    };

    const formatChangePercent = (changePercent) => {
      if (!changePercent || changePercent === 0) return "0.00%";
      const sign = changePercent > 0 ? "+" : "";
      return `${sign}${changePercent.toFixed(2)}%`;
    };

    const getChangeClass = (change) => {
      if (!change || change === 0) return "neutral";
      return change > 0 ? "positive" : "negative";
    };

    // Handle symbol click - dispatch symbol selection event
    const handleSymbolClick = (symbol) => {
      // Create symbol data object similar to what TopBar creates
      const symbolData = {
        symbol: symbol,
        description: `${symbol} Stock`, // Basic description, could be enhanced
        exchange: "Unknown", // Could be enhanced with real exchange data
        type: "stock"
      };

      // Dispatch the same event that TopBar uses for symbol selection
      window.dispatchEvent(
        new CustomEvent("symbol-selected", {
          detail: symbolData,
        })
      );
    };

    // Component cleanup system
    const cleanupComponentRegistrations = () => {
      console.log(`📝 WatchlistSection: Unregistering component ${componentId} with ${registeredSymbols.size} symbols`);
      
      // Unregister all symbols this component was using
      for (const symbol of registeredSymbols) {
        smartMarketDataStore.unregisterSymbolUsage(symbol, componentId);
      }
      
      // Clear local tracking
      registeredSymbols.clear();
    };

    // Watch for active watchlist changes to clean up old registrations
    watch(
      activeSymbols,
      (newSymbols, oldSymbols) => {
        if (newSymbols !== oldSymbols) {
          console.log(`📝 WatchlistSection: Active symbols changed. Cleaning up component registrations.`);
          
          // Unregister all current symbols
          cleanupComponentRegistrations();
        }
      },
      { immediate: true }
    );

    // Focus dialog inputs when opened
    watch(showCreateDialog, (show) => {
      if (show) {
        nextTick(() => {
          watchlistNameInput.value?.focus();
        });
      }
    });

    // Clean up when the component is unmounted
    onUnmounted(() => {
      console.log(`📝 WatchlistSection unmounted. Cleaning up component registrations.`);
      cleanupComponentRegistrations();
    });

    return {
      // State
      watchlists,
      activeWatchlistId,
      activeWatchlist,
      watchlistOptions,
      activeSymbols,
      isLoading,
      error,
      newSymbol,
      selectedWatchlistId,
      showSettings,
      showCreateDialog,
      showRenameDialog,
      newWatchlistName,
      renameWatchlistName,
      canAddSymbol,

      // Refs
      watchlistNameInput,
      renameInput,

      // Methods
      loadWatchlists,
      validateInput,
      handleAddSymbol,
      handleRemoveSymbol,
      handleWatchlistChange,
      handleCreateWatchlist,
      handleDeleteWatchlist,
      handleRenameWatchlist,
      handleConfirmRename,
      closeCreateDialog,
      closeRenameDialog,
      getSymbolPrice,
      getSymbolBid,
      getSymbolAsk,
      getSymbolChange,
      getSymbolChangePercent,
      formatPrice,
      formatChange,
      formatChangePercent,
      getChangeClass,
      handleSymbolClick,
    };
  },
};
</script>

<style scoped>
.watchlist-section {
  display: flex;
  flex-direction: column;
  height: 100%;
  background-color: var(--bg-primary);
}

/* Controls Section - Unified styling */
.controls-section {
  background-color: var(--bg-secondary);
  border-bottom: 1px solid var(--border-secondary);
}

.selector-row {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 12px 16px;
}

.watchlist-selector {
  flex: 1;
  height: 32px;
  padding: 0 10px;
  background-color: var(--bg-tertiary);
  border: 1px solid var(--border-secondary);
  border-radius: var(--radius-sm);
  color: var(--text-primary);
  font-size: var(--font-size-sm);
  font-weight: var(--font-weight-medium);
  cursor: pointer;
  transition: var(--transition-fast);
}

.watchlist-selector:focus {
  outline: none;
  border-color: var(--color-info);
  background-color: var(--bg-quaternary);
}

.settings-btn {
  background: none;
  border: none;
  color: var(--text-secondary);
  cursor: pointer;
  padding: 6px;
  border-radius: var(--radius-sm);
  transition: var(--transition-fast);
  width: 32px;
  height: 32px;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 14px;
}

.settings-btn:hover {
  background-color: var(--bg-tertiary);
  color: var(--text-primary);
}

.symbol-row {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 12px 16px;
  border-top: 1px solid var(--border-primary);
}

.symbol-input {
  flex: 1;
  height: 32px;
  padding: 0 10px;
  background-color: var(--bg-tertiary);
  border: 1px solid var(--border-secondary);
  border-radius: var(--radius-sm);
  color: var(--text-primary);
  font-size: var(--font-size-sm);
  transition: var(--transition-fast);
}

.symbol-input:focus {
  outline: none;
  border-color: var(--color-info);
  background-color: var(--bg-quaternary);
}

.symbol-input::placeholder {
  color: var(--text-tertiary);
}

.add-btn {
  height: 32px;
  padding: 0 12px;
  background-color: #ff6b35;
  border: none;
  border-radius: var(--radius-sm);
  color: white;
  font-size: var(--font-size-sm);
  font-weight: var(--font-weight-medium);
  cursor: pointer;
  transition: var(--transition-fast);
  white-space: nowrap;
}

.add-btn:hover:not(:disabled) {
  background-color: #e55a2b;
}

.add-btn:disabled {
  background-color: var(--bg-tertiary);
  color: var(--text-tertiary);
  cursor: not-allowed;
}

.watchlist-select {
  padding: 8px 12px;
  background-color: var(--bg-tertiary);
  border: 1px solid var(--border-secondary);
  border-radius: var(--radius-sm);
  color: var(--text-primary);
  font-size: var(--font-size-sm);
  cursor: pointer;
  min-width: 120px;
}

.watchlist-select:focus {
  outline: none;
  border-color: var(--color-info);
}

.settings-panel {
  padding: 12px 16px;
  background-color: var(--bg-secondary);
  border-bottom: 1px solid var(--border-secondary);
}

.settings-row {
  display: flex;
  gap: 8px;
}

.create-btn, .rename-btn, .delete-btn {
  padding: 6px 12px;
  border: 1px solid var(--border-secondary);
  border-radius: var(--radius-sm);
  font-size: var(--font-size-xs);
  cursor: pointer;
  transition: var(--transition-fast);
}

.create-btn {
  background-color: var(--color-success);
  color: white;
  border-color: var(--color-success);
}

.rename-btn {
  background-color: var(--bg-tertiary);
  color: var(--text-primary);
}

.delete-btn {
  background-color: var(--color-danger);
  color: white;
  border-color: var(--color-danger);
}

.create-btn:hover, .rename-btn:hover, .delete-btn:hover {
  opacity: 0.8;
}

.create-btn:disabled, .rename-btn:disabled, .delete-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.loading-state, .error-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 40px 16px;
  color: var(--text-secondary);
}

.loading-spinner {
  width: 20px;
  height: 20px;
  border: 2px solid var(--border-secondary);
  border-top: 2px solid var(--color-primary);
  border-radius: 50%;
  animation: spin 1s linear infinite;
  margin-bottom: 12px;
}

@keyframes spin {
  0% { transform: rotate(0deg); }
  100% { transform: rotate(360deg); }
}

.error-message {
  color: var(--color-danger);
  margin-bottom: 12px;
}

.retry-btn {
  padding: 6px 12px;
  background-color: var(--color-info);
  border: none;
  border-radius: var(--radius-sm);
  color: white;
  cursor: pointer;
}

.watchlist-content {
  flex: 1;
  overflow-y: auto;
}

.no-watchlist, .empty-watchlist {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 40px 16px;
  color: var(--text-secondary);
  text-align: center;
}

.hint {
  font-size: var(--font-size-xs);
  color: var(--text-tertiary);
  margin-top: 8px;
}

.create-first-btn {
  margin-top: 16px;
  padding: 8px 16px;
  background-color: var(--color-primary);
  border: none;
  border-radius: var(--radius-sm);
  color: white;
  cursor: pointer;
}

.symbols-table {
  display: flex;
  flex-direction: column;
}

.table-header {
  display: grid;
  grid-template-columns: 1fr 80px 80px 80px 30px;
  gap: 8px;
  padding: 8px 16px;
  background-color: var(--bg-secondary);
  border-bottom: 1px solid var(--border-secondary);
  font-size: var(--font-size-xs);
  font-weight: var(--font-weight-semibold);
  color: var(--text-secondary);
  text-transform: uppercase;
}

.table-header .symbol-col {
  text-align: left;
}

.table-header .last-col,
.table-header .change-col,
.table-header .change-percent-col {
  text-align: right;
}

.table-header .actions-col {
  text-align: center;
}

.symbol-data-row {
  display: grid;
  grid-template-columns: 1fr 80px 80px 80px 30px;
  gap: 8px;
  padding: 12px 16px;
  border-bottom: 1px solid var(--border-secondary);
  font-size: var(--font-size-sm);
  transition: var(--transition-fast);
  cursor: pointer;
}

.symbol-data-row:hover {
  background-color: var(--bg-tertiary);
}

.symbol-name {
  font-weight: var(--font-weight-semibold);
  color: var(--text-primary);
}

.last-price {
  text-align: right;
  font-family: monospace;
  color: var(--text-primary);
}

.change-amount, .change-percent {
  text-align: right;
  font-family: monospace;
  font-weight: var(--font-weight-medium);
}

.change-amount.positive, .change-percent.positive {
  color: var(--color-success);
}

.change-amount.negative, .change-percent.negative {
  color: var(--color-danger);
}

.change-amount.neutral, .change-percent.neutral {
  color: var(--text-secondary);
}

/* Legacy styles for backward compatibility */
.bid-price, .ask-price {
  text-align: right;
  font-family: monospace;
  color: var(--text-primary);
}

.net-change {
  text-align: right;
  font-family: monospace;
  font-weight: var(--font-weight-medium);
}

.net-change.positive {
  color: var(--color-success);
}

.net-change.negative {
  color: var(--color-danger);
}

.net-change.neutral {
  color: var(--text-secondary);
}

.remove-btn {
  background: none;
  border: none;
  color: var(--text-tertiary);
  cursor: pointer;
  font-size: 16px;
  line-height: 1;
  padding: 0;
  width: 20px;
  height: 20px;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: var(--radius-sm);
  transition: var(--transition-fast);
}

.remove-btn:hover {
  background-color: var(--color-danger);
  color: white;
}

/* Dialog Styles */
.dialog-overlay {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background-color: rgba(0, 0, 0, 0.5);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 1000;
}

.dialog {
  background-color: var(--bg-secondary);
  border-radius: var(--radius-md);
  box-shadow: var(--shadow-lg);
  min-width: 300px;
  max-width: 500px;
}

.dialog-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 16px 20px;
  border-bottom: 1px solid var(--border-secondary);
}

.dialog-header h4 {
  margin: 0;
  color: var(--text-primary);
  font-size: var(--font-size-md);
}

.close-btn {
  background: none;
  border: none;
  color: var(--text-secondary);
  cursor: pointer;
  font-size: 20px;
  padding: 0;
  width: 24px;
  height: 24px;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: var(--radius-sm);
}

.close-btn:hover {
  background-color: var(--bg-tertiary);
  color: var(--text-primary);
}

.dialog-body {
  padding: 20px;
}

.dialog-input {
  width: 100%;
  padding: 10px 12px;
  background-color: var(--bg-tertiary);
  border: 1px solid var(--border-secondary);
  border-radius: var(--radius-sm);
  color: var(--text-primary);
  font-size: var(--font-size-sm);
}

.dialog-input:focus {
  outline: none;
  border-color: var(--color-info);
}

.dialog-footer {
  display: flex;
  justify-content: flex-end;
  gap: 12px;
  padding: 16px 20px;
  border-top: 1px solid var(--border-secondary);
}

.cancel-btn, .confirm-btn {
  padding: 8px 16px;
  border: 1px solid var(--border-secondary);
  border-radius: var(--radius-sm);
  font-size: var(--font-size-sm);
  cursor: pointer;
  transition: var(--transition-fast);
}

.cancel-btn {
  background-color: var(--bg-tertiary);
  color: var(--text-primary);
}

.confirm-btn {
  background-color: var(--color-primary);
  color: white;
  border-color: var(--color-primary);
}

.cancel-btn:hover, .confirm-btn:hover {
  opacity: 0.8;
}

.confirm-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}
</style>
