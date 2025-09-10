"""
Declarative Iron Condor Strategy

This strategy demonstrates the pure declarative approach for options trading.
It implements a simple daily iron condor strategy:

- Entry: Every day at 1:30 PM
- Exit: Every day at 3:30 PM
- Structure: ATM±3 (sell), ATM±4 (buy), 1-dollar wide spreads
- One trade per day, automatic close

Key Features:
- Pure declarative flow definition
- Time-based execution (1:30 PM entry, 3:30 PM exit)
- 4-leg iron condor construction
- Automatic daily reset
- Options framework integration
"""

from datetime import datetime, timedelta
from typing import Dict, Any, Optional, List

from .base_strategy import BaseStrategy
from .actions import ActionContext
from .rules import Rules
from .options_models import OptionsChain, OptionsLeg, OptionsOrder

import logging
logger = logging.getLogger(__name__)


class DeclarativeIronCondorStrategy(BaseStrategy):
    """
    Pure Declarative Iron Condor Strategy
    
    This strategy implements a simple daily iron condor using the pure declarative approach.
    
    Strategy Logic:
    1. At 1:30 PM: Find ATM strike, build iron condor (ATM±3 sell, ATM±4 buy)
    2. At 3:30 PM: Close any existing position
    3. Repeat daily
    
    The flow engine automatically:
    - Updates options data when needed
    - Records all decision data for UI
    - Handles execution flow
    - Provides rich debugging information
    """
    
    def __init__(self, strategy_id: str, data_provider, order_executor, config: Dict[str, Any]):
        super().__init__(strategy_id, data_provider, order_executor, config)
        self.underlying = None
        self.entry_time = None
        self.exit_time = None
        self.log_info(f"DeclarativeIronCondorStrategy instance created with ID: {strategy_id}")
    
    async def initialize_strategy(self):
        """Initialize strategy parameters and define the pure declarative flow"""
        # Initialize parameters
        self.underlying = self.get_config_value("underlying", "SPXW")
        self.entry_time = self.get_config_value("entry_time", "13:30")  # 1:30 PM
        self.exit_time = self.get_config_value("exit_time", "15:30")   # 3:30 PM
        self.strike_distance = self.get_config_value("strike_distance", 3)
        self.spread_width = self.get_config_value("spread_width", 1)
        self.max_positions = self.get_config_value("max_positions", 1)
        
        # Initialize state
        self.set_state("underlying", self.underlying)
        self.set_state("entry_time", self.entry_time)
        self.set_state("exit_time", self.exit_time)
        self.set_state("strike_distance", self.strike_distance)
        self.set_state("spread_width", self.spread_width)
        self.set_state("max_positions", self.max_positions)
        self.set_state("current_position", None)
        self.set_state("position_legs", [])
        self.set_state("entry_triggered", False)
        self.set_state("exit_triggered", False)
        self.set_state("atm_strike", None)
        self.set_state("options_chain", None)
        
        # Add time-based triggers
        self.add_time_action(
            trigger_time=self.entry_time,
            callback=self.prepare_entry,
            name="iron_condor_entry_trigger"
        )
        
        self.add_time_action(
            trigger_time=self.exit_time,
            callback=self.prepare_exit,
            name="iron_condor_exit_trigger"
        )
        
        # Define the pure declarative flow
        self._define_strategy_flow()

        # Register data processors in order of execution
        self.register_data_processor(self.update_underlying_price)
        self.register_data_processor(self.update_options_chain)
        self.register_data_processor(self.calculate_atm_strike)
        
        self.log_info(f"Declarative Iron Condor Strategy initialized for {self.underlying}")
        self.add_checkpoint("strategy_initialized", {
            "underlying": self.underlying,
            "entry_time": self.entry_time,
            "exit_time": self.exit_time
        })
    
    def _define_strategy_flow(self):
        """Define the complete strategy with simplified conditions"""
        # Create action nodes
        entry_action = self.flow.add_action("Execute Iron Condor", self.execute_iron_condor)
        exit_action = self.flow.add_action("Close Iron Condor", self.close_iron_condor)
        
        # Entry Flow: Execute iron condor at 1:30 PM
        entry_flow = self.flow.add_decision(
            name="Iron Condor Entry Check",
            condition=Rules.AllOf(
                self.is_entry_time,
                self.can_build_iron_condor
            ),
            if_true=entry_action,
            if_false=None
        )
        
        # Exit Flow: Close position at 3:30 PM
        exit_flow = self.flow.add_decision(
            name="Iron Condor Exit Check", 
            condition=Rules.AllOf(
                self.is_exit_time
            ),
            if_true=exit_action,
            if_false=None
        )
        
        # Set up parallel flows
        self.flow.set_parallel_flows([entry_flow, exit_flow])
        
        self.log_info(f"Declarative iron condor flows defined with {self.flow.get_node_count()} nodes")
    
    async def prepare_entry(self, context: ActionContext):
        """Prepare for entry - called at 1:30 PM"""
        self.log_info("1:30 PM - Preparing iron condor entry")
        self.set_state("entry_triggered", True)
        self.add_checkpoint("entry_time_triggered")
    
    async def prepare_exit(self, context: ActionContext):
        """Prepare for exit - called at 3:30 PM"""
        self.log_info("3:30 PM - Preparing iron condor exit")
        self.set_state("exit_triggered", True)
        self.add_checkpoint("exit_time_triggered")
    
    # ========================================================================
    # Rule Methods - Individual boolean functions for the flow engine
    # ========================================================================
    
    def is_entry_time(self, context: ActionContext) -> bool:
        """Rule: Check if entry time has been triggered"""
        entry_triggered = self.get_state("entry_triggered", False)
        
        if self.debug and entry_triggered:
            self.log_info("DEBUG: Entry time triggered - ready for iron condor execution")
        
        return entry_triggered
    
    def can_build_iron_condor(self, context: ActionContext) -> bool:
        """Rule: Check if we can build the iron condor structure"""
        atm_strike = self.get_state("atm_strike")
        target_expiration = self.get_state("target_expiration")
        
        if not atm_strike or not target_expiration:
            return False
        
        # Try to load options chain if we don't have it
        options_chain = self.get_state("options_chain")
        if not options_chain:
            try:
                # Load options chain synchronously for the rule check
                if hasattr(self.data_provider, 'get_options_chain'):
                    # This is a hack to get async data in a sync method
                    # In a real implementation, you'd want to pre-load this data
                    import asyncio
                    loop = asyncio.get_event_loop()
                    if loop.is_running():
                        # Can't run async in sync context, return False for now
                        return False
                    else:
                        chain = loop.run_until_complete(
                            self.data_provider.get_options_chain(self.underlying, target_expiration)
                        )
                        if chain and chain.contracts:
                            self.set_state("options_chain", chain)
                            options_chain = chain
                        else:
                            return False
                else:
                    return False
            except Exception as e:
                self.log_error(f"Error loading options chain: {e}")
                return False
        
        if not options_chain:
            return False
        
        # Get configurable parameters
        strike_distance = self.get_state("strike_distance", 3)
        spread_width = self.get_state("spread_width", 1)
        
        # Check if we have the required strikes using configurable parameters
        required_strikes = [
            atm_strike - strike_distance - spread_width,  # Put buy
            atm_strike - strike_distance,                 # Put sell
            atm_strike + strike_distance,                 # Call sell
            atm_strike + strike_distance + spread_width   # Call buy
        ]
        
        # Check if all required contracts exist
        for strike in required_strikes:
            call_exists = any(c.strike_price == strike and c.type == "call" for c in options_chain.contracts)
            put_exists = any(c.strike_price == strike and c.type == "put" for c in options_chain.contracts)
            
            if strike > atm_strike and not call_exists:  # Call side
                return False
            elif strike < atm_strike and not put_exists:  # Put side
                return False
            elif strike == atm_strike + strike_distance and not call_exists:  # Short call
                return False
            elif strike == atm_strike - strike_distance and not put_exists:  # Short put
                return False
        
        return True
    
    def is_exit_time(self, context: ActionContext) -> bool:
        """Rule: Check if exit time has been triggered"""
        exit_triggered = self.get_state("exit_triggered", False)
        
        if self.debug and exit_triggered:
            self.log_info("DEBUG: Exit time triggered - ready to close position")
        
        return exit_triggered
    
    # ========================================================================
    # Action Methods - Functions executed by action nodes
    # ========================================================================
    
    async def execute_iron_condor(self, context: ActionContext):
        """Action: Execute 4-leg iron condor order"""
        try:
            options_chain = self.get_state("options_chain")
            atm_strike = self.get_state("atm_strike")
            
            if not options_chain or not atm_strike:
                self.log_error("Cannot execute iron condor - missing options data or ATM strike")
                return
            
            # Build iron condor legs
            legs = self._build_iron_condor_legs(options_chain, atm_strike)
            
            if not legs or len(legs) != 4:
                self.log_error("Cannot build iron condor - insufficient option contracts")
                return
            
            # Execute the 4-leg order
            if hasattr(self.order_executor, 'place_options_order'):
                order_id = await self.order_executor.place_options_order(legs, "market")
                
                if order_id:
                    # Store position information
                    self.set_state("current_position", order_id)
                    self.set_state("position_legs", [leg.to_provider_leg() for leg in legs])
                    
                    # Calculate net credit/debit
                    iron_condor_order = OptionsOrder(legs=legs)
                    net_cost = iron_condor_order.calculate_net_debit_credit()
                    
                    self.log_info(f"✅ IRON CONDOR EXECUTED: Order ID {order_id}")
                    self.log_info(f"   ATM Strike: {atm_strike}")
                    self.log_info(f"   Net Cost: ${net_cost:.2f}")
                    self.log_info(f"   Legs: {len(legs)} contracts")
                    
                    self.add_checkpoint("iron_condor_opened", {
                        "order_id": order_id,
                        "atm_strike": atm_strike,
                        "net_cost": net_cost,
                        "timestamp": context.current_time.isoformat()
                    })
                    
                    # Reset entry trigger
                    self.set_state("entry_triggered", False)
                else:
                    self.log_error("Iron condor execution failed - no order ID returned")
            else:
                self.log_error("Order executor does not support place_options_order")
            
        except Exception as e:
            self.log_error(f"Error executing iron condor: {e}")
    
    async def close_iron_condor(self, context: ActionContext):
        """Action: Close iron condor position"""
        try:
            current_position = self.get_state("current_position")
            position_legs = self.get_state("position_legs", [])
            
            if not current_position:
                self.log_info("No iron condor position to close")
                self.set_state("exit_triggered", False)
                return
            
            # Create closing legs (reverse the original order)
            closing_legs = []
            for leg_data in position_legs:
                # Find the original contract
                options_chain = self.get_state("options_chain")
                if not options_chain:
                    continue
                
                # Find contract by symbol
                contract = None
                for c in options_chain.contracts:
                    if c.symbol == leg_data["symbol"]:
                        contract = c
                        break
                
                if contract:
                    # Reverse the action
                    original_side = leg_data["side"]
                    closing_side = "buy" if original_side == "sell" else "sell"
                    
                    closing_leg = OptionsLeg(
                        contract=contract,
                        action=closing_side,
                        quantity=leg_data["qty"]
                    )
                    closing_legs.append(closing_leg)
            
            if closing_legs and len(closing_legs) == 4:
                # Execute closing order
                if hasattr(self.order_executor, 'place_options_order'):
                    close_order_id = await self.order_executor.place_options_order(closing_legs, "market")
                    
                    if close_order_id:
                        self.log_info(f"✅ IRON CONDOR CLOSED: Order ID {close_order_id}")
                        
                        # Reset position state
                        self.set_state("current_position", None)
                        self.set_state("position_legs", [])
                        
                        self.add_checkpoint("iron_condor_closed", {
                            "close_order_id": close_order_id,
                            "timestamp": context.current_time.isoformat()
                        })
                    else:
                        self.log_error("Iron condor closing failed - no order ID returned")
                else:
                    self.log_error("Order executor does not support place_options_order for closing")
            else:
                self.log_error(f"Cannot close iron condor - invalid legs count: {len(closing_legs)}")
            
            # Reset exit trigger regardless of success
            self.set_state("exit_triggered", False)
            
        except Exception as e:
            self.log_error(f"Error closing iron condor: {e}")
            self.set_state("exit_triggered", False)
    
    # ========================================================================
    # Data Processor Methods - Registered in order of execution
    # ========================================================================
    
    def update_underlying_price(self, context: ActionContext) -> bool:
        """Data Processor 1: Update underlying price"""
        try:
            current_price = self.get_current_price_from_provider()
            if current_price is None:
                return False
            
            # Store current price
            self.set_state("underlying_price", current_price)
            return True
            
        except Exception as e:
            self.log_error(f"Error updating underlying price: {e}")
            return False
    
    def update_options_chain(self, context: ActionContext) -> bool:
        """Data Processor 2: Update today's options chain"""
        try:
            current_time = context.current_time
            if not current_time:
                return False
            
            # Get today's date for 0DTE options
            today = current_time.date().strftime("%Y-%m-%d")
            
            # Check if we already have the chain for today
            stored_chain = self.get_state("options_chain")
            stored_date = self.get_state("options_chain_date")
            
            if stored_chain and stored_date == today:
                return True  # Already have today's chain
            
            # Set target expiration
            self.set_state("target_expiration", today)
            self.set_state("options_chain_date", today)
            
            # CRITICAL FIX: Actually load the options chain here using the aggregation service
            try:
                if hasattr(self.data_provider, 'data_provider') and hasattr(self.data_provider.data_provider, 'aggregation_service'):
                    # BacktestEngine has aggregation service
                    aggregation_service = self.data_provider.data_provider.aggregation_service
                    
                    # Get available expirations
                    expirations = aggregation_service.get_available_options_expirations(self.underlying, current_time)
                    
                    if expirations:
                        # Use the first available expiration (closest to today)
                        target_exp = expirations[0]
                        self.log_info(f"Loading options chain for {self.underlying} exp={target_exp}")
                        
                        # Get the options chain
                        chain = aggregation_service.get_options_chain(self.underlying, target_exp, current_time)
                        
                        if chain and hasattr(chain, 'contracts') and chain.contracts:
                            self.set_state("options_chain", chain)
                            self.set_state("target_expiration", target_exp)
                            self.log_info(f"✅ Loaded options chain with {len(chain.contracts)} contracts")
                            return True
                        else:
                            self.log_warning(f"No contracts in options chain for {self.underlying}")
                            return False
                    else:
                        self.log_warning(f"No available expirations for {self.underlying}")
                        return False
                else:
                    self.log_warning("No aggregation service available for options data")
                    return False
                    
            except Exception as e:
                self.log_error(f"Error loading options chain: {e}")
                return False
            
        except Exception as e:
            self.log_error(f"Error updating options chain: {e}")
            return False
    
    def calculate_atm_strike(self, context: ActionContext) -> bool:
        """Data Processor 3: Calculate ATM strike"""
        try:
            underlying_price = self.get_state("underlying_price")
            if not underlying_price:
                return False
            
            # Calculate ATM strike (round to nearest dollar)
            atm_strike = round(underlying_price)
            
            # Store ATM strike
            self.set_state("atm_strike", atm_strike)
            
            if self.debug:
                self.log_info(f"DEBUG: ATM strike calculated: {atm_strike} (underlying: ${underlying_price:.2f})")
            
            return True
            
        except Exception as e:
            self.log_error(f"Error calculating ATM strike: {e}")
            return False
    
    # ========================================================================
    # Helper Methods
    # ========================================================================
    
    def _build_iron_condor_legs(self, options_chain: OptionsChain, atm_strike: float) -> List[OptionsLeg]:
        """Build the 4 legs of the iron condor using configurable parameters"""
        try:
            legs = []
            
            # Get configurable parameters
            strike_distance = self.get_state("strike_distance", 3)
            spread_width = self.get_state("spread_width", 1)
            
            # Iron Condor Structure (configurable):
            # 1. Sell Put at ATM - strike_distance
            # 2. Buy Put at ATM - strike_distance - spread_width
            # 3. Sell Call at ATM + strike_distance
            # 4. Buy Call at ATM + strike_distance + spread_width
            
            strikes_and_actions = [
                (atm_strike - strike_distance, "put", "sell"),                        # Sell put
                (atm_strike - strike_distance - spread_width, "put", "buy"),          # Buy put
                (atm_strike + strike_distance, "call", "sell"),                       # Sell call
                (atm_strike + strike_distance + spread_width, "call", "buy")          # Buy call
            ]
            
            for strike, option_type, action in strikes_and_actions:
                # Find the contract
                contract = options_chain.get_contract_by_strike(strike, option_type)
                
                if contract:
                    leg = OptionsLeg(
                        contract=contract,
                        action=action,
                        quantity=1
                    )
                    legs.append(leg)
                else:
                    self.log_warning(f"Missing contract: {option_type} strike {strike}")
                    return []  # Return empty if any leg is missing
            
            if len(legs) == 4:
                self.log_info(f"Iron condor legs built successfully:")
                for i, leg in enumerate(legs):
                    self.log_info(f"  Leg {i+1}: {leg.action} {leg.contract.type} {leg.contract.strike_price}")
                return legs
            else:
                self.log_error(f"Incomplete iron condor: only {len(legs)} legs built")
                return []
            
        except Exception as e:
            self.log_error(f"Error building iron condor legs: {e}")
            return []
    
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
            self.log_error(f"Error getting current price: {e}")
            return None
    
    # ========================================================================
    # Strategy Metadata
    # ========================================================================
    
    def get_strategy_metadata(self) -> Dict[str, Any]:
        """Return strategy metadata for UI and monitoring"""
        return {
            "name": "Declarative Iron Condor Strategy",
            "description": "Daily iron condor strategy using pure declarative framework (ATM±3/±4, 1:30-3:30)",
            "version": "1.0.0",
            "author": "Options Framework",
            "risk_level": "HIGH",
            "max_positions": 1,
            "preferred_symbols": ["SPXW", "SPY"],
            "parameters": {
                "underlying": {
                    "type": "string",
                    "default": "SPXW",
                    "description": "Underlying symbol for options trading",
                    "category": "strategy"
                },
                "entry_time": {
                    "type": "string",
                    "default": "13:30",
                    "description": "Daily entry time (HH:MM format)",
                    "category": "strategy"
                },
                "exit_time": {
                    "type": "string",
                    "default": "15:30",
                    "description": "Daily exit time (HH:MM format)",
                    "category": "strategy"
                },
                "strike_distance": {
                    "type": "integer",
                    "default": 3,
                    "min": 1,
                    "max": 10,
                    "description": "Distance from ATM for short strikes ($)",
                    "category": "strategy"
                },
                "spread_width": {
                    "type": "integer",
                    "default": 1,
                    "min": 1,
                    "max": 5,
                    "description": "Width of each vertical spread ($)",
                    "category": "strategy"
                },
                "max_positions": {
                    "type": "integer",
                    "default": 1,
                    "min": 1,
                    "max": 5,
                    "description": "Maximum number of concurrent iron condors",
                    "category": "strategy"
                },
                "account_balance": {
                    "type": "float",
                    "default": 100000.0,
                    "min": 1000.0,
                    "max": 10000000.0,
                    "description": "Account balance for position sizing",
                    "category": "framework"
                },
                "commission_per_trade": {
                    "type": "float",
                    "default": 1.0,
                    "min": 0.0,
                    "max": 50.0,
                    "description": "Commission per trade ($)",
                    "category": "framework"
                }
            }
        }
    
    def get_flow_graph_data(self) -> Dict[str, Any]:
        """Get flow graph data for visualization"""
        return self.flow.to_graph_data()

    # NO execute_cycle method needed!
    # The flow engine handles everything automatically
