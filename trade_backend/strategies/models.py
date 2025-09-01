"""
SQLAlchemy models for strategy persistence.
Follows the same patterns as existing system components.
"""

from sqlalchemy import Column, String, Text, Integer, Float, Boolean, DateTime, JSON, ForeignKey
from sqlalchemy.ext.declarative import declarative_base
from sqlalchemy.orm import relationship
from sqlalchemy.sql import func
from datetime import datetime
from typing import Dict, Any, Optional

Base = declarative_base()

class Strategy(Base):
    """
    Strategy model - stores strategy metadata and Python code.
    Follows the same pattern as provider credentials storage.
    """
    __tablename__ = "strategies"
    
    # Primary identification
    strategy_id = Column(String, primary_key=True)
    user_id = Column(String, nullable=False, default="default_user", index=True)
    
    # Basic information
    name = Column(String, nullable=False)
    description = Column(Text)
    author = Column(String, default="User")
    version = Column(String, default="1.0.0")
    
    # Strategy code and validation
    python_code = Column(Text, nullable=False)  # Store the actual Python code
    file_hash = Column(String)  # For change detection
    filename = Column(String)   # Original filename
    file_size = Column(Integer) # File size in bytes
    
    # Metadata (JSON storage like provider credentials)
    strategy_metadata = Column(JSON)  # Strategy metadata as JSON
    validation_status = Column(String, default="pending")
    validation_details = Column(JSON)  # Validation results
    
    # Risk and configuration
    risk_level = Column(String, default="medium")  # low, medium, high
    max_positions = Column(Integer, default=1)
    preferred_symbols = Column(JSON)  # List of preferred symbols
    
    # Timestamps
    created_at = Column(DateTime, server_default=func.now())
    updated_at = Column(DateTime, server_default=func.now(), onupdate=func.now())
    last_used = Column(DateTime)
    
    # Statistics
    validation_count = Column(Integer, default=0)
    success_count = Column(Integer, default=0)
    error_count = Column(Integer, default=0)
    
    # Status
    is_active = Column(Boolean, default=True)  # Soft delete
    
    # Relationships
    executions = relationship("StrategyExecution", back_populates="strategy", cascade="all, delete-orphan")
    
    def to_dict(self) -> Dict[str, Any]:
        """Convert to dictionary for API responses"""
        return {
            "strategy_id": self.strategy_id,
            "user_id": self.user_id,
            "name": self.name,
            "description": self.description,
            "author": self.author,
            "version": self.version,
            "filename": self.filename,
            "file_size": self.file_size,
            "risk_level": self.risk_level,
            "max_positions": self.max_positions,
            "preferred_symbols": self.preferred_symbols or [],
            "created_at": self.created_at.isoformat() if self.created_at else None,
            "updated_at": self.updated_at.isoformat() if self.updated_at else None,
            "last_used": self.last_used.isoformat() if self.last_used else None,
            "validation_count": self.validation_count,
            "success_count": self.success_count,
            "error_count": self.error_count,
            "validation_status": self.validation_status,
            "metadata": self.strategy_metadata or {}
        }

class StrategyExecution(Base):
    """
    Strategy execution tracking - stores execution history and performance.
    """
    __tablename__ = "strategy_executions"
    
    # Primary identification
    execution_id = Column(String, primary_key=True)
    strategy_id = Column(String, ForeignKey("strategies.strategy_id"), nullable=False, index=True)
    user_id = Column(String, nullable=False, index=True)
    
    # Execution details
    mode = Column(String, nullable=False)  # 'live', 'backtest', 'paper'
    status = Column(String, default="running")  # 'running', 'paused', 'stopped', 'completed', 'failed'
    
    # Configuration
    configuration = Column(JSON)  # Strategy parameters used
    initial_capital = Column(Float)
    
    # Timestamps
    start_time = Column(DateTime, server_default=func.now())
    end_time = Column(DateTime)
    last_activity = Column(DateTime, server_default=func.now())
    
    # Performance metrics
    current_capital = Column(Float)
    total_pnl = Column(Float, default=0.0)
    total_trades = Column(Integer, default=0)
    winning_trades = Column(Integer, default=0)
    losing_trades = Column(Integer, default=0)
    win_rate = Column(Float, default=0.0)
    max_drawdown = Column(Float, default=0.0)
    sharpe_ratio = Column(Float)
    
    # Error tracking
    error_count = Column(Integer, default=0)
    last_error = Column(Text)
    last_error_time = Column(DateTime)
    
    # Relationships
    strategy = relationship("Strategy", back_populates="executions")
    trades = relationship("StrategyTrade", back_populates="execution", cascade="all, delete-orphan")
    
    def to_dict(self) -> Dict[str, Any]:
        """Convert to dictionary for API responses"""
        return {
            "execution_id": self.execution_id,
            "strategy_id": self.strategy_id,
            "user_id": self.user_id,
            "mode": self.mode,
            "status": self.status,
            "configuration": self.configuration or {},
            "initial_capital": self.initial_capital,
            "current_capital": self.current_capital,
            "total_pnl": self.total_pnl,
            "total_trades": self.total_trades,
            "winning_trades": self.winning_trades,
            "losing_trades": self.losing_trades,
            "win_rate": self.win_rate,
            "max_drawdown": self.max_drawdown,
            "sharpe_ratio": self.sharpe_ratio,
            "start_time": self.start_time.isoformat() if self.start_time else None,
            "end_time": self.end_time.isoformat() if self.end_time else None,
            "last_activity": self.last_activity.isoformat() if self.last_activity else None,
            "error_count": self.error_count,
            "last_error": self.last_error
        }

