"""
Statistical Edge Vertical Strategy - Phase 1: Monitoring

This strategy implements a dynamic credit spread monitoring system for SPX options.
Phase 1 focuses on finding and tracking the first credit spread paying $0.01.

Strategy Logic (Phase 1):
1. At 1:30 PM: Start monitoring SPX options
2. Find first OTM call credit spread (5-wide) paying ~$0.01
3. Track spread price in real-time throughout the day
4. Expose dynamic fields to UI: short_leg, long_leg, price_vertical

Key Features:
- Real-time vertical spread price tracking
- Dynamic UI field updates (like MA strategy)
- Professional options chain integration
- Configurable spread parameters
- Comprehensive monitoring system
"""

from datetime import datetime, timedelta
from typing import Dict, Any, Optional, List
import asyncio

from .base_strategy import BaseStrategy
from .actions import ActionContext
from .rules import Rules
from .options_models import OptionsChain

import logging
logger = logging.getLogger(__name__)


class StatisticalEdgeVerticalStrategy(BaseStrategy):
    """
    Statistical Edge Vertical Strategy - Phase 1: Monitoring
    
    This strategy implements dynamic credit spread monitoring for SPX options.
    It finds the first OTM credit spread paying $0.01 and tracks its price
    throughout the day, providing real-time updates to the UI.
    
    Phase 1 Logic:
    1. At 1:30 PM: Start monitoring
    2. Find first 5-wide call credit spread paying ~$0.01
    3. Track spread price continuously
    4. Update UI with real-time data
    """
    
    def __init__(self, strategy_id: str, data_provider, order_executor, config: Dict[str, Any]):
        super().__init__(strategy_id, data_provider, order_executor, config)
        self.underlying = None
        self.monitoring_start = None
        self.spread_width = None
        self.target_credit = None
        self.log_info(f"StatisticalEdgeVerticalStrategy instance created with ID: {strategy_id}")
    
    async def initialize_strategy(self):
        """Initialize strategy parameters and define the monitoring flow"""
        # Initialize parameters
        self.underlying = self.get_config_value("underlying", "SPX")
        self.monitoring_start = self.get_config_value("monitoring_start", "13:30")  # 1:30 PM
        self.spread_width = self.get_config_value("spread_width", 5)  # 5-wide spreads
        self.target_credit = self.get_config_value("target_credit", 0.01)  # Target $0.01
        self.option_type = self.get_config_value("option_type", "call")  # Call spreads
        self.min_dte = self.get_config_value("min_dte", 0)  # 0DTE options
        self.max_dte = self.get_config_value("max_dte", 7)  # Up to weekly
        
        # Map underlying to options symbol (SPX -> SPXW for options)
        self.options_symbol = self._get_options_symbol(self.underlying)
        
        # Register additional symbols for data loading (SPX strategy needs SPXW options data)
        if self.underlying == "SPX":
            self.register_additional_symbol("SPXW")
            self.log_info(f"Registered additional symbol for options data: SPXW")
        
        # Register UI states for flow engine context display
        self.register_ui_state("underlying_price")
        self.register_ui_state("short_leg")
        self.register_ui_state("long_leg")
        self.register_ui_state("price_vertical")
        self.register_ui_state("target_expiration")
        self.register_ui_state("strikes_monitored")
        self.register_ui_state("monitoring_active")
        self.log_info("Registered UI states for flow engine display")
        
        # Initialize state
        self.set_state("underlying", self.underlying)
        self.set_state("monitoring_start", self.monitoring_start)
        self.set_state("spread_width", self.spread_width)
        self.set_state("target_credit", self.target_credit)
        self.set_state("option_type", self.option_type)
        self.set_state("underlying_price", None)
        self.set_state("options_chain", None)
        self.set_state("monitoring_active", False)
        
        # Dynamic UI fields - updated every cycle
        self.set_state("short_leg", None)           # Short leg contract symbol
        self.set_state("long_leg", None)            # Long leg contract symbol  
        self.set_state("price_vertical", None)      # Current spread price
        self.set_state("target_expiration", None)   # Monitored expiration
        self.set_state("strikes_monitored", None)   # Strike range info
        
        # Add time-based trigger
        self.add_time_action(
            trigger_time=self.monitoring_start,
            callback=self.start_monitoring,
            name="vertical_monitoring_trigger"
        )
        
        # Define the monitoring flow
        self._define_strategy_flow()

        # Register data processors in order of execution
        self.register_data_processor(self.update_underlying_price)
        self.register_data_processor(self.update_options_chain)
        self.register_data_processor(self.update_vertical_price)
        
        self.log_info(f"Statistical Edge Vertical Strategy initialized for {self.underlying}")
        self.add_checkpoint("strategy_initialized", {
            "underlying": self.underlying,
            "monitoring_start": self.monitoring_start,
            "spread_width": self.spread_width,
            "target_credit": self.target_credit
        })
    
    def _define_strategy_flow(self):
        """Define the monitoring flow for Phase 1"""
        # Create action node for finding target vertical
        find_vertical_action = self.flow.add_action("Find Target Vertical", self.find_target_vertical_action)
        
        # Vertical Search Flow: Only runs when monitoring is active AND has options data AND no vertical found yet
        monitoring_active = self.flow.add_decision(
            name="Monitoring Active",
            condition=Rules.AllOf(
                self.is_monitoring_time
            ),
            if_true=find_vertical_action,
            if_false=None
        )

        vertical_search_flow = self.flow.add_decision(
            name="Vertical Search",
            condition=Rules.AllOf(
                self.has_options_data,
                self.needs_target_vertical
            ),
            if_true=find_vertical_action,
            if_false=None,
            execution_condition=self.is_monitoring_time  # Custom condition to control execution
        )

        #self.flow.set_parallel_flows([vertical_search_flow, monitoring_flow])
        self.flow.set_parallel_flows([vertical_search_flow])
        
        self.log_info(f"Statistical Edge Vertical declarative flows defined with {self.flow.get_node_count()} nodes")
    
    async def start_monitoring(self, context: ActionContext):
        """Start monitoring - called at 1:30 PM"""
        self.log_info("1:30 PM - Starting Statistical Edge Vertical monitoring")
        self.set_state("monitoring_active", True)
        self.add_checkpoint("monitoring_started")
    
    # ========================================================================
    # Rule Methods - Individual boolean functions for the flow engine
    # ========================================================================
    
    def is_monitoring_time(self, context: ActionContext) -> bool:
        """Rule: Check if monitoring time has been triggered"""
        monitoring_active = self.get_state("monitoring_active", False)
        
        if self.debug and monitoring_active:
            self.log_info("DEBUG: Monitoring time active - tracking vertical spreads")
        
        return monitoring_active
    
    def has_options_data(self, context: ActionContext) -> bool:
        """Rule: Check if we have options chain data available"""
        options_chain = self.get_state("options_chain")
        underlying_price = self.get_state("underlying_price")

        has_data = options_chain is not None and underlying_price is not None

        # Always log the detailed status when monitoring is active
        if self.get_state("monitoring_active", False):
            chain_status = f"chain: {options_chain is not None}"
            if options_chain is not None:
                chain_status += f" ({len(options_chain.contracts)} contracts)" if hasattr(options_chain, 'contracts') and options_chain.contracts else " (no contracts)"

            self.log_info(f"🔍 has_options_data check - {chain_status}, underlying_price: {underlying_price}, result: {has_data}")

        return has_data
    
    def needs_target_vertical(self, context: ActionContext) -> bool:
        """Rule: Check if we need to find a target vertical spread"""
        current_short = self.get_state("short_leg")
        current_long = self.get_state("long_leg")
        
        # Return True if we don't have both legs yet
        needs_vertical = not (current_short and current_long)
        
        if self.debug and self.get_state("monitoring_active", False):
            self.log_info(f"DEBUG: needs_target_vertical - short_leg: {current_short}, long_leg: {current_long}, needs: {needs_vertical}")
        
        return needs_vertical
    
    # ========================================================================
    # Execution Condition Methods - Control when flows should run
    # ========================================================================
    
    def _can_run_vertical_search(self, context: ActionContext) -> bool:
        """Execution condition: Vertical search only runs when monitoring is active AND has data AND needs vertical"""
        # All conditions are already checked in the decision condition
        # This method can add additional constraints if needed
        return True
    
    # ========================================================================
    # Action Methods - Functions executed by action nodes
    # ========================================================================
    
    async def find_target_vertical_action(self, context: ActionContext):
        """Action: Find the first credit spread paying ~$0.01"""
        try:
            underlying_price = self.get_state("underlying_price")
            options_chain = self.get_state("options_chain")
            
            if not underlying_price or not options_chain:
                self.log_error("Missing underlying price or options chain for vertical search")
                return
            
            # Find call spreads above current price (OTM)
            target_spread = self._find_target_credit_spread(options_chain, underlying_price)
            
            if target_spread:
                short_strike, long_strike, short_contract, long_contract = target_spread
                
                # Store the legs
                self.set_state("short_leg", short_contract.symbol)
                self.set_state("long_leg", long_contract.symbol)
                self.set_state("short_strike", short_strike)
                self.set_state("long_strike", long_strike)
                self.set_state("short_contract", short_contract)
                self.set_state("long_contract", long_contract)
                
                # Create display info
                strikes_info = f"{short_strike}/{long_strike} Call Spread ({self.spread_width}-wide)"
                self.set_state("strikes_monitored", strikes_info)
                
                self.log_info(f"🎯 TARGET VERTICAL FOUND:")
                self.log_info(f"   Short: {short_contract.symbol} @ ${short_strike}")
                self.log_info(f"   Long:  {long_contract.symbol} @ ${long_strike}")
                self.log_info(f"   Spread: {strikes_info}")
                
                self.add_checkpoint("target_vertical_found", {
                    "short_strike": short_strike,
                    "long_strike": long_strike,
                    "underlying_price": underlying_price,
                    "timestamp": context.current_time.isoformat()
                })
            else:
                self.log_warning("No suitable credit spread found paying target credit")
            
        except Exception as e:
            self.log_error(f"Error finding target vertical: {e}")
    
    # ========================================================================
    # Data Processor Methods - Registered in order of execution
    # ========================================================================
    
    def update_underlying_price(self, context: ActionContext) -> bool:
        """Data Processor 1: Update underlying SPX price"""
        try:
            current_price = self.get_current_price_from_provider()
            if current_price is None:
                return False
            
            # Store current price
            old_price = self.get_state("underlying_price")
            self.set_state("underlying_price", current_price)
            
            if self.debug and old_price != current_price:
                self.log_info(f"DEBUG: SPX price updated: ${current_price:.2f}")
            
            return True
            
        except Exception as e:
            self.log_error(f"Error updating underlying price: {e}")
            return False
    
    def update_options_chain(self, context: ActionContext) -> bool:
        """Data Processor 2: Update options chain for current day"""
        try:
            # ARCHITECTURAL FIX: Strategy is timezone-agnostic
            # Use virtual_date provided by backtest engine or system
            if not context.virtual_date:
                self.log_error("No virtual_date provided - strategy needs date context from system")
                return False
            
            today = context.virtual_date  # Format: "YYYY-MM-DD"
            self.log_info(f"�️ Using system date context: {today}")
            
            # REMOVED: Check if we already have the chain for today
            # always reload the chain to get fresh data
            
            # Log the SPX -> SPXW mapping
            self.log_info(f"📊 Loading options chain: {self.underlying} -> {self.options_symbol} for {today}")
            
            # Set target expiration
            self.set_state("target_expiration", today)
            self.set_state("options_chain_date", today)
            
            # Load the options chain
            try:
                if hasattr(self.data_provider, 'data_provider') and hasattr(self.data_provider.data_provider, 'aggregation_service'):
                    # BacktestEngine has aggregation service
                    aggregation_service = self.data_provider.data_provider.aggregation_service
                    
                    # OPTIMIZATION 1: Cache expirations by date (safe - expirations don't change intra-day)
                    cached_expirations_key = f"expirations_{self.options_symbol}_{today}"
                    expirations = self.get_state(cached_expirations_key)

                    if expirations is None:
                        # Only call expensive expiration discovery once per symbol per day
                        self.log_info(f"🔍 DISCOVERING expirations for {self.options_symbol}...")
                        expirations = aggregation_service.get_available_options_expirations(self.options_symbol, context.current_time)
                        self.set_state(cached_expirations_key, expirations)
                        self.log_info(f"📅 DISCOVERED and cached {len(expirations)} expirations")
                    else:
                        # Use cached expirations (fast!)
                        self.log_info(f"📅 Using cached {len(expirations)} expirations")

                    if expirations:
                        
                        # CRITICAL FIX: Look for options that EXPIRE on our target date (today)
                        target_exp = today
                        
                        if target_exp in expirations:
                            self.log_info(f"🎯 Found target expiration {target_exp} - loading options chain for {self.underlying} (options: {self.options_symbol})")
                        else:
                            self.log_warning(f"Target expiration {target_exp} not found in available expirations: {expirations}")
                            # Try to use the closest expiration as fallback
                            target_exp = expirations[0]
                            self.log_info(f"🔄 Using fallback expiration: {target_exp}")
                        
                        # Get the options chain using the options symbol and underlying price from strategy state
                        current_price = self.get_state("underlying_price")
                        chain = aggregation_service.get_options_chain(
                            self.options_symbol, target_exp, context.current_time,
                            strikes_around_atm=20, underlying_price=current_price
                        )
                        
                        if chain and hasattr(chain, 'contracts') and chain.contracts:
                            self.set_state("options_chain", chain)
                            self.set_state("target_expiration", target_exp)
                            self.log_info(f"✅ Successfully loaded SPXW options chain with {len(chain.contracts)} contracts")
                            return True
                        else:
                            self.log_error(f"❌ Options chain loaded but no contracts available for {self.options_symbol} exp={target_exp}")
                            return False
                    else:
                        self.log_error(f"❌ No available expirations found for {self.options_symbol} at {context.current_time}")
                        return False
                else:
                    self.log_error("❌ No aggregation service available for options data")
                    return False
                    
            except Exception as e:
                self.log_error(f"❌ Error loading options chain for {self.options_symbol}: {e}")
                import traceback
                self.log_error(f"Traceback: {traceback.format_exc()}")
                return False
            
        except Exception as e:
            self.log_error(f"❌ Error in update_options_chain: {e}")
            return False
    
    def update_vertical_price(self, context: ActionContext) -> bool:
        """Data Processor 4: Update the price of tracked vertical spread using fresh options chain data"""
        try:
            # Get the stored leg symbols (not the stale contract objects)
            short_symbol = self.get_state("short_leg")
            long_symbol = self.get_state("long_leg")
            
            if not short_symbol or not long_symbol:
                return False
            
            # Get current options chain (updated each cycle with fresh data)
            options_chain = self.get_state("options_chain")
            if not options_chain:
                return False
            
            # Find fresh contracts by symbol in current options chain
            short_contract = self._find_contract_by_symbol(options_chain, short_symbol)
            long_contract = self._find_contract_by_symbol(options_chain, long_symbol)
            
            if not short_contract or not long_contract:
                self.log_warning(f"Could not find contracts in current chain: short={short_symbol}, long={long_symbol}")
                return False
            
            # Get current prices from fresh contracts
            short_price = self._get_option_price(short_contract)
            long_price = self._get_option_price(long_contract)
            
            if short_price is not None and long_price is not None:
                # Calculate spread price (credit = short_price - long_price)
                vertical_price = short_price - long_price
                
                # Store current spread price
                old_price = self.get_state("price_vertical")
                self.set_state("price_vertical", vertical_price)
                self.set_state("short_price", short_price)
                self.set_state("long_price", long_price)
                
                if self.debug and old_price != vertical_price:
                    change = vertical_price - old_price if old_price else 0
                    direction = "↑" if change > 0 else "↓" if change < 0 else "→"
                    self.log_info(f"DEBUG: Vertical price updated: ${vertical_price:.3f} {direction} (${change:+.3f}) - Fresh chain data")
                
                return True
            else:
                self.log_warning("Could not get prices for vertical legs from fresh contracts")
                return False
            
        except Exception as e:
            self.log_error(f"Error updating vertical price: {e}")
            return False
    
    # ========================================================================
    # Helper Methods
    # ========================================================================
    
    def _get_options_symbol(self, underlying: str) -> str:
        """
        Map underlying symbol to options symbol.
        
        For SPX, options are traded under SPXW symbol.
        For other underlyings, use the same symbol.
        """
        symbol_mapping = {
            "SPX": "SPXW",
            # Add other mappings as needed
            # "SPY": "SPY",  # SPY options use same symbol
        }
        
        return symbol_mapping.get(underlying, underlying)
    
    def _find_target_credit_spread(self, options_chain: OptionsChain, underlying_price: float):
        """Find the first OTM credit spread paying target credit or less - simple logic with immediate return"""
        try:
            # Get call contracts and sort by strike
            call_contracts = [c for c in options_chain.contracts if c.type == "call"]
            call_contracts.sort(key=lambda x: x.strike_price)
            
            # Find strikes above current price (OTM calls)
            otm_calls = [c for c in call_contracts if c.strike_price > underlying_price]
            
            if len(otm_calls) < 2:
                self.log_warning("❌ Not enough OTM call contracts for spread construction")
                return None
            
            self.log_info(f"🔍 Searching for {self.spread_width}-wide credit spread with target credit ≤ ${self.target_credit:.3f}")
            
            # Simple single loop: check each spread, return first one that meets criteria
            for i, short_contract in enumerate(otm_calls):
                short_strike = short_contract.strike_price
                target_long_strike = short_strike + self.spread_width
                
                # Find the corresponding long leg
                long_contract = None
                for long_candidate in otm_calls[i+1:]:
                    if long_candidate.strike_price == target_long_strike:
                        long_contract = long_candidate
                        break
                
                if long_contract:
                    # Get current prices for both legs
                    short_price = self._get_option_price(short_contract)
                    long_price = self._get_option_price(long_contract)
                    
                    if short_price is not None and long_price is not None:
                        credit = short_price - long_price
                        
                        self.log_info(f"   Checking {short_strike}/{target_long_strike} spread: credit=${credit:.3f}")
                        
                        # Simple logic: If credit is target or less, take it immediately and exit!
                        if credit <= self.target_credit:
                            self.log_info(f"✅ FOUND FIRST ACCEPTABLE SPREAD: {short_strike}/{target_long_strike} credit=${credit:.3f} (≤ ${self.target_credit:.3f})")
                            return (short_strike, target_long_strike, short_contract, long_contract)
                        else:
                            self.log_info(f"   → Skipping: credit ${credit:.3f} > target ${self.target_credit:.3f}")
            
            # If we get here, no spread met the criteria
            self.log_warning(f"❌ No credit spread found paying ${self.target_credit:.3f} or less")
            return None
            
        except Exception as e:
            self.log_error(f"Error finding target credit spread: {e}")
            return None
    
    def _find_contract_by_symbol(self, options_chain: OptionsChain, symbol: str):
        """Find a specific contract by symbol in the current options chain"""
        try:
            if not options_chain or not hasattr(options_chain, 'contracts') or not options_chain.contracts:
                return None
            
            # Search through all contracts in the chain
            for contract in options_chain.contracts:
                if hasattr(contract, 'symbol') and contract.symbol == symbol:
                    return contract
            
            return None
            
        except Exception as e:
            self.log_error(f"Error finding contract by symbol {symbol}: {e}")
            return None
    
    def _get_option_price(self, contract) -> Optional[float]:
        """Get current price for an option contract"""
        try:
            # Use mid price if available, otherwise fallback to mark or last
            if hasattr(contract, 'bid') and hasattr(contract, 'ask'):
                if contract.bid is not None and contract.ask is not None:
                    return (contract.bid + contract.ask) / 2.0
            
            # Fallback to mark price
            if hasattr(contract, 'mark') and contract.mark is not None:
                return contract.mark
            
            # Fallback to last price
            if hasattr(contract, 'last') and contract.last is not None:
                return contract.last
            
            # Fallback to theoretical value if available
            if hasattr(contract, 'theoretical_value') and contract.theoretical_value is not None:
                return contract.theoretical_value
            
            return None
            
        except Exception as e:
            self.log_error(f"Error getting option price for {contract.symbol}: {e}")
            return None
    
    def get_current_price_from_provider(self) -> Optional[float]:
        """Get current price from the data provider"""
        try:
            if hasattr(self.data_provider, 'get_current_price'):
                price = self.data_provider.get_current_price(self.underlying)
                if price is not None:
                    return price

            self.log_warning(f"No price available from data provider for {self.underlying}")
            return None

        except Exception as e:
            self.log_error(f"Error getting current price for {self.underlying}: {e}")
            return None
    
    # ========================================================================
    # Strategy Metadata
    # ========================================================================
    
    def get_strategy_metadata(self) -> Dict[str, Any]:
        """Return strategy metadata for UI and monitoring"""
        return {
            "name": "Statistical Edge Vertical Strategy - Phase 1",
            "description": "Phase 1: Dynamic credit spread monitoring for SPX options - tracks first spread paying target credit amount",
            "version": "1.0.0",
            "author": "Options Framework",
            "risk_level": "MONITORING",
            "max_positions": 0,  # Phase 1: No positions
            "preferred_symbols": ["SPX", "SPXW"],
            "parameters": {
                "underlying": {
                    "type": "string",
                    "default": "SPX",
                    "description": "Underlying symbol for options monitoring (SPX recommended)",
                    "category": "strategy"
                },
                "monitoring_start_time": {
                    "type": "string",
                    "default": "13:30",
                    "description": "Daily monitoring start time (HH:MM format, e.g., 13:30 for 1:30 PM ET)",
                    "category": "strategy"
                },
                "target_credit": {
                    "type": "float",
                    "default": 0.04,
                    "min": 0.005,
                    "max": 0.50,
                    "step": 0.005,
                    "description": "Target credit amount to search for in spread ($0.01 = 1 cent)",
                    "category": "strategy"
                },
                "spread_width_call": {
                    "type": "integer",
                    "default": 5,
                    "min": 1,
                    "max": 50,
                    "description": "Call credit spread width in dollars (e.g., 5 = 5-point spread)",
                    "category": "strategy"
                },
                "spread_width_put": {
                    "type": "integer",
                    "default": 5,
                    "min": 1,
                    "max": 50,
                    "description": "Put credit spread width in dollars (e.g., 5 = 5-point spread)",
                    "category": "strategy"
                },
                "dte_target": {
                    "type": "integer",
                    "default": 0,
                    "min": 0,
                    "max": 30,
                    "description": "Target days to expiration (0 = same day, 1 = next day, etc.)",
                    "category": "strategy"
                },
                "monitor_interval": {
                    "type": "integer",
                    "default": 30,
                    "min": 5,
                    "max": 300,
                    "description": "Price monitoring interval in seconds (30 = update every 30 seconds)",
                    "category": "strategy"
                },
                "account_balance": {
                    "type": "float",
                    "default": 100000.0,
                    "min": 1000.0,
                    "max": 10000000.0,
                    "description": "Account balance for future position sizing calculations",
                    "category": "framework"
                },
                "commission_per_trade": {
                    "type": "float",
                    "default": 1.0,
                    "min": 0.0,
                    "max": 50.0,
                    "description": "Commission per trade ($) for future execution phases",
                    "category": "framework"
                }
            },
            "dynamic_fields": {
                "short_leg": {
                    "type": "string",
                    "description": "Currently monitored short leg contract symbol"
                },
                "long_leg": {
                    "type": "string", 
                    "description": "Currently monitored long leg contract symbol"
                },
                "price_vertical": {
                    "type": "float",
                    "description": "Real-time vertical spread price (credit amount)"
                },
                "spread_type": {
                    "type": "string",
                    "description": "Current spread type being monitored (call_credit or put_credit)"
                },
                "monitoring_status": {
                    "type": "string",
                    "description": "Current monitoring state (waiting, active, found_target)"
                }
            }
        }
    
    def get_flow_graph_data(self) -> Dict[str, Any]:
        """Get flow graph data for visualization"""
        return self.flow.to_graph_data()

    # NO execute_cycle method needed!
    # The flow engine handles everything automatically
