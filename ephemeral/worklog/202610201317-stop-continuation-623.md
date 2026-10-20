# Gimbal#623 Stop then same-session continuation

decision: Reproduced only one engine-level way a Stop ends the conversation: the adapter reports DeliveryUncertain for the stopped turn (Codex "has not confirmed the stop"). Interactive then recorded state.Unresolved and returned an error, failing the run; Manager shows it as not live. Plain cancel, native "interrupted" error, and empty output already settle to waiting and accept a next prompt on the same native session.
decision: Interactive now treats an unconfirmed Stop as settled when the prompt was already delivered (state.InputPublished); an undelivered uncertain prompt stays unresolved (no blind replay). Finish semantics unchanged.
friction: codex/codex.go stop confirmation fallback (thread history snapshot) was dead after the drain timeout because it reused the expired drainCtx -> read history with its own bounded context. Fake-daemon test added (5s, waits out controlTimeout).
friction: python str.replace with tab-indented Go silently missed or hit two places (steering test); use Edit tool for Go edits.
limitation: Could not observe the real Codex `sleep 45` + Stop; root cause on the live provider (missing turn/completed within 5s vs genuinely unconfirmed) is inferred, not seen. Stale "Running turn 0" label belongs to live-event projection (#622).
