# Local and Docker execution

- decision: Implement the accepted 2026-10-10 plan from Gimbal View commit 53e208d against Gimbal b7b7e78f. Message bus separation is the very next delivery after validation; this slice keeps concrete environment setup, effects, and observations separate.
- correction: The canceled implementation's receipt journals, fencing, guardians, and per-task Gimbal processes are excluded. Workflow processes remain independent; task handlers execute in daemon goroutines.
- decision: Keep workflow attribution and durable records at the existing runner. Use explicitly shared absolute workspace/artifact paths for this same-machine Docker slice. No host provider login-state copying.
- failure: A nil schema serialized as JSON null changed the native Pi turn contract. Omit absent schemas on the wire and check the native adapter receives no schema bytes.
- failure: Holding the daemon's ownership lock during native session creation blocked unrelated runs. Reserve identity under the lock, then create outside it with a cleanup-cancellable context.
- correction: Lost admission replies must reconcile the assigned task identity without replaying the effect. Keep cleanup ownership when delivery cannot be established.
- failure: Pi text/tool events lacked the assistant boundaries required by the console reducer. Test projected message content through that reducer, beyond checking event names.
- failure: The direct artifact-handler test missed the central project's path rewriting of escaped scope separators. Keep a full central HTTP integration test for nested command output.
- decision: Preserve the original failed-run verdict when cleanup later succeeds. Publish the later cleanup outcome using the existing observation records, after the workflow writer has stopped.
