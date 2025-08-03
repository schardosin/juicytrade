import { computed } from 'vue';
import { selectedLegsStore } from '../services/selectedLegsStore.js';

/**
 * Composable for managing selected option legs
 * Provides reactive access to the centralized selected legs store
 * 
 * @returns {Object} Object containing reactive state and methods
 */
export function useSelectedLegs() {
  return {
    // Reactive state
    selectedLegs: computed(() => selectedLegsStore.getAllLegs()),
    hasSelectedLegs: computed(() => selectedLegsStore.hasLegs()),
    metadata: selectedLegsStore.metadata,
    
    // Computed helpers
    totalLegs: computed(() => selectedLegsStore.metadata.value.totalLegs),
    totalQuantity: computed(() => selectedLegsStore.metadata.value.totalQuantity),
    netPremium: computed(() => selectedLegsStore.metadata.value.netPremium),
    hasCredit: computed(() => selectedLegsStore.metadata.value.hasCredit),
    hasDebit: computed(() => selectedLegsStore.metadata.value.hasDebit),
    sources: computed(() => selectedLegsStore.metadata.value.sources),
    
    // Core actions
    addLeg: (legData, source = 'options_chain') => {
      return selectedLegsStore.addLeg(legData, source);
    },
    
    removeLeg: (symbol) => {
      return selectedLegsStore.removeLeg(symbol);
    },
    
    updateQuantity: (symbol, quantity) => {
      return selectedLegsStore.updateLegQuantity(symbol, quantity);
    },
    
    clearAll: () => {
      selectedLegsStore.clearAllLegs();
    },
    
    clearBySource: (source) => {
      selectedLegsStore.clearLegsBySource(source);
    },
    
    // Utility methods
    isSelected: (symbol) => {
      return selectedLegsStore.isLegSelected(symbol);
    },
    
    getLeg: (symbol) => {
      return selectedLegsStore.getLeg(symbol);
    },
    
    getLegsFromSource: (source) => {
      return selectedLegsStore.getLegsFromSource(source);
    },
    
    getSummary: () => {
      return selectedLegsStore.getSummary();
    },
    
    // Convenience methods for specific sources
    addFromOptionsChain: (optionData, side = 'buy') => {
      return selectedLegsStore.addLeg({
        ...optionData,
        side: side,
        quantity: 1
      }, 'options_chain');
    },
    
    addFromPosition: (positionData) => {
      // For closing trades, use current market price as entry price
      // This treats the closing trade as a new trade at current market prices
      const currentMarketPrice = positionData.current_price || 0;
      
      return selectedLegsStore.addLeg({
        symbol: positionData.symbol,
        side: positionData.qty > 0 ? 'sell' : 'buy', // Opposite for closing
        quantity: Math.abs(positionData.qty),
        strike_price: positionData.strike_price,
        type: positionData.option_type,
        expiry: positionData.expiry_date,
        current_price: currentMarketPrice,
        bid: positionData.bid || 0,
        ask: positionData.ask || 0,
        // 🔑 KEY FIX: Use current market price as entry price for chart calculation
        avg_entry_price: currentMarketPrice,
        // Keep original data for reference
        original_quantity: Math.abs(positionData.qty),
        original_avg_entry_price: positionData.avg_entry_price,
        unrealized_pl: positionData.unrealized_pl
      }, 'positions');
    },
    
    // Batch operations
    addMultipleLegs: (legsArray, source = 'options_chain') => {
      const results = legsArray.map(legData => 
        selectedLegsStore.addLeg(legData, source)
      );
      return results.every(result => result === true);
    },
    
    // Selection state helpers
    getSelectionClass: (symbol) => {
      const leg = selectedLegsStore.getLeg(symbol);
      if (!leg) return '';
      
      return {
        'selected-buy': leg.side === 'buy',
        'selected-sell': leg.side === 'sell',
        'from-position': leg.source === 'positions',
        'from-options': leg.source === 'options_chain'
      };
    },
    
    // Quantity validation helpers
    canIncrementQuantity: (symbol) => {
      const leg = selectedLegsStore.getLeg(symbol);
      if (!leg) return false;
      
      if (leg.source === 'positions' && leg.original_quantity) {
        return leg.quantity < leg.original_quantity;
      }
      
      return leg.quantity < 100; // Max for options chain
    },
    
    canDecrementQuantity: (symbol) => {
      const leg = selectedLegsStore.getLeg(symbol);
      return leg && leg.quantity > 1;
    },
    
    getMaxQuantity: (symbol) => {
      const leg = selectedLegsStore.getLeg(symbol);
      if (!leg) return 1;
      
      if (leg.source === 'positions' && leg.original_quantity) {
        return leg.original_quantity;
      }
      
      return 100; // Max for options chain
    }
  };
}
