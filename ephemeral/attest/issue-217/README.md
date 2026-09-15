# Issue 217 attestation

Head: `cc2ca0c` (`Preserve Claude run-prompt failure diagnostics`)

Claim: a successful Claude turn delivers its final response.

Claude Haiku (`claude-haiku-4-5-20251001`) ran `gimble run-prompt` as run
`01M2HJ60GHQ6KFRHBBMS4SDA22.run-prompt`, exited 0, printed its `Logs:` path,
and returned `ISSUE-217-OK`. The durable local record is
`ephemeral/attest/issue-217/claude-success/runs/01M2HJ60GHQ6KFRHBBMS4SDA22.run-prompt`.

Claim: a provider failure remains a failure, exposes useful provider detail,
and still prints its durable run location.

Claude was invoked with the deliberately nonexistent model
`claude-definitely-invalid-issue-217` as run
`01M2HJBQJFCE65QG21ZAK700BJ.run-prompt`. It exited 1 with
`model_not_found`, preserved request ID `req_011Cf4Xfu1o4rz5F9UEpPFq9`, and
printed its `Logs:` path. The durable local record is
`ephemeral/attest/issue-217/claude-provider-failure-fixed/runs/01M2HJBQJFCE65QG21ZAK700BJ.run-prompt`.

Independent validation: Codex `gpt-5.6-luna` ran
`01M2HJCTPWFTMWN84TBA9BXEPP.run-prompt`, inspected both records, reproduced
the invalid-model probe itself, and returned PASS for all four issue claims.
Its durable local record is
`ephemeral/attest/issue-217/validator-final/runs/01M2HJCTPWFTMWN84TBA9BXEPP.run-prompt`.

Required checks: `just vet` and `just test` passed after `just build` supplied
the embedded web artifact that the web tests require.
