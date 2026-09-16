package gimble

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/tylergannon/gimble/internal/observation"
)

// TestRunCommandRecordsEachOutcome is issue 216 without a model: a command
// that succeeds, one that exits nonzero and is run again, one that cannot
// start, one its ctx cancels, and one in each of two parallel children. Each
// is its own record in the scope it ran in, and reading the finished run
// again, from any of its stores, returns the same rows.
func TestRunCommandRecordsEachOutcome(t *testing.T) {
	project := t.TempDir()
	workdir := t.TempDir()
	type result struct {
		code           int
		stdout, stderr string
		err            error
	}
	results := map[string]result{}
	record := func(id string) func(int, string, string, error) {
		return func(code int, stdout, stderr string, err error) {
			results[id] = result{code, stdout, stderr, err}
		}
	}
	var dir string
	if err := Run(Project(t.Context(), project), "commands", nil, func(ctx context.Context) error {
		dir = runDir(ctx)
		record("say.1")(RunCommand(ctx, "say", workdir, "sh", "-c", "printf out; printf err >&2"))
		if err := Scope(ctx, "retry", func(ctx context.Context) error {
			record("retry.1/check.1")(RunCommand(ctx, "check", workdir, "sh", "-c", "echo failing; exit 3"))
			record("retry.1/check.2")(RunCommand(ctx, "check", workdir, "sh", "-c", "echo passing"))
			return nil
		}); err != nil {
			return err
		}
		record("missing.1")(RunCommand(ctx, "missing", workdir, "gimble-no-such-command"))
		cancelled, cancel := context.WithTimeout(ctx, 300*time.Millisecond)
		defer cancel()
		record("wait.1")(RunCommand(cancelled, "wait", workdir, "sleep", "30"))

		group := Group(ctx, "attempts")
		for _, script := range []string{"exit 1", "exit 0"} {
			group.Go("attempt", func(ctx context.Context) error {
				_, _, _, err := RunCommand(ctx, "check", workdir, "sh", "-c", script)
				return err
			})
		}
		return group.Wait()
	}); err != nil {
		t.Fatal(err)
	}

	if got := results["say.1"]; got.code != 0 || got.stdout != "out" || got.stderr != "err" || got.err != nil {
		t.Errorf("say.1 = %+v, want exit 0 with its two streams", got)
	}
	if got := results["retry.1/check.1"]; got.code != 3 || got.stdout != "failing\n" || got.err != nil {
		t.Errorf("retry.1/check.1 = %+v, want exit 3 and no error", got)
	}
	if got := results["retry.1/check.2"]; got.code != 0 || got.err != nil {
		t.Errorf("retry.1/check.2 = %+v, want exit 0", got)
	}
	if got := results["missing.1"]; got.code != -1 || !errors.Is(got.err, exec.ErrNotFound) {
		t.Errorf("missing.1 = %+v, want -1 and exec.ErrNotFound", got)
	}
	if got := results["wait.1"]; got.code != -1 || !errors.Is(got.err, context.DeadlineExceeded) {
		t.Errorf("wait.1 = %+v, want -1 and the ctx's error", got)
	}

	rows := commandRows(t, dir)
	want := map[string]struct {
		scope       string
		exit        int
		errored     bool
		interrupted bool
	}{
		"say.1":                        {"", 0, false, false},
		"retry.1/check.1":              {"retry.1", 3, false, false},
		"retry.1/check.2":              {"retry.1", 0, false, false},
		"missing.1":                    {"", -1, true, false},
		"wait.1":                       {"", -1, true, true},
		"attempts.1/attempt.1/check.1": {"attempts.1/attempt.1", 1, false, false},
		"attempts.1/attempt.2/check.1": {"attempts.1/attempt.2", 0, false, false},
	}
	if len(rows) != len(want) {
		t.Fatalf("commands.json holds %d rows, want %d: %+v", len(rows), len(want), rows)
	}
	for _, row := range rows {
		w, ok := want[row.ID]
		if !ok {
			t.Errorf("commands.json has %q, which no command was", row.ID)
			continue
		}
		if row.Scope != w.scope || row.ExitCode != w.exit || (row.Error != "") != w.errored || row.Interrupted != w.interrupted {
			t.Errorf("%s = %+v, want scope %q, exit %d, error %v, interrupted %v", row.ID, row, w.scope, w.exit, w.errored, w.interrupted)
		}
		if row.Started == 0 || row.Ended < row.Started || row.Workdir != workdir {
			t.Errorf("%s = %+v, want its start, its end, and workdir %s", row.ID, row, workdir)
		}
	}
	if row := find(rows, "say.1"); row.Stdout != "out" || row.Stderr != "err" || row.Command != "sh" ||
		!reflect.DeepEqual(row.Args, []string{"-c", "printf out; printf err >&2"}) {
		t.Errorf("say.1 = %+v, want what ran and its output", row)
	}
	if row := find(rows, "wait.1"); row.Duration >= 5000 {
		t.Errorf("wait.1 took %d ms after its ctx ended", row.Duration)
	}

	// The same facts, however the finished run is read again: from its
	// durable snapshot, from its tables, and rebuilt from its log. Each read
	// writes the snapshot it read, so each drops that too.
	id := filepath.Base(dir)
	for _, drop := range [][]string{nil, {"observation.json", "observation-deltas.jsonl"}, {"observation.json", "commands.json"}} {
		for _, name := range drop {
			if err := os.Remove(filepath.Join(dir, name)); err != nil {
				t.Fatal(err)
			}
		}
		snapshot, err := observation.NewRegistry(project).Snapshot(id)
		if err != nil {
			t.Fatalf("read again without %v: %v", drop, err)
		}
		if len(snapshot.Commands) != len(rows) {
			t.Fatalf("read again without %v: %d commands, want %d", drop, len(snapshot.Commands), len(rows))
		}
		for _, row := range rows {
			if got := snapshot.Commands[row.ID]; !reflect.DeepEqual(got, row) {
				t.Errorf("read again without %v: %s = %+v, want %+v", drop, row.ID, got, row)
			}
		}
	}
	if rebuilt := commandRows(t, dir); !reflect.DeepEqual(rebuilt, rows) {
		t.Errorf("the rebuilt commands.json = %+v, want %+v", rebuilt, rows)
	}
}

