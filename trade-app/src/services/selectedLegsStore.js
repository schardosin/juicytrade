import { reactive, computed } from 'vue';

/**
 * Centralized store for managing selected option legs across all components
 * Supports legs from different sources: options_chain, positions, strategies
 */
class SelectedLegsStore {
  constructor() {
    // Core reactive state - Map of symbol -> leg data
    this.legs = reactive(new Map());
    
    // Computed metadata for easy access
    this.metadata = computed(() => {
      const legsArray = Array.from(this.legs.values());
      
      return {
        totalLegs: legsArray.length,
        totalQuantity: legsArray.reduce((sum, leg) => sum + leg.quantity, 0),
        netPremium: this.calculateNetPremium(legsArray),
        hasCredit: legsArray.some(leg => leg.side === 'sell'),
        hasDebit: legsArray.some(leg => leg.side === 'buy'),
        sources: [...new Set(legsArray.map(leg => leg.source))],
        bySource: this.groupLegsBySource(legsArray)
      };
    });
  }

  /**
   * Add a new leg to the selection
   * @param {Object} legData - The leg data
   * @param {string} source - Source of the leg (options_chain, positions, strategy)
   */
  addLeg(legData, source = 'options_chain') {
    if (!legData || !legData.symbol) {
      console.warn('Invalid leg data provided to addLeg:', legData);
      return false;
    }

    // Validate required fields
    if (!this.validateLeg(legData)) {
      console.warn('Leg validation failed:', legData);
      return false;
    }

    // Set default quantity based on source
    const quantity = legData.quantity || (source === 'positions' ? 
      Math.abs(legData.original_quantity || 1) : 1);

    // Create complete leg object
    const leg = {
      // Core identification
      symbol: legData.symbol,
      
      // Trading details
      side: legData.side || 'buy',
      quantity: Math.max(1, quantity),
      
      // Option details
      strike_price: legData.strike_price || legData.strike || 0,
      type: legData.type || legData.option_type || 'call',
      expiry: legData.expiry || legData.expiry_date || legData.expiration_date,
      
      // Pricing information - calculate current_price from bid/ask if not provided
      current_price: legData.current_price || (legData.bid && legData.ask ? (legData.bid + legData.ask) / 2 : 0),
      bid: legData.bid || 0,
      ask: legData.ask || 0,
      
      // Source tracking
      source: source,
      
      // Position-specific data (when source is "positions")
      original_quantity: legData.original_quantity || null,
      avg_entry_price: legData.avg_entry_price || null,
      unrealized_pl: legData.unrealized_pl || null,
      
      // Metadata
      timestamp: Date.now(),
      isSelected: true
    };

    // Store the leg
    this.legs.set(legData.symbol, leg);

    return true;
  }

  /**
   * Remove a leg from the selection
   * @param {string} symbol - The option symbol to remove
   */
  removeLeg(symbol) {
    if (this.legs.has(symbol)) {
      const leg = this.legs.get(symbol);
      this.legs.delete(symbol);
      
      console.log(`🗑️ Removed leg:`, {
        symbol: symbol,
        source: leg.source
      });
      
      return true;
    }
    return false;
  }

  /**
   * Update the quantity of a specific leg
   * @param {string} symbol - The option symbol
   * @param {number} newQuantity - The new quantity
   */
  updateLegQuantity(symbol, newQuantity) {
    const leg = this.legs.get(symbol);
    if (!leg) {
      console.warn(`Leg not found for quantity update: ${symbol}`);
      return false;
    }

    // Validate quantity based on source
    let validatedQuantity = Math.max(1, newQuantity);
    
    if (leg.source === 'positions' && leg.original_quantity) {
      // For positions, can't exceed original quantity
      validatedQuantity = Math.min(validatedQuantity, leg.original_quantity);
    } else {
      // For options chain, reasonable maximum
      validatedQuantity = Math.min(validatedQuantity, 100);
    }

    // Update the leg
    const updatedLeg = { ...leg, quantity: validatedQuantity };
    this.legs.set(symbol, updatedLeg);
    
    console.log(`📊 Updated leg quantity:`, {
      symbol: symbol,
      oldQuantity: leg.quantity,
      newQuantity: validatedQuantity,
      source: leg.source
    });

    return true;
  }

