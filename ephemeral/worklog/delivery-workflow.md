# Delivery workflow

decision: #248 uses the scoped planner task loop from #254 directly. There is no extra ordinary-iteration layer without work that requires one.
decision: A local brief holds the accepted requirements and references to any design material. The caller chooses a fixed validation command and positive task bound. An independent validator checks the actual change against the whole brief; command success alone is insufficient, and command failure cannot be overruled by prose.
correction: The first draft duplicated Loop.Tasks' reserved task value and returned success from inside the range. Exercising the actual workflow, rather than merely compiling it, is necessary to catch scope misuse; successful termination must pass through loop errors and cancellation observed after cleanup.
