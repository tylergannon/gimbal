# Codex Router research worklog

correction: The preserved legacy Codex adapter launches a separate app-server for each turn and resumes the thread. Future design should address provider identity rather than assume a single global process is the current blocker. Source: `ephemeral/legacy/harness/codex/adapter.go`.

decision: Use `modelProvider` on `thread/start` and `thread/resume` as the first app-server integration candidate. Codex 0.147.0 generated schema includes those fields; a local fake-server probe showed two provider threads in one app-server and a custom-provider turn reaching `/v1/responses`.

friction: `thread/start.config` accepts a dynamically supplied provider, but `thread/resume` has no `config` field in the generated schema. Do not rely on per-thread dynamic provider definitions across Gimbal's fresh-process resume path without a persistence test; prefer registered provider configuration or a stable dedicated `CODEX_HOME`.

friction: Portal's successful Codex smoke test used 0.155.0-alpha.16.3 while locally installed Codex is 0.147.0. Recheck provider behavior and end-to-end coding on the exact deployment version.
