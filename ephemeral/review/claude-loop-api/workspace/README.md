# semverbump

`semverbump` bumps a Semantic Version 2.0.0 version using only the Go standard
library.

Build and run it from this directory:

```sh
go build -o semverbump .
./semverbump 1.2.3 patch
# 1.2.4

./semverbump v1.2.3-rc.1+build.7 patch
# 1.2.3

./semverbump 1.2.3-rc.1 release
# 1.2.3
```

The kind may be `major`, `minor`, `patch`, or `release`. Invalid versions,
kinds, and argument lists produce a diagnostic on stderr and exit with status
2.
