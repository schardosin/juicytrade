"""
Position Manager for Automated Trading Framework

This module provides centralized position tracking for both backtesting and live trading.
It automatically tracks positions from order execution and provides a unified interface
for strategies to check their current positions.

Key Features:
- Automatic position updates from order execution
- Support for both stocks and options (including multi-leg)
- Intelligent opening/closing order classification
- Strategy-specific position isolation
- Cross-asset compatibility (stocks, options, combos)
"""

from typing import Dict, List, Any, Optional, Set
from datetime import datetime
from dataclasses import dataclass, field
from enum import Enum
import logging

logger = logging.getLogger(__name__)


class PositionSide(Enum):
    """Position side enumeration"""
    LONG = "long"
    SHORT = "short"
    FLAT = "flat"


class OrderAction(Enum):
    """Order action classification"""
    OPENING = "opening"
    CLOSING = "closing"
    ADDING = "adding"
    REDUCING = "reducing"


@dataclass
class Position:
    """
    Represents a position in a single instrument.
    
    This handles both stock and single-leg option positions.
    """
    symbol: str
    quantity: int
    side: PositionSide
    average_price: float = 0.0
    unrealized_pnl: float = 0.0
    strategy_id: str = ""
    created_at: datetime = field(default_factory=datetime.now)
    updated_at: datetime = field(default_factory=datetime.now)
    
    def is_flat(self) -> bool:
        """Check if position is flat (no quantity)"""
        return self.quantity == 0 or self.side == PositionSide.FLAT
    
    def is_long(self) -> bool:
        """Check if position is long"""
        return self.side == PositionSide.LONG and self.quantity > 0
    
    def is_short(self) -> bool:
        """Check if position is short"""
        return self.side == PositionSide.SHORT and self.quantity < 0
    
    def get_net_quantity(self) -> int:
        """Get net quantity (positive for long, negative for short)"""
        if self.side == PositionSide.LONG:
            return abs(self.quantity)
        elif self.side == PositionSide.SHORT:
            return -abs(self.quantity)
        return 0
    
    def update_from_order(self, order_quantity: int, order_price: float, order_side: str):
        """
        Update position from order execution.
        
        Args:
            order_quantity: Quantity from order (always positive)
            order_price: Execution price
            order_side: 'buy' or 'sell'
        """
        # Convert order side to signed quantity
        signed_quantity = order_quantity if order_side.lower() == 'buy' else -order_quantity
        
        # Calculate new position
        old_net_quantity = self.get_net_quantity()
        new_net_quantity = old_net_quantity + signed_quantity
        
        # Update average price (weighted average)
        if new_net_quantity != 0 and old_net_quantity != 0:
            # Weighted average for adding to position
            total_cost = (old_net_quantity * self.average_price) + (signed_quantity * order_price)
            self.average_price = total_cost / new_net_quantity
        elif new_net_quantity != 0:
            # New position or flipping sides
            self.average_price = order_price
        
        # Update quantity and side
        self.quantity = abs(new_net_quantity)
        if new_net_quantity > 0:
            self.side = PositionSide.LONG
        elif new_net_quantity < 0:
            self.side = PositionSide.SHORT
        else:
            self.side = PositionSide.FLAT
            self.quantity = 0
        
        self.updated_at = datetime.now()
    
    def to_dict(self) -> Dict[str, Any]:
        """Convert position to dictionary for serialization"""
        return {
            'symbol': self.symbol,
            'quantity': self.quantity,
            'side': self.side.value,
            'net_quantity': self.get_net_quantity(),
            'average_price': self.average_price,
            'unrealized_pnl': self.unrealized_pnl,
            'strategy_id': self.strategy_id,
            'created_at': self.created_at.isoformat(),
            'updated_at': self.updated_at.isoformat()
        }


