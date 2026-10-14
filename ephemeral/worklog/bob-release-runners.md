# Bob release capacity

correction: User explicitly directs use of LAN server tyler@bob for faster release work and persistent caches; it has idle capacity and a reusable Go module cache.

decision: Use two Gimbal runner containers on Bob so the parallel qualification/packaging change remains effective. Ubuntu 24.04 runner image; shared host Go modules, compiler/generator/frontend caches and tool installations. Separate job workspaces. Linux arm64 stays on hosted ARM; Mac signing stays on the Mac. Existing four SKGO runner containers remain untouched.

friction: Official Ubuntu runner image has an ubuntu UID 1000 account. Removing the account also removes its private group. Map Gimbal runner UID/GID to Tyler's 1000 for compatible host cache ownership; do not remove the already removed group again.
