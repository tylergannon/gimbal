//go:build unix

package codex

import "syscall"

// darwinOpenMax is the practical ceiling macOS enforces (via
// kern.maxfilesperproc) even when RLIMIT_NOFILE's hard limit reports as
// RLIM_INFINITY. Asking Setrlimit for the reported hard limit fails with
// EINVAL in that case; this is the fallback raiseFileDescriptorLimit tries
// next, chosen to comfortably clear the fan-out this adapter drives (see
// the codex package doc) without guessing at a value the kernel will also
// reject.
const darwinOpenMax = 65536

// raiseFileDescriptorLimit raises this process's soft RLIMIT_NOFILE, in
// place, before connect execs `codex app-server daemon start`. Go itself
// raises its own soft limit at startup but deliberately hands children the
// original value (see runtime/rlimit.go), so a daemon this process starts
// would otherwise inherit whatever small soft limit the login shell set
// (256 on a default macOS shell). Calling Setrlimit ourselves changes the
// process's actual limit, which is what a child started afterwards
// inherits.
//
// The target is the hard limit, capped at darwinOpenMax: on macOS the hard
// limit routinely reports as RLIM_INFINITY (observed: Getrlimit returning
// Max = math.MaxInt64) while the kernel enforces a real ceiling
// (kern.maxfilesperproc) underneath it. Setrlimit does not reject that
// sentinel outright — it happily sets Cur to it — but a soft limit of
// literal infinity is its own hazard (fd-table-sized allocations, select's
// FD_SETSIZE bitmap), so the cap is applied before calling Setrlimit, not
// only as a fallback after it fails.
//
// It never reports an error: a daemon that starts with the original,
// smaller limit is still better than one gimble refused to start over a
// housekeeping call. connect() calls this unconditionally right before
// `daemon start`; a daemon that is already running is never touched.
func raiseFileDescriptorLimit() {
	var limit syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &limit); err != nil {
		return
	}
	target := limit.Max
	if target > darwinOpenMax {
		target = darwinOpenMax
	}
	if target <= limit.Cur {
		return // already at least the target; nothing to raise
	}
	raised := limit
	raised.Cur = target
	_ = syscall.Setrlimit(syscall.RLIMIT_NOFILE, &raised)
}
