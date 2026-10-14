// Package skgo holds Gimbal's generated web implementation. Regenerate it with
// `go generate ./...`.
//
// appgen calls SKGO and Polytype through their pinned Go APIs. Each generator
// owns its analysis cache; cache misses lazily share one package session.
//
// `go tool skgo native build` runs `skgo generate` again with the hook package
// and the Swift selection in native/skgo-native.json. The directive below
// passes the same flags, so both write the same bindings; TestNativeSelection
// keeps the two lists equal. SKGO also emits canonical Go import groups.
//
// appgen writes the observation declarations after skgo generation because
// skgo removes any generated types.ts under web/src that it did not write.
package skgo

//go:generate go run ../generate/appgen generate --web ../../web --locals-package github.com/tylergannon/gimbal/internal/app --hook-package github.com/tylergannon/gimbal/internal/serverhooks --swift-out ../../native/Generated.swift --swift-remote src/lib/transcript.remote.ts#abortSession --swift-remote src/lib/transcript.remote.ts#acceptTranscript --swift-remote src/lib/transcript.remote.ts#activateSession --swift-remote src/lib/transcript.remote.ts#closeSession --swift-remote src/lib/transcript.remote.ts#openSession --swift-remote src/lib/transcript.remote.ts#touchSession
