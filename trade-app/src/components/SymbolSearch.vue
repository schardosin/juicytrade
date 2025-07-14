<template>
  <div class="symbol-search-container">
    <div class="search-input-wrapper">
      <input
        ref="searchInput"
        v-model="searchQuery"
        type="text"
        placeholder="Find a symbol or company"
        class="search-input"
        @input="handleInput"
        @focus="showDropdown = true"
        @blur="handleBlur"
        @keydown="handleKeydown"
      />
      <div class="search-icon">
        <svg width="16" height="16" viewBox="0 0 16 16" fill="none">
          <path
            d="M7.333 12.667A5.333 5.333 0 1 0 7.333 2a5.333 5.333 0 0 0 0 10.667ZM14 14l-2.9-2.9"
            stroke="currentColor"
            stroke-width="1.333"
            stroke-linecap="round"
            stroke-linejoin="round"
          />
        </svg>
      </div>
    </div>

    <!-- Search Results Dropdown -->
    <div
      v-if="
        showDropdown &&
        (searchResults.length > 0 || isLoading || searchQuery.length > 0)
      "
      class="search-dropdown"
    >
      <!-- Loading State -->
      <div v-if="isLoading" class="search-loading">
        <div class="loading-spinner"></div>
        <span>Searching...</span>
      </div>

      <!-- Recent Symbols (when no search query) -->
      <div
        v-else-if="searchQuery.length === 0 && recentSymbols.length > 0"
        class="recent-section"
      >
        <div class="section-header">Recent Symbols</div>
        <div
          v-for="symbol in recentSymbols"
          :key="symbol.symbol"
          class="search-result-item recent-item"
          @mousedown="selectSymbol(symbol)"
        >
          <div class="symbol-info">
            <div class="symbol-main">
              <span class="symbol-text">{{ symbol.symbol }}</span>
              <span class="symbol-type" :class="getTypeClass(symbol.type)">
                {{ getTypeLabel(symbol.type) }}
              </span>
            </div>
            <div class="symbol-description">{{ symbol.description }}</div>
          </div>
        </div>
      </div>

      <!-- Search Results -->
      <div v-else-if="searchResults.length > 0" class="results-section">
        <div
          v-for="(result, index) in searchResults"
          :key="result.symbol"
          class="search-result-item"
          :class="{ highlighted: index === highlightedIndex }"
          @mousedown="selectSymbol(result)"
          @mouseenter="highlightedIndex = index"
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
      <div v-else-if="searchQuery.length > 0" class="no-results">
        <div class="no-results-text">
          No symbols found for "{{ searchQuery }}"
        </div>
        <div class="no-results-suggestion">Try a different search term</div>
      </div>
    </div>
  </div>
</template>

<script>
import { ref, computed, onMounted, onUnmounted } from "vue";
import { api } from "../services/api.js";

