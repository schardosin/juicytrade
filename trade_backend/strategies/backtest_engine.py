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

class HistoricalDataProvider:
    """
    Mock historical data provider for backtesting.
    In production, this would connect to your actual data source.
    """
    
    def __init__(self):
        self.data_cache = {}
    
    def get_historical_data(
        self,
        symbol: str,
        start_date: datetime,
        end_date: datetime,
        interval: str = "1min"
    ) -> pd.DataFrame:
        """
        Get historical OHLCV data for a symbol.
        
        Returns DataFrame with columns: timestamp, open, high, low, close, volume
        """
        # Mock implementation - generates realistic-looking data
        date_range = pd.date_range(start=start_date, end=end_date, freq="1min")
        
        # Filter to market hours (9:30 AM - 4:00 PM ET)
        market_hours = date_range[
            (date_range.time >= pd.Timestamp("09:30").time()) &
            (date_range.time <= pd.Timestamp("16:00").time()) &
            (date_range.weekday < 5)  # Monday=0, Friday=4
        ]
        
        if len(market_hours) == 0:
            return pd.DataFrame()
        
        # Generate mock price data
        np.random.seed(hash(symbol) % 2**32)  # Consistent data for same symbol
        
        base_price = 100.0 if symbol == "SPY" else 50.0
        returns = np.random.normal(0, 0.001, len(market_hours))  # 0.1% volatility per minute
        prices = base_price * np.exp(np.cumsum(returns))
        
        # Generate OHLCV data
        data = []
        for i, timestamp in enumerate(market_hours):
            price = prices[i]
            noise = np.random.normal(0, 0.0005)  # Small noise for OHLC
            
            open_price = price + noise
            high_price = price + abs(noise) + np.random.exponential(0.001)
            low_price = price - abs(noise) - np.random.exponential(0.001)
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
        market_type: MarketType = MarketType.STOCK
    ):
        self.initial_capital = initial_capital
        self.commission_per_trade = commission_per_trade
        self.slippage_bps = slippage_bps
        self.market_type = market_type
        
        # Data provider
        self.data_provider = HistoricalDataProvider()
        
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
            
            # Generate results
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
                "equity_curve": [{"timestamp": ts.isoformat(), "equity": equity} for ts, equity in self.equity_curve],
                "action_log": strategy.get_action_log(),
                "checkpoints": strategy.get_checkpoints(),
                "state_history": strategy.get_state_history()
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
            
            data = self.data_provider.get_historical_data(
                symbol, start_date, end_date, "1min"
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
        for symbol_data in self.market_data_cache.values():
            all_timestamps.update(symbol_data['timestamp'])
        
        timestamps = sorted(all_timestamps)
        
        if not timestamps:
            logger.warning("No market data timestamps found")
            return
        
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
            
            # Record equity curve (every 100 periods to avoid too much data)
            if i % 100 == 0:
                current_equity = self._calculate_current_equity()
                self.equity_curve.append((timestamp, current_equity))
            
            # Add small delay for very fast backtests to prevent overwhelming
            if speed_multiplier < 1000:
                await asyncio.sleep(0.001 / speed_multiplier)
        
        # Stop the strategy
        await strategy.stop()
        
        # Final equity calculation
        final_equity = self._calculate_current_equity()
        self.equity_curve.append((end_date, final_equity))
        
        logger.info(f"Simulation completed: Final equity ${final_equity:,.2f}")
    
    def _update_current_prices(self, timestamp: datetime):
        """Update current prices for all symbols"""
        for symbol, data in self.market_data_cache.items():
            # Find the closest price data for this timestamp
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
        return self.current_prices.get(symbol)
    
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

# ============================================================================
# Global Backtest Engine Instance
# ============================================================================

# Default backtest engine
backtest_engine = StrategyBacktestEngine()
