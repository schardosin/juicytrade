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
                 if_true: Optional[Node] = None, if_false: Optional[Node] = None):
        super().__init__(node_id, name)
        self.condition = condition
        self.if_true = if_true
        self.if_false = if_false
    
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
        if_false: Optional[Node] = None
    ) -> Node:
        """
        Add a branching node to the flow.
        
        Args:
            name: Human-readable name for the decision
            condition: A condition object created by the Rules helper
            if_true: The node to execute if the condition is true
            if_false: The node to execute if the condition is false
        
        Returns:
            The created DecisionNode
        """
        node_id = self._generate_node_id()
        decision_node = DecisionNode(node_id, name, condition, if_true, if_false)
        self.nodes[node_id] = decision_node
        
        # Set as root node if this is the first node
        if self.root_node is None:
            self.root_node = decision_node
        
        self.logger.debug(f"Added decision node: {name} (ID: {node_id})")
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
        Execute the defined strategy graph from its root node.
        This will be called inside record_cycle_decision.
        """
        if not self.root_node:
            self.logger.warning("No root node defined - flow execution skipped")
            return
        
        current_node = self.root_node
        execution_path = []
        
        try:
            while current_node:
                execution_path.append(current_node.name)
                self.logger.debug(f"Executing node: {current_node.name}")
                
                # Execute current node and get next node
                next_node = await current_node.execute(context)
                current_node = next_node
                
                # Safety check to prevent infinite loops
                if len(execution_path) > 100:
                    self.logger.error("Flow execution exceeded maximum depth - possible infinite loop")
                    break
            
            self.logger.debug(f"Flow execution completed. Path: {' -> '.join(execution_path)}")
            
        except Exception as e:
            self.logger.error(f"Error during flow execution: {e}")
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
    
    def clear(self):
        """Clear all nodes and reset the flow"""
        self.nodes.clear()
        self.root_node = None
        self.node_counter = 0
        self.logger.debug("Flow cleared")
