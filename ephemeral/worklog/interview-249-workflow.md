# Interview build workflow

decision: Build #249 through a small Gimble PromiseLoop, starting with two parallel API reconnaissance nodes. Keep downloaded references untracked and expose their absolute directory through scoped context.
decision: User selected Claude/ChatGPT only: Sonnet/Terra for reconnaissance and scope-only supervision; Opus/Sol for hard planning, implementation, and independent validation. No Gemini.
decision: User explicitly requested supervisors to steer against over-engineering and work beyond the issue. Supervisors remain advisory; requirements and observed behavior determine completion.
correction: PromiseLoop exhaustion and a nil Err do not establish fulfillment. The workflow must preserve explicit overall acceptance and return incomplete when dispatch ends without it.