@dataclass
class OptionsLegPosition:
    """Represents a single leg within a multi-leg options position"""
    symbol: str
    quantity: int
    side: PositionSide
    action: str  # 'buy' or 'sell'
    option_type: str  # 'call' or 'put'
    strike_price: float
    expiration_date: str
    average_price: float = 0.0
    
    def is_flat(self) -> bool:
        """Check if leg is flat"""
        return self.quantity == 0 or self.side == PositionSide.FLAT
    
    def get_net_quantity(self) -> int:
        """Get net quantity for this leg"""
        if self.side == PositionSide.LONG:
            return abs(self.quantity)
        elif self.side == PositionSide.SHORT:
            return -abs(self.quantity)
        return 0
    
    def matches_closing_leg(self, closing_leg) -> bool:
        """Check if this leg matches a closing leg"""
        return (
            self.symbol == closing_leg.contract.symbol and
            self.strike_price == closing_leg.contract.strike_price and
            self.option_type == closing_leg.contract.type and
            self.expiration_date == closing_leg.contract.expiration_date
        )
    
    def apply_closing_order(self, closing_quantity: int, closing_side: str):
        """Apply a closing order to this leg"""
        # Convert closing order to signed quantity
        signed_quantity = closing_quantity if closing_side.lower() == 'buy' else -closing_quantity
        
        # Calculate new position (closing reduces the position)
        old_net_quantity = self.get_net_quantity()
        new_net_quantity = old_net_quantity - signed_quantity  # Note: minus for closing
        
        # Update quantity and side
        self.quantity = abs(new_net_quantity)
        if new_net_quantity > 0:
            self.side = PositionSide.LONG
        elif new_net_quantity < 0:
            self.side = PositionSide.SHORT
        else:
            self.side = PositionSide.FLAT
            self.quantity = 0


@dataclass 
class ComboPosition:
    """
    Represents a multi-leg options position (Iron Condor, Straddle, etc.).
    
    This handles complex positions with multiple option legs.
    """
    underlying_symbol: str
    legs: List[OptionsLegPosition]
    combo_type: str = "CUSTOM"
    strategy_id: str = ""
    created_at: datetime = field(default_factory=datetime.now)
    updated_at: datetime = field(default_factory=datetime.now)
    
    @property
    def symbol(self) -> str:
        """Generate a composite symbol for the combo position"""
        leg_symbols = [leg.symbol for leg in self.legs]
        return f"{self.underlying_symbol}_COMBO_{hash(tuple(sorted(leg_symbols))) % 10000:04d}"
    
    def is_flat(self) -> bool:
        """Check if all legs are flat (position is closed)"""
        return all(leg.is_flat() for leg in self.legs)
    
    def get_open_legs(self) -> List[OptionsLegPosition]:
        """Get list of legs that are still open"""
        return [leg for leg in self.legs if not leg.is_flat()]
    
    def apply_closing_order(self, closing_legs):
        """
        Apply a closing order to the combo position.
        
        Args:
            closing_legs: List of OptionsLeg objects representing the closing order
        """
        for closing_leg in closing_legs:
            # Find matching leg in the position
            matching_leg = None
            for leg in self.legs:
                if leg.matches_closing_leg(closing_leg):
                    matching_leg = leg
                    break
            
            if matching_leg:
                # Apply the closing order to the matching leg
                matching_leg.apply_closing_order(closing_leg.quantity, closing_leg.action)
                logger.info(f"Applied closing order to leg {matching_leg.symbol}: {closing_leg.quantity} {closing_leg.action}")
            else:
                logger.warning(f"No matching leg found for closing order: {closing_leg.contract.symbol}")
        
        self.updated_at = datetime.now()
    
    def to_dict(self) -> Dict[str, Any]:
        """Convert combo position to dictionary"""
        return {
            'symbol': self.symbol,
            'underlying_symbol': self.underlying_symbol,
            'combo_type': self.combo_type,
            'strategy_id': self.strategy_id,
            'is_flat': self.is_flat(),
            'legs': [
                {
                    'symbol': leg.symbol,
                    'quantity': leg.quantity,
                    'side': leg.side.value,
                    'net_quantity': leg.get_net_quantity(),
                    'option_type': leg.option_type,
                    'strike_price': leg.strike_price,
                    'expiration_date': leg.expiration_date,
                    'average_price': leg.average_price
                }
                for leg in self.legs
            ],
            'open_legs': len(self.get_open_legs()),
            'created_at': self.created_at.isoformat(),
            'updated_at': self.updated_at.isoformat()
        }


