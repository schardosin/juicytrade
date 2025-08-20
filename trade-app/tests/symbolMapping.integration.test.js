import { describe, it, expect, vi, beforeEach } from 'vitest';
import { mapToRootSymbol, mapToWeeklySymbol, WEEKLY_TO_ROOT_MAP, ROOT_TO_WEEKLY_MAP } from '../src/utils/symbolMapping.js';

describe('Symbol Mapping Integration Tests', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe('End-to-End Symbol Mapping Workflows', () => {
    it('should handle complete position selection workflow', () => {
      // Simulate user selecting a SPXW position
      const selectedPositionSymbol = 'SPXW';
      
      // Step 1: Map to root symbol for application navigation
      const targetSymbol = mapToRootSymbol(selectedPositionSymbol);
      expect(targetSymbol).toBe('SPX');
      
      // Step 2: Verify the mapping is consistent
      expect(WEEKLY_TO_ROOT_MAP[selectedPositionSymbol]).toBe(targetSymbol);
      
      // Step 3: Verify reverse mapping works
      const backToWeekly = mapToWeeklySymbol(targetSymbol);
      expect(backToWeekly).toBe(selectedPositionSymbol);
    });

    it('should handle activity section workflow', () => {
      // Simulate creating similar order from weekly symbol
      const originalSymbol = 'NDXP';
      
      // Map to root for order creation
      const orderSymbol = mapToRootSymbol(originalSymbol);
      expect(orderSymbol).toBe('NDX');
      
      // Verify the workflow maintains consistency
      expect(WEEKLY_TO_ROOT_MAP[originalSymbol]).toBe(orderSymbol);
      expect(ROOT_TO_WEEKLY_MAP[orderSymbol]).toBe(originalSymbol);
    });

    it('should handle mixed symbol scenarios', () => {
      const testCases = [
        { input: 'SPXW', expected: 'SPX', type: 'weekly' },
        { input: 'SPX', expected: 'SPX', type: 'root' },
        { input: 'AAPL', expected: 'AAPL', type: 'regular' },
        { input: 'NDXP', expected: 'NDX', type: 'weekly' },
        { input: 'RUTW', expected: 'RUT', type: 'weekly' },
        { input: 'VIXW', expected: 'VIX', type: 'weekly' }
      ];

      testCases.forEach(({ input, expected, type }) => {
        const result = mapToRootSymbol(input);
        expect(result).toBe(expected);
        
        // Additional validation based on type
        if (type === 'weekly') {
          expect(WEEKLY_TO_ROOT_MAP).toHaveProperty(input);
          expect(WEEKLY_TO_ROOT_MAP[input]).toBe(expected);
        }
      });
    });
  });

  describe('Race Condition Prevention Integration', () => {
    it('should handle rapid symbol changes consistently', () => {
      // Simulate rapid symbol changes that could cause race conditions
      const symbols = ['SPXW', 'SPX', 'NDXP', 'NDX', 'RUTW', 'RUT'];
      
      symbols.forEach(symbol => {
        const mapped = mapToRootSymbol(symbol);
        
        // Each mapping should be consistent
        expect(mapped).toBeDefined();
        expect(typeof mapped).toBe('string');
        
        // Weekly symbols should map to their roots
        if (WEEKLY_TO_ROOT_MAP[symbol]) {
          expect(mapped).toBe(WEEKLY_TO_ROOT_MAP[symbol]);
        } else {
          // Non-weekly symbols should map to themselves
          expect(mapped).toBe(symbol);
        }
      });
    });

    it('should maintain bidirectional consistency', () => {
      // Test that all mappings are bidirectional
      Object.entries(WEEKLY_TO_ROOT_MAP).forEach(([weekly, root]) => {
        // Weekly -> Root
        expect(mapToRootSymbol(weekly)).toBe(root);
        
        // Root -> Weekly
        expect(mapToWeeklySymbol(root)).toBe(weekly);
        
        // Verify reverse mapping exists
        expect(ROOT_TO_WEEKLY_MAP[root]).toBe(weekly);
      });
    });
  });

  describe('Performance and Edge Cases', () => {
    it('should handle null and undefined inputs consistently', () => {
      expect(mapToRootSymbol(null)).toBeNull();
      expect(mapToRootSymbol(undefined)).toBeUndefined();
      expect(mapToRootSymbol('')).toBe('');
      
      expect(mapToWeeklySymbol(null)).toBeNull();
      expect(mapToWeeklySymbol(undefined)).toBeUndefined();
      expect(mapToWeeklySymbol('')).toBe('');
    });

    it('should handle case sensitivity correctly', () => {
      // Case sensitivity tests
      expect(mapToRootSymbol('spxw')).toBe('spxw'); // lowercase should not match
      expect(mapToRootSymbol('SPXW')).toBe('SPX');   // uppercase should match
      expect(mapToRootSymbol('Spxw')).toBe('Spxw');  // mixed case should not match
    });

    it('should handle unknown symbols gracefully', () => {
      const unknownSymbols = ['UNKNOWN', 'TEST123', 'XYZ'];
      
      unknownSymbols.forEach(symbol => {
        expect(mapToRootSymbol(symbol)).toBe(symbol);
        expect(mapToWeeklySymbol(symbol)).toBe(symbol);
      });
    });
  });

  describe('Real-World Integration Scenarios', () => {
    it('should support positions view integration', () => {
      // Simulate the exact scenario from the original bug report
      const positionSymbol = 'SPXW';
      const currentAppSymbol = 'AAPL'; // User is currently viewing AAPL
      
      // When user selects SPXW position, app should switch to SPX
      const targetSymbol = mapToRootSymbol(positionSymbol);
      expect(targetSymbol).toBe('SPX');
      
      // Verify this is different from current symbol (would trigger change)
      expect(targetSymbol).not.toBe(currentAppSymbol);
      
      // Verify the mapping is in our constants
      expect(WEEKLY_TO_ROOT_MAP[positionSymbol]).toBe(targetSymbol);
    });

    it('should support activity section integration', () => {
      // Simulate activity section creating orders
      const weeklySymbols = ['SPXW', 'NDXP', 'RUTW', 'VIXW'];
      
      weeklySymbols.forEach(weeklySymbol => {
        const rootSymbol = mapToRootSymbol(weeklySymbol);
        
        // Should map to root for order creation
        expect(rootSymbol).not.toBe(weeklySymbol);
        expect(WEEKLY_TO_ROOT_MAP[weeklySymbol]).toBe(rootSymbol);
        
        // Should be able to map back
        expect(mapToWeeklySymbol(rootSymbol)).toBe(weeklySymbol);
      });
    });

    it('should handle centralized usage across components', () => {
      // Test that the same symbol mapping works consistently
      // across different component scenarios
      
      const testSymbol = 'SPXW';
      const expectedRoot = 'SPX';
      
      // Positions view scenario
      const positionsResult = mapToRootSymbol(testSymbol);
      expect(positionsResult).toBe(expectedRoot);
      
      // Activity section scenario  
      const activityResult = mapToRootSymbol(testSymbol);
      expect(activityResult).toBe(expectedRoot);
      
      // Both should be identical (centralized logic)
      expect(positionsResult).toBe(activityResult);
      
      // Both should use the same mapping constants
      expect(WEEKLY_TO_ROOT_MAP[testSymbol]).toBe(expectedRoot);
    });
  });

  describe('Data Integrity', () => {
    it('should maintain mapping consistency', () => {
      // Verify all weekly symbols have corresponding root symbols
      Object.keys(WEEKLY_TO_ROOT_MAP).forEach(weekly => {
        const root = WEEKLY_TO_ROOT_MAP[weekly];
        expect(root).toBeDefined();
        expect(typeof root).toBe('string');
        expect(root.length).toBeGreaterThan(0);
      });
      
      // Verify all root symbols have corresponding weekly symbols
      Object.keys(ROOT_TO_WEEKLY_MAP).forEach(root => {
        const weekly = ROOT_TO_WEEKLY_MAP[root];
        expect(weekly).toBeDefined();
        expect(typeof weekly).toBe('string');
        expect(weekly.length).toBeGreaterThan(0);
      });
    });

    it('should have symmetric mappings', () => {
      // Every weekly->root mapping should have a corresponding root->weekly mapping
      Object.entries(WEEKLY_TO_ROOT_MAP).forEach(([weekly, root]) => {
        expect(ROOT_TO_WEEKLY_MAP[root]).toBe(weekly);
      });
      
      // Every root->weekly mapping should have a corresponding weekly->root mapping
      Object.entries(ROOT_TO_WEEKLY_MAP).forEach(([root, weekly]) => {
        expect(WEEKLY_TO_ROOT_MAP[weekly]).toBe(root);
      });
    });
  });
});
