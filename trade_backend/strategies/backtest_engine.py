"""
Strategy Backtesting Engine

This module provides comprehensive backtesting capabilities for action-based strategies:
- Historical data replay with configurable speed
- Action execution against historical market conditions
- Performance tracking and metrics calculation
- Trade simulation with realistic fills
- Comprehensive reporting and analysis
"""

import asyncio
import logging
from datetime import datetime, timedelta
from typing import Dict, Any, List, Optional, Tuple
from dataclasses import dataclass
import pandas as pd
import numpy as np

from .base_strategy import BaseStrategy
from .actions import ActionContext, TradeAction
from .strategy_state import StrategyState
from .time_manager import TimeScheduler, MarketType, TradingSession

logger = logging.getLogger(__name__)

# ============================================================================
# Backtest Data Structures
# ============================================================================

@dataclass
class BacktestTrade:
    """Represents a trade executed during backtesting"""
    timestamp: datetime
    symbol: str
    action: str  # BUY, SELL, BUY_TO_OPEN, etc.
    quantity: int
    price: float
    order_type: str
    trade_id: str
    strategy_action: str  # Name of the action that generated this trade
    pnl: float = 0.0
    commission: float = 0.0
    
    def to_dict(self) -> Dict[str, Any]:
        return {
            "timestamp": self.timestamp.isoformat(),
            "symbol": self.symbol,
            "action": self.action,
            "quantity": self.quantity,
            "price": self.price,
            "order_type": self.order_type,
            "trade_id": self.trade_id,
            "strategy_action": self.strategy_action,
            "pnl": self.pnl,
            "commission": self.commission
        }

@dataclass
class BacktestPosition:
    """Represents a position during backtesting"""
    symbol: str
    quantity: int
    avg_price: float
    current_price: float
    unrealized_pnl: float
    realized_pnl: float
    
    def to_dict(self) -> Dict[str, Any]:
        return {
            "symbol": self.symbol,
            "quantity": self.quantity,
            "avg_price": self.avg_price,
            "current_price": self.current_price,
            "unrealized_pnl": self.unrealized_pnl,
            "realized_pnl": self.realized_pnl,
            "market_value": self.quantity * self.current_price
        }

@dataclass
class BacktestMetrics:
    """Comprehensive backtest performance metrics"""
    start_date: datetime
    end_date: datetime
    duration_days: int
    
    # Trading metrics
    total_trades: int
    winning_trades: int
    losing_trades: int
    win_rate: float
    
    # P&L metrics
    total_pnl: float
    total_return: float
    max_profit: float
    max_loss: float
    largest_win: float
    largest_loss: float
    
    # Risk metrics
    max_drawdown: float
    max_drawdown_duration: int
    sharpe_ratio: float
    sortino_ratio: float
    calmar_ratio: float
    
    # Position metrics
    max_positions: int
    avg_position_size: float
    avg_holding_period: float
    
    # Action metrics
    total_actions: int
    successful_actions: int
    failed_actions: int
    action_success_rate: float
    
    def to_dict(self) -> Dict[str, Any]:
        return {
            "period": {
                "start_date": self.start_date.isoformat(),
                "end_date": self.end_date.isoformat(),
                "duration_days": self.duration_days
            },
            "trading": {
                "total_trades": self.total_trades,
                "winning_trades": self.winning_trades,
                "losing_trades": self.losing_trades,
                "win_rate": self.win_rate
            },
            "pnl": {
                "total_pnl": self.total_pnl,
                "total_return": self.total_return,
                "max_profit": self.max_profit,
                "max_loss": self.max_loss,
                "largest_win": self.largest_win,
                "largest_loss": self.largest_loss
            },
            "risk": {
                "max_drawdown": self.max_drawdown,
                "max_drawdown_duration": self.max_drawdown_duration,
                "sharpe_ratio": self.sharpe_ratio,
                "sortino_ratio": self.sortino_ratio,
                "calmar_ratio": self.calmar_ratio
            },
            "positions": {
                "max_positions": self.max_positions,
                "avg_position_size": self.avg_position_size,
                "avg_holding_period": self.avg_holding_period
            },
            "actions": {
                "total_actions": self.total_actions,
                "successful_actions": self.successful_actions,
                "failed_actions": self.failed_actions,
                "action_success_rate": self.action_success_rate
            }
        }

