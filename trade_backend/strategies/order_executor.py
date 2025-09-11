"""
Order Execution Engine for Trading Strategies

This module handles order placement and management for automated trading strategies.
It provides a unified interface to place orders across different broker APIs.
"""

from typing import Dict, List, Any, Optional
from datetime import datetime
import logging

try:
    from ..provider_manager import provider_manager
except ImportError:
    # Fallback for direct execution
    import sys
    from pathlib import Path
    sys.path.append(str(Path(__file__).parent.parent))
    try:
        from provider_manager import provider_manager
    except ImportError:
        # Create a minimal mock provider manager
        class MockProviderManager:
            def place_order(self, order_data):
                return None
            def place_multi_leg_order(self, order_data):
                return None
        provider_manager = MockProviderManager()

logger = logging.getLogger(__name__)


class OrderExecutionError(Exception):
    """Exception raised when order execution fails."""

    def __init__(self, message: str, order_data: Dict[str, Any] = None, original_error: Exception = None):
        super().__init__(message)
        self.order_data = order_data
        self.original_error = original_error


class OrderExecutor:
    """
    Handles order execution for trading strategies.

    This class provides methods to place different types of orders (market, limit, stop)
    and handles the coordination between strategies and broker APIs.
    """

    def __init__(self, providers: Optional[Dict] = None):
        """
        Initialize the order executor.

        Args:
            providers: Optional provider configuration overrides
        """
        self.providers = providers or {}
        self.pending_orders = {}  # order_id -> order_data
        self.completed_orders = {}  # order_id -> completed_order_data
        self.order_callbacks = {}  # order_id -> callback_function

        # Statistics tracking
        self.stats = {
            'orders_placed': 0,
            'orders_filled': 0,
            'orders_cancelled': 0,
            'orders_rejected': 0,
            'total_value': 0.0,
            'total_fees': 0.0
        }

        logger.info("OrderExecutor initialized")

    def place_market_order(self, symbol: str, quantity: int, side: str,
                          reason: str = '', strategy_id: str = '') -> str:
        """
        Place a market order.

        Args:
            symbol: Trading symbol
            quantity: Number of shares/contracts
            side: 'buy' or 'sell'
            reason: Reason for the trade (for logging)
            strategy_id: ID of the strategy placing the order

        Returns:
            Order ID if successful, empty string if failed
        """
        return self._place_order('market', symbol, quantity, side, reason, strategy_id)

    def place_limit_order(self, symbol: str, quantity: int, side: str,
                         limit_price: float, reason: str = '', strategy_id: str = '') -> str:
        """
        Place a limit order.

        Args:
            symbol: Trading symbol
            quantity: Number of shares/contracts
            side: 'buy' or 'sell'
            limit_price: Limit price for the order
            reason: Reason for the trade (for logging)
            strategy_id: ID of the strategy placing the order

        Returns:
            Order ID if successful, empty string if failed
        """
        return self._place_order('limit', symbol, quantity, side, reason, strategy_id, limit_price)

    def place_stop_order(self, symbol: str, quantity: int, side: str,
                        stop_price: float, reason: str = '', strategy_id: str = '') -> str:
        """
        Place a stop order.

        Args:
            symbol: Trading symbol
            quantity: Number of shares/contracts
            side: 'buy' or 'sell'
            stop_price: Stop price for the order
            reason: Reason for the trade (for logging)
            strategy_id: ID of the strategy placing the order

        Returns:
            Order ID if successful, empty string if failed
        """
        return self._place_order('stop', symbol, quantity, side, reason, strategy_id, stop_price)

    def place_options_order(self, legs: List, order_type: str = "market", strategy_id: str = '') -> str:
        """
        Place an options order with multiple legs (Iron Condor, etc.).
        
        This method is specifically designed for options strategies like Iron Condors
        that use OptionsLeg objects from the options_models framework.

        Args:
            legs: List of OptionsLeg objects defining the multi-leg order
            order_type: Type of order ("market", "limit", etc.)
            strategy_id: ID of the strategy placing the order

        Returns:
            Order ID if successful, empty string if failed
        """
        try:
            if not legs:
                logger.error("No legs provided for options order")
                return ''

            # Convert OptionsLeg objects to order format
            order_details = self._convert_options_legs_to_order(legs, order_type)
            
            # For backtesting, we need to handle this differently than live trading
            # Check if we're in backtest mode (order_executor is actually the backtest engine)
            if hasattr(self, 'place_market_order') and hasattr(self, 'current_time'):
                # We're in backtest mode - execute each leg as separate trades
                return self._execute_options_order_in_backtest(legs, order_type, strategy_id)
            else:
                # Live trading mode - use the multi-leg order system
                return self.place_multi_leg_order(order_details, strategy_id)

        except Exception as e:
            logger.error(f"Error placing options order: {e}")
            return ''

    def _convert_options_legs_to_order(self, legs: List, order_type: str) -> Dict[str, Any]:
        """
        Convert OptionsLeg objects to multi-leg order format.
        
        Args:
            legs: List of OptionsLeg objects
            order_type: Order type
            
        Returns:
            Dictionary in multi-leg order format
        """
        if not legs:
            return {}
        
        # Use the first leg's underlying as the primary symbol
        primary_symbol = getattr(legs[0].contract, 'underlying_symbol', 'UNKNOWN')
        
        # Convert legs to provider format
        leg_data = []
        for leg in legs:
            leg_dict = leg.to_provider_leg() if hasattr(leg, 'to_provider_leg') else {
                'symbol': leg.contract.symbol,
                'side': leg.action,
                'qty': leg.quantity,
                'asset_class': 'us_option'
            }
            leg_data.append(leg_dict)
        
        return {
            'symbol': primary_symbol,
            'legs': leg_data,
            'order_type': order_type,
            'action': 'multi_leg',
            'quantity': 1,  # Multi-leg orders typically use quantity 1
            'reason': f'{len(legs)}-leg options order'
        }

    def _execute_options_order_in_backtest(self, legs: List, order_type: str, strategy_id: str) -> str:
        """
        Execute options order in backtest mode by placing individual leg trades.
        
        This method handles the special case where the order_executor is actually
        the backtest engine, and we need to execute each leg separately.
        
        Args:
            legs: List of OptionsLeg objects
            order_type: Order type
            strategy_id: Strategy ID
            
        Returns:
            Composite order ID representing all legs
        """
        try:
            executed_trades = []
            composite_order_id = f"OPTIONS_{len(legs)}LEG_{self.current_time.timestamp()}"
            
            logger.info(f"🎯 BACKTEST: Executing {len(legs)}-leg options order")
            
            # Execute each leg as a separate trade
            for i, leg in enumerate(legs):
                try:
                    # Map options action to standard trade side
                    side_mapping = {
                        'buy': 'BUY',
                        'sell': 'SELL',
                        'buy_to_open': 'BUY',
                        'sell_to_open': 'SELL',
                        'buy_to_close': 'BUY',
                        'sell_to_close': 'SELL'
                    }
                    
                    side = side_mapping.get(leg.action.lower(), 'BUY')
                    symbol = leg.contract.symbol
                    quantity = leg.quantity
                    
                    # Execute the individual leg trade
                    trade_id = self.place_market_order(
                        symbol=symbol,
                        quantity=quantity,
                        side=side,
                        reason=f"Options Leg {i+1}/{len(legs)} - {leg.action} {leg.contract.type} {leg.contract.strike_price}"
                    )
                    
                    if trade_id:
                        executed_trades.append(trade_id)
                        logger.info(f"✅ BACKTEST: Executed leg {i+1}: {side} {quantity} {symbol}")
                    else:
                        logger.error(f"❌ BACKTEST: Failed to execute leg {i+1}: {symbol}")
                        
                except Exception as e:
                    logger.error(f"Error executing options leg {i+1}: {e}")
            
            # Return composite order ID if all legs executed successfully
            if len(executed_trades) == len(legs):
                logger.info(f"🎉 BACKTEST: All {len(legs)} options legs executed successfully")
                logger.info(f"   Composite Order ID: {composite_order_id}")
                logger.info(f"   Individual Trade IDs: {executed_trades}")
                return composite_order_id
            else:
                logger.error(f"❌ BACKTEST: Only {len(executed_trades)}/{len(legs)} legs executed")
                return ''
                
        except Exception as e:
            logger.error(f"Error executing options order in backtest: {e}")
            return ''

    def place_multi_leg_order(self, order_details: Dict[str, Any],
                             strategy_id: str = '') -> str:
        """
        Place a multi-leg options order.

        Args:
            order_details: Dictionary containing multi-leg order details
            strategy_id: ID of the strategy placing the order

        Returns:
            Order ID if successful, empty string if failed
        """
        try:
            # Get appropriate provider for orders
            provider = provider_manager.get_provider_for_operation('orders')

            if not provider:
                logger.error("No provider available for orders")
                return ''

            # Prepare order data for multi-leg order
            order_data = self._prepare_multi_leg_order_data(order_details, strategy_id)

            logger.info(f"Placing multi-leg order for {order_details.get('symbol', 'unknown')}")

            # Place the order
            order = provider.place_multi_leg_order(order_data)

            if order:
                order_id = order.id
                self.pending_orders[order_id] = order_data
                self.stats['orders_placed'] += 1

                logger.info(f"Multi-leg order placed: {order_id}")
                return order_id
            else:
                logger.error("Multi-leg order placement failed")
                return ''

        except Exception as e:
            logger.error(f"Error placing multi-leg order: {e}")
            return ''

    def cancel_order(self, order_id: str) -> bool:
        """
        Cancel a pending order.

        Args:
            order_id: ID of the order to cancel

        Returns:
            True if cancellation successful, False otherwise
        """
        try:
            if order_id not in self.pending_orders:
                logger.warning(f"Order {order_id} not found in pending orders")
                return False

            # Get appropriate provider
            provider = provider_manager.get_provider_for_operation('orders')

            if not provider:
                logger.error("No provider available for order cancellation")
                return False

            # Cancel the order
            success = provider.cancel_order(order_id)

            if success:
                order_data = self.pending_orders.pop(order_id, {})
                self.stats['orders_cancelled'] += 1
                logger.info(f"Order cancelled: {order_id}")
                return True
            else:
                logger.error(f"Order cancellation failed: {order_id}")
                return False

        except Exception as e:
            logger.error(f"Error cancelling order {order_id}: {e}")
            return False

    def get_order_status(self, order_id: str) -> Optional[Dict[str, Any]]:
        """
        Get the status of an order.

        Args:
            order_id: ID of the order to check

        Returns:
            Order status dictionary or None if not found
        """
        try:
            # Check pending orders first
            if order_id in self.pending_orders:
                return {
                    'order_id': order_id,
                    'status': 'pending',
                    'data': self.pending_orders[order_id]
                }

            # Check completed orders
            if order_id in self.completed_orders:
                return self.completed_orders[order_id]

            # Query provider for real-time status
            provider = provider_manager.get_provider_for_operation('orders')
            if provider:
                order = provider.get_orders(order_id=order_id)
                if order:
                    return {
                        'order_id': order_id,
                        'status': self._map_order_status(order.status),
                        'data': order
                    }

        except Exception as e:
            logger.error(f"Error getting order status for {order_id}: {e}")

        return None

    def get_all_orders(self, strategy_id: str = '', status_filter: str = '') -> List[Dict[str, Any]]:
        """
        Get all orders, optionally filtered by strategy or status.

        Args:
            strategy_id: Filter by strategy ID
            status_filter: Filter by order status

        Returns:
            List of order dictionaries
        """
        try:
            provider = provider_manager.get_provider_for_operation('orders')
            if not provider:
                logger.error("No provider available for orders")
                return []

            # Get orders from provider
            orders = provider.get_orders(statusfilter=status_filter)

            result = []
            for order in orders:
                if strategy_id and order.strategy_id != strategy_id:
                    continue

                result.append({
                    'order_id': order.id,
                    'symbol': order.symbol,
                    'status': self._map_order_status(order.status),
                    'quantity': order.qty,
                    'filled_qty': order.filled_qty,
                    'price': order.avg_fill_price,
                    'timestamp': order.submitted_at
                })

            return result

        except Exception as e:
            logger.error(f"Error getting orders: {e}")
            return []

    def on_order_update(self, order_id: str, status: str, details: Dict[str, Any]):
        """
        Called when an order status changes.

        This should be connected to the provider's order update callbacks.

        Args:
            order_id: ID of the order that changed
            status: New order status
            details: Additional order details
        """
        try:
            # Update internal tracking
            if order_id in self.pending_orders:
                if status in ['filled', 'cancelled', 'rejected']:
                    order_data = self.pending_orders.pop(order_id, {})
                    self.completed_orders[order_id] = {
                        'order_id': order_id,
                        'status': status,
                        'data': details,
                        'completed_at': datetime.now().isoformat()
                    }

                    # Update statistics
                    if status == 'filled':
                        self.stats['orders_filled'] += 1
                        self.stats['total_value'] += details.get('value', 0.0)
                        self.stats['total_fees'] += details.get('fees', 0.0)
                    elif status == 'cancelled':
                        self.stats['orders_cancelled'] += 1
                    elif status == 'rejected':
                        self.stats['orders_rejected'] += 1

            # Notify strategy callback if registered
            if order_id in self.order_callbacks:
                callback = self.order_callbacks[order_id]
                try:
                    callback(order_id, status, details)
                except Exception as e:
                    logger.error(f"Error in order callback for {order_id}: {e}")
                finally:
                    # Clean up callback
                    del self.order_callbacks[order_id]

            logger.info(f"Order update: {order_id} -> {status}")

        except Exception as e:
            logger.error(f"Error handling order update for {order_id}: {e}")

    def get_execution_stats(self) -> Dict[str, Any]:
        """
        Get execution statistics.

        Returns:
            Dictionary with execution statistics
        """
        return self.stats.copy()

    def register_order_callback(self, order_id: str, callback: callable):
        """
        Register a callback function for order status updates.

        Args:
            order_id: ID of the order to monitor
            callback: Function to call when order status changes
        """
        self.order_callbacks[order_id] = callback

    def _place_order(self, order_type: str, symbol: str, quantity: int,
                    side: str, reason: str, strategy_id: str,
                    price: float = None) -> str:
        """
        Internal method to place an order.

        Args:
            order_type: Type of order ('market', 'limit', 'stop')
            symbol: Trading symbol
            quantity: Number of shares/contracts
            side: 'buy' or 'sell'
            reason: Reason for the trade
            strategy_id: ID of the strategy placing the order
            price: Price for limit/stop orders

        Returns:
            Order ID if successful, empty string if failed
        """
        try:
            # Validate inputs
            if not self._validate_order_inputs(order_type, symbol, quantity, side, price):
                return ''

            # Get appropriate provider
            provider = provider_manager.get_provider_for_operation('orders')

            if not provider:
                logger.error("No provider available for orders")
                return ''

            # Prepare order data
            order_data = self._prepare_order_data(
                order_type, symbol, quantity, side, price, reason, strategy_id
            )

            # Log the order
            logger.info(f"Placing {order_type} order: {symbol} {side} {quantity} @ {price or 'market'} ({reason})")

            # Place the order
            if self._is_option_symbol(symbol):
                # Use multi-leg order for options (even single leg)
                order = provider.place_order(order_data)
            else:
                # Use single-leg order for stocks
                order = provider.place_order(order_data)

            if order:
                order_id = order.id
                self.pending_orders[order_id] = order_data
                self.stats['orders_placed'] += 1

                logger.info(f"Order placed successfully: {order_id}")
                return order_id
            else:
                logger.error("Order placement failed")
                return ''

        except Exception as e:
            logger.error(f"Error placing {order_type} order: {e}")
            return ''

    def _validate_order_inputs(self, order_type: str, symbol: str,
                              quantity: int, side: str, price: float = None) -> bool:
        """
        Validate order input parameters.
        """
        # Validate symbol
        if not symbol or not isinstance(symbol, str):
            logger.error("Invalid symbol")
            return False

        # Validate quantity
        if not isinstance(quantity, int) or quantity <= 0:
            logger.error("Invalid quantity")
            return False

        # Validate side
        if side not in ['buy', 'sell']:
            logger.error(f"Invalid side: {side}")
            return False

        # Validate order type
        if order_type not in ['market', 'limit', 'stop']:
            logger.error(f"Invalid order type: {order_type}")
            return False

        # Validate price for limit/stop orders
        if order_type in ['limit', 'stop'] and (price is None or price <= 0):
            logger.error("Invalid price for limit/stop order")
            return False

        # Validate quantity limits
        max_quantity = 1000000  # Provider-specific limit
        if quantity > max_quantity:
            logger.error(f"Quantity exceeds maximum: {quantity} > {max_quantity}")
            return False

        # Validate price limits
        if price and price > 1000000:
            logger.error(f"Price exceeds maximum: {price}")
            return False

        return True

    def _prepare_order_data(self, order_type: str, symbol: str, quantity: int,
                           side: str, price: float, reason: str, strategy_id: str) -> Dict[str, Any]:
        """
        Prepare order data dictionary for provider API.
        """
        order_data = {
            'symbol': symbol,
            'quantity': quantity,
            'side': side,
            'order_type': order_type,
            'reason': reason,
            'strategy_id': strategy_id,
            'timestamp': datetime.now().isoformat()
        }

        # Add price for limit/stop orders
        if order_type in ['limit', 'stop']:
            order_data['price'] = price

        # Add time in force (default to DAY)
        order_data['time_in_force'] = 'DAY'

        return order_data

    def _prepare_multi_leg_order_data(self, order_details: Dict[str, Any],
                                     strategy_id: str) -> Dict[str, Any]:
        """
        Prepare order data for multi-leg orders.
        """
        return {
            'symbol': order_details.get('symbol', ''),
            'action': order_details.get('action', ''),
            'quantity': order_details.get('quantity', 0),
            'price': order_details.get('limit_price', 0),
            'order_type': order_details.get('order_type', 'market'),
            'side': 'buy' if order_details.get('action', '').upper() == 'BUY' else 'sell',
            'strategy_id': strategy_id,
            'reason': order_details.get('reason', ''),
            'rule_id': order_details.get('rule_id', ''),
            'timestamp': datetime.now().isoformat()
        }

    def _is_option_symbol(self, symbol: str) -> bool:
        """
        Check if symbol is an option symbol.
        """
        # Simple check for OCC format
        return len(symbol) > 10 and ('C' in symbol or 'P' in symbol)

    def _map_order_status(self, status: str) -> str:
        """
        Map provider-specific order status to standardized status.
        """
        status_map = {
            'new': 'pending',
            'pending': 'pending',
            'filled': 'filled',
            'partial': 'partial_fill',
            'cancelled': 'cancelled',
            'expired': 'expired',
            'rejected': 'rejected'
        }
        return status_map.get(status.lower(), status)

    def __str__(self):
        """String representation of the OrderExecutor."""
        return f"OrderExecutor(orders_placed={self.stats['orders_placed']}, filled={self.stats['orders_filled']})"
