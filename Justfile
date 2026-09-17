origin := env("ORIGIN", "http://127.0.0.1:8080")
base_url := env("BASE_URL", origin)

build:
    go mod tidy
    cd web && pnpm install
    go generate ./...
    cd web && pnpm exec vp build
    go build -o bin/gimble ./cmd/gimble

dev-web:
    cd web && ORIGIN='{{origin}}' pnpm exec vp dev --host 127.0.0.1 --port 5173 --strictPort

dev-go:
    GIMBLE_WEB_PROXY=http://127.0.0.1:5173 GIMBLE_WEB_ORIGIN='{{origin}}' go run ./cmd/gimble --port 8080

e2e run="run":
    cd e2e && pnpm install
    cd e2e && pnpm exec playwright install chromium
    cd e2e && BASE_URL="{{base_url}}" SKGO_E2E_RUN={{run}} pnpm test

vet:
    go vet ./...
    go build -o bin/gimble ./cmd/gimble
    ./bin/gimble lint ./...

test:
    go test -count=1 ./...
    cd web && pnpm test

prices:
    go run ./internal/observation/modelpricesgen