# ============================================================================
# Historical Data Provider (Mock Implementation)
# ============================================================================

class RealHistoricalDataProvider:
    """
    Real historical data provider for backtesting using the actual provider system.
    """
    
    def __init__(self, provider_manager=None):
        self.provider_manager = provider_manager
        self.data_cache = {}
    
    async def get_historical_data(
        self,
        symbol: str,
        start_date: datetime,
        end_date: datetime,
        interval: str = "1min"
    ) -> pd.DataFrame:
        """
        Get historical OHLCV data for a symbol using real providers.
        
        Returns DataFrame with columns: timestamp, open, high, low, close, volume
        """
        if not self.provider_manager:
            logger.warning("No provider manager available, falling back to mock data")
            return self._generate_mock_data(symbol, start_date, end_date, interval)
        
        try:
            # Map interval to provider format
            timeframe_map = {
                "1min": "1m",
                "5min": "5m", 
                "15min": "15m",
                "1hour": "1h",
                "1day": "D",
                "D": "D"
            }
            
            provider_timeframe = timeframe_map.get(interval, "1m")
            
            # Get historical bars from real provider
            bars = await self.provider_manager.get_historical_bars(
                symbol=symbol,
                timeframe=provider_timeframe,
                start_date=start_date.strftime('%Y-%m-%d'),
                end_date=end_date.strftime('%Y-%m-%d'),
                limit=100000  # Large limit to get all data
            )
            
            if not bars:
                logger.warning(f"No historical data returned for {symbol}, using mock data")
                return self._generate_mock_data(symbol, start_date, end_date, interval)
            
            # Convert bars to DataFrame
            data = []
            for i, bar in enumerate(bars):
                # Debug: Log first few bars to see the structure
                if i < 3:
                    logger.info(f"Bar {i}: {bar}")
                
                # Parse timestamp - handle different formats
                timestamp = None
                if isinstance(bar.get('timestamp'), str):
                    timestamp = pd.to_datetime(bar['timestamp'])
                elif bar.get('timestamp') is not None:
                    timestamp = bar.get('timestamp')
                elif bar.get('time') is not None:
                    # Try 'time' field as alternative
                    timestamp = pd.to_datetime(bar['time'])
                elif bar.get('datetime') is not None:
                    # Try 'datetime' field as alternative
                    timestamp = pd.to_datetime(bar['datetime'])
                else:
                    # Log the bar structure to understand what fields are available
                    logger.warning(f"No timestamp found in bar: {list(bar.keys())}")
                
                data.append({
                    "timestamp": timestamp,
                    "open": float(bar.get('open', 0)),
                    "high": float(bar.get('high', 0)),
                    "low": float(bar.get('low', 0)),
                    "close": float(bar.get('close', 0)),
                    "volume": int(bar.get('volume', 0))
                })
            
            df = pd.DataFrame(data)
            
            # Sort by timestamp
            if not df.empty:
                df = df.sort_values('timestamp').reset_index(drop=True)
                logger.info(f"Retrieved {len(df)} real historical data points for {symbol}")
            
            return df
            
        except Exception as e:
            logger.error(f"Error getting real historical data for {symbol}: {e}")
            logger.info("Falling back to mock data generation")
            return self._generate_mock_data(symbol, start_date, end_date, interval)
    
    def _generate_mock_data(
        self,
        symbol: str,
        start_date: datetime,
        end_date: datetime,
        interval: str = "1min"
    ) -> pd.DataFrame:
        """
        Fallback mock data generation when real data is not available.
        """
        logger.info(f"Generating mock data for {symbol} from {start_date} to {end_date}")
        
        # Generate time range based on interval
        if interval == "1day" or interval == "D":
            freq = "D"
        elif interval == "1hour":
            freq = "H"
        elif interval == "15min":
            freq = "15min"
        elif interval == "5min":
            freq = "5min"
        else:
            freq = "1min"
        
        date_range = pd.date_range(start=start_date, end=end_date, freq=freq)
        
        # Filter to market hours for intraday data
        if freq != "D":
            market_hours = date_range[
                (date_range.time >= pd.Timestamp("09:30").time()) &
                (date_range.time <= pd.Timestamp("16:00").time()) &
                (date_range.weekday < 5)  # Monday=0, Friday=4
            ]
        else:
            # For daily data, just filter weekdays
            market_hours = date_range[date_range.weekday < 5]
        
        if len(market_hours) == 0:
            return pd.DataFrame()
        
        # Generate realistic price data
        np.random.seed(hash(symbol) % 2**32)  # Consistent data for same symbol
        
        base_price = 100.0 if symbol == "SPY" else 50.0
        
        # Adjust volatility based on timeframe
        if freq == "D":
            volatility = 0.02  # 2% daily volatility
        elif freq == "H":
            volatility = 0.005  # 0.5% hourly volatility
        else:
            volatility = 0.001  # 0.1% minute volatility
        
        returns = np.random.normal(0, volatility, len(market_hours))
        prices = base_price * np.exp(np.cumsum(returns))
        
        # Generate OHLCV data
        data = []
        for i, timestamp in enumerate(market_hours):
            price = prices[i]
            noise = np.random.normal(0, volatility * 0.5)
            
            open_price = price + noise
            high_price = price + abs(noise) + np.random.exponential(volatility * 0.5)
            low_price = price - abs(noise) - np.random.exponential(volatility * 0.5)
            close_price = price
            volume = int(np.random.exponential(1000))
            
            data.append({
                "timestamp": timestamp,
                "open": round(open_price, 2),
                "high": round(high_price, 2),
                "low": round(low_price, 2),
                "close": round(close_price, 2),
                "volume": volume
            })
        
        return pd.DataFrame(data)
    
    def get_options_data(
        self,
        underlying: str,
        start_date: datetime,
        end_date: datetime
    ) -> pd.DataFrame:
        """
        Get historical options data.
        Mock implementation - would connect to real options data in production.
        """
        # Mock options data generation
        # This would be much more complex in reality
        return pd.DataFrame()