class StrategyTrade(Base):
    """
    Individual trade tracking - stores detailed trade history.
    """
    __tablename__ = "strategy_trades"
    
    # Primary identification
    trade_id = Column(String, primary_key=True)
    execution_id = Column(String, ForeignKey("strategy_executions.execution_id"), nullable=False, index=True)
    strategy_id = Column(String, nullable=False, index=True)
    
    # Trade details
    symbol = Column(String, nullable=False, index=True)
    action = Column(String, nullable=False)  # 'BUY', 'SELL', 'BUY_TO_OPEN', 'SELL_TO_CLOSE', etc.
    quantity = Column(Integer, nullable=False)
    price = Column(Float, nullable=False)
    order_type = Column(String, default="MARKET")  # 'MARKET', 'LIMIT', 'STOP'
    
    # Execution details
    timestamp = Column(DateTime, server_default=func.now(), index=True)
    order_id = Column(String)  # Broker order ID
    fill_price = Column(Float)  # Actual fill price
    fees = Column(Float, default=0.0)
    
    # P&L and performance
    pnl = Column(Float, default=0.0)
    unrealized_pnl = Column(Float, default=0.0)
    
    # Strategy context
    reason = Column(Text)  # Why the trade was made
    rule_id = Column(String)  # Which rule triggered the trade
    strategy_state = Column(JSON)  # Strategy state at time of trade
    
    # Market context
    market_price = Column(Float)  # Market price at time of signal
    bid_price = Column(Float)
    ask_price = Column(Float)
    volume = Column(Integer)
    
    # Relationships
    execution = relationship("StrategyExecution", back_populates="trades")
    
    def to_dict(self) -> Dict[str, Any]:
        """Convert to dictionary for API responses"""
        return {
            "trade_id": self.trade_id,
            "execution_id": self.execution_id,
            "strategy_id": self.strategy_id,
            "symbol": self.symbol,
            "action": self.action,
            "quantity": self.quantity,
            "price": self.price,
            "order_type": self.order_type,
            "timestamp": self.timestamp.isoformat() if self.timestamp else None,
            "order_id": self.order_id,
            "fill_price": self.fill_price,
            "fees": self.fees,
            "pnl": self.pnl,
            "unrealized_pnl": self.unrealized_pnl,
            "reason": self.reason,
            "rule_id": self.rule_id,
            "market_price": self.market_price,
            "bid_price": self.bid_price,
            "ask_price": self.ask_price,
            "volume": self.volume
        }

class StrategyPerformance(Base):
    """
    Daily performance snapshots - for analytics and reporting.
    """
    __tablename__ = "strategy_performance"
    
    # Primary identification
    performance_id = Column(String, primary_key=True)
    strategy_id = Column(String, ForeignKey("strategies.strategy_id"), nullable=False, index=True)
    execution_id = Column(String, ForeignKey("strategy_executions.execution_id"), index=True)
    
    # Date and time
    date = Column(DateTime, nullable=False, index=True)
    
    # Daily metrics
    daily_pnl = Column(Float, default=0.0)
    cumulative_pnl = Column(Float, default=0.0)
    daily_trades = Column(Integer, default=0)
    cumulative_trades = Column(Integer, default=0)
    
    # Performance ratios
    win_rate = Column(Float, default=0.0)
    profit_factor = Column(Float, default=0.0)
    sharpe_ratio = Column(Float)
    max_drawdown = Column(Float, default=0.0)
    
    # Capital tracking
    starting_capital = Column(Float)
    ending_capital = Column(Float)
    peak_capital = Column(Float)
    
    # Risk metrics
    var_95 = Column(Float)  # Value at Risk (95%)
    volatility = Column(Float)
    beta = Column(Float)
    
    def to_dict(self) -> Dict[str, Any]:
        """Convert to dictionary for API responses"""
        return {
            "performance_id": self.performance_id,
            "strategy_id": self.strategy_id,
            "execution_id": self.execution_id,
            "date": self.date.isoformat() if self.date else None,
            "daily_pnl": self.daily_pnl,
            "cumulative_pnl": self.cumulative_pnl,
            "daily_trades": self.daily_trades,
            "cumulative_trades": self.cumulative_trades,
            "win_rate": self.win_rate,
            "profit_factor": self.profit_factor,
            "sharpe_ratio": self.sharpe_ratio,
            "max_drawdown": self.max_drawdown,
            "starting_capital": self.starting_capital,
            "ending_capital": self.ending_capital,
            "peak_capital": self.peak_capital,
            "var_95": self.var_95,
            "volatility": self.volatility,
            "beta": self.beta
        }
