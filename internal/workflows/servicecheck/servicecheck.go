// Package servicecheck runs a database, web service, parallel command consumers,
// and an agent through scoped orchestration and secrets backends. The database
// and web server publish their connection values through ENV_FILE. The agent
// obtains its values by calling back into Gimbal from its consuming scope.
//
// The execution environment needs Docker, psql, Python 3, and access to the
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
		if err := gimbal.Service(gimbal.WithYields(ctx, "", "DATABASE_URL"), "database", env.WorkDir, database); err != nil {
			return err
		}
		ctx = gimbal.WithRequirements(ctx, gimbal.StoreRequirement{Names: []string{"DATABASE_URL"}})
		if err := gimbal.Service(gimbal.WithYields(ctx, "", "WEB_URL"), "web", env.WorkDir, web); err != nil {
			return err
		}
		ctx = gimbal.WithRequirements(ctx, gimbal.StoreRequirement{Names: []string{"WEB_URL"}})
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
			code, _, _, err := gimbal.RunCommand(ctx, "fetch", env.WorkDir, "python3", "-c", consume)
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
const web = `exec python3 -c 'import http.server,os,subprocess,threading,signal
class Handler(http.server.BaseHTTPRequestHandler):
 def do_GET(self):
  body=subprocess.check_output(["psql","-X",os.environ["DATABASE_URL"],"-Atc","SELECT '\''GIMBAL_SERVICE_OK'\''"])
  self.send_response(200);self.end_headers();self.wfile.write(body)
server=http.server.ThreadingHTTPServer(("127.0.0.1",0),Handler)
threading.Thread(target=server.serve_forever,daemon=True).start()
with open(os.environ["ENV_FILE"]+".tmp","w") as f:f.write("WEB_URL=http://127.0.0.1:%d\n"%server.server_port)
os.replace(os.environ["ENV_FILE"]+".tmp",os.environ["ENV_FILE"])
signal.signal(signal.SIGINT,lambda *_:exit(0))
signal.pause()'`
const consume = `import os, urllib.request
with urllib.request.urlopen(os.environ["WEB_URL"], timeout=5) as response:
 assert response.status == 200, response.status
 body = response.read().decode().strip()
assert body == "GIMBAL_SERVICE_OK", body
print("Command fetched database-backed service:", body)`
