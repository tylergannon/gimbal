# Interview build workflow

decision: Build #249 through a small Gimble PromiseLoop, starting with two parallel API reconnaissance nodes. Keep downloaded references untracked and expose their absolute directory through scoped context.
decision: User selected Claude/ChatGPT only: Sonnet/Terra for reconnaissance and scope-only supervision; Opus/Sol for hard planning, implementation, and independent validation. No Gemini.
decision: User explicitly requested supervisors to steer against over-engineering and work beyond the issue. Supervisors remain advisory; requirements and observed behavior determine completion.
correction: PromiseLoop exhaustion and a nil Err do not establish fulfillment. The workflow must preserve explicit overall acceptance and return incomplete when dispatch ends without it.
friction: Frontend reconnaissance produced a long proposed implementation alongside API facts. Keep research packets factual and proposed integration notes brief so they do not become a second specification; an operator steer to the planner landed.
friction: The first Opus assignment prescribed types, data structures, and method names despite the outcome-focused Task contract. Preserve the exact dispatch record, steer the worker that the recipe is advisory, and send the planner an outcome-focused instruction for subsequent decisions.
