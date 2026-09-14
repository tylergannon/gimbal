# #152: Cost only appears on session and scope totals, never on a message row

https://github.com/tylergannon/gimble/issues/152

Follow-up from the token accounting port (#149).

No harness states a per-call cost. Claude Code states cost per turn in the `result` message's `modelUsage`; Codex and Antigravity state none. So every `session.step.ended` carries `cost: 0`, every assistant message renders `$0`, and dollars appear only in the session running total (`session.usage.updated`) and in the scope live query.

Options if per-message dollars are wanted: attribute a Claude turn's stated cost to its steps in proportion to tokens, or accept turn-level cost as the finest grain. Not needed for the current page.
