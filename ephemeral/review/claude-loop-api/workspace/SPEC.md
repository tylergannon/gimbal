# semverbump

A command-line tool in ordinary Go using only the standard library.

Usage: `semverbump <version> <kind>` where kind is `major`, `minor`, `patch`, or `release`.
It prints the new version to stdout followed by one newline, and exits 0.

Versions follow Semantic Versioning 2.0.0. An optional leading `v` is accepted on
input and never printed. Build metadata (`+...`) is accepted on input and never
printed.

- `major` bumps X and resets Y and Z to 0. `minor` bumps Y and resets Z. `patch` bumps Z.
- `release` removes a pending prerelease (`1.2.3-rc.1` becomes `1.2.3`). On a version with
  no prerelease, `release` is an error.
- A pending prerelease is a promise about the version it precedes. Bumping toward that
  same version finalizes the promise instead of skipping past it, as SemVer 2.0.0
  precedence requires: `1.2.3-rc.1 patch` prints `1.2.3`, not `1.2.4`.
  A bump of a higher component moves on: `1.2.3-rc.1 minor` prints `1.3.0`.

Errors: any invalid version (leading zeros, missing components, empty or malformed
prerelease identifiers, non-numeric components), an unknown kind, or a wrong number of
arguments exits 2 with a one-line diagnostic on stderr and nothing on stdout.

Supply tests and a short README with runnable examples. An external black-box
acceptance program checks the behavior and cannot be changed.