# ============================================================================
# Backtest Engine
# ============================================================================

class StrategyBacktestEngine:
    """
    Comprehensive backtesting engine for action-based strategies.
    
    Features:
    - Historical data replay with configurable speed
    - Action execution simulation
    - Realistic trade fills and slippage
    - Performance tracking and metrics
    - Detailed logging and debugging
    """
    
    def __init__(
        self,
        initial_capital: float = 100000.0,
        commission_per_trade: float = 1.0,
        slippage_bps: float = 2.0,  # Basis points
        market_type: MarketType = MarketType.STOCK,
        provider_manager=None,
        timeframe: str = "1min"  # Add configurable timeframe
    ):
        self.initial_capital = initial_capital
        self.commission_per_trade = commission_per_trade
        self.slippage_bps = slippage_bps
        self.market_type = market_type
        self.timeframe = timeframe  # Store timeframe for data fetching
        
        # Data provider - use real provider if available
        self.data_provider = RealHistoricalDataProvider(provider_manager)
        
        # Backtest state
        self.current_capital = initial_capital
        self.positions: Dict[str, BacktestPosition] = {}
        self.trades: List[BacktestTrade] = []
        self.equity_curve: List[Tuple[datetime, float]] = []
        
        # Execution tracking
        self.current_time: Optional[datetime] = None
        self.market_data_cache: Dict[str, pd.DataFrame] = {}
        self.current_prices: Dict[str, float] = {}
        
        logger.info(f"BacktestEngine initialized with ${initial_capital:,.2f} capital")
    
    async def run_backtest(
        self,
        strategy: BaseStrategy,
        start_date: datetime,
        end_date: datetime,
        symbols: List[str] = None,
        speed_multiplier: int = 1
    ) -> Dict[str, Any]:
        """
        Run comprehensive backtest for an action-based strategy.
        
        Args:
            strategy: BaseStrategy instance to backtest
            start_date: Backtest start date
            end_date: Backtest end date
            symbols: List of symbols to include (default: ["SPY"])
            speed_multiplier: Speed multiplier for backtest (1=real-time, 1000=very fast)
        
        Returns:
            Comprehensive backtest results
        """
        try:
            logger.info(f"Starting backtest: {start_date} to {end_date}")
            
            # Initialize backtest
            symbols = symbols or ["SPY"]
            await self._initialize_backtest(strategy, start_date, end_date, symbols)
            
            # Load historical data
            await self._load_historical_data(symbols, start_date, end_date)
            
            # Run backtest simulation
            await self._run_simulation(strategy, start_date, end_date, speed_multiplier)
            
            # Calculate metrics
            metrics = self._calculate_metrics(start_date, end_date, strategy)
            
            # Generate results with proper JSON serialization
            results = {
                "success": True,
                "strategy_id": strategy.strategy_id,
                "config": {
                    "start_date": start_date.isoformat(),
                    "end_date": end_date.isoformat(),
                    "symbols": symbols,
                    "initial_capital": self.initial_capital,
                    "commission_per_trade": self.commission_per_trade,
                    "slippage_bps": self.slippage_bps,
                    "speed_multiplier": speed_multiplier
                },
                "metrics": metrics.to_dict(),
                "trades": [trade.to_dict() for trade in self.trades],
                "final_positions": {symbol: pos.to_dict() for symbol, pos in self.positions.items()},
                "equity_curve": [{"timestamp": ts.isoformat() if ts and hasattr(ts, 'isoformat') else str(ts) if ts else None, "equity": equity} for ts, equity in self.equity_curve],
                "action_log": self._serialize_json_safe(strategy.get_action_log()),
                "checkpoints": self._serialize_json_safe(strategy.get_checkpoints()),
                "state_history": self._serialize_json_safe(strategy.get_state_history()),
                "decision_timeline": self._serialize_json_safe(strategy.get_decision_timeline())  # Add decision timeline
            }
            
            logger.info(f"Backtest completed: {metrics.total_trades} trades, ${metrics.total_pnl:,.2f} P&L")
            return results
            
        except Exception as e:
            logger.error(f"Backtest failed: {e}")
            return {
                "success": False,
                "error": str(e),
                "strategy_id": strategy.strategy_id if strategy else "unknown"
            }
    
    async def _initialize_backtest(
        self,
        strategy: BaseStrategy,
        start_date: datetime,
        end_date: datetime,
        symbols: List[str]
    ):
        """Initialize backtest state"""
        # Reset state
        self.current_capital = self.initial_capital
        self.positions.clear()
        self.trades.clear()
        self.equity_curve.clear()
        self.market_data_cache.clear()
        self.current_prices.clear()
        
        # Initialize strategy in backtest mode
        strategy.dry_run = False  # We want to simulate real trades
        strategy.data_provider = self
        strategy.order_executor = self
        
        # Add initial equity point
        self.equity_curve.append((start_date, self.initial_capital))
        
        logger.info(f"Backtest initialized for {len(symbols)} symbols")
    
    async def _load_historical_data(
        self,
        symbols: List[str],
        start_date: datetime,
        end_date: datetime
    ):
        """Load historical data for all symbols"""
        for symbol in symbols:
            logger.info(f"Loading historical data for {symbol}")
            
            data = await self.data_provider.get_historical_data(
                symbol, start_date, end_date, self.timeframe
            )
            
            if not data.empty:
                self.market_data_cache[symbol] = data
                logger.info(f"Loaded {len(data)} data points for {symbol}")
            else:
                logger.warning(f"No data available for {symbol}")
    
    async def _run_simulation(
        self,
        strategy: BaseStrategy,
        start_date: datetime,
        end_date: datetime,
        speed_multiplier: int
    ):
        """Run the main backtest simulation"""
        # Start the strategy
        await strategy.start()
        
        # Get all timestamps from market data
        all_timestamps = set()
        for symbol, symbol_data in self.market_data_cache.items():
            logger.info(f"Processing timestamps for {symbol}: DataFrame shape {symbol_data.shape}")
            
            # Handle both pandas Series and regular lists
            if hasattr(symbol_data, 'iterrows'):
                # DataFrame - iterate through rows
                timestamp_count = 0
                for _, row in symbol_data.iterrows():
                    timestamp = row['timestamp']
                    # Convert pandas timestamp to python datetime for consistency
                    if hasattr(timestamp, 'to_pydatetime'):
                        timestamp = timestamp.to_pydatetime()
                    all_timestamps.add(timestamp)
                    timestamp_count += 1
                    
                    # Debug: Log first few timestamps
                    if timestamp_count <= 5:
                        logger.info(f"Timestamp {timestamp_count}: {timestamp} (type: {type(timestamp)})")
                
                logger.info(f"Added {timestamp_count} timestamps from {symbol}")
            else:
                # Regular data structure
                all_timestamps.update(symbol_data['timestamp'])
        
        timestamps = sorted(all_timestamps)
        
        if not timestamps:
            logger.warning("No market data timestamps found")
            return
        
        logger.info(f"Total unique timestamps collected: {len(timestamps)}")
        logger.info(f"First timestamp: {timestamps[0]}")
        logger.info(f"Last timestamp: {timestamps[-1]}")
        logger.info(f"Simulating {len(timestamps)} time periods")
        
        # Simulate each time period
        for i, timestamp in enumerate(timestamps):
            self.current_time = timestamp
            
            # Update current prices
            self._update_current_prices(timestamp)
            
            # Update positions with current prices
            self._update_positions()
            
            # Execute strategy cycle
            await strategy.execute_cycle()
            
            # CRITICAL FIX: Process any trade actions that were queued by the strategy
            await self._process_strategy_trade_actions(strategy)
            
            # Record equity curve (every 100 periods to avoid too much data)
            if i % 100 == 0:
                current_equity = self._calculate_current_equity()
                self.equity_curve.append((timestamp, current_equity))
            
            # Add small delay for very fast backtests to prevent overwhelming
            if speed_multiplier < 1000:
                await asyncio.sleep(0.001 / speed_multiplier)
        
        # Stop the strategy
        await strategy.stop()
        
        # CRITICAL FIX: Close all open positions at end of backtest
        await self._close_all_positions_at_end(end_date)
        
        # Final equity calculation
        final_equity = self._calculate_current_equity()
        self.equity_curve.append((end_date, final_equity))
        
        logger.info(f"Simulation completed: Final equity ${final_equity:,.2f}")
    
    def _update_current_prices(self, timestamp: datetime):
        """Update current prices for all symbols"""
        for symbol, data in self.market_data_cache.items():
            # Find the exact price data for this timestamp or the closest one
            exact_match = data[data['timestamp'] == timestamp]
            if not exact_match.empty:
                # Use exact timestamp match
                self.current_prices[symbol] = exact_match.iloc[0]['close']
            else:
                # Find the closest price data before or at this timestamp
                symbol_data = data[data['timestamp'] <= timestamp]
                if not symbol_data.empty:
                    latest_data = symbol_data.iloc[-1]
                    self.current_prices[symbol] = latest_data['close']
    
    def _update_positions(self):
        """Update position values with current prices"""
        for symbol, position in self.positions.items():
            if symbol in self.current_prices:
                position.current_price = self.current_prices[symbol]
                position.unrealized_pnl = (position.current_price - position.avg_price) * position.quantity
    
    def _calculate_current_equity(self) -> float:
        """Calculate current total equity"""
        equity = self.current_capital
        
        for position in self.positions.values():
            equity += position.quantity * position.current_price
        
        return equity
    
    # ========================================================================
    # Mock Data Provider Interface (for strategy)
    # ========================================================================
    
    def get_current_price(self, symbol: str) -> Optional[float]:
        """Get current price for a symbol"""
        price = self.current_prices.get(symbol)
        if price is not None:
            logger.info(f"BacktestEngine: Returning current price for {symbol}: ${price:.2f} at {self.current_time}")
        else:
            logger.warning(f"BacktestEngine: No current price available for {symbol} at {self.current_time}")
        return price
    
    def get_market_data(self, symbol: str) -> Dict[str, Any]:
        """Get current market data for a symbol"""
        if symbol not in self.current_prices:
            return {}
        
        return {
            "symbol": symbol,
            "price": self.current_prices[symbol],
            "timestamp": self.current_time.isoformat() if self.current_time else None
        }
    
    # ========================================================================
    # Mock Order Executor Interface (for strategy)
    # ========================================================================
    
    def place_market_order(
        self,
        symbol: str,
        quantity: int,
        side: str,
        reason: str = ""
    ) -> str:
        """Simulate market order execution"""
        if symbol not in self.current_prices:
            logger.warning(f"No price data for {symbol}")
            return ""
        
        # Calculate execution price with slippage
        base_price = self.current_prices[symbol]
        slippage_factor = self.slippage_bps / 10000.0
        
        if side.upper() in ["BUY", "BUY_TO_OPEN"]:
            execution_price = base_price * (1 + slippage_factor)
        else:
            execution_price = base_price * (1 - slippage_factor)
        
        # Generate trade
        trade_id = f"TRADE_{len(self.trades) + 1}_{self.current_time.timestamp()}"
        
        trade = BacktestTrade(
            timestamp=self.current_time,
            symbol=symbol,
            action=side.upper(),
            quantity=quantity,
            price=execution_price,
            order_type="MARKET",
            trade_id=trade_id,
            strategy_action=reason,
            commission=self.commission_per_trade
        )
        
        # Execute the trade
        self._execute_trade(trade)
        
        logger.info(f"Executed trade: {side} {quantity} {symbol} @ ${execution_price:.2f}")
        return trade_id
    
    def _execute_trade(self, trade: BacktestTrade):
        """Execute a trade and update positions"""
        symbol = trade.symbol
        
        # Calculate trade value
        trade_value = trade.quantity * trade.price
        
        # Update capital (subtract for buys, add for sells)
        if trade.action in ["BUY", "BUY_TO_OPEN"]:
            self.current_capital -= trade_value + trade.commission
        else:
            self.current_capital += trade_value - trade.commission
        
        # Update positions
        if symbol not in self.positions:
            self.positions[symbol] = BacktestPosition(
                symbol=symbol,
                quantity=0,
                avg_price=0.0,
                current_price=trade.price,
                unrealized_pnl=0.0,
                realized_pnl=0.0
            )
        
        position = self.positions[symbol]
        
        if trade.action in ["BUY", "BUY_TO_OPEN"]:
            # Adding to position
            total_cost = (position.quantity * position.avg_price) + (trade.quantity * trade.price)
            position.quantity += trade.quantity
            position.avg_price = total_cost / position.quantity if position.quantity > 0 else 0
        else:
            # Reducing position
            if position.quantity >= trade.quantity:
                # Calculate realized P&L
                realized_pnl = (trade.price - position.avg_price) * trade.quantity
                trade.pnl = realized_pnl
                position.realized_pnl += realized_pnl
                position.quantity -= trade.quantity
                
                if position.quantity == 0:
                    position.avg_price = 0.0
            else:
                logger.warning(f"Insufficient position for {symbol}: have {position.quantity}, trying to sell {trade.quantity}")
        
        # Add trade to history
        self.trades.append(trade)
    
    # ========================================================================
    # Metrics Calculation
    # ========================================================================
    
    def _calculate_metrics(
        self,
        start_date: datetime,
        end_date: datetime,
        strategy: BaseStrategy
    ) -> BacktestMetrics:
        """Calculate comprehensive backtest metrics"""
        
        # Basic metrics
        duration_days = (end_date - start_date).days
        total_trades = len(self.trades)
        
        # P&L metrics
        total_pnl = sum(trade.pnl for trade in self.trades)
        final_equity = self._calculate_current_equity()
        total_return = (final_equity - self.initial_capital) / self.initial_capital
        
        # Trade analysis
        winning_trades = len([t for t in self.trades if t.pnl > 0])
        losing_trades = len([t for t in self.trades if t.pnl < 0])
        win_rate = winning_trades / total_trades if total_trades > 0 else 0
        
        trade_pnls = [t.pnl for t in self.trades if t.pnl != 0]
        max_profit = max(trade_pnls) if trade_pnls else 0
        max_loss = min(trade_pnls) if trade_pnls else 0
        largest_win = max([t.pnl for t in self.trades]) if self.trades else 0
        largest_loss = min([t.pnl for t in self.trades]) if self.trades else 0
        
        # Risk metrics
        equity_values = [equity for _, equity in self.equity_curve]
        max_drawdown = self._calculate_max_drawdown(equity_values)
        
        # Calculate ratios (simplified)
        returns = np.diff(equity_values) / equity_values[:-1] if len(equity_values) > 1 else [0]
        sharpe_ratio = np.mean(returns) / np.std(returns) * np.sqrt(252) if np.std(returns) > 0 else 0
        
        # Sortino ratio (downside deviation)
        negative_returns = [r for r in returns if r < 0]
        downside_std = np.std(negative_returns) if negative_returns else 0.001
        sortino_ratio = np.mean(returns) / downside_std * np.sqrt(252) if downside_std > 0 else 0
        
        # Calmar ratio
        calmar_ratio = total_return / abs(max_drawdown) if max_drawdown != 0 else 0
        
        # Position metrics
        max_positions = max([len(self.positions)] + [0])
        avg_position_size = np.mean([abs(t.quantity * t.price) for t in self.trades]) if self.trades else 0
        
        # Action metrics
        action_stats = strategy.action_executor.get_execution_stats()
        
        return BacktestMetrics(
            start_date=start_date,
            end_date=end_date,
            duration_days=duration_days,
            total_trades=total_trades,
            winning_trades=winning_trades,
            losing_trades=losing_trades,
            win_rate=win_rate,
            total_pnl=total_pnl,
            total_return=total_return,
            max_profit=max_profit,
            max_loss=max_loss,
            largest_win=largest_win,
            largest_loss=largest_loss,
            max_drawdown=max_drawdown,
            max_drawdown_duration=0,  # Simplified
            sharpe_ratio=sharpe_ratio,
            sortino_ratio=sortino_ratio,
            calmar_ratio=calmar_ratio,
            max_positions=max_positions,
            avg_position_size=avg_position_size,
            avg_holding_period=0,  # Simplified
            total_actions=action_stats.get("total_executed", 0),
            successful_actions=action_stats.get("successful", 0),
            failed_actions=action_stats.get("failed", 0),
            action_success_rate=action_stats.get("successful", 0) / max(action_stats.get("total_executed", 1), 1)
        )
    
    def _calculate_max_drawdown(self, equity_values: List[float]) -> float:
        """Calculate maximum drawdown"""
        if len(equity_values) < 2:
            return 0.0
        
        peak = equity_values[0]
        max_dd = 0.0
        
        for value in equity_values[1:]:
            if value > peak:
                peak = value
            
            drawdown = (peak - value) / peak
            max_dd = max(max_dd, drawdown)
        
        return max_dd
    
    async def _process_strategy_trade_actions(self, strategy: BaseStrategy):
        """
        Process trade actions that were queued by the strategy.
        
        This is the critical bridge between the strategy's action-based framework
        and the backtest engine's trade execution system.
        """
        try:
            # Get all trade actions from the strategy's action queue
            trade_actions = []
            for action in strategy.action_queue.get_active_actions():
                if isinstance(action, TradeAction) and not action.is_completed():
                    trade_actions.append(action)
            
            # Process each trade action
            for trade_action in trade_actions:
                try:
                    # Map trade action to backtest engine execution
                    symbol = trade_action.symbol
                    quantity = trade_action.quantity
                    trade_type = trade_action.trade_type.upper()
                    reason = trade_action.name
                    
                    # Map trade types to standard order sides
                    side_mapping = {
                        "BUY": "BUY",
                        "BUY_TO_OPEN": "BUY",
                        "SELL": "SELL",
                        "SELL_TO_CLOSE": "SELL",
                        "SELL_SHORT": "SELL",
                        "SELL_TO_OPEN": "SELL",
                        "BUY_TO_COVER": "BUY"
                    }
                    
                    side = side_mapping.get(trade_type, "BUY")
                    
                    # Execute the trade through the backtest engine
                    trade_id = self.place_market_order(
                        symbol=symbol,
                        quantity=quantity,
                        side=side,
                        reason=f"Strategy Action: {reason}"
                    )
                    
                    # Mark the action as completed
                    trade_action.mark_completed()
                    
                    logger.info(f"✅ FRAMEWORK: Processed trade action {reason} -> Trade ID: {trade_id}")
                    
                except Exception as e:
                    logger.error(f"Error processing trade action {trade_action.name}: {e}")
                    trade_action.mark_failed(str(e))
                    
        except Exception as e:
            logger.error(f"Error processing strategy trade actions: {e}")

    async def _close_all_positions_at_end(self, end_date: datetime):
        """
        Close all open positions at the end of the backtest period.
        This ensures that unrealized P&L is captured in the final results.
        """
        logger.info("Closing all open positions at end of backtest")
        
        positions_to_close = [(symbol, pos) for symbol, pos in self.positions.items() if pos.quantity != 0]
        
        if not positions_to_close:
            logger.info("No open positions to close")
            return
        
        for symbol, position in positions_to_close:
            if symbol not in self.current_prices:
                logger.warning(f"No current price for {symbol}, cannot close position")
                continue
            
            # Determine the closing action
            if position.quantity > 0:
                # Close long position
                side = "SELL"
                quantity = position.quantity
            else:
                # Close short position
                side = "BUY_TO_COVER"
                quantity = abs(position.quantity)
            
            # Execute closing trade at current market price
            closing_price = self.current_prices[symbol]
            
            # Create closing trade
            trade_id = f"CLOSE_EOB_{symbol}_{end_date.timestamp()}"
            
            trade = BacktestTrade(
                timestamp=end_date,
                symbol=symbol,
                action=side,
                quantity=quantity,
                price=closing_price,
                order_type="MARKET",
                trade_id=trade_id,
                strategy_action="End of backtest position closure",
                commission=self.commission_per_trade
            )
            
            # Calculate realized P&L for the closing trade
            if position.quantity > 0:
                # Closing long position
                realized_pnl = (closing_price - position.avg_price) * quantity
            else:
                # Closing short position
                realized_pnl = (position.avg_price - closing_price) * quantity
            
            trade.pnl = realized_pnl
            
            # Execute the trade
            self._execute_trade(trade)
            
            logger.info(f"Closed position: {side} {quantity} {symbol} @ ${closing_price:.2f}, P&L: ${realized_pnl:.2f}")
        
        logger.info(f"Closed {len(positions_to_close)} positions at end of backtest")

    def _serialize_json_safe(self, data: Any) -> Any:
        """Convert data to JSON-safe format by handling datetime objects and all boolean types"""
        if data is None:
            return None
        elif isinstance(data, dict):
            return {key: self._serialize_json_safe(value) for key, value in data.items()}
        elif isinstance(data, list):
            return [self._serialize_json_safe(item) for item in data]
        elif isinstance(data, tuple):
            return [self._serialize_json_safe(item) for item in data]
        elif hasattr(data, 'isoformat'):
            # Handle datetime objects (including pandas Timestamp)
            return data.isoformat()
        elif hasattr(data, 'to_pydatetime'):
            # Handle pandas Timestamp specifically
            return data.to_pydatetime().isoformat()
        elif isinstance(data, (pd.Timestamp, pd.DatetimeIndex)):
            # Handle pandas datetime types
            return str(data)
        elif hasattr(data, 'dtype') and 'bool' in str(data.dtype):
            # Handle numpy boolean types
            return bool(data)
        elif str(type(data)).startswith('<class \'numpy.bool'):
            # Handle numpy boolean types (alternative check)
            return bool(data)
        elif hasattr(data, 'item') and callable(data.item):
            # Handle numpy scalars
            try:
                return data.item()
            except (ValueError, TypeError):
                return str(data)
        elif isinstance(data, (bool, int, float, str)):
            # Handle basic Python types
            return data
        else:
            # Convert anything else to string as fallback
            try:
                # Try to convert to basic Python type first
                if hasattr(data, '__bool__'):
                    return bool(data)
                elif hasattr(data, '__int__'):
                    return int(data)
                elif hasattr(data, '__float__'):
                    return float(data)
                else:
                    return str(data)
            except (ValueError, TypeError):
                return str(data)

# ============================================================================
# Global Backtest Engine Instance
# ============================================================================

# Default backtest engine
backtest_engine = StrategyBacktestEngine()
