"""
Rules Helper System for Declarative Strategy Flow Engine

This module provides the Rules static class that creates structured, inspectable
conditions for use in the declarative flow engine. These conditions replace
opaque lambda functions with transparent, debuggable rule objects.

Key Features:
- AllOf: AND logic - all rules must be true
- AnyOf: OR logic - at least one rule must be true  
- Not: NOT logic - inverts a rule result
- Structured and inspectable for visualization
- Function name inspection for human-readable descriptions
"""

from typing import Callable, Any
from abc import ABC, abstractmethod


class RuleCondition(ABC):
    """
    Abstract base class for all rule conditions in the declarative flow engine.
    
    Rule conditions are structured, inspectable objects that replace opaque
    lambda functions in strategy logic. They can be combined using logical
    operators and provide metadata for visualization.
    """
    
    @abstractmethod
    async def evaluate(self, context: Any) -> bool:
        """
        Evaluate the rule condition with the given context.
        
        Args:
            context: ActionContext containing current market data and state
            
        Returns:
            bool: True if the condition is met, False otherwise
        """
        pass
    
    @abstractmethod
    def get_description(self) -> str:
        """
        Get a human-readable description of this rule condition.
        
        Returns:
            str: Description of what this rule checks
        """
        pass
    
    @abstractmethod
    def get_rule_names(self) -> list:
        """
        Get the names of all rule functions used in this condition.
        
        Returns:
            list: List of rule function names for visualization
        """
        pass


class AllOfCondition(RuleCondition):
    """
    Rule condition that requires ALL sub-rules to be true (AND logic).
    """
    
    def __init__(self, *rule_callables: Callable):
        self.rule_callables = rule_callables
    
    async def evaluate(self, context: Any) -> bool:
        """Evaluate all rules - return True only if ALL are true."""
        try:
            import asyncio
            for rule_callable in self.rule_callables:
                if isinstance(rule_callable, RuleCondition):
                    # Nested rule condition
                    if not await rule_callable.evaluate(context):
                        return False
                else:
                    # Direct callable rule - handle both sync and async
                    if asyncio.iscoroutinefunction(rule_callable):
                        result = await rule_callable(context)
                    else:
                        result = rule_callable(context)
                    
                    if not result:
                        return False
            return True
        except Exception as e:
            # Log error but don't crash the strategy
            print(f"Error evaluating AllOfCondition: {e}")
            return False
    
    def get_description(self) -> str:
        """Get description of this AND condition."""
        rule_names = self.get_rule_names()
        if len(rule_names) == 1:
            return rule_names[0]
        return f"ALL OF ({', '.join(rule_names)})"
    
    def get_rule_names(self) -> list:
        """Get names of all rules in this condition."""
        names = []
        for rule_callable in self.rule_callables:
            if isinstance(rule_callable, RuleCondition):
                names.extend(rule_callable.get_rule_names())
            else:
                names.append(getattr(rule_callable, '__name__', str(rule_callable)))
        return names


class AnyOfCondition(RuleCondition):
    """
    Rule condition that requires ANY sub-rule to be true (OR logic).
    """
    
    def __init__(self, *rule_callables: Callable):
        self.rule_callables = rule_callables
    
    async def evaluate(self, context: Any) -> bool:
        """Evaluate all rules - return True if ANY is true."""
        try:
            import asyncio
            for rule_callable in self.rule_callables:
                if isinstance(rule_callable, RuleCondition):
                    # Nested rule condition
                    if await rule_callable.evaluate(context):
                        return True
                else:
                    # Direct callable rule - handle both sync and async
                    if asyncio.iscoroutinefunction(rule_callable):
                        result = await rule_callable(context)
                    else:
                        result = rule_callable(context)
                    
                    if result:
                        return True
            return False
        except Exception as e:
            # Log error but don't crash the strategy
            print(f"Error evaluating AnyOfCondition: {e}")
            return False
    
    def get_description(self) -> str:
        """Get description of this OR condition."""
        rule_names = self.get_rule_names()
        if len(rule_names) == 1:
            return rule_names[0]
        return f"ANY OF ({', '.join(rule_names)})"
    
    def get_rule_names(self) -> list:
        """Get names of all rules in this condition."""
        names = []
        for rule_callable in self.rule_callables:
            if isinstance(rule_callable, RuleCondition):
                names.extend(rule_callable.get_rule_names())
            else:
                names.append(getattr(rule_callable, '__name__', str(rule_callable)))
        return names


