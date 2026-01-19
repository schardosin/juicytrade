"""
Declarative Strategy Flow Engine

This module implements the declarative flow engine that allows strategies to define
their logic as a graph of interconnected rules and actions during initialization,
rather than using imperative logic in record_cycle_decision.

Key Components:
- FlowEngine: Manages the strategy graph and execution
- Node: Base class for graph nodes (Decision and Action nodes)
- RuleCondition: Structured conditions created by Rules helper
"""

import logging
import asyncio
from typing import Dict, Any, List, Optional, Callable, Union
from abc import ABC, abstractmethod
from dataclasses import dataclass
from datetime import datetime

from .actions import ActionContext
from .rules import RuleCondition, AllOfCondition, AnyOfCondition, NotCondition

logger = logging.getLogger(__name__)


class Node(ABC):
    """Base class for flow graph nodes"""
    
    def __init__(self, node_id: str, name: str):
        self.node_id = node_id
        self.name = name
    
    @abstractmethod
    async def execute(self, context: ActionContext) -> Optional['Node']:
        """Execute this node and return the next node to execute (or None to stop)"""
        pass
    
    @abstractmethod
    def to_dict(self) -> Dict[str, Any]:
        """Convert node to dictionary for graph visualization"""
        pass


class DecisionNode(Node):
    """Decision node that evaluates a condition and branches to different paths"""
    
    def __init__(self, node_id: str, name: str, condition: RuleCondition, 
                 if_true: Optional[Node] = None, if_false: Optional[Node] = None,
                 skip_when_satisfied: bool = False, execution_condition: Optional[Callable] = None):
        super().__init__(node_id, name)
        self.condition = condition
        self.if_true = if_true
        self.if_false = if_false
        self.skip_when_satisfied = skip_when_satisfied  # Skip this node once it becomes true
        self.satisfied = False  # Track if this node has been satisfied
        self.execution_condition = execution_condition  # Condition to check before executing this node
    
    async def execute(self, context: ActionContext) -> Optional[Node]:
        """Evaluate condition and return appropriate next node"""
        try:
            logger.info(f"🔍 FLOW ENGINE: Evaluating decision node '{self.name}'")
            result = await self.condition.evaluate(context)
            logger.info(f"🔍 FLOW ENGINE: Decision node '{self.name}' condition result: {result}")
            
            if result and self.if_true:
                logger.info(f"🔍 FLOW ENGINE: Decision node '{self.name}' taking TRUE path to '{self.if_true.name}'")
                return self.if_true
            elif not result and self.if_false:
                logger.info(f"🔍 FLOW ENGINE: Decision node '{self.name}' taking FALSE path to '{self.if_false.name}'")
                return self.if_false
            else:
                logger.info(f"🔍 FLOW ENGINE: Decision node '{self.name}' ending execution (no path available)")
                return None  # End execution
                
        except Exception as e:
            logger.error(f"🚨 FLOW ENGINE: Error executing decision node {self.name}: {e}")
            import traceback
            logger.error(f"🚨 FLOW ENGINE: Traceback: {traceback.format_exc()}")
            return None
    
    def to_dict(self) -> Dict[str, Any]:
        """Convert to dictionary for visualization"""
        return {
            "id": self.node_id,
            "type": "Decision",
            "label": self.name,
            "condition_description": self.condition.get_description(),
            "rule_names": self.condition.get_rule_names()
        }


class ActionNode(Node):
    """Action node that executes a function"""
    
    def __init__(self, node_id: str, name: str, action_callable: Callable):
        super().__init__(node_id, name)
        self.action_callable = action_callable
    
    async def execute(self, context: ActionContext) -> Optional[Node]:
        """Execute this node and return the next node to execute (or None to stop)"""
        try:
            logger.info(f"🚀 FLOW ENGINE: Executing action node '{self.name}' with function {self.action_callable.__name__}")
            
            if asyncio.iscoroutinefunction(self.action_callable):
                await self.action_callable(context)
            else:
                self.action_callable(context)
            
            logger.info(f"✅ FLOW ENGINE: Action node '{self.name}' executed successfully")
            return None  # Action nodes are terminal
            
        except Exception as e:
            logger.error(f"🚨 FLOW ENGINE: Error executing action node {self.name}: {e}")
            import traceback
            logger.error(f"🚨 FLOW ENGINE: Traceback: {traceback.format_exc()}")
            return None
    
    def to_dict(self) -> Dict[str, Any]:
        """Convert to dictionary for visualization"""
        return {
            "id": self.node_id,
            "type": "Action",
            "label": self.name,
            "action_function": self.action_callable.__name__
        }


