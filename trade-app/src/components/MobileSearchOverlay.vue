<template>
  <!-- Mobile Search Overlay -->
  <div
    v-if="visible"
    class="mobile-search-overlay"
    :class="{ 'overlay-open': visible }"
  >
    <!-- Header -->
    <div class="search-header">
      <button
        class="back-button"
        @click="$emit('close')"
        aria-label="Close search"
      >
        <i class="pi pi-arrow-left"></i>
      </button>
      <div class="search-title">Search Symbols</div>
    </div>

    <!-- Search Input -->
    <div class="search-input-section">
      <div class="search-input-container">
        <i class="pi pi-search search-icon"></i>
        <input
          ref="searchInput"
          v-model="searchQuery"
          type="text"
          placeholder="Enter symbol or company name"
          class="mobile-search-input"
          @input="handleInput"
          @keyup.enter="performSearch"
          autocomplete="off"
          autocapitalize="none"
          spellcheck="false"
        />
        <button
          v-if="searchQuery"
          class="clear-button"
          @click="clearSearch"
          aria-label="Clear search"
        >
          <i class="pi pi-times"></i>
        </button>
      </div>
    </div>

    <!-- Search Results -->
    <div class="search-results-section">
      <!-- Loading State -->
      <div v-if="searchLoading" class="search-loading">
        <div class="loading-spinner"></div>
        <span>Searching...</span>
      </div>

      <!-- Search Results -->
      <div v-else-if="searchResults.length > 0" class="results-list">
        <div
          v-for="(result, index) in searchResults"
          :key="result.symbol"
          class="result-item"
          @click="selectSymbol(result)"
        >
          <div class="symbol-info">
            <div class="symbol-main">
              <span class="symbol-text">{{ result.symbol }}</span>
              <span class="symbol-type" :class="getTypeClass(result.type)">
                {{ getTypeLabel(result.type) }}
              </span>
            </div>
            <div class="symbol-description">{{ result.description }}</div>
          </div>
          <div class="symbol-exchange">{{ result.exchange }}</div>
        </div>
      </div>

      <!-- No Results -->
      <div v-else-if="searchQuery.length > 0 && !searchLoading" class="no-results">
        <div class="no-results-icon">
          <i class="pi pi-search"></i>
        </div>
        <div class="no-results-text">
          No symbols found for "{{ searchQuery }}"
        </div>
        <div class="no-results-suggestion">Try a different search term</div>
      </div>

      <!-- Initial State -->
      <div v-else class="search-placeholder">
        <div class="placeholder-icon">
          <i class="pi pi-search"></i>
        </div>
        <div class="placeholder-text">Search for stocks, ETFs, and options</div>
        <div class="placeholder-examples">
          <div class="example-title">Popular symbols:</div>
          <div class="example-symbols">
            <button
              v-for="example in exampleSymbols"
              :key="example"
              class="example-symbol"
              @click="searchExample(example)"
            >
              {{ example }}
            </button>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script>
import { ref, watch, nextTick, onMounted, onUnmounted } from "vue";
import { useMarketData } from "../composables/useMarketData.js";

