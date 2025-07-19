"""
Options Strategy Detection and Labeling Utility
Centralized logic for identifying and labeling options strategies
"""

import re
from typing import List, Dict, Any, Optional


def detectStrategy(legs: List[Dict[str, Any]]) -> str:
    """
    Detect the strategy type based on order legs
    
    Args:
        legs: List of order legs with symbol, side, qty
        
    Returns:
        Strategy name
    """
    if not legs or len(legs) == 0:
        return "Single Leg"
    
    if len(legs) == 1:
        return "Single Leg"
    
    if len(legs) == 2:
        return detect_two_leg_strategy(legs)
    
    if len(legs) == 4:
        return detect_four_leg_strategy(legs)
    
    return f"{len(legs)}-Leg Strategy"


def detect_two_leg_strategy(legs: List[Dict[str, Any]]) -> str:
    """
    Detect two-leg strategy types
    
    Args:
        legs: List of 2 order legs
        
    Returns:
        Strategy name
    """
    leg1 = parse_option_leg(legs[0])
    leg2 = parse_option_leg(legs[1])
    
    if not leg1 or not leg2:
        return "Two-Leg Strategy"
    
    # Same expiration and same option type (both calls or both puts)
    if leg1['expiry'] == leg2['expiry'] and leg1['type'] == leg2['type']:
        has_buy = any('buy' in leg['side'].lower() for leg in legs)
        has_sell = any('sell' in leg['side'].lower() for leg in legs)
        
        if has_buy and has_sell:
            if leg1['type'] in ['call', 'c']:
                # Determine if it's debit or credit spread
                long_leg = next((leg for leg in legs if 'buy' in leg['side'].lower()), None)
                short_leg = next((leg for leg in legs if 'sell' in leg['side'].lower()), None)
                
                if long_leg and short_leg:
                    long_strike = parse_option_leg(long_leg).get('strike') if parse_option_leg(long_leg) else None
                    short_strike = parse_option_leg(short_leg).get('strike') if parse_option_leg(short_leg) else None
                    
                    if long_strike and short_strike:
                        if long_strike < short_strike:
                            return "Call Debit Spread"
                        else:
                            return "Call Credit Spread"
                
                return "Call Spread"
            else:
                # Put spread
                long_leg = next((leg for leg in legs if 'buy' in leg['side'].lower()), None)
                short_leg = next((leg for leg in legs if 'sell' in leg['side'].lower()), None)
                
                if long_leg and short_leg:
                    long_strike = parse_option_leg(long_leg).get('strike') if parse_option_leg(long_leg) else None
                    short_strike = parse_option_leg(short_leg).get('strike') if parse_option_leg(short_leg) else None
                    
                    if long_strike and short_strike:
                        if long_strike > short_strike:
                            return "Put Debit Spread"
                        else:
                            return "Put Credit Spread"
                
                return "Put Spread"
    
    # Different option types (call + put)
    if leg1['type'] != leg2['type']:
        all_buy = all('buy' in leg['side'].lower() for leg in legs)
        all_sell = all('sell' in leg['side'].lower() for leg in legs)
        
        if all_buy:
            return "Long Straddle"
        elif all_sell:
            return "Short Straddle"
        else:
            return "Synthetic Position"
    
    return "Two-Leg Strategy"


def detect_four_leg_strategy(legs: List[Dict[str, Any]]) -> str:
    """
    Detect four-leg strategy types
    
    Args:
        legs: List of 4 order legs
        
    Returns:
        Strategy name
    """
    parsed_legs = [parse_option_leg(leg) for leg in legs]
    parsed_legs = [leg for leg in parsed_legs if leg is not None]
    
    if len(parsed_legs) != 4:
        return "Four-Leg Strategy"
    
    # Check if it's an Iron Condor (2 calls + 2 puts, same expiry)
    calls = [leg for leg in parsed_legs if leg['type'] in ['call', 'c']]
    puts = [leg for leg in parsed_legs if leg['type'] in ['put', 'p']]
    
    if len(calls) == 2 and len(puts) == 2:
        same_expiry = all(leg['expiry'] == parsed_legs[0]['expiry'] for leg in parsed_legs)
        
        if same_expiry:
            return "Iron Condor"
    
    # Check if it's an Iron Butterfly
    strikes = list(set(leg['strike'] for leg in parsed_legs))
    if len(strikes) == 3:
        return "Iron Butterfly"
    
    return "Four-Leg Strategy"


