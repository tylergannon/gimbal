# Portability boundary excerpt

Sources: [`macos-setpgid.md`](../sources/macos-setpgid.md), [`linux-setsid.md`](../sources/linux-setsid.md), [`macos-launchd-boundary.md`](../sources/macos-launchd-boundary.md), and [`go-exec-source.txt`](../../topic-009/sources/go-exec-source.txt).

Both platforms expose process groups and signals, but the downloaded sources do not establish one portable Go recipe for creating, naming, and proving an empty descendant set after a shell or nested tool changes process-group/session membership. Go's documented default cancellation kills only the direct process. Linux's `setsid` precondition and Apple's distinct `setpgid` documentation make the exact setup and verification implementation-dependent.

Apple's launchd guidance is not a solution for issue 276: it describes independently managed daemon jobs, startup/relaunch behavior, and configuration rules, and explicitly belongs to launchd-managed daemons. It is evidence for keeping launchd out of the scope-owned foreground contract.