export default {
  name: "MobileSearchOverlay",
  props: {
    visible: {
      type: Boolean,
      default: false,
    },
  },
  emits: ["close", "symbol-selected"],
  setup(props, { emit }) {
    const { lookupSymbols } = useMarketData();

    // Reactive data
    const searchInput = ref(null);
    const searchQuery = ref("");
    const searchResults = ref([]);
    const searchLoading = ref(false);

    let searchTimeout = null;

    // Example symbols for quick access
    const exampleSymbols = ["AAPL", "TSLA", "MSFT", "GOOGL", "AMZN", "NVDA"];

    // Methods
    const handleInput = () => {
      // Clear any existing timeout
      if (searchTimeout) {
        clearTimeout(searchTimeout);
      }

      if (searchQuery.value.length === 0) {
        searchResults.value = [];
        searchLoading.value = false;
        return;
      }

      if (searchQuery.value.length < 1) {
        return;
      }

      // Show loading state
      searchLoading.value = true;

      searchTimeout = setTimeout(async () => {
        try {
          const results = await lookupSymbols(searchQuery.value);
          searchResults.value = results || [];
        } catch (error) {
          console.error("Error searching symbols:", error);
          searchResults.value = [];
        } finally {
          searchLoading.value = false;
        }
      }, 300); // 300ms debounce
    };

    const performSearch = async () => {
      if (searchQuery.value.trim()) {
        try {
          const results = await lookupSymbols(searchQuery.value);
          if (results && results.length > 0) {
            selectSymbol(results[0]);
          }
        } catch (error) {
          console.error("Error searching for symbol:", error);
        }
      }
    };

    const selectSymbol = (symbol) => {
      // Clear search state
      searchResults.value = [];
      searchLoading.value = false;
      searchQuery.value = "";
      
      // Clear any pending search timeout
      if (searchTimeout) {
        clearTimeout(searchTimeout);
        searchTimeout = null;
      }

      // Emit symbol selection event
      emit("symbol-selected", symbol);
      
      // Close overlay
      emit("close");
    };

    const clearSearch = () => {
      searchQuery.value = "";
      searchResults.value = [];
      searchLoading.value = false;
      
      if (searchTimeout) {
        clearTimeout(searchTimeout);
        searchTimeout = null;
      }
      
      // Focus input after clearing
      nextTick(() => {
        if (searchInput.value) {
          searchInput.value.focus();
        }
      });
    };

    const searchExample = (symbol) => {
      searchQuery.value = symbol;
      handleInput();
    };

    // Get type class for styling
    const getTypeClass = (type) => {
      switch (type?.toLowerCase()) {
        case "stock":
          return "type-stock";
        case "etf":
          return "type-etf";
        case "index":
          return "type-index";
        case "option":
          return "type-option";
        default:
          return "type-default";
      }
    };

    // Get type label
    const getTypeLabel = (type) => {
      switch (type?.toLowerCase()) {
        case "stock":
          return "Stock";
        case "etf":
          return "ETF";
        case "index":
          return "Index";
        case "option":
          return "Option";
        default:
          return type || "Security";
      }
    };

    // Watch for visibility changes
    watch(
      () => props.visible,
      (isVisible) => {
        if (isVisible) {
          // Focus input when overlay opens
          nextTick(() => {
            if (searchInput.value) {
              searchInput.value.focus();
            }
          });
          
          // Prevent body scroll
          document.body.style.overflow = "hidden";
        } else {
          // Clear search when closing
          searchQuery.value = "";
          searchResults.value = [];
          searchLoading.value = false;
          
          // Restore body scroll
          document.body.style.overflow = "";
          
          // Clear timeout
          if (searchTimeout) {
            clearTimeout(searchTimeout);
            searchTimeout = null;
          }
        }
      }
    );

    // Cleanup on unmount
    onUnmounted(() => {
      if (searchTimeout) {
        clearTimeout(searchTimeout);
      }
      document.body.style.overflow = "";
    });

    return {
      searchInput,
      searchQuery,
      searchResults,
      searchLoading,
      exampleSymbols,
      handleInput,
      performSearch,
      selectSymbol,
      clearSearch,
      searchExample,
      getTypeClass,
      getTypeLabel,
    };
  },
};
</script>

