# Review run completion and synchronization contracts

URL: https://github.com/tylergannon/gimble/issues/126
State: open
Updated: 2026-09-13T16:37:36Z

Run, turn, and reader call sites currently manufacture buffered error channels such as `runDone`, `turnDone`, `readDone`, and `readErr` to move blocking operations into goroutines and recover their errors later. Review whether the public run, cancellation, observation, and joining contracts are forcing this pattern, and define the smallest idiomatic Go contract that makes ownership and completion explicit without bespoke done channels spread through workflows and tests.

The result must preserve context-driven cancellation, deterministic joining, one authoritative error result, and freedom from goroutine leaks; do not introduce a high-level workflow wrapper merely to hide the channels.