// TestRunCommandKeepsLongOutputInAFile: a stream longer than a record keeps
// is written whole beside the run log, and the record holds its tail and
// the file's name. The caller gets all of it either way.
func TestRunCommandKeepsLongOutputInAFile(t *testing.T) {
	var dir, stdout string
	if err := runTest(t, nil, func(ctx context.Context) error {
		dir = runDir(ctx)
		var err error
		_, stdout, _, err = RunCommand(ctx, "long", "", "sh", "-c", "head -c 70000 /dev/zero | tr '\\0' x; printf end")
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if len(stdout) != 70003 {
		t.Fatalf("RunCommand returned %d bytes of stdout, want 70003", len(stdout))
	}
	row := find(commandRows(t, dir), "long.1")
	if len(row.Stdout) != commandOutputLimit || !strings.HasSuffix(row.Stdout, "xend") || row.StdoutFile != "commands/long.1.stdout" || row.StderrFile != "" {
		t.Fatalf("long.1 keeps %d bytes ending %q, file %q; want the last %d and commands/long.1.stdout",
			len(row.Stdout), row.Stdout[max(0, len(row.Stdout)-4):], row.StdoutFile, commandOutputLimit)
	}
	whole, err := os.ReadFile(filepath.Join(dir, row.StdoutFile))
	if err != nil || string(whole) != stdout {
		t.Fatalf("%s holds %d bytes (%v), want all %d", row.StdoutFile, len(whole), err, len(stdout))
	}
}

// TestRunCommandThatExitedKeepsItsExit: a command that exits on its own
// keeps its exit status, though its ctx ends while a process it left behind
// still holds its output open. An exec.Cmd that is only constructed is no
// record at all.
func TestRunCommandThatExitedKeepsItsExit(t *testing.T) {
	var dir, stdout string
	var code int
	var err error
	if runErr := runTest(t, nil, func(ctx context.Context) error {
		dir = runDir(ctx)
		_ = exec.CommandContext(ctx, "true")
		exited, cancel := context.WithCancel(ctx)
		defer cancel()
		time.AfterFunc(300*time.Millisecond, cancel)
		code, stdout, _, err = RunCommand(exited, "exited", "", "sh", "-c", "sleep 1 & printf done; exit 7")
		return nil
	}); runErr != nil {
		t.Fatal(runErr)
	}
	if code != 7 || stdout != "done" || err != nil {
		t.Fatalf("RunCommand = %d, %q, %v; want 7, \"done\", and no error", code, stdout, err)
	}
	if rows := commandRows(t, dir); len(rows) != 1 || rows[0].ExitCode != 7 || rows[0].Interrupted || rows[0].Error != "" {
		t.Fatalf("commands.json = %+v, want the one command, which exited 7", rows)
	}
}

// TestRunCommandNeedsARun: outside a run there is no scope to record in.
func TestRunCommandNeedsARun(t *testing.T) {
	code, _, _, err := RunCommand(t.Context(), "say", "", "true")
	if code != -1 || err == nil {
		t.Fatalf("RunCommand outside a run = %d, %v; want -1 and an error", code, err)
	}
}

func commandRows(t *testing.T, dir string) []observation.CommandRow {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(dir, "commands.json"))
	if err != nil {
		t.Fatal(err)
	}
	var rows []observation.CommandRow
	if err := json.Unmarshal(raw, &rows); err != nil {
		t.Fatal(err)
	}
	return rows
}

func find(rows []observation.CommandRow, id string) observation.CommandRow {
	for _, row := range rows {
		if row.ID == id {
			return row
		}
	}
	return observation.CommandRow{}
}