class FlowEngine:
    """
    Flow Engine that manages the strategy's declarative graph.
    
    This engine allows strategies to define their logic as a graph of interconnected
    rules and actions during initialization, then executes the graph on each cycle.
    """
    
    def __init__(self, strategy_instance):
        self.strategy = strategy_instance
        self.nodes: Dict[str, Node] = {}
        self.root_node: Optional[Node] = None
        self.parallel_flows: List[Node] = []  # Support for parallel independent flows
        self.node_counter = 0
        self.logger = logging.getLogger(f"FlowEngine.{strategy_instance.strategy_id}")
    
    def _generate_node_id(self) -> str:
        """Generate unique node ID"""
        self.node_counter += 1
        return f"node_{self.node_counter}"
    
    def add_decision(
        self,
        name: str,
        condition: RuleCondition,
        if_true: Optional[Node] = None,
        if_false: Optional[Node] = None,
        skip_when_satisfied: bool = False,
        execution_condition: Optional[Callable] = None
    ) -> Node:
        """
        Add a branching node to the flow.
        
        Args:
            name: Human-readable name for the decision
            condition: A condition object created by the Rules helper
            if_true: The node to execute if the condition is true
            if_false: The node to execute if the condition is false
            skip_when_satisfied: If True, skip this node once it becomes true
        
        Returns:
            The created DecisionNode
        """
        node_id = self._generate_node_id()
        decision_node = DecisionNode(node_id, name, condition, if_true, if_false, skip_when_satisfied, execution_condition)
        self.nodes[node_id] = decision_node
        
        # Set as root node if this is the first node
        if self.root_node is None:
            self.root_node = decision_node
        
        self.logger.debug(f"Added decision node: {name} (ID: {node_id}, skip_when_satisfied: {skip_when_satisfied})")
        return decision_node
    
    def add_action(self, name: str, action_callable: Callable) -> Node:
        """
        Add a terminal action node to the flow.
        
        Args:
            name: Human-readable name for the action
            action_callable: The function to execute
        
        Returns:
            The created ActionNode
        """
        node_id = self._generate_node_id()
        action_node = ActionNode(node_id, name, action_callable)
        self.nodes[node_id] = action_node
        
        # Set as root node if this is the first node
        if self.root_node is None:
            self.root_node = action_node
        
        self.logger.debug(f"Added action node: {name} (ID: {node_id})")
        return action_node
    
    async def execute(self, context: ActionContext):
        """
        Execute the defined strategy flows with support for parallel execution.
        
        If parallel_flows are defined, execute them in parallel.
        Otherwise, execute the single root_node flow.
        """
        if self.parallel_flows:
            # Execute multiple flows in parallel
            await self._execute_parallel_flows(context)
        elif self.root_node:
            # Execute single root node flow
            await self._execute_single_flow(self.root_node, context)
        else:
            self.logger.warning("No flows defined - execution skipped")
    
    async def _execute_parallel_flows(self, context: ActionContext):
        """Execute multiple independent flows in parallel"""
        try:
            self.logger.debug(f"Executing {len(self.parallel_flows)} parallel flows")
            
            # Execute all flows concurrently
            tasks = []
            for flow_root in self.parallel_flows:
                task = asyncio.create_task(self._execute_single_flow(flow_root, context))
                tasks.append(task)
            
            # Wait for all flows to complete
            await asyncio.gather(*tasks, return_exceptions=True)
            
            self.logger.debug("All parallel flows completed")
            
        except Exception as e:
            self.logger.error(f"Error during parallel flow execution: {e}")
            import traceback
            self.logger.error(f"Traceback: {traceback.format_exc()}")
    
    async def _execute_single_flow(self, root_node: Node, context: ActionContext):
        """Execute a single flow starting from the given root node"""
        current_node = root_node
        execution_path = []
        
        try:
            # Check execution condition for the root node before starting
            if isinstance(current_node, DecisionNode) and current_node.execution_condition:
                try:
                    if asyncio.iscoroutinefunction(current_node.execution_condition):
                        should_execute = await current_node.execution_condition(context)
                    else:
                        should_execute = current_node.execution_condition(context)
                    
                    if not should_execute:
                        self.logger.debug(f"Skipping flow '{current_node.name}' - execution condition not met")
                        return  # Skip this entire flow without recording anything
                except Exception as e:
                    self.logger.error(f"Error checking execution condition for {current_node.name}: {e}")
                    return  # Skip on error
            
            while current_node:
                execution_path.append(current_node.name)
                self.logger.debug(f"Executing node: {current_node.name}")
                
                if isinstance(current_node, DecisionNode):
                    # Check if this node should be skipped
                    if await self._should_skip_node(current_node, context):
                        # Don't record skipped decisions - just return execution info for flow control
                        current_node = current_node.if_true  # Continue with the true path (skip satisfied nodes)
                        continue
                    
                    # Evaluate decision and record details automatically
                    decision_info = await self._evaluate_and_record_decision(current_node, context)
                    
                    # Mark node as satisfied if result is True and skip_when_satisfied is enabled
                    if decision_info["result"] and current_node.skip_when_satisfied:
                        current_node.satisfied = True
                    
                    # Add to strategy's decision timeline
                    self.strategy.add_decision_timeline(decision_info)
                    
                    # Continue execution based on result
                    current_node = decision_info["next_node"]
                    
                elif isinstance(current_node, ActionNode):
                    # Execute action and record details
                    action_info = await self._execute_and_record_action(current_node, context)
                    
                    # Add to strategy's decision timeline
                    self.strategy.add_decision_timeline(action_info)
                    
                    # Actions are terminal
                    current_node = None
                
                # Safety check to prevent infinite loops
                if len(execution_path) > 100:
                    self.logger.error("Flow execution exceeded maximum depth - possible infinite loop")
                    break
            
            self.logger.debug(f"Flow execution completed. Path: {' -> '.join(execution_path)}")
            
        except Exception as e:
            self.logger.error(f"Error during single flow execution: {e}")
            import traceback
            self.logger.error(f"Traceback: {traceback.format_exc()}")
    
    def to_graph_data(self) -> Dict[str, Any]:
        """
        Export the defined flow into a structured JSON-serializable dictionary
        for visualization.
        
        Returns:
            Dictionary with 'nodes' and 'edges' keys for graph visualization
        """
        nodes = []
        edges = []
        
        # Convert all nodes to dictionaries
        for node in self.nodes.values():
            nodes.append(node.to_dict())
        
        # Generate edges based on node relationships
        for node in self.nodes.values():
            if isinstance(node, DecisionNode):
                # Add condition edge (always present for decision nodes)
                condition_node_id = f"{node.node_id}_condition"
                nodes.append({
                    "id": condition_node_id,
                    "type": "ConditionGroup",
                    "label": node.condition.get_description(),
                    "rules": node.condition.get_rule_names()
                })
                
                edges.append({
                    "source": node.node_id,
                    "target": condition_node_id,
                    "label": "condition"
                })
                
                # Add if_true edge
                if node.if_true:
                    edges.append({
                        "source": node.node_id,
                        "target": node.if_true.node_id,
                        "label": "if_true"
                    })
                
                # Add if_false edge
                if node.if_false:
                    edges.append({
                        "source": node.node_id,
                        "target": node.if_false.node_id,
                        "label": "if_false"
                    })
        
        return {
            "nodes": nodes,
            "edges": edges,
            "root_node_id": self.root_node.node_id if self.root_node else None,
            "total_nodes": len(self.nodes),
            "execution_metadata": {
                "strategy_id": self.strategy.strategy_id,
                "generated_at": datetime.now().isoformat()
            }
        }
    
    def set_root_node(self, node: Node):
        """Set the root node for execution"""
        self.root_node = node
        self.logger.debug(f"Set root node: {node.name}")
    
    def get_node_count(self) -> int:
        """Get total number of nodes in the flow"""
        return len(self.nodes)
    
    async def _evaluate_and_record_decision(self, node: DecisionNode, context: ActionContext) -> Dict[str, Any]:
        """Evaluate decision and capture all details for UI"""
        try:
            # Evaluate the condition
            result = await node.condition.evaluate(context)
            
            # Capture context values (current prices, indicators, etc.)
            context_values = self._capture_context_values(context)
            
            # Capture evaluation details (which rules were evaluated, their results)
            evaluation_details = await self._capture_evaluation_details(node.condition, context)
            
            # Determine next node
            next_node = node.if_true if result else node.if_false
            
            # Determine signal type for UI filtering
            signal_type = self._determine_signal_type(node.name, result, next_node)
            
            # Add signal classification to evaluation details
            evaluation_details["signal_type"] = signal_type
            
            return {
                "timestamp": context.current_time.isoformat() if context.current_time else datetime.now().isoformat(),
                "rule_description": node.name,
                "condition_description": node.condition.get_description(),
                "result": result,
                "context_values": context_values,
                "evaluation_details": evaluation_details,
                "next_action": next_node.name if next_node else "End Flow",
                "node_type": "Decision",
                "node_id": node.node_id,
                "signal_type": signal_type,  # Add signal type at top level for UI filtering
                "next_node": next_node  # Internal use for flow control
            }
            
        except Exception as e:
            self.logger.error(f"Error evaluating decision node {node.name}: {e}")
            return {
                "timestamp": context.current_time.isoformat() if context.current_time else datetime.now().isoformat(),
                "rule_description": node.name,
                "condition_description": f"Error: {e}",
                "result": False,
                "context_values": {},
                "evaluation_details": {"error": str(e), "signal_type": "evaluation"},
                "next_action": "Error - End Flow",
                "node_type": "Decision",
                "node_id": node.node_id,
                "signal_type": "evaluation",
                "next_node": None
            }
    
    async def _execute_and_record_action(self, node: ActionNode, context: ActionContext) -> Dict[str, Any]:
        """Execute action and capture details for UI"""
        try:
            # Execute the action
            if asyncio.iscoroutinefunction(node.action_callable):
                await node.action_callable(context)
            else:
                node.action_callable(context)
            
            # Capture context values after action
            context_values = self._capture_context_values(context)
            
            return {
                "timestamp": context.current_time.isoformat() if context.current_time else datetime.now().isoformat(),
                "rule_description": node.name,
                "condition_description": f"Action: {node.action_callable.__name__}",
                "result": True,
                "context_values": context_values,
                "evaluation_details": {
                    "action_function": node.action_callable.__name__,
                    "action_type": "async" if asyncio.iscoroutinefunction(node.action_callable) else "sync"
                },
                "next_action": "Action Completed",
                "node_type": "Action",
                "node_id": node.node_id
            }
            
        except Exception as e:
            self.logger.error(f"Error executing action node {node.name}: {e}")
            return {
                "timestamp": context.current_time.isoformat() if context.current_time else datetime.now().isoformat(),
                "rule_description": node.name,
                "condition_description": f"Action Error: {e}",
                "result": False,
                "context_values": {},
                "evaluation_details": {"error": str(e)},
                "next_action": "Action Failed",
                "node_type": "Action",
                "node_id": node.node_id
            }
    
    def _capture_context_values(self, context: ActionContext) -> Dict[str, Any]:
        """Capture current market context for UI display"""
        try:
            context_values = {}
            
            # Try to get current price
            current_price = None
            if hasattr(self.strategy, 'get_current_price_from_provider'):
                try:
                    current_price = self.strategy.get_current_price_from_provider()
                    context_values["current_price"] = current_price
                except:
                    pass
            
            if hasattr(self.strategy, 'get_state'):
                # Get UI states registered by the strategy
                ui_states = []
                if hasattr(self.strategy, 'get_ui_states'):
                    ui_states = self.strategy.get_ui_states()
                    self.logger.debug(f"Using strategy-registered UI states: {ui_states}")
                else:
                    # No UI states registered - only capture standard framework fields
                    self.logger.debug("No UI states registered by strategy")
                
                for state_key in ui_states:
                    try:
                        value = self.strategy.get_state(state_key)
                        if value is not None:
                            context_values[state_key] = value
                    except:
                        pass
            
            # **FRAMEWORK ENHANCEMENT**: Automatically calculate unrealized P&L
            try:
                unrealized_pnl = self._calculate_unrealized_pnl(current_price, context_values)
                if unrealized_pnl is not None:
                    context_values["unrealized_pnl"] = unrealized_pnl
            except Exception as e:
                self.logger.debug(f"Could not calculate unrealized P&L: {e}")
            
            # Try to get account balance from order executor
            if hasattr(self.strategy, 'order_executor') and hasattr(self.strategy.order_executor, 'current_capital'):
                try:
                    context_values["account_balance"] = self.strategy.order_executor.current_capital
                except:
                    pass
            
            # Add timestamp
            context_values["timestamp"] = context.current_time.isoformat() if context.current_time else datetime.now().isoformat()
            
            return context_values
            
        except Exception as e:
            self.logger.error(f"Error capturing context values: {e}")
            return {"error": str(e)}
    
    async def _capture_evaluation_details(self, condition: RuleCondition, context: ActionContext) -> Dict[str, Any]:
        """Capture detailed rule evaluation for debugging"""
        try:
            details = {
                "condition_type": condition.__class__.__name__,
                "rule_names": condition.get_rule_names(),
                "individual_results": {},
                "decision_state": {}  # Add decision_state for UI compatibility
            }
            
            # For AllOf/AnyOf conditions, capture each sub-rule result
            if hasattr(condition, 'rule_callables'):
                for rule_callable in condition.rule_callables:
                    rule_name = getattr(rule_callable, '__name__', str(rule_callable))
                    try:
                        if isinstance(rule_callable, RuleCondition):
                            # Nested rule condition
                            rule_result = await rule_callable.evaluate(context)
                        else:
                            # Direct callable rule
                            if asyncio.iscoroutinefunction(rule_callable):
                                rule_result = await rule_callable(context)
                            else:
                                rule_result = rule_callable(context)
                        
                        details["individual_results"][rule_name] = rule_result
                        
                        # Add to decision_state using the rule function name directly
                        # The strategy can override this mapping if needed
                        details["decision_state"][rule_name] = bool(rule_result)
                        
                    except Exception as e:
                        details["individual_results"][rule_name] = f"Error: {e}"
                        details["decision_state"][rule_name] = False
            
            return details
            
        except Exception as e:
            self.logger.error(f"Error capturing evaluation details: {e}")
            return {"error": str(e)}
    
    def _calculate_unrealized_pnl(self, current_price: Optional[float], context_values: Dict[str, Any]) -> Optional[float]:
        """
        Automatically calculate unrealized P&L for any strategy.
        
        This is a framework-level enhancement that provides P&L calculation
        without requiring each strategy to implement it.
        
        Args:
            current_price: Current market price
            context_values: Context values containing position and entry price info
            
        Returns:
            Unrealized P&L as float, or None if calculation not possible
        """
        try:
            # Get position information
            current_position = context_values.get("current_position", 0)
            entry_price = context_values.get("entry_price")
            
            # Need all three values to calculate P&L
            if current_position == 0 or not entry_price or not current_price:
                return None
            
            # Calculate P&L based on position type
            if current_position > 0:  # Long position
                pnl = (current_price - entry_price) * current_position
            else:  # Short position
                pnl = (entry_price - current_price) * abs(current_position)
            
            return round(pnl, 2)
            
        except Exception as e:
            self.logger.debug(f"Error calculating unrealized P&L: {e}")
            return None
    
    def _determine_signal_type(self, node_name: str, result: bool, next_node: Optional[Node]) -> str:
        """Determine the signal type for UI filtering"""
        # If the decision result is False, it's just an evaluation
        if not result:
            return "evaluation"
        
        # If the decision result is True but doesn't lead to an action, it's still just an evaluation
        if not next_node or not isinstance(next_node, ActionNode):
            return "evaluation"
        
        # If the decision result is True and leads to an action, classify by action type
        if isinstance(next_node, ActionNode):
            action_name = next_node.name.lower()
            if "buy" in action_name or "open" in action_name or "enter" in action_name:
                return "entry_signal"
            elif "sell" in action_name or "close" in action_name or "exit" in action_name:
                return "exit_signal"
        
        # Default to evaluation for any other case
        return "evaluation"
    
    async def _should_skip_node(self, node: DecisionNode, context: ActionContext) -> bool:
        """Determine if a node should be skipped based on its configuration and state"""
        # Check skip_when_satisfied condition
        if node.skip_when_satisfied and node.satisfied:
            return True
        
        return False  # Don't skip by default
    
    def add_parallel_flow(self, root_node: Node):
        """Add a parallel flow that will be executed independently"""
        self.parallel_flows.append(root_node)
        self.logger.debug(f"Added parallel flow: {root_node.name}")
    
    def set_parallel_flows(self, flows: List[Node]):
        """Set multiple parallel flows to be executed independently"""
        self.parallel_flows = flows
        self.logger.debug(f"Set {len(flows)} parallel flows")
    
    def clear(self):
        """Clear all nodes and reset the flow"""
        self.nodes.clear()
        self.root_node = None
        self.parallel_flows.clear()
        self.node_counter = 0
        self.logger.debug("Flow cleared")
