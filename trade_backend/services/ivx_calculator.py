import math
from typing import List, Dict, Optional, Tuple
from ..models import OptionContract

# Configuration constants for IVx calculation
IVX_CALCULATION_METHOD = "price_based"
MIN_STRIKES_REQUIRED = 3
MIN_OPTION_PRICE = 0.01
MAX_BID_ASK_SPREAD_RATIO = 0.5
ENABLE_CALCULATION_LOGGING = False
MIN_VARIANCE_THRESHOLD = 0.0001
MAX_VOLATILITY_THRESHOLD = 5.0

import logging
logger = logging.getLogger(__name__)

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

def calculate_option_price(option: OptionContract) -> Optional[float]:
    """Calculate option price using best available data with quality checks."""
    price = None
    
    # Priority: 1) Midpoint of bid/ask, 2) Close price
    if option.bid is not None and option.ask is not None and option.bid > 0 and option.ask > 0:
        midpoint = (option.bid + option.ask) / 2
        spread = option.ask - option.bid
        
        # Check if spread is reasonable (not too wide)
        if midpoint > 0 and (spread / midpoint) <= MAX_BID_ASK_SPREAD_RATIO:
            price = midpoint
    
    # Fallback to close price if bid/ask is not suitable
    if price is None and option.close_price is not None and option.close_price > 0:
        price = option.close_price
    
    # Validate minimum price threshold
    if price is not None and price >= MIN_OPTION_PRICE:
        return price
    
    return None

def select_option_strikes(options_chain: List[OptionContract], underlying_price: float) -> List[Dict]:
    """Select and organize options for price-based calculation."""
    if not options_chain:
        return []
    
    # Find ATM strike
    atm_strike = find_atm_strike(options_chain, underlying_price)
    if atm_strike is None:
        return []
    
    selected_options = []
    
    # Group options by strike and type
    options_by_strike = {}
    for option in options_chain:
        strike = option.strike_price
        if strike not in options_by_strike:
            options_by_strike[strike] = {'calls': [], 'puts': []}
        
        if option.type.lower() == 'call':
            options_by_strike[strike]['calls'].append(option)
        elif option.type.lower() == 'put':
            options_by_strike[strike]['puts'].append(option)
    
    # Process each strike
    for strike in sorted(options_by_strike.keys()):
        calls = options_by_strike[strike]['calls']
        puts = options_by_strike[strike]['puts']
        
        if strike < atm_strike:
            # OTM puts for strikes below ATM
            for put in puts:
                price = calculate_option_price(put)
                if price is not None and price > 0:
                    selected_options.append({
                        'strike': strike,
                        'price': price,
                        'type': 'put'
                    })
                    break  # Use first valid put at this strike
                    
        elif strike > atm_strike:
            # OTM calls for strikes above ATM
            for call in calls:
                price = calculate_option_price(call)
                if price is not None and price > 0:
                    selected_options.append({
                        'strike': strike,
                        'price': price,
                        'type': 'call'
                    })
                    break  # Use first valid call at this strike
                    
        else:  # strike == atm_strike
            # ATM: Average call and put prices
            call_price = None
            put_price = None
            
            for call in calls:
                call_price = calculate_option_price(call)
                if call_price is not None and call_price > 0:
                    break
                    
            for put in puts:
                put_price = calculate_option_price(put)
                if put_price is not None and put_price > 0:
                    break
            
            # Use average if both available, otherwise use whichever is available
            if call_price is not None and put_price is not None:
                avg_price = (call_price + put_price) / 2
                selected_options.append({
                    'strike': strike,
                    'price': avg_price,
                    'type': 'atm'
                })
            elif call_price is not None:
                selected_options.append({
                    'strike': strike,
                    'price': call_price,
                    'type': 'call'
                })
            elif put_price is not None:
                selected_options.append({
                    'strike': strike,
                    'price': put_price,
                    'type': 'put'
                })
    
    return selected_options

def calculate_delta_k(strikes: List[float], current_index: int) -> float:
    """Calculate the strike interval (ΔK) for a given strike."""
    if len(strikes) < 2:
        return 0
        
    if current_index == 0:  # First strike
        return strikes[1] - strikes[0]
    elif current_index == len(strikes) - 1:  # Last strike
        return strikes[-1] - strikes[-2]
    else:  # Middle strikes
        return (strikes[current_index + 1] - strikes[current_index - 1]) / 2

