
import asyncio
import logging
import pandas as pd
from datetime import datetime
from unittest.mock import MagicMock, AsyncMock

# Setup logging
logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

# Import strategy service components
from src.backtest.backtest_engine import StrategyBacktestEngine
from src.core.base_strategy import BaseStrategy
from src.models.strategy_state import StrategyState
from src.data.models import OHLCVData

# Mock Strategy
class SimpleMAStrategy(BaseStrategy):
    def __init__(self, strategy_id="test_strat", config=None):
        super().__init__(strategy_id, config)
        self.fast_period = 10
        self.slow_period = 20
        
    def get_strategy_metadata(self):
        return {
            "name": "Simple MA",
            "description": "Test Strategy",
            "version": "1.0",
            "author": "Test",
            "parameters": []
        }
        
    async def initialize_strategy(self):
        logger.info("Initializing strategy logic")
        
    async def record_cycle_decision(self, context):
        self.on_tick(context)
        
    def on_tick(self, context):
        price = context.get_price("SPY")
        if not price:
            return
            
        current_time = context.current_time
        if not current_time:
            return
            
        minute = current_time.minute
        
        # Use helper method to check positions
        has_pos = self.has_open_positions("SPY")
        
        if minute % 2 == 0:
            if not has_pos:
                logger.info(f"BUY signal at {current_time} price {price}")
                self.add_trade_action("buy_signal", "BUY", "SPY", 10)
        else:
            if has_pos:
                logger.info(f"SELL signal at {current_time} price {price}")
                self.add_trade_action("sell_signal", "SELL", "SPY", 10)

async def test_backtest():
    logger.info("Starting backtest test...")
    
    # Mock Data Aggregation Service
    mock_aggregation_service = MagicMock()
    mock_aggregation_service.get_available_symbols.return_value = ["SPY"]
    
    # Create mock data
    start_date = datetime(2023, 1, 1, 9, 30)
    end_date = datetime(2023, 1, 1, 10, 30)
    
    dates = pd.date_range(start=start_date, end=end_date, freq="1min")
    data = []
    price = 100.0
    
    for date in dates:
        price += (hash(str(date)) % 100) / 100.0 - 0.5  # Random walk
        data.append(OHLCVData(
            timestamp=date,
            open=price,
            high=price + 0.5,
            low=price - 0.5,
            close=price,
            volume=1000
        ))
        
    # Mock response
    mock_result = MagicMock()
    mock_result.data = data
    mock_aggregation_service.get_aggregated_data.return_value = mock_result
    
    # Initialize engine with mocked service
    engine = StrategyBacktestEngine(
        initial_capital=100000.0,
        aggregation_service=mock_aggregation_service
    )
    
    # Initialize strategy
    strategy = SimpleMAStrategy()
    
    # Run backtest
    result = await engine.run_backtest(
        strategy=strategy,
        start_date=start_date,
        end_date=end_date,
        symbols=["SPY"],
        speed_multiplier=1000
    )
    
    # Verify results
    logger.info(f"Backtest success: {result['success']}")
    if result['success']:
        metrics = result['metrics']
        logger.info(f"Total trades: {metrics['trading']['total_trades']}")
        logger.info(f"Total PnL: ${metrics['pnl']['total_pnl']:.2f}")
        logger.info(f"Win rate: {metrics['trading']['win_rate']:.2%}")
    else:
        logger.error(f"Error: {result.get('error')}")

if __name__ == "__main__":
    asyncio.run(test_backtest())
