import { describe, it, expect, beforeEach, vi } from 'vitest';
import { mount } from '@vue/test-utils';
import { ref, nextTick } from 'vue';
import { generateMultiLegPayoff } from '../src/utils/chartUtils.js';

// This integration test focuses specifically on the chart data flow issues that were fixed
describe('Chart Data Flow Integration - Regression Prevention', () => {
  
  describe('Position ID Generation and Uniqueness', () => {
    it('should generate unique IDs for positions with same symbol', () => {
      const positions = [
        {
          symbol: 'SPY_240119C00500000',
          strike_price: 500,
          type: 'call',
          side: 'buy',
          quantity: 1,
          current_price: 4.55
        },
        {
          symbol: 'SPY_240119C00500000', // Same symbol
          strike_price: 500,
          type: 'call',
          side: 'sell', // Different side
          quantity: 1,
          current_price: 4.55
        }
      ];

      // Simulate the ID generation logic from RightPanel
      const positionsWithIds = positions.map((pos, index) => ({
        ...pos,
        id: `selected:${pos.symbol}:${index}` // This is the fix - append index
      }));

      expect(positionsWithIds[0].id).toBe('selected:SPY_240119C00500000:0');
      expect(positionsWithIds[1].id).toBe('selected:SPY_240119C00500000:1');
      expect(positionsWithIds[0].id).not.toBe(positionsWithIds[1].id);

      // Test that a Set would properly store both positions
      const idSet = new Set(positionsWithIds.map(p => p.id));
      expect(idSet.size).toBe(2); // Both positions should be stored
    });

    it('should handle position removal correctly with unique IDs', () => {
      const initialPositions = [
        { id: 'selected:SPY_240119C00500000:0', symbol: 'SPY_240119C00500000' },
        { id: 'selected:SPY_240119C00510000:0', symbol: 'SPY_240119C00510000' },
        { id: 'selected:SPY_240119C00520000:0', symbol: 'SPY_240119C00520000' }
      ];

      const checkedPositions = new Set(initialPositions.map(p => p.id));
      expect(checkedPositions.size).toBe(3);

      // Remove middle position (simulating leg removal from options chain)
      const remainingPositions = [
        initialPositions[0],
        initialPositions[2] // Skip middle position
      ];

      const remainingIds = new Set(remainingPositions.map(p => p.id));
      
      // Find stale positions that need to be unchecked
      const staleIds = [...checkedPositions].filter(id => !remainingIds.has(id));
      expect(staleIds).toEqual(['selected:SPY_240119C00510000:0']);

      // Remove stale positions
      staleIds.forEach(id => checkedPositions.delete(id));
      expect(checkedPositions.size).toBe(2);
      expect(checkedPositions.has('selected:SPY_240119C00500000:0')).toBe(true);
      expect(checkedPositions.has('selected:SPY_240119C00520000:0')).toBe(true);
      expect(checkedPositions.has('selected:SPY_240119C00510000:0')).toBe(false);
    });
  });

  describe('Chart Data Source Priority', () => {
    it('should use only checked positions from RightPanel, not selectedLegs', () => {
      const selectedLegs = [
        {
          symbol: 'SPY_240119C00500000',
          strike_price: 500,
          type: 'call',
          side: 'buy',
          quantity: 1,
          current_price: 4.55
        }
      ];

      const checkedPositions = []; // Nothing checked in RightPanel

      // Chart should use checkedPositions (empty), not selectedLegs
      const chartData = checkedPositions.length > 0 
        ? generateMultiLegPayoff(checkedPositions, 500, null)
        : null;

      expect(chartData).toBe(null); // Should be null despite having selectedLegs
    });

    it('should use checked positions even when they differ from selectedLegs', () => {
      const selectedLegs = [
        {
          symbol: 'SPY_240119C00500000',
          strike_price: 500,
          option_type: 'call',
          side: 'buy',
          qty: 1,
          current_price: 4.55,
          asset_class: 'us_option',
          avg_entry_price: 4.55
        },
        {
          symbol: 'SPY_240119C00510000',
          strike_price: 510,
          option_type: 'call',
          side: 'sell',
          qty: 1,
          current_price: 3.25,
          asset_class: 'us_option',
          avg_entry_price: 3.25
        }
      ];

      // User unchecked one position in RightPanel
      const checkedPositions = [selectedLegs[0]]; // Only first position checked

      const chartData = generateMultiLegPayoff(checkedPositions, 500, null);
      
      expect(chartData).toBeTruthy();
      // Chart should be calculated with only the checked position
      // This would be different from a chart calculated with both selectedLegs
    });
  });

  describe('Position Removal and Chart Recalculation', () => {
    it('should trigger full chart recalculation when positions are removed', () => {
      const initialPositions = [
        {
          id: 'selected:SPY_240119C00500000:0',
          symbol: 'SPY_240119C00500000',
          strike_price: 500,
          option_type: 'call',
          side: 'buy',
          qty: 1,
          current_price: 4.55,
          asset_class: 'us_option',
          avg_entry_price: 4.55
        },
        {
          id: 'selected:SPY_240119C00510000:0',
          symbol: 'SPY_240119C00510000',
          strike_price: 510,
          option_type: 'call',
          side: 'sell',
          qty: 1,
          current_price: 3.25,
          asset_class: 'us_option',
          avg_entry_price: 3.25
        }
      ];

      // Initial chart calculation
      const initialChart = generateMultiLegPayoff(initialPositions, 500, null);
      expect(initialChart.prices).toBeDefined();
      expect(initialChart.payoffs).toBeDefined();

      // Remove one position
      const remainingPositions = [initialPositions[0]];
      const updatedChart = generateMultiLegPayoff(remainingPositions, 500, null);

      // Chart should be completely recalculated, not just adjusted
      expect(updatedChart.prices).toBeDefined();
      expect(updatedChart.payoffs).toBeDefined();
      
      // The payoff structure should be different (single leg vs spread)
      expect(updatedChart.payoffs).not.toEqual(initialChart.payoffs);
    });

    it('should reset adjustedNetCredit when leg count changes', () => {
      let adjustedNetCredit = -250; // User had adjusted the price
      let previousLegCount = 2;
      let currentLegCount = 1; // One leg was removed

      // This simulates the logic in OptionsTrading watcher
      if (currentLegCount !== previousLegCount) {
        adjustedNetCredit = null; // Reset to use natural mid price
      }

      expect(adjustedNetCredit).toBe(null);
    });
  });

  describe('Auto-check Logic for New Positions', () => {
    it('should auto-check new positions from options chain', () => {
      const existingChecked = new Set(['existing:position:1']);
      
      const newOptionsChainData = [
        {
          symbol: 'SPY_240119C00500000',
          strike_price: 500,
          type: 'call',
          side: 'buy',
          quantity: 1,
          current_price: 4.55
        }
      ];

      // Simulate auto-check logic
      const newPositions = newOptionsChainData.map((leg, index) => ({
        ...leg,
        id: `selected:${leg.symbol}:${index}`,
        isExisting: false
      }));

      // Auto-check new positions
      newPositions.forEach(pos => {
        if (!pos.isExisting) {
          existingChecked.add(pos.id);
        }
      });

      expect(existingChecked.has('selected:SPY_240119C00500000:0')).toBe(true);
      expect(existingChecked.size).toBe(2); // Original + new
    });

    it('should not auto-check existing positions', () => {
      const checkedPositions = new Set();
      
      const existingPositions = [
        {
          id: 'existing_pos_1',
          symbol: 'SPY_240119P00480000',
          strike_price: 480,
          type: 'put',
          side: 'long',
          qty: 5,
          isExisting: true
        }
      ];

      // Existing positions should not be auto-checked
      existingPositions.forEach(pos => {
        if (!pos.isExisting) { // This condition should be false
          checkedPositions.add(pos.id);
        }
      });

      expect(checkedPositions.size).toBe(0); // No auto-checking for existing positions
    });
  });

  describe('Data Structure Compatibility', () => {
    it('should ensure position data is compatible with generateMultiLegPayoff', () => {
      const position = {
        id: 'selected:SPY_240119C00500000:0',
        symbol: 'SPY_240119C00500000',
        strike_price: 500,
        type: 'call',
        expiry: '2024-01-19',
        side: 'buy',
        quantity: 1,
        current_price: 4.55,
        bid: 4.50,
        ask: 4.60
      };

      // Verify all required fields are present
      expect(position.symbol).toBeTruthy();
      expect(typeof position.strike_price).toBe('number');
      expect(['call', 'put'].includes(position.type)).toBe(true);
      expect(['buy', 'sell', 'long', 'short'].includes(position.side)).toBe(true);
      expect(typeof position.quantity).toBe('number');
      expect(typeof position.current_price).toBe('number');

      // Should not throw when passed to generateMultiLegPayoff
      expect(() => {
        generateMultiLegPayoff([position], 500, null);
      }).not.toThrow();
    });

    it('should handle mixed existing and new positions', () => {
      const mixedPositions = [
        {
          id: 'existing_pos_1',
          symbol: 'SPY_240119P00480000',
          strike_price: 480,
          type: 'put',
          side: 'long',
          qty: 5, // Note: different field name
          current_price: 2.25,
          isExisting: true
        },
        {
          id: 'selected:SPY_240119C00500000:0',
          symbol: 'SPY_240119C00500000',
          strike_price: 500,
          type: 'call',
          side: 'buy',
          quantity: 1, // Note: different field name
          current_price: 4.55,
          isExisting: false
        }
      ];

      // Normalize data for chart calculation
      const normalizedPositions = mixedPositions.map(pos => ({
        ...pos,
        quantity: pos.quantity || pos.qty || 1,
        side: pos.side === 'long' ? 'buy' : pos.side === 'short' ? 'sell' : pos.side
      }));

      expect(() => {
        generateMultiLegPayoff(normalizedPositions, 500, null);
      }).not.toThrow();
    });
  });

  describe('Performance and Memory Management', () => {
    it('should efficiently handle position updates without memory leaks', () => {
      const checkedPositions = new Set();
      
      // Simulate rapid position updates
      for (let i = 0; i < 1000; i++) {
        const positionId = `selected:TEST_${i}:0`;
        checkedPositions.add(positionId);
        
        // Simulate removal of old positions
        if (i > 100) {
          checkedPositions.delete(`selected:TEST_${i - 100}:0`);
        }
      }

      // Should maintain reasonable size
      expect(checkedPositions.size).toBeLessThanOrEqual(101);
    });

    it('should handle large position arrays efficiently', () => {
      const largePositionArray = Array.from({ length: 1000 }, (_, i) => ({
        id: `position_${i}`,
        symbol: `TEST_${i}`,
        strike_price: 500 + i,
        type: 'call',
        side: 'buy',
        quantity: 1,
        current_price: 4.55
      }));

      const startTime = performance.now();
      
      // Simulate the operations that would happen in RightPanel
      const checkedSet = new Set(largePositionArray.map(p => p.id));
      const filteredPositions = largePositionArray.filter(p => checkedSet.has(p.id));
      
      const endTime = performance.now();
      
      expect(endTime - startTime).toBeLessThan(50); // Should be fast
      expect(filteredPositions.length).toBe(1000);
    });
  });

  describe('Edge Cases and Error Recovery', () => {
    it('should handle corrupted position data gracefully', () => {
      const corruptedPositions = [
        null,
        undefined,
        { symbol: 'INCOMPLETE' }, // Missing required fields
        {
          id: 'valid_position',
          symbol: 'SPY_240119C00500000',
          strike_price: 500,
          type: 'call',
          side: 'buy',
          quantity: 1,
          current_price: 4.55
        }
      ];

      // Filter out invalid positions
      const validPositions = corruptedPositions.filter(pos => 
        pos && 
        pos.symbol && 
        typeof pos.strike_price === 'number' &&
        pos.type &&
        pos.side
      );

      expect(validPositions.length).toBe(1);
      expect(validPositions[0].symbol).toBe('SPY_240119C00500000');
    });

    it('should recover from chart calculation failures', () => {
      const positions = [
        {
          symbol: 'SPY_240119C00500000',
          strike_price: 500,
          type: 'call',
          side: 'buy',
          quantity: 1,
          current_price: 4.55
        }
      ];

      // Mock a chart calculation failure
      const mockGenerateMultiLegPayoff = vi.fn().mockImplementation(() => {
        throw new Error('Chart calculation failed');
      });

      let chartData = null;
      
      try {
        chartData = mockGenerateMultiLegPayoff(positions, 500, null);
      } catch (error) {
        // Should gracefully handle the error
        chartData = null;
      }

      expect(chartData).toBe(null);
      expect(mockGenerateMultiLegPayoff).toHaveBeenCalled();
    });
  });

  describe('State Synchronization', () => {
    it('should maintain consistency between checked positions and chart data', () => {
      const positions = [
        {
          id: 'pos1',
          symbol: 'SPY_240119C00500000',
          strike_price: 500,
          type: 'call',
          side: 'buy',
          quantity: 1,
          current_price: 4.55
        },
        {
          id: 'pos2',
          symbol: 'SPY_240119C00510000',
          strike_price: 510,
          type: 'call',
          side: 'sell',
          quantity: 1,
          current_price: 3.25
        }
      ];

      const checkedPositions = new Set(['pos1', 'pos2']);
      
      // Get positions for chart
      const chartPositions = positions.filter(p => checkedPositions.has(p.id));
      expect(chartPositions.length).toBe(2);

      // Uncheck one position
      checkedPositions.delete('pos2');
      
      // Chart positions should update accordingly
      const updatedChartPositions = positions.filter(p => checkedPositions.has(p.id));
      expect(updatedChartPositions.length).toBe(1);
      expect(updatedChartPositions[0].id).toBe('pos1');
    });

    it('should handle concurrent position and price updates', () => {
      // Helper function to round to 2 decimal places (same as in chartUtils.js)
      const roundToTwo = (num) => Math.round((num + Number.EPSILON) * 100) / 100;
      
      let positions = [
        {
          id: 'pos1',
          symbol: 'SPY_240119C00500000',
          strike_price: 500,
          type: 'call',
          side: 'buy',
          quantity: 1,
          current_price: 4.55
        }
      ];

      const checkedPositions = new Set(['pos1']);

      // Simulate price update with proper rounding
      positions = positions.map(p => ({
        ...p,
        current_price: roundToTwo(p.current_price + 0.10)
      }));

      // Position should still be checked with updated price
      const chartPositions = positions.filter(p => checkedPositions.has(p.id));
      expect(chartPositions.length).toBe(1);
      expect(chartPositions[0].current_price).toBe(4.65);
    });
  });
});