class NotCondition(RuleCondition):
    """
    Rule condition that inverts a sub-rule (NOT logic).
    """
    
    def __init__(self, rule_callable: Callable):
        self.rule_callable = rule_callable
    
    async def evaluate(self, context: Any) -> bool:
        """Evaluate rule and return the opposite."""
        try:
            import asyncio
            if isinstance(self.rule_callable, RuleCondition):
                # Nested rule condition
                return not await self.rule_callable.evaluate(context)
            else:
                # Direct callable rule - handle both sync and async
                if asyncio.iscoroutinefunction(self.rule_callable):
                    result = await self.rule_callable(context)
                else:
                    result = self.rule_callable(context)
                
                return not result
        except Exception as e:
            # Log error but don't crash the strategy
            print(f"Error evaluating NotCondition: {e}")
            return False
    
    def get_description(self) -> str:
        """Get description of this NOT condition."""
        if isinstance(self.rule_callable, RuleCondition):
            inner_desc = self.rule_callable.get_description()
        else:
            inner_desc = getattr(self.rule_callable, '__name__', str(self.rule_callable))
        return f"NOT ({inner_desc})"
    
    def get_rule_names(self) -> list:
        """Get names of all rules in this condition."""
        if isinstance(self.rule_callable, RuleCondition):
            return self.rule_callable.get_rule_names()
        else:
            return [getattr(self.rule_callable, '__name__', str(self.rule_callable))]


class Rules:
    """
    Static helper class for building structured, inspectable conditions.
    
    This class provides methods to combine individual rule functions into
    complex logical conditions that can be used in the declarative flow engine.
    
    Example usage:
        # Simple rule
        condition = Rules.AllOf(self.is_bullish_crossover)
        
        # Complex rule
        condition = Rules.AllOf(
            self.has_enough_data,
            Rules.AnyOf(
                self.is_bullish_crossover,
                self.is_oversold
            ),
            Rules.Not(self.is_in_position)
        )
    """
    
    @staticmethod
    def AllOf(*rule_callables: Callable) -> RuleCondition:
        """
        Create a condition that is TRUE only if all sub-rules are true (AND logic).
        
        Args:
            *rule_callables: A list of functions to evaluate (e.g., self.is_bullish_crossover).
                           Each function should accept an ActionContext and return a boolean.
        
        Returns:
            RuleCondition that evaluates to True only if all sub-rules return True.
        
        Example:
            # All conditions must be true
            condition = Rules.AllOf(
                self.has_enough_data,
                self.is_bullish_crossover,
                self.is_not_in_position
            )
        """
        return AllOfCondition(*rule_callables)
    
    @staticmethod
    def AnyOf(*rule_callables: Callable) -> RuleCondition:
        """
        Create a condition that is TRUE if at least one sub-rule is true (OR logic).
        
        Args:
            *rule_callables: A list of functions to evaluate.
                           Each function should accept an ActionContext and return a boolean.
        
        Returns:
            RuleCondition that evaluates to True if at least one sub-rule returns True.
        
        Example:
            # Any of these conditions can trigger
            condition = Rules.AnyOf(
                self.is_bullish_crossover,
                self.is_oversold_bounce,
                self.is_breakout_signal
            )
        """
        return AnyOfCondition(*rule_callables)
    
    @staticmethod
    def Not(rule_callable: Callable) -> RuleCondition:
        """
        Create a condition that inverts the result of a sub-rule (NOT logic).
        
        Args:
            rule_callable: The function to evaluate and invert.
                         Function should accept an ActionContext and return a boolean.
        
        Returns:
            RuleCondition that evaluates to the opposite of the sub-rule result.
        
        Example:
            # Invert a condition
            condition = Rules.Not(self.is_in_position)  # True when NOT in position
        """
        return NotCondition(rule_callable)


# Convenience aliases for common patterns
class RulePatterns:
    """
    Common rule patterns for strategy development.
    
    These are convenience methods that combine Rules in typical ways
    used by trading strategies.
    """
    
    @staticmethod
    def EntrySignal(*entry_rules: Callable, not_in_position_rule: Callable) -> RuleCondition:
        """
        Standard entry signal pattern: all entry conditions must be true AND not in position.
        
        Args:
            *entry_rules: Rules that must be true for entry
            not_in_position_rule: Rule that checks if not in position
        
        Returns:
            RuleCondition for entry signal
        """
        return Rules.AllOf(
            *entry_rules,
            not_in_position_rule
        )
    
    @staticmethod
    def ExitSignal(*exit_rules: Callable, in_position_rule: Callable) -> RuleCondition:
        """
        Standard exit signal pattern: any exit condition is true AND in position.
        
        Args:
            *exit_rules: Rules that can trigger exit (OR logic)
            in_position_rule: Rule that checks if in position
        
        Returns:
            RuleCondition for exit signal
        """
        return Rules.AllOf(
            Rules.AnyOf(*exit_rules),
            in_position_rule
        )
    
    @staticmethod
    def RiskManagement(*risk_rules: Callable, in_position_rule: Callable) -> RuleCondition:
        """
        Risk management pattern: any risk condition triggers exit if in position.
        
        Args:
            *risk_rules: Risk management rules (stop loss, take profit, etc.)
            in_position_rule: Rule that checks if in position
        
        Returns:
            RuleCondition for risk management exit
        """
        return Rules.AllOf(
            Rules.AnyOf(*risk_rules),
            in_position_rule
        )
