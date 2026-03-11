from .actions import Rule
from src.persistence.models import Decision

class DecisionChain:
    def __init__(self, max_steps=10, max_evaluations=100):
        self.steps = []
        self.max_steps = max_steps
        self.max_evaluations = max_evaluations

    def add_step(self, rule: Rule):
        if len(self.steps) >= self.max_steps:
            raise ValueError(f"Decision chain cannot have more than {self.max_steps} steps.")
        self.steps.append(rule)

    def evaluate(self, context):
        eval_count = 0
        overall_result = True
        for step in self.steps:
            if eval_count >= self.max_evaluations:
                raise RecursionError(f"Exceeded maximum evaluations ({self.max_evaluations}) in decision chain.")
            
            decision = step.evaluate(context)
            if not decision.result:
                overall_result = False
            eval_count += 1
        return overall_result, self.get_state(context)

    def get_state(self, context):
        state = []
        for step in self.steps:
            decision = step.evaluate(context)
            state.append({
                "rule_name": step.name,
                "result": decision.result,
                "context_snapshot": decision.context_snapshot
            })
        return state
