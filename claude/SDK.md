# Upstream revision

Gimble depends on the forked module:

```text
github.com/tylergannon/claude-agent-sdk-go
v1.1.1
revision ee4bad38b6a1aa11fa71def9530e0e609c2e18f8
```

The fork retains the upstream MIT license and declares its own module path.
The raw message observer was added to the current fork main at v1.1.1.

Local change:

- `Options.RawMessageObserver` and `WithRawMessageObserver` synchronously expose
  an owned copy of each complete, non-empty stdout JSON message immediately
  before `ParseMessage`.

The fork source patch is 163 added lines across `options.go`, `transport.go`,
and `transport_test.go` (17, 31, and 115 lines respectively).
