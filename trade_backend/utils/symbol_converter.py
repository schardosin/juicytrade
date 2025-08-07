"""
Universal Symbol Converter for Multi-Provider Trading System

This module provides bidirectional symbol conversion between different broker formats:
- Standard OCC format (used internally and by most providers)
- TastyTrade format (6-character padded root with spaces)
- Future provider formats can be added here

Performance optimized with caching for high-frequency trading scenarios.
"""

import logging
import re
from typing import Dict, List, Optional, Tuple
from enum import Enum

logger = logging.getLogger(__name__)

class SymbolFormat(Enum):
    """Supported symbol formats."""
    STANDARD_OCC = "standard_occ"  # SPY250808P00632000
    TASTYTRADE = "tastytrade"      # SPY   250808P00632000
    UNKNOWN = "unknown"

class SymbolConverter:
    """
    High-performance symbol converter with caching for trading systems.
    
    Converts between different broker symbol formats while maintaining
    performance for real-time trading scenarios.
    """
    
    # Conversion cache for performance optimization
    _conversion_cache: Dict[str, str] = {}
    _format_cache: Dict[str, SymbolFormat] = {}
    
    # Cache size limits to prevent memory bloat
    MAX_CACHE_SIZE = 10000
    
    @classmethod
    def clear_cache(cls):
        """Clear conversion caches (useful for testing or memory management)."""
        cls._conversion_cache.clear()
        cls._format_cache.clear()
        logger.info("Symbol converter caches cleared")
    
    @classmethod
    def get_cache_stats(cls) -> Dict[str, int]:
        """Get cache statistics for monitoring."""
        return {
            "conversion_cache_size": len(cls._conversion_cache),
            "format_cache_size": len(cls._format_cache),
            "max_cache_size": cls.MAX_CACHE_SIZE
        }
    
    @classmethod
    def detect_format(cls, symbol: str) -> SymbolFormat:
        """
        Detect the format of an option symbol.
        
        Args:
            symbol: Option symbol to analyze
            
        Returns:
            SymbolFormat enum indicating the detected format
        """
        if not symbol:
            return SymbolFormat.UNKNOWN
        
        # Check cache first
        if symbol in cls._format_cache:
            return cls._format_cache[symbol]
        
        # Detect format based on symbol characteristics
        format_result = cls._detect_format_uncached(symbol)
        
        # Cache the result (with size limit)
        if len(cls._format_cache) < cls.MAX_CACHE_SIZE:
            cls._format_cache[symbol] = format_result
        
        return format_result
    
    @classmethod
    def _detect_format_uncached(cls, symbol: str) -> SymbolFormat:
        """Internal format detection without caching."""
        # Check if it's an option symbol (has C/P and digits)
        if not cls._is_option_symbol(symbol):
            return SymbolFormat.STANDARD_OCC  # Stock symbols are the same across providers
        
        # TastyTrade format detection:
        # - Has spaces in the first 6 characters
        # - Total length typically 21+ characters
        # - Pattern: ROOT(padded to 6) + YYMMDD + C/P + STRIKE
        if len(symbol) >= 15 and '  ' in symbol[:6]:
            return SymbolFormat.TASTYTRADE
        
        # Standard OCC format:
        # - No spaces in symbol
        # - Pattern: ROOT + YYMMDD + C/P + STRIKE
        if len(symbol) >= 15 and ' ' not in symbol:
            return SymbolFormat.STANDARD_OCC
        
        return SymbolFormat.UNKNOWN
    
    @classmethod
    def _is_option_symbol(cls, symbol: str) -> bool:
        """Check if symbol appears to be an option symbol."""
        if len(symbol) < 15:
            return False
        
        # Look for option type indicator (C or P) and strike digits
        return any(c in symbol for c in ['C', 'P']) and any(c.isdigit() for c in symbol[-8:])
    
    @classmethod
    def to_standard_occ(cls, symbol: str) -> str:
        """
        Convert any symbol format to standard OCC format.
        
        Args:
            symbol: Symbol in any supported format
            
        Returns:
            Symbol in standard OCC format (SPY250808P00632000)
        """
        if not symbol:
            return symbol
        
        # Check cache first
        cache_key = f"to_occ:{symbol}"
        if cache_key in cls._conversion_cache:
            return cls._conversion_cache[cache_key]
        
        # Detect current format and convert
        current_format = cls.detect_format(symbol)
        
        if current_format == SymbolFormat.STANDARD_OCC:
            result = symbol  # Already in standard format
        elif current_format == SymbolFormat.TASTYTRADE:
            result = cls._tastytrade_to_occ(symbol)
        else:
            # Unknown format or stock symbol - return as-is
            result = symbol
        
        # Cache the result (with size limit)
        if len(cls._conversion_cache) < cls.MAX_CACHE_SIZE:
            cls._conversion_cache[cache_key] = result
        
        return result
    
    @classmethod
    def to_tastytrade(cls, symbol: str) -> str:
        """
        Convert any symbol format to TastyTrade format.
        
        Args:
            symbol: Symbol in any supported format
            
        Returns:
            Symbol in TastyTrade format (SPY   250808P00632000)
        """
        if not symbol:
            return symbol
        
        # Check cache first
        cache_key = f"to_tt:{symbol}"
        if cache_key in cls._conversion_cache:
            return cls._conversion_cache[cache_key]
        
        # Detect current format and convert
        current_format = cls.detect_format(symbol)
        
        if current_format == SymbolFormat.TASTYTRADE:
            result = symbol  # Already in TastyTrade format
        elif current_format == SymbolFormat.STANDARD_OCC:
            result = cls._occ_to_tastytrade(symbol)
        else:
            # Unknown format or stock symbol - return as-is
            result = symbol
        
        # Cache the result (with size limit)
        if len(cls._conversion_cache) < cls.MAX_CACHE_SIZE:
            cls._conversion_cache[cache_key] = result
        
        return result
    
    @classmethod
    def to_provider_format(cls, symbol: str, provider_type: str) -> str:
        """
        Convert symbol to specific provider format.
        
        Args:
            symbol: Symbol in any supported format
            provider_type: Target provider type ('tastytrade', 'tradier', 'alpaca', etc.)
            
        Returns:
            Symbol in the provider's expected format
        """
        provider_type_lower = provider_type.lower()
        
        if provider_type_lower == 'tastytrade':
            return cls.to_tastytrade(symbol)
        else:
            # Most providers use standard OCC format
            return cls.to_standard_occ(symbol)
    
    @classmethod
    def batch_convert_to_provider_format(cls, symbols: List[str], provider_type: str) -> List[str]:
        """
        Convert multiple symbols to provider format efficiently.
        
        Args:
            symbols: List of symbols in any supported format
            provider_type: Target provider type
            
        Returns:
            List of symbols in the provider's expected format
        """
        return [cls.to_provider_format(symbol, provider_type) for symbol in symbols]
    
    @classmethod
    def batch_convert_to_standard_occ(cls, symbols: List[str]) -> List[str]:
        """
        Convert multiple symbols to standard OCC format efficiently.
        
        Args:
            symbols: List of symbols in any supported format
            
        Returns:
            List of symbols in standard OCC format
        """
        return [cls.to_standard_occ(symbol) for symbol in symbols]
    
    @classmethod
    def _tastytrade_to_occ(cls, symbol: str) -> str:
        """Convert TastyTrade format to standard OCC format."""
        try:
            if not cls._is_option_symbol(symbol):
                return symbol
            
            # TastyTrade format: ROOT(padded to 6) + YYMMDD + C/P + STRIKE
            # Standard OCC format: ROOT + YYMMDD + C/P + STRIKE
            
            if len(symbol) >= 15:
                # Extract the root (first 6 chars, remove trailing spaces)
                root = symbol[:6].rstrip()
                # Get the rest (date + type + strike)
                date_and_rest = symbol[6:]
                
                # Combine without padding
                occ_symbol = root + date_and_rest
                
                logger.debug(f"Converted TastyTrade -> OCC: {symbol} -> {occ_symbol}")
                return occ_symbol
            
            return symbol
            
        except Exception as e:
            logger.error(f"Error converting TastyTrade symbol {symbol} to OCC: {e}")
            return symbol
    
    @classmethod
    def _occ_to_tastytrade(cls, symbol: str) -> str:
        """Convert standard OCC format to TastyTrade format."""
        try:
            if not cls._is_option_symbol(symbol):
                return symbol
            
            # Parse the standard OCC symbol to find where the date starts
            # OCC format: ROOT + YYMMDD(6) + C/P(1) + STRIKE(8)
            
            if len(symbol) >= 15:
                # Find the date part by looking for 6 consecutive digits
                # The date should be YYMMDD where YY >= 20 (year 2020+)
                for i in range(1, len(symbol) - 14):  # Start from position 1, leave room for date+type+strike
                    potential_date = symbol[i:i+6]
                    if (potential_date.isdigit() and 
                        len(potential_date) == 6 and
                        int(potential_date[:2]) >= 20):  # Year 20xx
                        
                        # Found the date, extract root
                        root = symbol[:i]
                        date_and_rest = symbol[i:]
                        
                        # TastyTrade format: ROOT padded to exactly 6 characters + date_and_rest
                        padded_root = root.ljust(6)  # Left-justify and pad with spaces to 6 chars
                        tastytrade_symbol = f"{padded_root}{date_and_rest}"
                        
                        logger.debug(f"Converted OCC -> TastyTrade: {symbol} -> {tastytrade_symbol}")
                        return tastytrade_symbol
                
                # If we couldn't find a valid date pattern, return as-is
                logger.warning(f"Could not parse date pattern in OCC symbol {symbol}")
                return symbol
            
            return symbol
            
        except Exception as e:
            logger.error(f"Error converting OCC symbol {symbol} to TastyTrade: {e}")
            return symbol
    
    @classmethod
    def parse_option_symbol(cls, symbol: str) -> Optional[Dict[str, any]]:
        """
        Parse option symbol to extract components (underlying, expiry, strike, type).
        
        Args:
            symbol: Option symbol in any supported format
            
        Returns:
            Dict with parsed components or None if parsing fails
        """
        try:
            # Convert to standard OCC format for consistent parsing
            occ_symbol = cls.to_standard_occ(symbol)
            
            if not cls._is_option_symbol(occ_symbol):
                return None
            
            # Parse standard OCC format: ROOT + YYMMDD + C/P + STRIKE
            # Find the date part
            for i in range(1, len(occ_symbol) - 14):
                potential_date = occ_symbol[i:i+6]
                if (potential_date.isdigit() and 
                    len(potential_date) == 6 and
                    int(potential_date[:2]) >= 20):
                    
                    # Extract components
                    root = occ_symbol[:i]
                    date_part = potential_date
                    option_type = occ_symbol[i+6]
                    strike_part = occ_symbol[i+7:]
                    
                    # Parse expiry date
                    year = 2000 + int(date_part[:2])
                    month = int(date_part[2:4])
                    day = int(date_part[4:6])
                    expiry_date = f"{year}-{month:02d}-{day:02d}"
                    
                    # Parse strike price (divide by 1000 for standard format)
                    strike_price = float(strike_part) / 1000
                    
                    return {
                        "underlying": root,
                        "expiry": expiry_date,
                        "strike": strike_price,
                        "type": "call" if option_type == "C" else "put",
                        "original_symbol": symbol,
                        "occ_symbol": occ_symbol
                    }
            
            return None
            
        except Exception as e:
            logger.error(f"Error parsing option symbol {symbol}: {e}")
            return None

# Convenience functions for common use cases
def convert_symbols_for_provider(symbols: List[str], provider_type: str) -> List[str]:
    """Convert list of symbols to provider format."""
    return SymbolConverter.batch_convert_to_provider_format(symbols, provider_type)

def convert_symbols_to_standard(symbols: List[str]) -> List[str]:
    """Convert list of symbols to standard OCC format."""
    return SymbolConverter.batch_convert_to_standard_occ(symbols)

def is_tastytrade_format(symbol: str) -> bool:
    """Check if symbol is in TastyTrade format."""
    return SymbolConverter.detect_format(symbol) == SymbolFormat.TASTYTRADE

def is_standard_occ_format(symbol: str) -> bool:
    """Check if symbol is in standard OCC format."""
    return SymbolConverter.detect_format(symbol) == SymbolFormat.STANDARD_OCC
