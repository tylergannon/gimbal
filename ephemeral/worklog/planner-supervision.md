decision: PromiseLoop accepts the existing AgentOptions for each planner turn; supervisor timing and non-gating behavior remain unchanged.
friction: A planner turn can finish before the supervisor's first interval. Attaching supervision alone does not demonstrate that an objection reached or changed a decision; live validation must observe both the landed steer and resulting assignment.
