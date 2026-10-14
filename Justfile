origin := env("ORIGIN", "http://127.0.0.1:8080")

build:
    go mod tidy
    cd web && pnpm install
    go generate ./...
    cd web && pnpm exec vp fmt
    cd web && pnpm exec vp build
    test -s web/build/skgo.manifest.json
    go build -o bin/gimbal ./cmd/gimbal

dev:
    go run ./internal/generate/appgen dev

# Build the Debug macOS app around a fresh CLI. Native generation republishes SKGO outputs;
# restore Gimbal's separate observation and workflow projections afterward.
app: build
    mkdir -p native/Resources
    install -m 755 bin/gimbal native/Resources/gimbal
    go tool skgo native build --platform macos
    go generate ./internal/skgo ./workflow
    cd web && pnpm exec vp fmt

# Open the Debug app; a directory argument keeps its catalog in that directory instead of ~/.gimbal/desktop.
app-run instance="":
    {{ if instance == "" { "open -n" } else { "mkdir -p '" + instance + "' && open -n --env GIMBAL_INSTANCE_DIR=\"$(cd '" + instance + "' && pwd)\"" } }} native/build/macos/DerivedData/Build/Products/Debug/Gimbal.app

dev-web:
    cd web && ORIGIN='{{origin}}' pnpm exec vp dev --host 127.0.0.1 --port 5173 --strictPort

dev-go:
    GIMBAL_WEB_PROXY=http://127.0.0.1:5173 GIMBAL_WEB_ORIGIN='{{origin}}' go run ./cmd/gimbal --port 8080

e2e run="run":
    cd e2e && pnpm install
    cd e2e && pnpm exec playwright install chromium
    cd e2e && SKGO_E2E_RUN={{run}} pnpm test

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
    cd web && pnpm check

check-hooks:
    python3 -B -m unittest discover -s scripts -p 'staged_hook_test.py' -v
    python3 -B -m unittest discover -s scripts -p 'fast_hook_test.py' -v

# Identical uncached smoke tests, without the commit deadline. Warms build caches.
warm-hook:
    python3 -B scripts/pre-commit.py --warm

# Advisory timing report; force the pack without staging changes.
benchmark-hook:
    python3 -B scripts/pre-commit.py --benchmark

# Pre-release executable protocol/routing gate. Run just build first; reuse its CLI.
check-cli-release:
    test -x bin/gimbal
    GIMBAL_TEST_BINARY="$(pwd)/bin/gimbal" go test -count=1 -tags=releasecheck ./cmd/gimbal -run '^TestBinaryAnalysisAndOrdinaryCLIRoutes$'

test:
    go test -count=1 ./...
    cd web && pnpm test

storybook:
    cd web && pnpm storybook --host 127.0.0.1

fmt:
    cd web && pnpm exec vp fmt

fmt-check:
    cd web && pnpm exec vp fmt --check

# Refresh generated prices, aliases and Diffusion limits; needs DIFFUSION_API_KEY.
models:
    go run ./internal/modelcatalog