class PositionManager:
    """
    Centralized position tracking manager.
    
    This class provides automatic position tracking from order execution
    and offers a unified interface for strategies to check positions.
    """
    
    def __init__(self):
        # Positions by symbol
        self.positions: Dict[str, Position] = {}
        self.combo_positions: Dict[str, ComboPosition] = {}
        
        # Strategy-specific position tracking
        self.strategy_positions: Dict[str, Set[str]] = {}  # strategy_id -> set of position symbols
        
        # Order tracking for debugging
        self.processed_orders: List[Dict[str, Any]] = []
        
        logger.info("PositionManager initialized")
    
    # ========================================================================
    # Core Position Management
    # ========================================================================
    
    def has_positions(self, strategy_id: str = None, underlying: str = None) -> bool:
        """
        Check if there are any open positions.
        
        Args:
            strategy_id: Filter by strategy ID
            underlying: Filter by underlying symbol
            
        Returns:
            True if any positions exist matching criteria
        """
        positions_to_check = []
        
        if strategy_id:
            # Get positions for specific strategy
            strategy_symbols = self.strategy_positions.get(strategy_id, set())
            positions_to_check = [
                pos for symbol, pos in self.positions.items() 
                if symbol in strategy_symbols and not pos.is_flat()
            ]
            combo_positions_to_check = [
                pos for symbol, pos in self.combo_positions.items()
                if symbol in strategy_symbols and not pos.is_flat()
            ]
        else:
            # Check all positions
            positions_to_check = [pos for pos in self.positions.values() if not pos.is_flat()]
            combo_positions_to_check = [pos for pos in self.combo_positions.values() if not pos.is_flat()]
        
        # Filter by underlying if specified
        if underlying:
            positions_to_check = [
                pos for pos in positions_to_check 
                if self._extract_underlying_symbol(pos.symbol) == underlying
            ]
            combo_positions_to_check = [
                pos for pos in combo_positions_to_check
                if pos.underlying_symbol == underlying
            ]
        
        return len(positions_to_check) > 0 or len(combo_positions_to_check) > 0
    
    def get_position(self, symbol: str, strategy_id: str = None) -> Optional[Position]:
        """
        Get position for a specific symbol.
        
        Args:
            symbol: Position symbol
            strategy_id: Filter by strategy ID
            
        Returns:
            Position object or None if not found
        """
        position = self.positions.get(symbol)
        
        if position and strategy_id:
            # Check if position belongs to this strategy
            strategy_symbols = self.strategy_positions.get(strategy_id, set())
            if symbol not in strategy_symbols:
                return None
        
        return position if position and not position.is_flat() else None
    
    def get_combo_position(self, symbol: str, strategy_id: str = None) -> Optional[ComboPosition]:
        """Get combo position for a specific symbol"""
        combo_position = self.combo_positions.get(symbol)
        
        if combo_position and strategy_id:
            strategy_symbols = self.strategy_positions.get(strategy_id, set())
            if symbol not in strategy_symbols:
                return None
        
        return combo_position if combo_position and not combo_position.is_flat() else None
    
    def get_all_positions(self, strategy_id: str = None, underlying: str = None) -> Dict[str, Any]:
        """
        Get all positions, optionally filtered by strategy and/or underlying.
        
        Returns:
            Dictionary with 'positions' and 'combo_positions' keys
        """
        result = {
            'positions': {},
            'combo_positions': {}
        }
        
        # Filter positions
        positions_to_include = {}
        if strategy_id:
            strategy_symbols = self.strategy_positions.get(strategy_id, set())
            positions_to_include = {
                symbol: pos for symbol, pos in self.positions.items()
                if symbol in strategy_symbols and not pos.is_flat()
            }
            combo_positions_to_include = {
                symbol: pos for symbol, pos in self.combo_positions.items()
                if symbol in strategy_symbols and not pos.is_flat()
            }
        else:
            positions_to_include = {
                symbol: pos for symbol, pos in self.positions.items()
                if not pos.is_flat()
            }
            combo_positions_to_include = {
                symbol: pos for symbol, pos in self.combo_positions.items()
                if not pos.is_flat()
            }
        
        # Filter by underlying if specified
        if underlying:
            positions_to_include = {
                symbol: pos for symbol, pos in positions_to_include.items()
                if self._extract_underlying_symbol(pos.symbol) == underlying
            }
            combo_positions_to_include = {
                symbol: pos for symbol, pos in combo_positions_to_include.items()
                if pos.underlying_symbol == underlying
            }
        
        result['positions'] = positions_to_include
        result['combo_positions'] = combo_positions_to_include
        
        return result
    
    # ========================================================================
    # Order Processing
    # ========================================================================
    
    def process_stock_order(self, symbol: str, quantity: int, side: str, price: float, 
                           strategy_id: str = "", order_id: str = ""):
        """
        Process a stock order and update positions.
        
        Args:
            symbol: Stock symbol
            quantity: Order quantity (always positive) 
            side: 'buy' or 'sell'
            price: Execution price
            strategy_id: Strategy that placed the order
            order_id: Order ID for tracking
        """
        try:
            # Get or create position
            position = self.positions.get(symbol)
            
            if position is None:
                # Create new position
                initial_side = PositionSide.LONG if side.lower() == 'buy' else PositionSide.SHORT
                position = Position(
                    symbol=symbol,
                    quantity=quantity,
                    side=initial_side,
                    average_price=price,
                    strategy_id=strategy_id
                )
                self.positions[symbol] = position
                logger.info(f"Created new stock position: {symbol} {quantity} {side} @ ${price}")
            else:
                # Update existing position
                position.update_from_order(quantity, price, side)
                logger.info(f"Updated stock position: {symbol} -> {position.get_net_quantity()} @ ${position.average_price}")
            
            # Track strategy ownership
            if strategy_id:
                if strategy_id not in self.strategy_positions:
                    self.strategy_positions[strategy_id] = set()
                self.strategy_positions[strategy_id].add(symbol)
            
            # Remove flat positions
            if position.is_flat():
                self._remove_flat_position(symbol, strategy_id)
            
            # Record order for debugging
            self.processed_orders.append({
                'order_id': order_id,
                'symbol': symbol,
                'quantity': quantity,
                'side': side,
                'price': price,
                'strategy_id': strategy_id,
                'timestamp': datetime.now().isoformat(),
                'position_after': position.to_dict() if not position.is_flat() else None
            })
            
        except Exception as e:
            logger.error(f"Error processing stock order {symbol}: {e}")
    
    def process_options_order(self, legs: List, strategy_id: str = "", order_id: str = ""):
        """
        Process an options order with multiple legs.
        
        Args:
            legs: List of OptionsLeg objects
            strategy_id: Strategy that placed the order
            order_id: Order ID for tracking
        """
        try:
            if not legs:
                logger.warning("No legs provided for options order processing")
                return
            
            # Determine underlying symbol
            underlying_symbol = legs[0].contract.underlying_symbol
            
            # Check if this is a closing order for an existing combo position
            existing_combo = self._find_matching_combo_position(legs, strategy_id)
            
            if existing_combo:
                # This is a closing order for an existing combo position
                logger.info(f"Processing closing order for combo position: {existing_combo.symbol}")
                existing_combo.apply_closing_order(legs)
                
                # Remove combo if flat
                if existing_combo.is_flat():
                    self._remove_flat_combo_position(existing_combo.symbol, strategy_id)
                    logger.info(f"Closed combo position: {existing_combo.symbol}")
                else:
                    logger.info(f"Partially closed combo position: {existing_combo.symbol}")
            else:
                # This is an opening order - create new combo position
                logger.info(f"Creating new combo position for {len(legs)} legs")
                combo_legs = []
                
                for leg in legs:
                    # Create OptionsLegPosition from OptionsLeg
                    leg_position = OptionsLegPosition(
                        symbol=leg.contract.symbol,
                        quantity=leg.quantity,
                        side=PositionSide.LONG if leg.action.lower() in ['buy', 'buy_to_open'] else PositionSide.SHORT,
                        action=leg.action,
                        option_type=leg.contract.type,
                        strike_price=leg.contract.strike_price,
                        expiration_date=leg.contract.expiration_date,
                        average_price=getattr(leg.contract, 'mark', 0.0)  # Use mark price if available
                    )
                    combo_legs.append(leg_position)
                
                # Create combo position
                combo_position = ComboPosition(
                    underlying_symbol=underlying_symbol,
                    legs=combo_legs,
                    combo_type=self._determine_combo_type(legs),
                    strategy_id=strategy_id
                )
                
                # Store combo position
                combo_symbol = combo_position.symbol
                self.combo_positions[combo_symbol] = combo_position
                
                # Track strategy ownership
                if strategy_id:
                    if strategy_id not in self.strategy_positions:
                        self.strategy_positions[strategy_id] = set()
                    self.strategy_positions[strategy_id].add(combo_symbol)
                
                logger.info(f"Created new combo position: {combo_symbol} ({combo_position.combo_type})")
            
            # Record order for debugging
            self.processed_orders.append({
                'order_id': order_id,
                'type': 'options',
                'underlying_symbol': underlying_symbol,
                'legs': [
                    {
                        'symbol': leg.contract.symbol,
                        'action': leg.action,
                        'quantity': leg.quantity,
                        'type': leg.contract.type,
                        'strike': leg.contract.strike_price,
                        'expiration': leg.contract.expiration_date
                    }
                    for leg in legs
                ],
                'strategy_id': strategy_id,
                'timestamp': datetime.now().isoformat()
            })
            
        except Exception as e:
            logger.error(f"Error processing options order: {e}")
    
    # ========================================================================
    # Helper Methods
    # ========================================================================
    
    def _find_matching_combo_position(self, closing_legs: List, strategy_id: str = "") -> Optional[ComboPosition]:
        """
        Find an existing combo position that matches the closing legs.
        
        This is used to determine if an order is closing an existing position.
        """
        strategy_symbols = self.strategy_positions.get(strategy_id, set()) if strategy_id else set()
        
        for combo_symbol, combo_position in self.combo_positions.items():
            # Skip if not owned by this strategy
            if strategy_id and combo_symbol not in strategy_symbols:
                continue
            
            # Skip if combo is already flat
            if combo_position.is_flat():
                continue
            
            # Check if closing legs match existing combo legs
            if self._legs_match_combo_for_closing(closing_legs, combo_position):
                return combo_position
        
        return None
    
    def _legs_match_combo_for_closing(self, closing_legs: List, combo_position: ComboPosition) -> bool:
        """
        Check if closing legs are the reverse of an existing combo position.
        
        For a closing order, we expect:
        - Same underlying symbol
        - Same number of legs
        - Each closing leg matches a combo leg but with opposite action
        """
        if len(closing_legs) != len(combo_position.legs):
            return False
        
        # Check if underlying symbols match
        underlying_symbol = closing_legs[0].contract.underlying_symbol
        if underlying_symbol != combo_position.underlying_symbol:
            return False
        
        # Try to match each closing leg with a combo leg
        matched_legs = set()
        
        for closing_leg in closing_legs:
            found_match = False
            
            for i, combo_leg in enumerate(combo_position.legs):
                if i in matched_legs:
                    continue  # Already matched this combo leg
                
                # Check if this closing leg matches this combo leg
                if (closing_leg.contract.symbol == combo_leg.symbol and
                    closing_leg.contract.strike_price == combo_leg.strike_price and
                    closing_leg.contract.type == combo_leg.option_type and
                    closing_leg.contract.expiration_date == combo_leg.expiration_date):
                    
                    # Check if action is opposite (closing action)
                    if self._is_closing_action(closing_leg.action, combo_leg.action):
                        matched_legs.add(i)
                        found_match = True
                        break
            
            if not found_match:
                return False
        
        # All closing legs must match combo legs
        return len(matched_legs) == len(combo_position.legs)
    
    def _is_closing_action(self, closing_action: str, original_action: str) -> bool:
        """Check if closing action is the opposite of original action"""
        closing_lower = closing_action.lower()
        original_lower = original_action.lower()
        
        # Mapping of opening actions to their closing counterparts
        closing_map = {
            'buy': 'sell',
            'buy_to_open': 'sell_to_close',
            'sell': 'buy',
            'sell_to_open': 'buy_to_close'
        }
        
        expected_closing = closing_map.get(original_lower, '')
        return closing_lower == expected_closing or closing_lower.replace('_to_close', '') == closing_map.get(original_lower, '').replace('_to_close', '')
    
    def _determine_combo_type(self, legs: List) -> str:
        """Determine the type of combo position based on legs"""
        if len(legs) == 1:
            return "SINGLE_LEG"
        elif len(legs) == 2:
            # Could be vertical, straddle, strangle, etc.
            if legs[0].contract.type == legs[1].contract.type:
                return "VERTICAL"
            else:
                return "STRADDLE_STRANGLE"
        elif len(legs) == 4:
            return "IRON_CONDOR"
        else:
            return "CUSTOM"
    
    def _extract_underlying_symbol(self, symbol: str) -> str:
        """Extract underlying symbol from option or stock symbol"""
        # For options symbols, extract the underlying
        if len(symbol) > 6 and ('C' in symbol or 'P' in symbol):
            # OCC format: AAPL240119C00150000
            # Extract first part before date
            import re
            match = re.match(r'^([A-Z]+)', symbol)
            if match:
                return match.group(1)
        
        # For stock symbols or if extraction fails, return as-is
        return symbol
    
    def _remove_flat_position(self, symbol: str, strategy_id: str = ""):
        """Remove a flat position from tracking"""
        if symbol in self.positions:
            del self.positions[symbol]
            logger.info(f"Removed flat position: {symbol}")
        
        # Remove from strategy tracking
        if strategy_id and strategy_id in self.strategy_positions:
            self.strategy_positions[strategy_id].discard(symbol)
    
    def _remove_flat_combo_position(self, combo_symbol: str, strategy_id: str = ""):
        """Remove a flat combo position from tracking"""
        if combo_symbol in self.combo_positions:
            del self.combo_positions[combo_symbol]
            logger.info(f"Removed flat combo position: {combo_symbol}")
        
        # Remove from strategy tracking
        if strategy_id and strategy_id in self.strategy_positions:
            self.strategy_positions[strategy_id].discard(combo_symbol)
    
    # ========================================================================
    # Status and Debugging
    # ========================================================================
    
    def get_status(self) -> Dict[str, Any]:
        """Get comprehensive position manager status"""
        return {
            'total_positions': len([p for p in self.positions.values() if not p.is_flat()]),
            'total_combo_positions': len([p for p in self.combo_positions.values() if not p.is_flat()]),
            'strategies_tracked': len(self.strategy_positions),
            'orders_processed': len(self.processed_orders),
            'positions_by_strategy': {
                strategy_id: len(symbols)
                for strategy_id, symbols in self.strategy_positions.items()
            }
        }
    
    def get_debug_info(self) -> Dict[str, Any]:
        """Get detailed debug information"""
        return {
            'positions': {symbol: pos.to_dict() for symbol, pos in self.positions.items() if not pos.is_flat()},
            'combo_positions': {symbol: pos.to_dict() for symbol, pos in self.combo_positions.items() if not pos.is_flat()},
            'strategy_positions': {k: list(v) for k, v in self.strategy_positions.items()},
            'recent_orders': self.processed_orders[-20:] if self.processed_orders else []
        }


    def reset(self):
        """Reset the position manager - clear all positions and tracking"""
        self.positions.clear()
        self.combo_positions.clear()
        self.strategy_positions.clear()
        self.processed_orders.clear()
        logger.info("PositionManager reset - all positions cleared")


# Global position manager instance
position_manager = PositionManager()
