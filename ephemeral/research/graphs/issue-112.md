# go vet fails in a fresh clone until just build

URL: https://github.com/tylergannon/gimble/issues/112
State: closed
Updated: 2026-09-14T01:01:13Z

`web/dist.go` embeds `all:build`, and `web/build` is gitignored. In a fresh clone or worktree, `go vet ./...` and `go build ./...` fail until `just build` has created `web/build`. Every sprint run's checks depend on that step.

