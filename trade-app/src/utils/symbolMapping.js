/**
 * Centralized Symbol Mapping Utility
 * 
 * Handles mapping between weekly/special symbols and their root symbols.
 * Used across multiple components to ensure consistent symbol behavior.
 */

/**
 * Map of weekly/special symbols to their root symbols
 */
export const WEEKLY_TO_ROOT_MAP = {
  'SPXW': 'SPX',   // S&P 500 Weekly
  'NDXP': 'NDX',   // NASDAQ-100 Weekly  
  'RUTW': 'RUT',   // Russell 2000 Weekly
  'VIXW': 'VIX',   // VIX Weekly
  // Add more weekly symbols here as needed
};

/**
 * Map of root symbols to their weekly equivalents (reverse mapping)
 */
export const ROOT_TO_WEEKLY_MAP = Object.fromEntries(
  Object.entries(WEEKLY_TO_ROOT_MAP).map(([weekly, root]) => [root, weekly])
);

/**
 * Maps a weekly/special symbol to its root symbol
 * @param {string} symbol - The symbol to map
 * @returns {string} The root symbol, or the original symbol if no mapping exists
 */
export function mapToRootSymbol(symbol) {
  if (!symbol) return symbol;
  return WEEKLY_TO_ROOT_MAP[symbol] || symbol;
}

/**
 * Maps a root symbol to its weekly equivalent
 * @param {string} symbol - The root symbol to map
 * @returns {string} The weekly symbol, or the original symbol if no mapping exists
 */
export function mapToWeeklySymbol(symbol) {
  if (!symbol) return symbol;
  return ROOT_TO_WEEKLY_MAP[symbol] || symbol;
}

/**
 * Checks if a symbol is a weekly/special symbol
 * @param {string} symbol - The symbol to check
 * @returns {boolean} True if the symbol is a weekly symbol
 */
export function isWeeklySymbol(symbol) {
  return symbol && WEEKLY_TO_ROOT_MAP.hasOwnProperty(symbol);
}

/**
 * Checks if a symbol has a weekly equivalent
 * @param {string} symbol - The symbol to check
 * @returns {boolean} True if the symbol has a weekly equivalent
 */
export function hasWeeklyEquivalent(symbol) {
  return symbol && ROOT_TO_WEEKLY_MAP.hasOwnProperty(symbol);
}

/**
 * Gets all weekly symbols
 * @returns {string[]} Array of all weekly symbols
 */
export function getAllWeeklySymbols() {
  return Object.keys(WEEKLY_TO_ROOT_MAP);
}

/**
 * Gets all root symbols that have weekly equivalents
 * @returns {string[]} Array of all root symbols with weekly equivalents
 */
export function getAllRootSymbols() {
  return Object.keys(ROOT_TO_WEEKLY_MAP);
}

/**
 * Gets the complete mapping object (for debugging/inspection)
 * @returns {object} The complete weekly-to-root mapping
 */
export function getWeeklyToRootMap() {
  return { ...WEEKLY_TO_ROOT_MAP };
}

/**
 * Gets the complete reverse mapping object (for debugging/inspection)
 * @returns {object} The complete root-to-weekly mapping
 */
export function getRootToWeeklyMap() {
  return { ...ROOT_TO_WEEKLY_MAP };
}
