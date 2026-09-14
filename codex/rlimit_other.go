//go:build !unix

package codex

// raiseFileDescriptorLimit is a no-op on platforms without RLIMIT_NOFILE
// (Windows, plan9, wasm). See rlimit_unix.go for the real thing.
func raiseFileDescriptorLimit() {}