def calculate_price_based_ivx(options_chain: List[OptionContract], underlying_price: float, dte: int) -> Optional[float]:
    """
    Calculate IVx using option prices (VIX-style methodology).
    
    Formula: σ² = (2/T) * Σ(ΔK/K²) * option_price
    Where:
    - T = time to expiration (in years)
    - ΔK = strike interval
    - K = strike price
    - option_price = midpoint of bid/ask or close price
    """
    if not options_chain or underlying_price is None or dte is None or dte <= 0:
        if ENABLE_CALCULATION_LOGGING:
            logger.debug(f"Invalid inputs for price-based IVx: chain_len={len(options_chain) if options_chain else 0}, underlying={underlying_price}, dte={dte}")
        return None
    
    # Select relevant options
    selected_options = select_option_strikes(options_chain, underlying_price)
    
    if len(selected_options) < MIN_STRIKES_REQUIRED:
        if ENABLE_CALCULATION_LOGGING:
            logger.debug(f"Insufficient strikes for price-based IVx: {len(selected_options)} < {MIN_STRIKES_REQUIRED}")
        return None
    
    # Calculate time to expiration in years
    T = dte / 365.0
    if T <= 0:
        return None
    
    # Sort options by strike
    selected_options.sort(key=lambda x: x['strike'])
    strikes = [opt['strike'] for opt in selected_options]
    
    # Calculate variance contribution from each strike
    total_variance_contribution = 0
    contributions_count = 0
    
    if ENABLE_CALCULATION_LOGGING:
        logger.debug(f"Calculating price-based IVx with {len(selected_options)} strikes, T={T:.4f}")
    
    for i, option in enumerate(selected_options):
        strike = option['strike']
        price = option['price']
        
        # Calculate ΔK for this strike
        delta_k = calculate_delta_k(strikes, i)
        
        if delta_k > 0 and strike > 0:
            # Add contribution: (ΔK/K²) * price
            contribution = (delta_k / (strike ** 2)) * price
            total_variance_contribution += contribution
            contributions_count += 1
            
            if ENABLE_CALCULATION_LOGGING:
                logger.debug(f"Strike {strike}: price={price:.4f}, ΔK={delta_k:.2f}, contribution={contribution:.8f}")
    
    if total_variance_contribution <= 0 or contributions_count == 0:
        if ENABLE_CALCULATION_LOGGING:
            logger.debug(f"No valid contributions: total={total_variance_contribution}, count={contributions_count}")
        return None
    
    # Calculate variance: σ² = (2/T) * Σ(ΔK/K²) * price
    variance = (2 / T) * total_variance_contribution
    
    # Quality control checks
    if variance < MIN_VARIANCE_THRESHOLD:
        if ENABLE_CALCULATION_LOGGING:
            logger.debug(f"Variance too low: {variance} < {MIN_VARIANCE_THRESHOLD}")
        return None
    
    # Convert to annualized volatility
    if variance > 0:
        volatility = math.sqrt(variance)
        
        # Additional quality control
        if volatility > MAX_VOLATILITY_THRESHOLD:
            if ENABLE_CALCULATION_LOGGING:
                logger.warning(f"Volatility too high: {volatility:.4f} > {MAX_VOLATILITY_THRESHOLD}")
            return None
        
        if ENABLE_CALCULATION_LOGGING:
            logger.info(f"Price-based IVx calculated: {volatility:.4f} ({volatility*100:.2f}%) from {contributions_count} strikes")
        
        return volatility  # Return as decimal (will be converted to percentage later)
    
    return None

def calculate_expected_move(underlying_price: float, ivx: float, dte: int) -> Optional[float]:
    """Calculate the expected move in dollars."""
    if underlying_price is None or ivx is None or dte is None:
        return None
        
    # Formula: Expected Move = Stock Price * Implied Volatility * sqrt(Days to Expiration / 365)
    return underlying_price * ivx * math.sqrt(dte / 365)

def calculate_ivx_data(options_chain: List[OptionContract], underlying_price: float, dte: int) -> Dict[str, Optional[float]]:
    """
    Calculate IVx and expected move for an options chain.
    Uses price-based calculation by default, falls back to IV average if insufficient data.
    """
    if not options_chain or underlying_price is None or dte is None:
        return {"ivx_percent": None, "expected_move_dollars": None}
    
    ivx = None
    calculation_method_used = None
    
    # Try price-based calculation first
    if IVX_CALCULATION_METHOD == "price_based":
        ivx = calculate_price_based_ivx(options_chain, underlying_price, dte)
        if ivx is not None:
            calculation_method_used = "price_based"
    
    # Fallback to IV average method if price-based fails or is not selected
    if ivx is None:
        atm_strike = find_atm_strike(options_chain, underlying_price)
        if atm_strike is not None:
            ivx = get_atm_implied_volatility(options_chain, atm_strike)
            if ivx is not None:
                calculation_method_used = "iv_average"
    
    if ivx is None:
        return {"ivx_percent": None, "expected_move_dollars": None}
    
    # Calculate expected move
    expected_move_dollars = calculate_expected_move(underlying_price, ivx, dte)
    
    return {
        "ivx_percent": round(ivx * 100, 2) if ivx is not None else None,
        "expected_move_dollars": round(expected_move_dollars, 2) if expected_move_dollars is not None else None,
        "calculation_method": calculation_method_used  # For debugging/monitoring
    }

# Legacy function for backward compatibility
def calculate_ivx_data_legacy(options_chain: List[OptionContract], underlying_price: float, dte: int) -> Dict[str, Optional[float]]:
    """Legacy IV-average based calculation (for comparison/fallback)."""
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
        "expected_move_dollars": round(expected_move_dollars, 2) if expected_move_dollars is not None else None,
        "calculation_method": "iv_average"
    }
