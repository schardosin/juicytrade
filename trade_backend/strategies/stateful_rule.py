from .actions import Rule
from .models import Decision

class StatefulRule(Rule):
    def __init__(self, name, condition, max_evaluations=100):
        super().__init__(name, condition)
        self.max_evaluations = max_evaluations
        self.last_result = None
        self.eval_count = 0

    def evaluate(self, context):
        if self.eval_count >= self.max_evaluations:
            raise RecursionError(f"Exceeded maximum evaluations ({self.max_evaluations}) for rule '{self.name}'.")

        result = self.condition(context)
        
        if result == self.last_result:
            self.eval_count += 1
        else:
            self.eval_count = 0

        self.last_result = result
        
        return Decision(
            rule_name=self.name,
            result=result,
            context_snapshot=context.get_snapshot()
        )