def parse_option_leg(leg: Dict[str, Any]) -> Optional[Dict[str, Any]]:
    """
    Parse option symbol to extract details
    
    Args:
        leg: Order leg with symbol
        
    Returns:
        Parsed option details or None
    """
    if not leg or not leg.get('symbol'):
        return None
    
    symbol = leg['symbol']
    
    # Check if it's an option symbol
    if not is_option_symbol(symbol):
        return None
    
    try:
        match = re.match(r'^([A-Z]+)(\d{6})([CP])(\d{8})$', symbol)
        if not match:
            return None
        
        underlying, date_str, option_type, strike_str = match.groups()
        
        # Parse date: YYMMDD -> YYYY-MM-DD
        year = 2000 + int(date_str[:2])
        month = date_str[2:4]
        day = date_str[4:6]
        expiry = f"{year}-{month}-{day}"
        
        # Parse strike: 8 digits with 3 decimal places
        strike = int(strike_str) / 1000
        
        return {
            'underlying': underlying,
            'expiry': expiry,
            'type': option_type.lower(),
            'strike': strike,
            'side': leg.get('side', '')
        }
    except Exception as e:
        print(f"Error parsing option leg: {symbol}, {e}")
        return None


def is_option_symbol(symbol: str) -> bool:
    """
    Check if symbol is an option symbol
    
    Args:
        symbol: Symbol to check
        
    Returns:
        True if option symbol
    """
    if not symbol or len(symbol) <= 10:
        return False
    
    return bool(re.search(r'[CP]\d{8}$', symbol))


def get_strategy_description(strategy: str) -> str:
    """
    Get strategy description for display
    
    Args:
        strategy: Strategy name
        
    Returns:
        Display description
    """
    descriptions = {
        "Single Leg": "Single option position",
        "Call Debit Spread": "Bullish spread - buy lower strike, sell higher strike",
        "Call Credit Spread": "Bearish spread - sell lower strike, buy higher strike",
        "Put Debit Spread": "Bearish spread - buy higher strike, sell lower strike",
        "Put Credit Spread": "Bullish spread - sell higher strike, buy lower strike",
        "Call Spread": "Vertical call spread",
        "Put Spread": "Vertical put spread",
        "Long Straddle": "Buy call and put at same strike",
        "Short Straddle": "Sell call and put at same strike",
        "Iron Condor": "Sell call spread and put spread",
        "Iron Butterfly": "Sell straddle and buy protective wings",
        "Synthetic Position": "Combination replicating stock position",
    }
    
    return descriptions.get(strategy, strategy)


def get_strategy_risk_profile(strategy: str) -> Dict[str, str]:
    """
    Get strategy risk profile
    
    Args:
        strategy: Strategy name
        
    Returns:
        Risk profile with max profit/loss info
    """
    profiles = {
        "Call Debit Spread": {
            "max_loss": "Limited",
            "max_profit": "Limited",
            "bias": "Bullish",
        },
        "Call Credit Spread": {
            "max_loss": "Limited",
            "max_profit": "Limited",
            "bias": "Bearish",
        },
        "Put Debit Spread": {
            "max_loss": "Limited",
            "max_profit": "Limited",
            "bias": "Bearish",
        },
        "Put Credit Spread": {
            "max_loss": "Limited",
            "max_profit": "Limited",
            "bias": "Bullish",
        },
        "Iron Condor": {
            "max_loss": "Limited",
            "max_profit": "Limited",
            "bias": "Neutral",
        },
        "Iron Butterfly": {
            "max_loss": "Limited",
            "max_profit": "Limited",
            "bias": "Neutral",
        },
        "Long Straddle": {
            "max_loss": "Limited",
            "max_profit": "Unlimited",
            "bias": "Volatile",
        },
        "Short Straddle": {
            "max_loss": "Unlimited",
            "max_profit": "Limited",
            "bias": "Stable",
        },
    }
    
    return profiles.get(strategy, {
        "max_loss": "Variable",
        "max_profit": "Variable",
        "bias": "Unknown",
    })
