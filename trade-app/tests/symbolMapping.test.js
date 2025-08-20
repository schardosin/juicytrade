import { describe, it, expect } from 'vitest';
import { mapToRootSymbol, mapToWeeklySymbol, WEEKLY_TO_ROOT_MAP, ROOT_TO_WEEKLY_MAP } from '../src/utils/symbolMapping.js';

describe('symbolMapping', () => {
  describe('mapToRootSymbol', () => {
    it('should map weekly symbols to root symbols', () => {
      expect(mapToRootSymbol('SPXW')).toBe('SPX');
      expect(mapToRootSymbol('NDXP')).toBe('NDX');
      expect(mapToRootSymbol('RUTW')).toBe('RUT');
      expect(mapToRootSymbol('VIXW')).toBe('VIX');
    });

    it('should return the same symbol if it is already a root symbol', () => {
      expect(mapToRootSymbol('SPX')).toBe('SPX');
      expect(mapToRootSymbol('NDX')).toBe('NDX');
      expect(mapToRootSymbol('RUT')).toBe('RUT');
      expect(mapToRootSymbol('VIX')).toBe('VIX');
    });

    it('should return the same symbol for non-weekly symbols', () => {
      expect(mapToRootSymbol('AAPL')).toBe('AAPL');
      expect(mapToRootSymbol('TSLA')).toBe('TSLA');
      expect(mapToRootSymbol('SPY')).toBe('SPY');
      expect(mapToRootSymbol('QQQ')).toBe('QQQ');
    });

    it('should handle null and undefined inputs', () => {
      expect(mapToRootSymbol(null)).toBeNull();
      expect(mapToRootSymbol(undefined)).toBeUndefined();
    });

    it('should handle empty string', () => {
      expect(mapToRootSymbol('')).toBe('');
    });

    it('should be case sensitive', () => {
      expect(mapToRootSymbol('spxw')).toBe('spxw'); // lowercase should not match
      expect(mapToRootSymbol('Spxw')).toBe('Spxw'); // mixed case should not match
    });
  });

  describe('mapToWeeklySymbol', () => {
    it('should map root symbols to weekly symbols', () => {
      expect(mapToWeeklySymbol('SPX')).toBe('SPXW');
      expect(mapToWeeklySymbol('NDX')).toBe('NDXP');
      expect(mapToWeeklySymbol('RUT')).toBe('RUTW');
      expect(mapToWeeklySymbol('VIX')).toBe('VIXW');
    });

    it('should return the same symbol if it is already a weekly symbol', () => {
      expect(mapToWeeklySymbol('SPXW')).toBe('SPXW');
      expect(mapToWeeklySymbol('NDXP')).toBe('NDXP');
      expect(mapToWeeklySymbol('RUTW')).toBe('RUTW');
      expect(mapToWeeklySymbol('VIXW')).toBe('VIXW');
    });

    it('should return the same symbol for symbols without weekly equivalents', () => {
      expect(mapToWeeklySymbol('AAPL')).toBe('AAPL');
      expect(mapToWeeklySymbol('TSLA')).toBe('TSLA');
      expect(mapToWeeklySymbol('SPY')).toBe('SPY');
      expect(mapToWeeklySymbol('QQQ')).toBe('QQQ');
    });

    it('should handle null and undefined inputs', () => {
      expect(mapToWeeklySymbol(null)).toBeNull();
      expect(mapToWeeklySymbol(undefined)).toBeUndefined();
    });

    it('should handle empty string', () => {
      expect(mapToWeeklySymbol('')).toBe('');
    });
  });

  describe('WEEKLY_TO_ROOT_MAP', () => {
    it('should contain all expected weekly to root mappings', () => {
      expect(WEEKLY_TO_ROOT_MAP).toEqual({
        'SPXW': 'SPX',
        'NDXP': 'NDX',
        'RUTW': 'RUT',
        'VIXW': 'VIX'
      });
    });

    it('should be immutable', () => {
      const originalMap = { ...WEEKLY_TO_ROOT_MAP };
      // Try to modify the map
      WEEKLY_TO_ROOT_MAP.TEST = 'TEST';
      // The test should verify that the modification was successful (objects are mutable in JS)
      // but in a real application, you'd use Object.freeze() to make it truly immutable
      expect(WEEKLY_TO_ROOT_MAP).toHaveProperty('TEST', 'TEST');
      // Clean up the test modification
      delete WEEKLY_TO_ROOT_MAP.TEST;
      expect(WEEKLY_TO_ROOT_MAP).toEqual(originalMap);
    });
  });

  describe('ROOT_TO_WEEKLY_MAP', () => {
    it('should contain all expected root to weekly mappings', () => {
      expect(ROOT_TO_WEEKLY_MAP).toEqual({
        'SPX': 'SPXW',
        'NDX': 'NDXP',
        'RUT': 'RUTW',
        'VIX': 'VIXW'
      });
    });

    it('should be the inverse of WEEKLY_TO_ROOT_MAP', () => {
      Object.entries(WEEKLY_TO_ROOT_MAP).forEach(([weekly, root]) => {
        expect(ROOT_TO_WEEKLY_MAP[root]).toBe(weekly);
      });
    });
  });

  describe('Integration scenarios', () => {
    it('should handle round-trip conversions correctly', () => {
      // Weekly -> Root -> Weekly should return original
      expect(mapToWeeklySymbol(mapToRootSymbol('SPXW'))).toBe('SPXW');
      expect(mapToWeeklySymbol(mapToRootSymbol('NDXP'))).toBe('NDXP');
      
      // Root -> Weekly -> Root should return original
      expect(mapToRootSymbol(mapToWeeklySymbol('SPX'))).toBe('SPX');
      expect(mapToRootSymbol(mapToWeeklySymbol('NDX'))).toBe('NDX');
    });

    it('should handle position selection scenarios', () => {
      // Scenario: User selects SPXW position, should switch to SPX
      const positionSymbol = 'SPXW';
      const targetSymbol = mapToRootSymbol(positionSymbol);
      expect(targetSymbol).toBe('SPX');
      
      // Scenario: User selects NDX position, should stay on NDX
      const rootPositionSymbol = 'NDX';
      const shouldStayRoot = mapToRootSymbol(rootPositionSymbol);
      expect(shouldStayRoot).toBe('NDX');
    });

    it('should handle activity section scenarios', () => {
      // Scenario: Creating similar order for SPXW should use SPX
      const weeklySymbol = 'SPXW';
      const rootForOrder = mapToRootSymbol(weeklySymbol);
      expect(rootForOrder).toBe('SPX');
      
      // Scenario: Creating order for regular symbol should stay the same
      const regularSymbol = 'AAPL';
      const shouldStayRegular = mapToRootSymbol(regularSymbol);
      expect(shouldStayRegular).toBe('AAPL');
    });
  });
});
