// Package sessionstate is a hand-written Go port of OpenCode's client session
// projection: the event-driven session branches of the private handleEvent
// inside createData.
//
// # Upstream authority
//
// Ported from OpenCode at commit
// c55ee2a8152603f04a409163bd3edf79c425fbd7.
//
//	packages/client/src/solid/data.ts   blob 83c6eeb32ecccb1e4e9fb589647ea8e33f96b71e
//	packages/schema/src/session-event.ts   blob 6e43890ff8ae67d88f4990d710875f0d67b8db65
//	packages/schema/src/session-message.ts blob 7e8ce0482ae9db77bb1ddc8e9bb3fb0b4c41dcdf
//	packages/schema/src/session-inbox.ts   blob baadcc947b18c267e10fee76cb648f140ccdffe6
//	LICENSE                                blob 6439474beed8e0271df9862eff97ffd70ec2464c
//
// The upstream work is MIT licensed, Copyright (c) 2025 opencode. The full
// notice is in NOTICE.md beside this file. Every file in this package is a
// port of that source and carries the same notice by reference.
//
// # What this package is
//
// One ordered reduction of the native session events Gimbal emits into the
// state produced by the upstream client's event branches. Client cache reads
// and promise completion behavior are outside this server-side projection.
//
// It holds no Gimbal identity. Run, conversation and invocation identity
// live outside the native event; the caller dispatches to a projection
// before applying a payload, so native IDs cannot collide across concurrent
// invocations.
//
// # What this package is not
//
// The upstream store also caches projects, locations, agents, commands,
// models, providers, shells and skills. Those are excluded: they are
// application caches, not session observation.
package sessionstate
