package researcheval

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/tylergannon/gimbal"
	"github.com/tylergannon/gimbal/internal/observation"
)

func readFile(ctx context.Context, path string) ([]byte, error) {
	code, body, stderr, err := gimbal.RunCommand(ctx, "read-file", "/", "cat", "--", path)
	if err != nil || code != 0 {
		return nil, fmt.Errorf("read %s: exit %d: %s: %w", path, code, stderr, err)
	}
	return []byte(body), nil
}
func writeFile(ctx context.Context, path string, data []byte, _ uint32) error {
	code, _, stderr, err := gimbal.RunCommand(ctx, "write-file", "/", "sh", "-c", `printf '%s' "$2" > "$1.tmp" && mv -- "$1.tmp" "$1"`, "write-file", path, string(data))
	if err != nil || code != 0 {
		return fmt.Errorf("write %s: exit %d: %s: %w", path, code, stderr, err)
	}
	return nil
}
func makeDirectory(ctx context.Context, path string, _ uint32) error {
	code, _, stderr, err := gimbal.RunCommand(ctx, "create-directory", "/", "mkdir", "-p", "--", path)
	if err != nil || code != 0 {
		return fmt.Errorf("create %s: exit %d: %s: %w", path, code, stderr, err)
	}
	return nil
}
func removePath(ctx context.Context, path string) error {
	code, _, stderr, err := gimbal.RunCommand(ctx, "remove-path", "/", "rm", "-rf", "--", path)
	if err != nil || code != 0 {
		return fmt.Errorf("remove %s: exit %d: %s: %w", path, code, stderr, err)
	}
	return nil
}
func movePath(ctx context.Context, source, target string) error {
	code, _, stderr, err := gimbal.RunCommand(ctx, "move-path", "/", "mv", "--", source, target)
	if err != nil || code != 0 {
		return fmt.Errorf("move %s: exit %d: %s: %w", source, code, stderr, err)
	}
	return nil
}
func realPath(ctx context.Context, path string) (string, error) {
	code, body, stderr, err := gimbal.RunCommand(ctx, "resolve-path", "/", "python3", "-c", `import os,sys; print(os.path.realpath(sys.argv[1], strict=True))`, path)
	if err != nil || code != 0 {
		return "", fmt.Errorf("resolve %s: exit %d: %s: %w", path, code, stderr, err)
	}
	return strings.TrimSpace(body), nil
}
func globPaths(ctx context.Context, pattern string) ([]string, error) {
	var paths []string
	code, body, stderr, err := gimbal.RunCommand(ctx, "list-paths", "/", "python3", "-c", `import glob,json,sys; print(json.dumps(sorted(glob.glob(sys.argv[1]))))`, pattern)
	if err != nil || code != 0 {
		return nil, fmt.Errorf("list paths: exit %d: %s: %w", code, stderr, err)
	}
	err = json.Unmarshal([]byte(body), &paths)
	return paths, err
}
func clock(ctx context.Context) (float64, error) {
	code, body, stderr, err := gimbal.RunCommand(ctx, "read-clock", "/", "python3", "-c", "import time; print(time.monotonic())")
	if err != nil || code != 0 {
		return 0, fmt.Errorf("clock: exit %d: %s: %w", code, stderr, err)
	}
	return strconv.ParseFloat(strings.TrimSpace(body), 64)
}
func snapshot(ctx context.Context, project, id string) (observation.RunSnapshot, error) {
	var result observation.RunSnapshot
	code, body, stderr, err := gimbal.RunCommand(ctx, "observe-run", project, "python3", "-c", snapshotCommand, project, id)
	if err != nil || code != 0 {
		return result, fmt.Errorf("observe %s: exit %d: %s: %w", id, code, stderr, err)
	}
	err = json.Unmarshal([]byte(body), &result)
	return result, err
}
func runningIDs(ctx context.Context, project string) ([]string, error) {
	var result struct {
		Runs []struct {
			ID string `json:"id"`
		} `json:"runs"`
	}
	code, body, stderr, err := gimbal.RunCommand(ctx, "list-runs", project, "gimbal", "runs", "--work-dir", project)
	if err != nil || code != 0 {
		return nil, fmt.Errorf("list runs: exit %d: %s: %w", code, stderr, err)
	}
	if err = json.Unmarshal([]byte(body), &result); err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(result.Runs))
	for _, row := range result.Runs {
		ids = append(ids, row.ID)
	}
	return ids, nil
}

const snapshotCommand = `import subprocess,sys
p=subprocess.Popen(["gimbal","watch","--work-dir",sys.argv[1],sys.argv[2]],stdout=subprocess.PIPE,text=True)
try:
 line=p.stdout.readline()
 if not line: raise RuntimeError("run snapshot unavailable")
 print(line,end="")
finally:
 p.terminate()
 p.wait()`

const readCorpusCommand = `import os,json,sys
root=sys.argv[1]
result={}
for base,dirs,files in os.walk(root):
 for name in dirs+files:
  if os.path.islink(os.path.join(base,name)): raise RuntimeError("source symlink "+name)
 for name in sorted(files):
  path=os.path.join(base,name)
  with open(path) as source: result[os.path.relpath(path,root)]=source.read()
print(json.dumps(result,sort_keys=True))`

const cancelResearchCommand = `import glob,http.client,json,os,socket,sys
project,run=sys.argv[1:]
failures=[]
for path in sorted(glob.glob(os.path.join(project,".gimbal","control","*.json"))):
 try:
  with open(path) as source: control=json.load(source)
  connection=http.client.HTTPConnection("localhost")
  connection.sock=socket.socket(socket.AF_UNIX,socket.SOCK_STREAM)
  connection.sock.connect(control["socket"])
  connection.request("POST","/control/cancel",json.dumps({"run":run}),{"X-Gimbal-Project":project,"Content-Type":"application/json"})
  response=connection.getresponse()
  body=response.read()
  connection.close()
  if response.status==200: break
  failures.append(body.decode())
 except Exception as error: failures.append(str(error))
else: raise RuntimeError("cancel research: "+"; ".join(failures))`
