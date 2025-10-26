from .actions import Action, ActionContext, ActionResult
from typing import Optional, Tuple

class SelectLegsAction(Action):
    def __init__(
        self,
        name: str,
        delta_range: Optional[Tuple[float, float]] = None,
        days_to_expiration: Optional[Tuple[int, int]] = None,
        price_range: Optional[Tuple[float, float]] = None,
        distance_from_money: Optional[Tuple[int, int]] = None,
        custom_filter: Optional[callable] = None,
        **kwargs
    ):
        super().__init__(name, **kwargs)
        self.delta_range = delta_range
        self.days_to_expiration = days_to_expiration
        self.price_range = price_range
        self.distance_from_money = distance_from_money
        self.custom_filter = custom_filter

    async def execute(self, context: ActionContext) -> ActionResult:
        try:
            # Get the underlying symbol from the context
            underlying_symbol = context.strategy_state.get("symbol")
            if not underlying_symbol:
                return ActionResult(success=False, error="Underlying symbol not found in context.")

            # Get the data provider from the context
            data_provider = context.strategy_state.get("data_provider")
            if not data_provider:
                return ActionResult(success=False, error="Data provider not found in context.")

            # Fetch the options chain
            options_chain = await data_provider.get_options_chain(underlying_symbol)
            if not options_chain:
                return ActionResult(success=False, error="Could not fetch options chain.")

            filtered_legs = options_chain

            if self.delta_range:
                filtered_legs = [
                    leg for leg in filtered_legs
                    if self.delta_range[0] <= abs(leg.get("greeks", {}).get("delta", 0)) <= self.delta_range[1]
                ]

            if self.days_to_expiration:
                # This assumes the options chain data includes DTE.
                # If not, it would need to be calculated from the expiration date.
                filtered_legs = [
                    leg for leg in filtered_legs
                    if self.days_to_expiration[0] <= leg.get("days_to_expiration", float('inf')) <= self.days_to_expiration[1]
                ]

            if self.price_range:
                filtered_legs = [
                    leg for leg in filtered_legs
                    if self.price_range[0] <= leg.get("last_price", 0) <= self.price_range[1]
                ]

            if self.custom_filter:
                filtered_legs = [
                    leg for leg in filtered_legs
                    if self.custom_filter(leg)
                ]

            context.strategy_state["selected_legs"] = filtered_legs
            
            return ActionResult(
                success=True,
                data={"selected_legs": filtered_legs},
                details=f"Successfully selected {len(filtered_legs)} option legs."
            )
        except Exception as e:
            return ActionResult(success=False, error=str(e))
