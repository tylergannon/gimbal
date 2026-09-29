# Operations note: unsettled default
The v2 quick-start says the default visibility timeout is 45 seconds. The v2 operations runbook says the default visibility timeout is 60 seconds. Both statements describe the default for the same v2 queue configuration. The discrepancy is unresolved; this note supplies no authority rule for selecting one value.

Set the timeout explicitly if the distinction matters to an experiment. This is operational advice, not a correction to either source. A visibility timeout is neither message retention nor a network request timeout.

The quick-start and runbook statements are preserved here as attributed evidence of the disagreement. Do not average them, infer a newer release, or silently pick the larger value. No service-level uptime percentage or paid plan price is supplied by these documents.
