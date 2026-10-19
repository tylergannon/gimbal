# PR 616 release lint follow-up

friction: PR affected tests passed but release v0.17.0 full lint found two unchecked test-writer Close calls and a staticcheck De Morgan simplification. Check Close errors in the affected readback tests and express the equivalent final-flush guard directly; keep identity, native timing, and cancellation behavior unchanged.
