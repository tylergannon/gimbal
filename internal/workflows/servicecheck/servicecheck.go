// Package servicecheck runs a database, web service, parallel command consumers,
// and an agent through scoped orchestration and secrets backends. The database
// and web server publish their connection values through ENV_FILE. The agent
// obtains its values by calling back into Gimbal from its consuming scope.
//
// The execution environment needs Docker, psql, Node, and access to the
// postgres:17-alpine image. Services are stopped when their declaring scope ends.
// The default devops-tasks model is gpt-5.6-luna; --devops-tasks overrides it.
// Success means the database query, web response, and agent's fetched response
// all contain GIMBAL_SERVICE_OK, followed by successful scoped cleanup.
//
// Example:
//
//	gimbal run service-check --work-dir /abs/work --task-route @/abs/route.json --devops-tasks gpt-5.6-luna
package servicecheck

import (
	"context"
	"fmt"

	"github.com/tylergannon/gimbal"
)

//go:generate go tool polytype --validate
//go:generate go run github.com/tylergannon/gimbal/internal/generate/gimbalgen -entry ServiceCheck -name service-check

type Result struct {
	// Summary describes the actual response the agent fetched.
	Summary string `json:"summary"`
}
type Params struct {
	// Task adds instructions after the agent fetches the web service.
	Task string
}

// ServiceCheck runs a database and web service with scoped values and consumers.
func ServiceCheck(ctx context.Context, env gimbal.Env, params Params) error {
	agent := gimbal.NewSession(ctx, gimbal.RoleDevOpsTasks, env.WorkDir)
	return gimbal.Scope(ctx, "services", func(ctx context.Context) error {
		if err := gimbal.Service(gimbal.WithYields(ctx, gimbal.DefaultStore, "DATABASE_URL"), "database", env.WorkDir, database); err != nil {
			return err
		}
		ctx = gimbal.WithRequirements(ctx, gimbal.DefaultStore, "DATABASE_URL")
		if err := gimbal.Service(gimbal.WithYields(ctx, gimbal.DefaultStore, "WEB_URL"), "web", env.WorkDir, web); err != nil {
			return err
		}
		ctx = gimbal.WithRequirements(ctx, gimbal.DefaultStore, "WEB_URL")
		group := gimbal.Group(ctx, "consumers")
		group.Go("database", func(ctx context.Context) error {
			code, _, _, err := gimbal.RunCommand(ctx, "query", env.WorkDir, "sh", "-c", `test "$(psql -X "$DATABASE_URL" -Atc "SELECT 'GIMBAL_SERVICE_OK'")" = GIMBAL_SERVICE_OK`)
			if err != nil {
				return err
			}
			if code != 0 {
				return fmt.Errorf("database query exited %d", code)
			}
			return nil
		})
		group.Go("web", func(ctx context.Context) error {
			code, _, _, err := gimbal.RunCommand(ctx, "fetch", env.WorkDir, "node", "--input-type=module", "-e", consume)
			if err != nil {
				return err
			}
			if code != 0 {
				return fmt.Errorf("web fetch exited %d", code)
			}
			return nil
		})
		if err := group.Wait(); err != nil {
			return err
		}
		gimbal.Set(ctx, "task", params.Task)
		_, err := agent.Generate[Result](ctx, agentPrompt)
		return err
	})
}

const agentPrompt = "Call the Gimbal secrets callback to obtain WEB_URL, then fetch that URL using an HTTP client. Require HTTP 200 and response body GIMBAL_SERVICE_OK. Do not merely display the URL. Return a short description of the actual response. Do not start or stop infrastructure. Complete any additional task below."
const database = `set -eu
name=gimbal-service-$$
cleanup() { docker stop -t 7 "$name" >/dev/null 2>&1 || true; wait "$pid" || true; exit 0; }
trap cleanup INT
docker run --rm --name "$name" -e POSTGRES_PASSWORD=gimbal-check -p 127.0.0.1::5432 postgres:17-alpine &
pid=$!
while ! docker exec "$name" pg_isready -U postgres >/dev/null 2>&1; do kill -0 "$pid"; sleep 0.2; done
port=$(docker port "$name" 5432/tcp | sed 's/.*://')
printf 'DATABASE_URL=postgresql://postgres:gimbal-check@127.0.0.1:%s/postgres\n' "$port" > "$ENV_FILE.tmp"
mv "$ENV_FILE.tmp" "$ENV_FILE"
wait "$pid"`
const web = `exec node --input-type=module -e '
import { createServer } from "node:http";
import { execFile } from "node:child_process";
import { writeFileSync, renameSync } from "node:fs";
const server = createServer((request, response) => {
 execFile("psql", ["-X", process.env.DATABASE_URL, "-Atc", "SELECT '\''GIMBAL_SERVICE_OK'\''"], (error, body) => {
  response.writeHead(error ? 500 : 200);
  response.end(error ? error.message : body);
 });
});
process.once("SIGINT", () => server.close(() => process.exit(0)));
server.listen(0, "127.0.0.1", () => {
 const path = process.env.ENV_FILE;
 writeFileSync(path + ".tmp", "WEB_URL=http://127.0.0.1:" + server.address().port + "\n");
 renameSync(path + ".tmp", path);
});'`
const consume = `const response = await fetch(process.env.WEB_URL, { signal: AbortSignal.timeout(5000) });
if (response.status !== 200) throw new Error("HTTP " + response.status);
const body = (await response.text()).trim();
if (body !== "GIMBAL_SERVICE_OK") throw new Error("Unexpected response: " + body);
console.log("Command fetched database-backed service:", body);`