export default {
  name: "SymbolSearch",
  emits: ["symbol-selected"],
  setup(props, { emit }) {
    const searchInput = ref(null);
    const searchQuery = ref("");
    const searchResults = ref([]);
    const showDropdown = ref(false);
    const isLoading = ref(false);
    const highlightedIndex = ref(-1);
    const recentSymbols = ref([]);

    let searchTimeout = null;

    // Load recent symbols from localStorage
    const loadRecentSymbols = () => {
      try {
        const stored = localStorage.getItem("recentSymbols");
        if (stored) {
          recentSymbols.value = JSON.parse(stored).slice(0, 5); // Keep only 5 most recent
        }
      } catch (error) {
        console.error("Error loading recent symbols:", error);
      }
    };

    // Save symbol to recent symbols
    const saveToRecentSymbols = (symbol) => {
      try {
        // Remove if already exists
        const filtered = recentSymbols.value.filter(
          (s) => s.symbol !== symbol.symbol
        );
        // Add to beginning
        const updated = [symbol, ...filtered].slice(0, 5);
        recentSymbols.value = updated;
        localStorage.setItem("recentSymbols", JSON.stringify(updated));
      } catch (error) {
        console.error("Error saving recent symbol:", error);
      }
    };

    // Handle input with debouncing
    const handleInput = () => {
      if (searchTimeout) {
        clearTimeout(searchTimeout);
      }

      if (searchQuery.value.length === 0) {
        searchResults.value = [];
        isLoading.value = false;
        return;
      }

      if (searchQuery.value.length < 1) {
        return;
      }

      isLoading.value = true;
      highlightedIndex.value = -1;

      searchTimeout = setTimeout(async () => {
        try {
          const results = await api.lookupSymbols(searchQuery.value);
          searchResults.value = results || [];
        } catch (error) {
          console.error("Error searching symbols:", error);
          searchResults.value = [];
        } finally {
          isLoading.value = false;
        }
      }, 300); // 300ms debounce
    };

    // Handle blur with delay to allow click events
    const handleBlur = () => {
      setTimeout(() => {
        showDropdown.value = false;
        highlightedIndex.value = -1;
      }, 150);
    };

    // Handle keyboard navigation
    const handleKeydown = (event) => {
      if (!showDropdown.value) return;

      const results =
        searchQuery.value.length === 0
          ? recentSymbols.value
          : searchResults.value;

      switch (event.key) {
        case "ArrowDown":
          event.preventDefault();
          highlightedIndex.value = Math.min(
            highlightedIndex.value + 1,
            results.length - 1
          );
          break;
        case "ArrowUp":
          event.preventDefault();
          highlightedIndex.value = Math.max(highlightedIndex.value - 1, -1);
          break;
        case "Enter":
          event.preventDefault();
          if (highlightedIndex.value >= 0 && results[highlightedIndex.value]) {
            selectSymbol(results[highlightedIndex.value]);
          }
          break;
        case "Escape":
          showDropdown.value = false;
          searchInput.value?.blur();
          break;
      }
    };

    // Select a symbol
    const selectSymbol = (symbol) => {
      searchQuery.value = symbol.symbol;
      showDropdown.value = false;
      saveToRecentSymbols(symbol);
      emit("symbol-selected", symbol);
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

    // Focus the input
    const focus = () => {
      searchInput.value?.focus();
    };

    onMounted(() => {
      loadRecentSymbols();
    });

    onUnmounted(() => {
      if (searchTimeout) {
        clearTimeout(searchTimeout);
      }
    });

    return {
      searchInput,
      searchQuery,
      searchResults,
      showDropdown,
      isLoading,
      highlightedIndex,
      recentSymbols,
      handleInput,
      handleBlur,
      handleKeydown,
      selectSymbol,
      getTypeClass,
      getTypeLabel,
      focus,
    };
  },
};
</script>

<style scoped>
.symbol-search-container {
  position: relative;
  width: 100%;
}

.search-input-wrapper {
  position: relative;
  display: flex;
  align-items: center;
}

.search-input {
  width: 100%;
  height: 32px;
  padding: 0 36px 0 12px;
  background: #141519 !important;
  border: 1px solid #333;
  border-radius: 4px;
  color: #fff;
  font-size: 14px;
  outline: none;
  transition: border-color 0.2s;
}

.search-input:focus {
  border-color: #0066cc;
}

.search-input::placeholder {
  color: #666;
}

.search-icon {
  position: absolute;
  right: 10px;
  color: #666;
  pointer-events: none;
}

.search-dropdown {
  position: absolute;
  top: 100%;
  left: 0;
  right: 0;
  background: #1a1a1a;
  border: 1px solid #333;
  border-top: none;
  border-radius: 0 0 4px 4px;
  max-height: 300px;
  overflow-y: auto;
  z-index: 1000;
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.3);
}

.search-loading {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 12px;
  color: #666;
  font-size: 14px;
}

.loading-spinner {
  width: 16px;
  height: 16px;
  border: 2px solid #333;
  border-top: 2px solid #0066cc;
  border-radius: 50%;
  animation: spin 1s linear infinite;
}

@keyframes spin {
  0% {
    transform: rotate(0deg);
  }
  100% {
    transform: rotate(360deg);
  }
}

.section-header {
  padding: 8px 12px;
  font-size: 12px;
  font-weight: 600;
  color: #666;
  text-transform: uppercase;
  letter-spacing: 0.5px;
  border-bottom: 1px solid #333;
}

.search-result-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 8px 12px;
  cursor: pointer;
  transition: background-color 0.2s;
  border-bottom: 1px solid #2a2a2a;
}

.search-result-item:hover,
.search-result-item.highlighted {
  background: #2a2a2a;
}

.search-result-item:last-child {
  border-bottom: none;
}

.symbol-info {
  flex: 1;
  min-width: 0;
}

.symbol-main {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 2px;
}

.symbol-text {
  font-weight: 600;
  color: #fff;
  font-size: 14px;
}

.symbol-type {
  padding: 2px 6px;
  border-radius: 3px;
  font-size: 10px;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.5px;
}

.type-stock {
  background: #0066cc;
  color: #fff;
}

.type-etf {
  background: #00aa44;
  color: #fff;
}

.type-index {
  background: #ff6600;
  color: #fff;
}

.type-option {
  background: #9966cc;
  color: #fff;
}

.type-default {
  background: #666;
  color: #fff;
}

.symbol-description {
  color: #999;
  font-size: 12px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.symbol-exchange {
  color: #666;
  font-size: 11px;
  font-weight: 500;
  margin-left: 12px;
}

.no-results {
  padding: 16px 12px;
  text-align: center;
}

.no-results-text {
  color: #999;
  font-size: 14px;
  margin-bottom: 4px;
}

.no-results-suggestion {
  color: #666;
  font-size: 12px;
}

.recent-item {
  opacity: 0.8;
}

.recent-item:hover {
  opacity: 1;
}
</style>
