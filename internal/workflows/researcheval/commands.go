package researcheval

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"slices"
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
	code, body, stderr, err := gimbal.RunCommand(ctx, "resolve-path", "/", "node", "-e", `console.log(require("node:fs").realpathSync(process.argv[1]))`, path)
	if err != nil || code != 0 {
		return "", fmt.Errorf("resolve %s: exit %d: %s: %w", path, code, stderr, err)
	}
	return strings.TrimSpace(body), nil
}
func globPaths(ctx context.Context, pattern string) ([]string, error) {
	var names []string
	code, body, stderr, err := gimbal.RunCommand(ctx, "list-paths", "/", "node", "-e", `const fs=require("node:fs");try{console.log(JSON.stringify(fs.readdirSync(process.argv[1])))}catch(error){if(error.code!=="ENOENT"&&error.code!=="ENOTDIR")throw error;console.log("[]")}`, filepath.Dir(pattern))
	if err != nil || code != 0 {
		return nil, fmt.Errorf("list paths: exit %d: %s: %w", code, stderr, err)
	}
	if err = json.Unmarshal([]byte(body), &names); err != nil {
		return nil, err
	}
	paths := []string{}
	for _, name := range names {
		if strings.HasPrefix(name, ".") && !strings.HasPrefix(filepath.Base(pattern), ".") {
			continue
		}
		matched, err := filepath.Match(filepath.Base(pattern), name)
		if err != nil {
			return nil, err
		}
		if matched {
			paths = append(paths, filepath.Join(filepath.Dir(pattern), name))
		}
	}
	slices.Sort(paths)
	return paths, nil
}
func clock(ctx context.Context) (float64, error) {
	code, body, stderr, err := gimbal.RunCommand(ctx, "read-clock", "/", "node", "-e", "console.log(Number(process.hrtime.bigint())/1e9)")
	if err != nil || code != 0 {
		return 0, fmt.Errorf("clock: exit %d: %s: %w", code, stderr, err)
	}
	return strconv.ParseFloat(strings.TrimSpace(body), 64)
}
func snapshot(ctx context.Context, project, id string) (observation.RunSnapshot, error) {
	var result observation.RunSnapshot
	code, body, stderr, err := gimbal.RunCommand(ctx, "observe-run", project, "node", "-e", snapshotCommand, project, id)
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

const snapshotCommand = `const {spawn}=require("node:child_process");
const {createInterface}=require("node:readline");
const child=spawn("gimbal",["watch","--work-dir",process.argv[1],process.argv[2]],{stdio:["ignore","pipe","inherit"]});
let received=false;
const lines=createInterface({input:child.stdout});
lines.once("line",line=>{received=true;process.stdout.write(line+"\n");lines.close();child.kill("SIGTERM")});
child.once("error",error=>{console.error(error.message);process.exitCode=1});
child.once("exit",()=>{if(!received){console.error("run snapshot unavailable");process.exitCode=1}});`

const readCorpusCommand = `const fs=require("node:fs"),path=require("node:path");
const root=process.argv[1],result={};
function walk(dir){
 for(const entry of fs.readdirSync(dir,{withFileTypes:true}).sort((a,b)=>a.name.localeCompare(b.name))){
  const file=path.join(dir,entry.name);
  if(entry.isSymbolicLink())throw Error("source symlink "+entry.name);
  if(entry.isDirectory())walk(file);
  else result[path.relative(root,file)]=new TextDecoder("utf-8",{fatal:true}).decode(fs.readFileSync(file));
 }
}
walk(root);console.log(JSON.stringify(result));`
