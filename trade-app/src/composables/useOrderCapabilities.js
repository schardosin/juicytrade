import { ref, computed, watch, onMounted } from "vue";
import api from "../services/api.js";
import { useMarketData } from "./useMarketData.js";

// Module-level cache for provider types (shared across all component instances)
let providerTypesCache = null;
let providerTypesFetchPromise = null;

/**
 * Composable for accessing order capabilities based on the selected trade account provider.
 * 
 * @param {Ref<string>} assetClass - "equity" or "options" (reactive ref)
 * @returns {Object} Order capabilities for the specified asset class
 */
export function useOrderCapabilities(assetClass) {
  const { getProviderConfig } = useMarketData();
  
  // Local refs
  const providerTypes = ref(null);
  const isLoading = ref(false);
  const error = ref(null);

  // Get reactive provider config
  const providerConfig = getProviderConfig();

  // Default capabilities (fallback when provider data not loaded)
  const defaultCapabilities = {
    enabled: true,
    order_types: [
      { value: "limit", label: "Limit", requires_price: true, requires_stop_price: false },
      { value: "market", label: "Market", requires_price: false, requires_stop_price: false },
    ],
    time_in_force: [
      { value: "day", label: "Day", requires_date: false },
      { value: "gtc", label: "GTC", requires_date: false },
    ],
    supports_multi_leg: true,
  };

  /**
   * Fetch provider types from API (with caching)
   */
  const fetchProviderTypes = async () => {
    // Return cached data if available
    if (providerTypesCache) {
      providerTypes.value = providerTypesCache;
      return;
    }

    // If a fetch is already in progress, wait for it
    if (providerTypesFetchPromise) {
      try {
        providerTypes.value = await providerTypesFetchPromise;
        return;
      } catch (e) {
        // If the existing promise failed, we'll try again below
      }
    }

    isLoading.value = true;
    error.value = null;

    providerTypesFetchPromise = api.getProviderTypes();

    try {
      const data = await providerTypesFetchPromise;
      providerTypesCache = data;
      providerTypes.value = data;
    } catch (e) {
      console.error("Failed to fetch provider types:", e);
      error.value = e;
    } finally {
      isLoading.value = false;
      providerTypesFetchPromise = null;
    }
  };

  // Fetch on mount
  onMounted(() => {
    fetchProviderTypes();
  });

  // Get the current trade_account instance ID from config
  const tradeAccountInstanceId = computed(() => {
    return providerConfig.value?.trade_account || null;
  });

  // Get the provider type (e.g., "tastytrade", "tradier") from the instance
  // This requires looking up the instance to find its provider_type
  const tradeAccountProviderType = computed(() => {
    const instanceId = tradeAccountInstanceId.value;
    if (!instanceId) return null;

    // The instance ID format is typically: {provider_type}_{account_type}_{number}
    // e.g., "tastytrade_live_1" -> provider_type is "tastytrade"
    // We extract the provider type from the instance ID
    const parts = instanceId.split("_");
    if (parts.length >= 2) {
      // Handle provider names that might have underscores (none currently, but future-proof)
      // Known providers: alpaca, tradier, tastytrade, public
      const knownProviders = ["alpaca", "tradier", "tastytrade", "public"];
      for (const provider of knownProviders) {
        if (instanceId.startsWith(provider + "_")) {
          return provider;
        }
      }
    }
    return parts[0] || null;
  });

  // Get order capabilities for the current asset class
  const capabilities = computed(() => {
    const providerType = tradeAccountProviderType.value;
    const types = providerTypes.value;
    const assetClassValue = assetClass?.value || "options";

    if (!providerType || !types || !types[providerType]) {
      return defaultCapabilities;
    }

    const orderCaps = types[providerType].order_capabilities;
    if (!orderCaps) {
      return defaultCapabilities;
    }

    const assetCaps = assetClassValue === "equity" ? orderCaps.equity : orderCaps.options;
    if (!assetCaps) {
      return defaultCapabilities;
    }

    return assetCaps;
  });

  // Exposed computed properties
  const isEnabled = computed(() => capabilities.value?.enabled ?? true);
  const orderTypes = computed(() => capabilities.value?.order_types || defaultCapabilities.order_types);
  const timeInForceOptions = computed(() => capabilities.value?.time_in_force || defaultCapabilities.time_in_force);
  const supportsMultiLeg = computed(() => capabilities.value?.supports_multi_leg ?? true);

  /**
   * Check if a specific order type requires a limit price
   */
  const requiresPrice = (orderTypeValue) => {
    const type = orderTypes.value.find((t) => t.value === orderTypeValue);
    return type?.requires_price || false;
  };

  /**
   * Check if a specific order type requires a stop price
   */
  const requiresStopPrice = (orderTypeValue) => {
    const type = orderTypes.value.find((t) => t.value === orderTypeValue);
    return type?.requires_stop_price || false;
  };

  /**
   * Check if a specific time-in-force option requires a date
   */
  const requiresDate = (tifValue) => {
    const tif = timeInForceOptions.value.find((t) => t.value === tifValue);
    return tif?.requires_date || false;
  };

  /**
   * Force refresh provider types from API
   */
  const refresh = async () => {
    providerTypesCache = null;
    await fetchProviderTypes();
  };

  return {
    // Capabilities
    isEnabled,
    orderTypes,
    timeInForceOptions,
    supportsMultiLeg,
    
    // Helper functions
    requiresPrice,
    requiresStopPrice,
    requiresDate,
    
    // State
    isLoading,
    error,
    tradeAccountProviderType,
    
    // Actions
    refresh,
  };
}

export default useOrderCapabilities;