  /**
   * Replace a leg with a new one, preserving quantity and side.
   * @param {string} oldSymbol - The symbol of the leg to replace.
   * @param {Object} newLegData - The data for the new leg.
   */
  replaceLeg(oldSymbol, newLegData) {
    const oldLeg = this.legs.get(oldSymbol);
    if (!oldLeg) {
      console.warn(`Leg not found for replacement: ${oldSymbol}`);
      return false;
    }

    if (!newLegData || !newLegData.symbol) {
      console.warn('Invalid new leg data provided to replaceLeg:', newLegData);
      return false;
    }

    // Preserve quantity and side from the old leg
    const preservedData = {
      ...newLegData,
      quantity: oldLeg.quantity,
      side: oldLeg.side,
      source: oldLeg.source, // Preserve the original source
    };

    // Remove the old leg
    this.removeLeg(oldSymbol);

    // Add the new leg
    this.addLeg(preservedData, preservedData.source);
    
    return true;
  }

  /**
   * Clear all selected legs
   */
  clearAllLegs() {
    const count = this.legs.size;
    this.legs.clear();
  }

  /**
   * Clear legs from a specific source
   * @param {string} source - The source to clear (options_chain, positions, etc.)
   */
  clearLegsBySource(source) {
    const toRemove = [];
    
    this.legs.forEach((leg, symbol) => {
      if (leg.source === source) {
        toRemove.push(symbol);
      }
    });

    toRemove.forEach(symbol => {
      this.legs.delete(symbol);
    });

    console.log(`🧹 Cleared ${toRemove.length} legs from source: ${source}`);
  }

  /**
   * Get all legs as an array
   * @returns {Array} Array of leg objects
   */
  getAllLegs() {
    return Array.from(this.legs.values());
  }

  /**
   * Check if any legs are selected
   * @returns {boolean}
   */
  hasLegs() {
    return this.legs.size > 0;
  }

  /**
   * Check if a specific leg is selected
   * @param {string} symbol - The option symbol
   * @returns {boolean}
   */
  isLegSelected(symbol) {
    return this.legs.has(symbol);
  }

  /**
   * Get a specific leg by symbol
   * @param {string} symbol - The option symbol
   * @returns {Object|null} The leg object or null
   */
  getLeg(symbol) {
    return this.legs.get(symbol) || null;
  }

  /**
   * Get legs from a specific source
   * @param {string} source - The source to filter by
   * @returns {Array} Array of legs from the specified source
   */
  getLegsFromSource(source) {
    return this.getAllLegs().filter(leg => leg.source === source);
  }

  /**
   * Validate leg data before adding
   * @param {Object} legData - The leg data to validate
   * @returns {boolean} True if valid
   */
  validateLeg(legData) {
    // Required fields
    if (!legData.symbol) return false;
    
    // Valid side
    if (legData.side && !['buy', 'sell'].includes(legData.side)) return false;
    
    // Valid type - handle both full names and single letters, both cases
    if (legData.type && !['call', 'put', 'C', 'P', 'c', 'p'].includes(legData.type)) return false;
    
    return true;
  }

  /**
   * Calculate net premium for all legs
   * @param {Array} legsArray - Array of leg objects
   * @returns {number} Net premium (positive for credit, negative for debit)
   */
  calculateNetPremium(legsArray) {
    return legsArray.reduce((total, leg) => {
      const price = leg.current_price || ((leg.bid + leg.ask) / 2) || 0;
      const premium = leg.side === 'buy' ? -price : price;
      return total + (premium * leg.quantity);
    }, 0);
  }

  /**
   * Group legs by source
   * @param {Array} legsArray - Array of leg objects
   * @returns {Object} Object with source as key and legs array as value
   */
  groupLegsBySource(legsArray) {
    return legsArray.reduce((groups, leg) => {
      if (!groups[leg.source]) {
        groups[leg.source] = [];
      }
      groups[leg.source].push(leg);
      return groups;
    }, {});
  }

  /**
   * Get summary statistics
   * @returns {Object} Summary statistics
   */
  getSummary() {
    const legs = this.getAllLegs();
    const meta = this.metadata.value;
    
    return {
      totalLegs: meta.totalLegs,
      totalQuantity: meta.totalQuantity,
      netPremium: meta.netPremium,
      isCredit: meta.netPremium > 0,
      isDebit: meta.netPremium < 0,
      sources: meta.sources,
      breakdown: {
        options_chain: this.getLegsFromSource('options_chain').length,
        positions: this.getLegsFromSource('positions').length,
        strategies: this.getLegsFromSource('strategies').length
      }
    };
  }
}

// Create and export the global store instance
export const selectedLegsStore = new SelectedLegsStore();

// Export the class for testing purposes
export { SelectedLegsStore };
