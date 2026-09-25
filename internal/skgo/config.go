// Package skgo holds Gimbal's generated web implementation. Regenerate it with
// `go generate ./...`.
//
// `go tool skgo` builds the generator from the module cache, so nothing here
// needs skgo to be a writable checkout.
package skgo

//go:generate go run github.com/tylergannon/gimbal/internal/generate/stockgen ../..
//go:generate go tool skgo generate --web ../../web
//go:generate go tool polytype --target ../observation --typescript ../../web/src/lib/skgo/observation