<style scoped>
.mobile-search-overlay {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background-color: var(--bg-primary, #0b0d10);
  z-index: 1001;
  display: flex;
  flex-direction: column;
  transform: translateY(100%);
  transition: transform 0.3s ease-in-out;
}

.mobile-search-overlay.overlay-open {
  transform: translateY(0);
}

.search-header {
  display: flex;
  align-items: center;
  padding: 16px 20px;
  border-bottom: 1px solid var(--border-primary, #1a1d23);
  background-color: var(--bg-secondary, #141519);
  flex-shrink: 0;
}

.back-button {
  background: none;
  border: none;
  color: var(--text-primary, #ffffff);
  font-size: 20px;
  cursor: pointer;
  padding: 8px;
  border-radius: 4px;
  transition: all 0.2s ease;
  display: flex;
  align-items: center;
  justify-content: center;
  width: 40px;
  height: 40px;
  margin-right: 16px;
}

.back-button:hover {
  background-color: var(--bg-tertiary, #1a1d23);
}

.search-title {
  font-size: 18px;
  font-weight: 600;
  color: var(--text-primary, #ffffff);
}

.search-input-section {
  padding: 20px;
  flex-shrink: 0;
}

.search-input-container {
  position: relative;
  display: flex;
  align-items: center;
}

.search-icon {
  position: absolute;
  left: 16px;
  color: var(--text-tertiary, #888888);
  font-size: 18px;
  z-index: 1;
}

.mobile-search-input {
  width: 100%;
  padding: 16px 16px 16px 48px;
  background-color: var(--bg-secondary, #141519);
  border: 2px solid var(--border-secondary, #2a2d33);
  border-radius: 12px;
  color: var(--text-primary, #ffffff);
  font-size: 16px;
  outline: none;
  transition: border-color 0.2s ease;
}

.mobile-search-input:focus {
  border-color: var(--color-info, #007bff);
}

.mobile-search-input::placeholder {
  color: var(--text-tertiary, #888888);
}

.clear-button {
  position: absolute;
  right: 12px;
  background: none;
  border: none;
  color: var(--text-tertiary, #888888);
  font-size: 16px;
  cursor: pointer;
  padding: 8px;
  border-radius: 4px;
  transition: all 0.2s ease;
  display: flex;
  align-items: center;
  justify-content: center;
  width: 32px;
  height: 32px;
}

.clear-button:hover {
  background-color: var(--bg-tertiary, #1a1d23);
  color: var(--text-primary, #ffffff);
}

.search-results-section {
  flex: 1;
  overflow-y: auto;
  padding: 0 20px 20px;
}

.search-loading {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 12px;
  padding: 40px 20px;
  color: var(--text-tertiary, #888888);
  font-size: 16px;
}

.loading-spinner {
  width: 20px;
  height: 20px;
  border: 2px solid var(--border-secondary, #2a2d33);
  border-top: 2px solid var(--color-info, #007bff);
  border-radius: 50%;
  animation: spin 1s linear infinite;
}

@keyframes spin {
  0% { transform: rotate(0deg); }
  100% { transform: rotate(360deg); }
}

.results-list {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.result-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 16px;
  background-color: var(--bg-secondary, #141519);
  border-radius: 8px;
  cursor: pointer;
  transition: all 0.2s ease;
  min-height: 60px;
}

.result-item:hover {
  background-color: var(--bg-tertiary, #1a1d23);
}

.result-item:active {
  transform: scale(0.98);
}

.symbol-info {
  flex: 1;
  min-width: 0;
}

.symbol-main {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 4px;
}

.symbol-text {
  font-weight: 600;
  color: var(--text-primary, #ffffff);
  font-size: 16px;
}

.symbol-type {
  padding: 2px 6px;
  border-radius: 4px;
  font-size: 10px;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.5px;
}

.type-stock {
  background: var(--color-info, #007bff);
  color: white;
}

.type-etf {
  background: var(--color-success, #00c851);
  color: white;
}

.type-index {
  background: var(--color-brand, #007bff);
  color: white;
}

.type-option {
  background: var(--color-primary, #007bff);
  color: white;
}

.type-default {
  background: var(--text-tertiary, #888888);
  color: white;
}

.symbol-description {
  color: var(--text-secondary, #cccccc);
  font-size: 14px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.symbol-exchange {
  color: var(--text-tertiary, #888888);
  font-size: 12px;
  font-weight: 500;
  margin-left: 12px;
}

.no-results {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 60px 20px;
  text-align: center;
}

.no-results-icon {
  font-size: 48px;
  color: var(--text-tertiary, #888888);
  margin-bottom: 16px;
}

.no-results-text {
  color: var(--text-secondary, #cccccc);
  font-size: 16px;
  margin-bottom: 8px;
}

.no-results-suggestion {
  color: var(--text-tertiary, #888888);
  font-size: 14px;
}

.search-placeholder {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 60px 20px;
  text-align: center;
}

.placeholder-icon {
  font-size: 64px;
  color: var(--text-tertiary, #888888);
  margin-bottom: 24px;
  opacity: 0.5;
}

.placeholder-text {
  color: var(--text-secondary, #cccccc);
  font-size: 18px;
  margin-bottom: 32px;
}

.placeholder-examples {
  width: 100%;
  max-width: 300px;
}

.example-title {
  color: var(--text-tertiary, #888888);
  font-size: 14px;
  margin-bottom: 12px;
}

.example-symbols {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  justify-content: center;
}

.example-symbol {
  background: var(--bg-secondary, #141519);
  border: 1px solid var(--border-secondary, #2a2d33);
  color: var(--text-primary, #ffffff);
  padding: 8px 16px;
  border-radius: 20px;
  font-size: 14px;
  font-weight: 500;
  cursor: pointer;
  transition: all 0.2s ease;
}

.example-symbol:hover {
  background-color: var(--bg-tertiary, #1a1d23);
  border-color: var(--color-info, #007bff);
}

.example-symbol:active {
  transform: scale(0.95);
}

/* Scrollbar styling */
.search-results-section::-webkit-scrollbar {
  width: 4px;
}

.search-results-section::-webkit-scrollbar-track {
  background: transparent;
}

.search-results-section::-webkit-scrollbar-thumb {
  background: var(--border-secondary, #2a2d33);
  border-radius: 2px;
}

.search-results-section::-webkit-scrollbar-thumb:hover {
  background: var(--border-tertiary, #3a3d43);
}

/* Animation for smooth opening */
@media (prefers-reduced-motion: reduce) {
  .mobile-search-overlay {
    transition: none;
  }
}
</style>
