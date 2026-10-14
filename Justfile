origin := env("ORIGIN", "http://127.0.0.1:8080")

build: build-assets
    go build -o bin/gimbal ./cmd/gimbal

# Portable generated source and frontend: prepared once for a release.
# Invoke vp directly; Vite+ manages the package-manager version.
build-assets:
    go mod tidy
    cd web && vp install
    go generate ./...
    cd web && vp fmt
    cd web && vp build
    test -s web/build/skgo.manifest.json

dev:
    go run ./internal/generate/appgen dev

# Build the Debug macOS app around a fresh CLI. Native generation republishes SKGO outputs;
# restore Gimbal's separate observation and workflow projections afterward.
app: build
    mkdir -p native/Resources
    install -m 755 bin/gimbal native/Resources/gimbal
    go tool skgo native build --platform macos
    go generate ./internal/skgo ./workflow
    cd web && vp fmt

# Real microphone validation needs a stable identity so macOS permissions survive
# rebuilds. Ordinary source builds remain usable without a signing certificate.
app-validate: app
    scripts/sign-native-dev.sh native/build/macos/DerivedData/Build/Products/Debug/Gimbal.app

# Open the Debug app; a directory argument keeps its catalog in that directory instead of ~/.gimbal/desktop.
app-run instance="":
    {{ if instance == "" { "open -n" } else { "mkdir -p '" + instance + "' && open -n --env GIMBAL_INSTANCE_DIR=\"$(cd '" + instance + "' && pwd)\"" } }} native/build/macos/DerivedData/Build/Products/Debug/Gimbal.app

dev-web:
    cd web && ORIGIN='{{origin}}' vp dev --host 127.0.0.1 --port 5173 --strictPort

dev-go:
    GIMBAL_WEB_PROXY=http://127.0.0.1:5173 GIMBAL_WEB_ORIGIN='{{origin}}' go run ./cmd/gimbal --port 8080

e2e run="run":
    cd e2e && vp install
    cd e2e && vp exec playwright install chromium
    cd e2e && SKGO_E2E_RUN={{run}} vp run test

vet:
    go vet ./...
    go build -o bin/gimbal ./cmd/gimbal
    ./bin/gimbal lint ./...

# Repository-wide checks belong in explicit validation and CI, not commit/push hooks.
lint-go:
    golangci-lint run ./...

# Opt-in source rewrite; inspect the resulting diff before staging.
modernize:
    go tool modernize -fix ./...

check-web:
    cd web && vp run check

# Release/CI commands and self-contained installed skill helpers.
check-automation:
    bash scripts/check-automation.sh

check-hooks:
    node --test scripts/staged-hook.test.mts scripts/fast-hook.test.mts

# Identical uncached smoke tests, without the commit deadline. Warms build caches.
warm-hook:
    bash scripts/pre-commit.sh --warm

# Advisory timing report; force the pack without staging changes.
benchmark-hook:
    bash scripts/pre-commit.sh --benchmark

# Pre-release executable protocol/routing gate. Run just build first; reuse its CLI.
check-cli-release:
    test -x bin/gimbal
    GIMBAL_TEST_BINARY="$(pwd)/bin/gimbal" go test -count=1 -tags=releasecheck ./cmd/gimbal -run '^TestBinaryAnalysisAndOrdinaryCLIRoutes$'

test:
    go test -count=1 ./...
    cd web && vp run test

storybook:
    cd web && vp run storybook --host 127.0.0.1

fmt:
    cd web && vp fmt

fmt-check:
    cd web && vp fmt --check

# Refresh generated prices, aliases and Diffusion limits; needs DIFFUSION_API_KEY.
models:
    go run ./internal/modelcatalog
