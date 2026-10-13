// Package skgo holds Gimbal's generated web implementation. Regenerate it with
// `go generate ./...`.
//
// `go tool skgo` builds the generator from the module cache, so nothing here
// needs skgo to be a writable checkout.
//
// `go tool skgo native build` runs `skgo generate` again with the hook package
// and the Swift selection in native/skgo-native.json. The directive below
// passes the same flags, so both write the same bindings; TestNativeSelection
// keeps the two lists equal. goimports then groups the imports the way the
// commit hook does.
//
// polytype writes the observation declarations after skgo generate because
// skgo removes any generated types.ts under web/src that it did not write.
package skgo

//go:generate go run github.com/tylergannon/gimbal/internal/generate/stockgen ../..
//go:generate go tool skgo generate --web ../../web --locals-package github.com/tylergannon/gimbal/internal/app --hook-package github.com/tylergannon/gimbal/internal/serverhooks --swift-out ../../native/Generated.swift --swift-remote src/lib/transcript.remote.ts#abortSession --swift-remote src/lib/transcript.remote.ts#acceptTranscript --swift-remote src/lib/transcript.remote.ts#activateSession --swift-remote src/lib/transcript.remote.ts#closeSession --swift-remote src/lib/transcript.remote.ts#openSession --swift-remote src/lib/transcript.remote.ts#touchSession
//go:generate go tool goimports -w .
//go:generate go tool polytype --target ../observation --typescript ../../web/src/lib/observation
