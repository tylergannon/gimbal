decision: Plan #466 and #467 against their actual consumer together; wait for real #468 squash merge, then discard its historical ancestry in favor of origin/main.
doc_bug: skills/gimbal-workflows still advertises FulfillService for remote service bindings, but messaging routes explicitly reject it; service-check still uses a shared-workdir address file. This is the exact consumer gap for #467.
decision: HTTP messaging routes distinguish agent/process producers; one task-route name cannot establish the actual consumer's network identity.
