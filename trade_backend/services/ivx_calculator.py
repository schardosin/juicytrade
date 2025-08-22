import math
from typing import List, Dict, Optional
from ..models import OptionContract

def find_atm_strike(options_chain: List[OptionContract], underlying_price: float) -> Optional[float]:
    """Find the at-the-money (ATM) strike price."""
    if not options_chain:
        return None
    
    min_diff = float('inf')
    atm_strike = None
    
    for contract in options_chain:
        diff = abs(contract.strike_price - underlying_price)
        if diff < min_diff:
            min_diff = diff
            atm_strike = contract.strike_price
            
    return atm_strike

def get_atm_implied_volatility(options_chain: List[OptionContract], atm_strike: float) -> Optional[float]:
    """Get the average implied volatility of the ATM options."""
    atm_ivs = [
        c.implied_volatility for c in options_chain 
        if c.strike_price == atm_strike and c.implied_volatility is not None
    ]
    
    if not atm_ivs:
        return None
        
    return sum(atm_ivs) / len(atm_ivs)

def calculate_expected_move(underlying_price: float, ivx: float, dte: int) -> Optional[float]:
    """Calculate the expected move in dollars."""
    if underlying_price is None or ivx is None or dte is None:
        return None
        
    # Formula: Expected Move = Stock Price * Implied Volatility * sqrt(Days to Expiration / 365)
    return underlying_price * ivx * math.sqrt(dte / 365)

def calculate_ivx_data(options_chain: List[OptionContract], underlying_price: float, dte: int) -> Dict[str, Optional[float]]:
    """Calculate IVx and expected move for an options chain."""
    if not options_chain or underlying_price is None or dte is None:
        return {"ivx_percent": None, "expected_move_dollars": None}
        
    atm_strike = find_atm_strike(options_chain, underlying_price)
    if atm_strike is None:
        return {"ivx_percent": None, "expected_move_dollars": None}
        
    ivx = get_atm_implied_volatility(options_chain, atm_strike)
    if ivx is None:
        return {"ivx_percent": None, "expected_move_dollars": None}
        
    expected_move_dollars = calculate_expected_move(underlying_price, ivx, dte)
    
    return {
        "ivx_percent": round(ivx * 100, 2) if ivx is not None else None,
        "expected_move_dollars": round(expected_move_dollars, 2) if expected_move_dollars is not None else None
    }
