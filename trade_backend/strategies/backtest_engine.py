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
    legs: Optional[List[Dict[str, Any]]] = None  # For grouped UI display
    status: str = "filled"  # For UI compatibility
    avg_fill_price: Optional[float] = None  # For UI compatibility
    limit_price: Optional[float] = None  # For UI compatibility
    
    def to_dict(self) -> Dict[str, Any]:
        result = {
            "timestamp": self.timestamp.isoformat(),
            "symbol": self.symbol,
            "action": self.action,
            "quantity": self.quantity,
            "price": self.price,
            "order_type": self.order_type,
            "trade_id": self.trade_id,
            "strategy_action": self.strategy_action,
            "pnl": self.pnl,
            "commission": self.commission,
            "status": self.status,
            "avg_fill_price": self.avg_fill_price or self.price,
            "limit_price": self.limit_price or self.price,
            # UI compatibility fields (mapping backend fields to UI expected names)
            "id": self.trade_id,  # UI expects 'id' field
            "submitted_at": self.timestamp.isoformat(),  # UI expects 'submitted_at' field
            "qty": self.quantity,  # UI expects 'qty' field
            "side": self.action.lower()  # UI expects 'side' field
        }
        
        # Add legs for grouped display if available
        if self.legs:
            result["legs"] = self.legs
            
        return result

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
    Real historical data provider for backtesting using imported parquet data.
    
    This provider uses the CBBODataService for raw bid/ask data (no OHLCV conversion)
    and DataAggregationService for OHLCV data when needed.
    """
    
    def __init__(self, aggregation_service=None, provider_manager=None, cbbo_service=None):
        self.aggregation_service = aggregation_service
        self.cbbo_service = cbbo_service
        self.provider_manager = provider_manager  # Fallback for compatibility
        self.data_cache = {}
    
    async def get_historical_data(
        self,
        symbol: str,
        start_date: datetime,
        end_date: datetime,
        interval: str = "1min"
    ) -> pd.DataFrame:
        """
        Get historical data for a symbol using imported parquet data ONLY.
        
        🚨 BACKTEST MODE: NO FALLBACKS - Only uses imported parquet data
        
        Priority for timeframe-based backtests:
        1. Aggregation service for OHLCV data (preferred for timeframe aggregation)
        2. CBBO service for raw bid/ask data (only for tick-level analysis)
        
        Returns DataFrame with columns: timestamp, open, high, low, close, volume OR timestamp, bid, ask, mid, spread
        """
        logger.info(f"🎯 DATA LOADING: Requesting {symbol} data with interval '{interval}'")
        
        # For timeframe-based backtests, prioritize aggregation service
        # CBBO service returns raw tick data which is too granular for most backtests
        if self.aggregation_service and interval != "tick":
            logger.info(f"🎯 Using aggregation service for timeframe-based data: {interval}")
            try:
                return await self._get_parquet_data(symbol, start_date, end_date, interval)
            except Exception as e:
                logger.warning(f"Aggregation service failed for {symbol}: {e}, falling back to CBBO service")
        
        # Fallback to CBBO service for tick-level data or when aggregation fails
        if self.cbbo_service:
            logger.info(f"🎯 Using CBBO service for raw tick data")
            try:
                return await self._get_cbbo_data(symbol, start_date, end_date, interval)
            except Exception as e:
                logger.error(f"CBBO service failed for {symbol}: {e}")
        
        # NO FALLBACKS IN BACKTEST MODE
        logger.error(f"❌ BACKTEST FAILED: No data services available for {symbol}")
        raise ValueError(f"Backtest requires CBBO or DataAggregation service with imported parquet data. No services configured.")
    
    async def _get_cbbo_data(
        self,
        symbol: str,
        start_date: datetime,
        end_date: datetime,
        interval: str = "1min"
    ) -> pd.DataFrame:
        """
        Get historical CBBO data using CBBODataService.
        
        Returns DataFrame with columns: timestamp, bid, ask, mid, spread, bid_size, ask_size
        """
        try:
            logger.info(f"Requesting CBBO data for {symbol} from {start_date} to {end_date}")
            
            # Get CBBO data from service (not async)
            cbbo_data = self.cbbo_service.get_cbbo_data(
                symbol=symbol,
                start_date=start_date.strftime('%Y-%m-%d'),
                end_date=end_date.strftime('%Y-%m-%d')
            )
            
            if not cbbo_data:
                logger.error(f"❌ BACKTEST FAILED: No CBBO data available for {symbol}")
                raise ValueError(f"No CBBO data available for {symbol}. Backtest requires imported CBBO data.")
            
            # Convert CBBOData objects to DataFrame
            data = []
            for cbbo_point in cbbo_data:
                data.append({
                    "timestamp": cbbo_point.timestamp,
                    "bid": cbbo_point.bid,
                    "ask": cbbo_point.ask,
                    "mid": cbbo_point.mid,
                    "spread": cbbo_point.spread,
                    "bid_size": cbbo_point.bid_size,
                    "ask_size": cbbo_point.ask_size,
                    # For compatibility with existing backtest engine, also provide 'close' as mid price
                    "close": cbbo_point.mid
                })
            
            df = pd.DataFrame(data)
            
            if not df.empty:
                # Ensure timestamp is datetime
                df['timestamp'] = pd.to_datetime(df['timestamp'])
                
                # Sort by timestamp
                df = df.sort_values('timestamp').reset_index(drop=True)
                
                logger.info(f"Retrieved {len(df)} CBBO data points for {symbol}")
                logger.info(f"Data range: {df['timestamp'].min()} to {df['timestamp'].max()}")
                
                # Log first few data points for verification
                if len(df) > 0:
                    first_row = df.iloc[0]
                    logger.info(f"First CBBO: {first_row['timestamp']} Bid/Ask: {first_row['bid']:.2f}/{first_row['ask']:.2f} Mid: {first_row['mid']:.2f} Spread: {first_row['spread']:.4f}")
                if len(df) > 1:
                    second_row = df.iloc[1]
                    logger.info(f"Second CBBO: {second_row['timestamp']} Bid/Ask: {second_row['bid']:.2f}/{second_row['ask']:.2f} Mid: {second_row['mid']:.2f} Spread: {second_row['spread']:.4f}")
            
            return df
            
        except Exception as e:
            logger.error(f"❌ BACKTEST FAILED: Error getting CBBO data for {symbol}: {e}")
            raise ValueError(f"Backtest requires CBBO data for {symbol}. Error: {e}")

    async def _get_parquet_data(
        self,
        symbol: str,
        start_date: datetime,
        end_date: datetime,
        interval: str = "1min"
    ) -> pd.DataFrame:
        """
        Get historical data from imported parquet files using DataAggregationService.
        
        This method leverages the powerful aggregation system to provide:
        - High-quality imported data (DBN + CSV)
        - Perfect time alignment (9:30, 9:35, 9:40...)
        - Flexible timeframes (1min, 5min, 15min, 30min, 1hr, daily)
        - Market hours filtering
        - Automatic price scaling correction
        """
        try:
            # Import the aggregation models
            from ..services.data_aggregation.models import AggregationRequest
            
            # Map backtest intervals to aggregation service timeframes
            timeframe_map = {
                # Frontend format -> Backend format
                "1m": "1min",
                "5m": "5min",
                "15m": "15min", 
                "30m": "30min",
                "1h": "1hr",
                "4h": "1hr",  # Map 4h to 1hr (closest available)
                "D": "daily",
                # Legacy formats
                "1min": "1min",
                "5min": "5min", 
                "15min": "15min",
                "30min": "30min",
                "1hour": "1hr",
                "1hr": "1hr",
                "1day": "daily",
                "daily": "daily"
            }
            
            aggregation_timeframe = timeframe_map.get(interval, "1min")
            
            logger.info(f"🎯 TIMEFRAME MAPPING: '{interval}' -> '{aggregation_timeframe}'")
            
            # Create aggregation request
            request = AggregationRequest(
                symbol=symbol,
                timeframe=aggregation_timeframe,
                start_date=start_date.strftime('%Y-%m-%d'),
                end_date=end_date.strftime('%Y-%m-%d'),
                market_hours_only=True,  # Use market hours filtering
                limit=None  # Get all available data
            )
            
            logger.info(f"Requesting parquet data for {symbol} ({aggregation_timeframe}) from {request.start_date} to {request.end_date}")
            
            # Get aggregated data
            result = self.aggregation_service.get_aggregated_data(request)
            
            # Check if we got data (AggregatedData object structure)
            if not hasattr(result, 'data') or not result.data:
                logger.error(f"❌ BACKTEST FAILED: No parquet data available for {symbol} ({aggregation_timeframe})")
                logger.error(f"❌ Available symbols: {self.aggregation_service.get_available_symbols() if self.aggregation_service else 'N/A'}")
                raise ValueError(f"No parquet data available for {symbol}. Backtest requires imported data.")
            
            # Convert aggregated data to DataFrame format expected by backtest engine
            data = []
            for bar in result.data:
                # Convert OHLCVData to dictionary format
                data.append({
                    "timestamp": bar.timestamp,
                    "open": bar.open,
                    "high": bar.high,
                    "low": bar.low,
                    "close": bar.close,
                    "volume": bar.volume
                })
            
            df = pd.DataFrame(data)
            
            if not df.empty:
                # Ensure timestamp is datetime
                df['timestamp'] = pd.to_datetime(df['timestamp'])
                
                # Sort by timestamp
                df = df.sort_values('timestamp').reset_index(drop=True)
                
                logger.info(f"Retrieved {len(df)} parquet data points for {symbol} ({aggregation_timeframe})")
                logger.info(f"Data range: {df['timestamp'].min()} to {df['timestamp'].max()}")
                
                # Log first few data points for verification
                if len(df) > 0:
                    logger.info(f"First bar: {df.iloc[0]['timestamp']} OHLCV: {df.iloc[0]['open']:.2f}/{df.iloc[0]['high']:.2f}/{df.iloc[0]['low']:.2f}/{df.iloc[0]['close']:.2f}/{df.iloc[0]['volume']}")
                if len(df) > 1:
                    logger.info(f"Second bar: {df.iloc[1]['timestamp']} OHLCV: {df.iloc[1]['open']:.2f}/{df.iloc[1]['high']:.2f}/{df.iloc[1]['low']:.2f}/{df.iloc[1]['close']:.2f}/{df.iloc[1]['volume']}")
            
            return df
            
        except Exception as e:
            logger.error(f"❌ BACKTEST FAILED: Error getting parquet data for {symbol}: {e}")
            raise ValueError(f"Backtest requires parquet data for {symbol}. Error: {e}")
    
    async def _get_broker_data(
        self,
        symbol: str,
        start_date: datetime,
        end_date: datetime,
        interval: str = "1min"
    ) -> pd.DataFrame:
        """
        Get historical data from broker (original implementation as fallback).
        """
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
                logger.warning(f"No broker data returned for {symbol}")
                return pd.DataFrame()
            
            # Convert bars to DataFrame
            data = []
            for i, bar in enumerate(bars):
                # Debug: Log first few bars to see the structure
                if i < 3:
                    logger.info(f"Broker bar {i}: {bar}")
                
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
                    logger.warning(f"No timestamp found in broker bar: {list(bar.keys())}")
                
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
                logger.info(f"Retrieved {len(df)} broker data points for {symbol}")
            
            return df
            
        except Exception as e:
            logger.error(f"Error getting broker data for {symbol}: {e}")
            return pd.DataFrame()
    
    async def _try_fallback_data(
        self,
        symbol: str,
        start_date: datetime,
        end_date: datetime,
        interval: str = "1min"
    ) -> pd.DataFrame:
        """
        Try fallback data sources in order of preference.
        """
        # Try broker data if available
        if self.provider_manager:
            logger.info(f"Trying broker data fallback for {symbol}")
            broker_data = await self._get_broker_data(symbol, start_date, end_date, interval)
            if not broker_data.empty:
                return broker_data
        
        # Final fallback to mock data
        logger.info(f"Using mock data fallback for {symbol}")
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
        aggregation_service=None,  # Support for OHLCV data
        cbbo_service=None,  # NEW: Support for CBBO data (preferred)
        timeframe: str = "1min"  # Add configurable timeframe
    ):
        self.initial_capital = initial_capital
        self.commission_per_trade = commission_per_trade
        self.slippage_bps = slippage_bps
        self.market_type = market_type
        self.timeframe = timeframe  # Store timeframe for data fetching
        
        # Data provider - prioritize CBBO service for options strategies
        self.data_provider = RealHistoricalDataProvider(
            aggregation_service=aggregation_service,
            cbbo_service=cbbo_service,
            provider_manager=provider_manager
        )
        
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
            
            # Load historical data (with strategy for additional symbols)
            await self._load_historical_data(symbols, start_date, end_date, strategy)
            
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
        end_date: datetime,
        strategy: BaseStrategy = None
    ):
        """Load historical data for all symbols (primary + additional from strategy)"""
        # Get additional symbols from strategy if provided
        all_symbols = set(symbols)
        if strategy and hasattr(strategy, 'get_additional_symbols'):
            additional_symbols = strategy.get_additional_symbols()
            if additional_symbols:
                all_symbols.update(additional_symbols)
                logger.info(f"Strategy registered additional symbols: {additional_symbols}")
        
        logger.info(f"Loading data for {len(all_symbols)} symbols: {sorted(all_symbols)}")
        
        for symbol in all_symbols:
            logger.info(f"Loading historical data for {symbol} with timeframe: {self.timeframe}")
            
            data = await self.data_provider.get_historical_data(
                symbol, start_date, end_date, self.timeframe
            )
            
            if not data.empty:
                self.market_data_cache[symbol] = data
                logger.info(f"Loaded {len(data)} data points for {symbol} using {self.timeframe} timeframe")
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
        
        # ARCHITECTURAL FIX: Calculate the virtual date for the backtest
        # This is the date the strategy thinks it's "living" on
        # Convert start_date to Eastern Time to get the proper date
        from zoneinfo import ZoneInfo
        eastern_tz = ZoneInfo("America/New_York")
        
        # Convert start_date to Eastern Time and extract date
        if start_date.tzinfo is None:
            import pytz
            start_date_utc = pytz.utc.localize(start_date)
        else:
            start_date_utc = start_date
        
        eastern_date = start_date_utc.astimezone(eastern_tz).date()
        virtual_date = eastern_date.strftime('%Y-%m-%d')
        
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
            
            # ARCHITECTURAL FIX: Inject virtual_date into strategy context
            # The strategy will receive this and never need to calculate "what day is it"
            if hasattr(strategy, '_set_virtual_date'):
                strategy._set_virtual_date(virtual_date)
            
            # Execute strategy cycle
            await strategy.execute_cycle()
            
            # CRITICAL FIX: Process any trade actions that were queued by the strategy
            await self._process_strategy_trade_actions(strategy)
            
            # Record equity curve at every data point for proper drawdown calculation
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
        # Try to get price data for the symbol
        # For options: ALWAYS get fresh price (no caching) since prices change significantly
        if symbol not in self.current_prices or self._is_option_symbol(symbol):
            if self._is_option_symbol(symbol):
                price = self._get_option_contract_price(symbol)
                if price is not None:
                    self.current_prices[symbol] = price
                    logger.info(f"Fresh option price: {symbol} = ${price:.4f} at {self.current_time}")
                else:
                    logger.warning(f"No price data available for option contract: {symbol}")
                    return ""
            else:
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

    def _is_option_symbol(self, symbol: str) -> bool:
        """Check if symbol is an option contract symbol"""
        # Option symbols are typically longer and contain specific patterns
        # Example: "SPY   250812P00639000" (SPY options)
        return len(symbol) > 10 and ('C' in symbol[-9:] or 'P' in symbol[-9:])

    def _get_option_contract_price(self, option_symbol: str) -> Optional[float]:
        """
        Get price for an individual option contract using PriceQueryService.
        
        This method uses the PriceQueryService to find the closest price before
        the current backtest time for the specific option contract.
        """
        try:
            # Import the price query service
            from ..services.data_aggregation.price_query_service import get_price_query_service
            
            price_service = get_price_query_service()
            
            # Get price data for this option contract at current backtest time
            price_data = price_service.get_price_before(option_symbol, self.current_time)
            
            if price_data:
                # Use mid price (average of bid/ask) for execution
                mid_price = (price_data.bid + price_data.ask) / 2.0 if price_data.bid > 0 and price_data.ask > 0 else 0.0
                
                if mid_price > 0:
                    logger.info(f"Found option price: {option_symbol} bid={price_data.bid:.2f} ask={price_data.ask:.2f} mid={mid_price:.2f}")
                    return mid_price
                else:
                    logger.warning(f"Invalid option price data: {option_symbol} bid={price_data.bid} ask={price_data.ask}")
                    return None
            else:
                logger.warning(f"No price data found for option contract: {option_symbol}")
                return None
                
        except Exception as e:
            logger.error(f"Error getting option contract price for {option_symbol}: {e}")
            return None

    def place_options_order(self, legs: List, order_type: str = "market", strategy_id: str = '') -> str:
        """
        Place an options order with multiple legs (Iron Condor, etc.) in backtest mode.
        
        FIXED: Creates grouped trades for UI display like in the image.
        Creates one grouped trade with legs array for UI to display properly.

        Args:
            legs: List of OptionsLeg objects defining the multi-leg order
            order_type: Type of order ("market", "limit", etc.)
            strategy_id: ID of the strategy placing the order

        Returns:
            Composite order ID if successful, empty string if failed
        """
        try:
            if not legs:
                logger.error("No legs provided for options order")
                return ''

            composite_order_id = f"OPTIONS_{len(legs)}LEG_{self.current_time.timestamp()}"
            
            logger.info(f"🎯 BACKTEST: Executing {len(legs)}-leg options order with UI grouping")
            
            # Execute each leg and collect data for grouped trade
            leg_data = []
            total_net_premium = 0.0
            all_legs_executed = True
            
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
                    
                    # Get fresh price for this leg
                    if self._is_option_symbol(symbol):
                        price = self._get_option_contract_price(symbol)
                        if price is None:
                            logger.error(f"❌ BACKTEST: No price for leg {i+1}: {symbol}")
                            all_legs_executed = False
                            break
                    else:
                        logger.error(f"❌ BACKTEST: Invalid option symbol: {symbol}")
                        all_legs_executed = False
                        break
                    
                    # Update positions and capital directly (no individual trades)
                    self._update_position_for_leg(symbol, quantity, side, price)
                    
                    # Track net premium for group (FIXED: Use original leg action)
                    # Apply options multiplier (100 shares per contract)
                    options_multiplier = 100
                    if leg.action.lower() in ['sell', 'sell_to_open']:
                        total_net_premium += price * quantity * options_multiplier  # Premium collected (positive)
                    else:
                        total_net_premium -= price * quantity * options_multiplier  # Premium paid (negative)
                    
                    # Store leg data for UI display (matches ActivitySection.vue format)
                    leg_data.append({
                        'symbol': symbol,
                        'side': leg.action.lower(),  # Use original action from strategy
                        'qty': quantity,  # Always positive quantity
                        'price': price
                    })
                    
                    logger.info(f"✅ BACKTEST: Executed leg {i+1}: {side} {quantity} {symbol} @ ${price:.4f}")
                        
                except Exception as e:
                    logger.error(f"Error executing options leg {i+1}: {e}")
                    all_legs_executed = False
                    break
            
            if all_legs_executed:
                # Extract underlying symbol from first leg for header
                underlying_symbol = ""
                if legs and hasattr(legs[0], 'contract') and hasattr(legs[0].contract, 'symbol'):
                    option_symbol = legs[0].contract.symbol
                    if self._is_option_symbol(option_symbol):
                        import re
                        underlying_match = re.match(r'^([A-Z]+)', option_symbol)
                        if underlying_match:
                            underlying_symbol = underlying_match.group(1)

                # Create ONE grouped trade for UI display (like in your image)
                # For credit spreads, net premium should be negative (we collect money)
                net_credit = -total_net_premium  # Invert sign for credit display
                
                grouped_trade = BacktestTrade(
                    timestamp=self.current_time,
                    symbol=underlying_symbol,  # Use extracted underlying symbol for header
                    action="STRATEGY_ORDER",
                    quantity=len(legs),  # Number of legs
                    price=net_credit,  # Net credit for the strategy (negative = credit)
                    order_type="MARKET",
                    trade_id=composite_order_id,
                    strategy_action=f"Iron Condor {len(legs)}-Leg Order",
                    pnl=total_net_premium,  # Set P&L to the net premium collected/paid
                    commission=self.commission_per_trade,
                    legs=leg_data,  # Individual legs for UI display
                    status="filled",
                    avg_fill_price=net_credit,
                    limit_price=net_credit
                )
                
                # Add to trades list (this will show in UI as grouped order)
                self.trades.append(grouped_trade)
                
                logger.info(f"🎉 BACKTEST: {len(legs)}-leg options order executed with UI grouping")
                logger.info(f"   Composite Order ID: {composite_order_id}")
                logger.info(f"   Net Premium: ${total_net_premium:.2f}")
                
                return composite_order_id
            else:
                logger.error(f"❌ BACKTEST: Failed to execute all legs")
                return ''
                
        except Exception as e:
            logger.error(f"Error executing options order in backtest: {e}")
            return ''
    
    def _execute_trade(self, trade: BacktestTrade):
        """Execute a trade and update positions"""
        symbol = trade.symbol
        
        # Calculate trade value with options multiplier
        options_multiplier = 100 if self._is_option_symbol(symbol) else 1
        trade_value = trade.quantity * trade.price * options_multiplier
        
        # Update capital (subtract for buys, add for sells)
        if trade.action in ["BUY", "BUY_TO_OPEN"]:
            self.current_capital -= trade_value + trade.commission
            # For options, buying means paying premium (immediate cash outflow)
            if self._is_option_symbol(trade.symbol):
                trade.pnl = -trade_value  # Negative P&L for premium paid
        else:
            self.current_capital += trade_value - trade.commission
            # For options, selling means collecting premium (immediate cash inflow)
            if self._is_option_symbol(trade.symbol):
                trade.pnl = trade_value   # Positive P&L for premium collected
        
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
            # Adding to position (long)
            if position.quantity >= 0:
                # Adding to existing long position or creating new long position
                total_cost = (position.quantity * position.avg_price) + (trade.quantity * trade.price)
                position.quantity += trade.quantity
                position.avg_price = total_cost / position.quantity if position.quantity > 0 else 0
            else:
                # Covering short position
                if abs(position.quantity) >= trade.quantity:
                    # Partial or full cover
                    realized_pnl = (position.avg_price - trade.price) * trade.quantity
                    trade.pnl = realized_pnl
                    position.realized_pnl += realized_pnl
                    position.quantity += trade.quantity
                    
                    if position.quantity == 0:
                        position.avg_price = 0.0
                else:
                    # Over-covering (short to long)
                    cover_quantity = abs(position.quantity)
                    realized_pnl = (position.avg_price - trade.price) * cover_quantity
                    trade.pnl = realized_pnl
                    position.realized_pnl += realized_pnl
                    
                    # Remaining quantity becomes new long position
                    remaining_quantity = trade.quantity - cover_quantity
                    position.quantity = remaining_quantity
                    position.avg_price = trade.price
        else:
            # SELL action
            if position.quantity > 0:
                # Reducing long position
                if position.quantity >= trade.quantity:
                    # Partial or full sale
                    realized_pnl = (trade.price - position.avg_price) * trade.quantity
                    trade.pnl = realized_pnl
                    position.realized_pnl += realized_pnl
                    position.quantity -= trade.quantity
                    
                    if position.quantity == 0:
                        position.avg_price = 0.0
                else:
                    # Over-selling (long to short)
                    sell_quantity = position.quantity
                    realized_pnl = (trade.price - position.avg_price) * sell_quantity
                    trade.pnl = realized_pnl
                    position.realized_pnl += realized_pnl
                    
                    # Remaining quantity becomes new short position
                    remaining_quantity = trade.quantity - sell_quantity
                    position.quantity = -remaining_quantity
                    position.avg_price = trade.price
            else:
                # Adding to short position or creating new short position
                if position.quantity <= 0:
                    total_value = (abs(position.quantity) * position.avg_price) + (trade.quantity * trade.price)
                    position.quantity -= trade.quantity
                    position.avg_price = total_value / abs(position.quantity) if position.quantity != 0 else 0
                else:
                    # This shouldn't happen with the logic above, but handle it
                    logger.warning(f"Unexpected position state for {symbol}: quantity={position.quantity}, action={trade.action}")
        
        # Add trade to history
        self.trades.append(trade)
        
        # Log trade execution with P&L
        if trade.pnl != 0:
            logger.info(f"Trade executed with P&L: {trade.action} {trade.quantity} {symbol} @ ${trade.price:.2f}, P&L: ${trade.pnl:.2f}")
        else:
            logger.info(f"Trade executed (opening): {trade.action} {trade.quantity} {symbol} @ ${trade.price:.2f}")
    
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
        duration_days = max((end_date - start_date).days, 1)  # Ensure at least 1 day
        
        # Count all trades including strategy orders (Iron Condor, etc.)
        # For options strategies, each multi-leg order counts as one trade
        total_trades = len(self.trades)
        completed_trades = [t for t in self.trades if t.pnl != 0]
        
        # P&L metrics
        total_pnl = sum(trade.pnl for trade in self.trades)
        final_equity = self._calculate_current_equity()
        total_return = (final_equity - self.initial_capital) / self.initial_capital if self.initial_capital > 0 else 0
        
        # Trade analysis - use completed trades for all trade-based metrics
        winning_trades = len([t for t in completed_trades if t.pnl > 0])
        losing_trades = len([t for t in completed_trades if t.pnl < 0])
        win_rate = winning_trades / total_trades if total_trades > 0 else 0
        
        # P&L analysis - calculate from equity curve for portfolio-level metrics
        equity_values = [equity for _, equity in self.equity_curve]
        if len(equity_values) > 0:
            # Portfolio-level max profit/loss (from initial capital)
            max_equity = max(equity_values)
            min_equity = min(equity_values)
            max_profit = max_equity - self.initial_capital  # Max gain from initial capital
            max_loss = min_equity - self.initial_capital    # Max loss from initial capital (will be negative)
        else:
            max_profit = 0
            max_loss = 0
        
        # Individual trade analysis for largest win/loss
        trade_pnls = [t.pnl for t in completed_trades]
        if trade_pnls:
            largest_win = max(trade_pnls)
            largest_loss = min(trade_pnls)  # This will be negative for losses
        else:
            largest_win = 0
            largest_loss = 0
        
        # Risk metrics
        equity_values = [equity for _, equity in self.equity_curve]
        max_drawdown = self._calculate_max_drawdown(equity_values)
        
        # Calculate ratios with better error handling
        sharpe_ratio = 0
        sortino_ratio = 0
        calmar_ratio = 0
        
        if len(equity_values) > 1:
            # Calculate returns
            returns = []
            for i in range(1, len(equity_values)):
                if equity_values[i-1] > 0:
                    ret = (equity_values[i] - equity_values[i-1]) / equity_values[i-1]
                    returns.append(ret)
            
            if returns:
                mean_return = np.mean(returns)
                std_return = np.std(returns)
                
                # Sharpe ratio (annualized)
                if std_return > 0:
                    sharpe_ratio = mean_return / std_return * np.sqrt(252)
                
                # Sortino ratio (downside deviation)
                negative_returns = [r for r in returns if r < 0]
                if negative_returns:
                    downside_std = np.std(negative_returns)
                    if downside_std > 0:
                        sortino_ratio = mean_return / downside_std * np.sqrt(252)
                
                # Calmar ratio
                if max_drawdown > 0:
                    annualized_return = mean_return * 252
                    calmar_ratio = annualized_return / max_drawdown
        
        # Position metrics
        max_positions = len(self.positions) if self.positions else 0
        avg_position_size = np.mean([abs(t.quantity * t.price) for t in self.trades]) if self.trades else 0
        
        # Action metrics - safely get stats
        try:
            if hasattr(strategy, 'action_executor') and hasattr(strategy.action_executor, 'get_execution_stats'):
                action_stats = strategy.action_executor.get_execution_stats()
            else:
                action_stats = {}
        except Exception as e:
            logger.warning(f"Could not get action stats: {e}")
            action_stats = {}
        
        total_actions = action_stats.get("total_executed", 0)
        successful_actions = action_stats.get("successful", 0)
        failed_actions = action_stats.get("failed", 0)
        action_success_rate = successful_actions / max(total_actions, 1) if total_actions > 0 else 0
        
        # Log metrics for debugging
        logger.info(f"Calculated metrics: trades={total_trades}, pnl=${total_pnl:.2f}, return={total_return:.2%}, win_rate={win_rate:.2%}")
        logger.info(f"Max profit=${max_profit:.2f}, Max loss=${max_loss:.2f}, Drawdown={max_drawdown:.2%}, Sharpe={sharpe_ratio:.2f}")
        
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
            total_actions=total_actions,
            successful_actions=successful_actions,
            failed_actions=failed_actions,
            action_success_rate=action_success_rate
        )
    
    def _calculate_max_drawdown(self, equity_values: List[float]) -> float:
        """Calculate maximum drawdown from peak to trough"""
        if len(equity_values) < 2:
            return 0.0
        
        max_dd = 0.0
        peak = equity_values[0]
        
        for value in equity_values:
            # Update peak if current value is higher
            if value > peak:
                peak = value
            
            # Calculate drawdown from current peak
            if peak > 0:
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
        
        For options strategies, we skip automatic closing since the strategy
        should handle its own position management.
        """
        logger.info("Checking for positions to close at end of backtest")
        
        positions_to_close = [(symbol, pos) for symbol, pos in self.positions.items() if pos.quantity != 0]
        
        if not positions_to_close:
            logger.info("No open positions to close")
            return
        
        # For options strategies, don't auto-close positions
        # The strategy should handle its own closing logic
        options_positions = [(symbol, pos) for symbol, pos in positions_to_close if self._is_option_symbol(symbol)]
        equity_positions = [(symbol, pos) for symbol, pos in positions_to_close if not self._is_option_symbol(symbol)]
        
        if options_positions:
            logger.info(f"Skipping auto-close for {len(options_positions)} options positions - strategy should handle closing")
        
        # Only auto-close equity positions
        for symbol, position in equity_positions:
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
                # Close short position
                realized_pnl = (position.avg_price - closing_price) * quantity
            
            trade.pnl = realized_pnl
            
            # Execute the trade
            self._execute_trade(trade)
            
            logger.info(f"Closed equity position: {side} {quantity} {symbol} @ ${closing_price:.2f}, P&L: ${realized_pnl:.2f}")
        
        if equity_positions:
            logger.info(f"Closed {len(equity_positions)} equity positions at end of backtest")

    def _update_position_for_leg(self, symbol: str, quantity: int, side: str, price: float):
        """Update position for individual leg without creating separate trade record"""
        # Calculate trade value with options multiplier (100 shares per contract)
        options_multiplier = 100 if self._is_option_symbol(symbol) else 1
        trade_value = quantity * price * options_multiplier
        
        # Update capital (subtract for buys, add for sells)
        if side in ["BUY", "BUY_TO_OPEN"]:
            self.current_capital -= trade_value + (self.commission_per_trade / 4)  # Split commission across legs
        else:
            self.current_capital += trade_value - (self.commission_per_trade / 4)
        
        # Update positions
        if symbol not in self.positions:
            self.positions[symbol] = BacktestPosition(
                symbol=symbol,
                quantity=0,
                avg_price=0.0,
                current_price=price,
                unrealized_pnl=0.0,
                realized_pnl=0.0
            )
        
        position = self.positions[symbol]
        
        if side in ["BUY", "BUY_TO_OPEN"]:
            if position.quantity >= 0:
                # Adding to long position
                total_cost = (position.quantity * position.avg_price) + (quantity * price)
                position.quantity += quantity
                position.avg_price = total_cost / position.quantity if position.quantity > 0 else 0
            else:
                # Covering short position
                if abs(position.quantity) >= quantity:
                    position.quantity += quantity
                    if position.quantity == 0:
                        position.avg_price = 0.0
        else:
            if position.quantity > 0:
                # Reducing long position
                position.quantity -= quantity
                if position.quantity == 0:
                    position.avg_price = 0.0
            else:
                # Adding to short position
                if position.quantity <= 0:
                    total_value = (abs(position.quantity) * position.avg_price) + (quantity * price)
                    position.quantity -= quantity
                    position.avg_price = total_value / abs(position.quantity) if position.quantity != 0 else 0

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

# Note: No global instance - each backtest should create its own engine
# to ensure proper initialization and avoid state conflicts
