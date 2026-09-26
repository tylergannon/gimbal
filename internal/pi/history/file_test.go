package history

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/tylergannon/gimbal/internal/pi/model"
)

var uuidV7Pattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-7[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func sessionHeaderLine(id, cwd string, version *int) string {
	header := model.SessionHeader{Type: "session", ID: id, Timestamp: "2025-01-01T00:00:00Z", Cwd: cwd, Version: version}
	raw, _ := jsonMarshal(header)
	return raw
}

func jsonMarshal(value any) (string, error) {
	raw, err := json.Marshal(value)
	return string(raw), err
}

func TestLoadEntriesFromFileMissingAndInvalid(t *testing.T) {
	dir := t.TempDir()
	header, entries, err := LoadEntriesFromFile(filepath.Join(dir, "nonexistent.jsonl"))
	if err != nil || header != nil || entries != nil {
		t.Fatalf("missing: %v %v %v", header, entries, err)
	}

	empty := filepath.Join(dir, "empty.jsonl")
	writeFile(t, empty, "")
	if header, entries, _ := LoadEntriesFromFile(empty); header != nil || entries != nil {
		t.Fatalf("empty: %v %v", header, entries)
	}

	noHeader := filepath.Join(dir, "no-header.jsonl")
	writeFile(t, noHeader, `{"type":"message","id":"1"}`+"\n")
	if header, entries, _ := LoadEntriesFromFile(noHeader); header != nil || entries != nil {
		t.Fatalf("no header: %v %v", header, entries)
	}

	malformed := filepath.Join(dir, "malformed.jsonl")
	writeFile(t, malformed, "not json\n")
	if header, entries, _ := LoadEntriesFromFile(malformed); header != nil || entries != nil {
		t.Fatalf("malformed: %v %v", header, entries)
	}

	valid := filepath.Join(dir, "valid.jsonl")
	writeFile(t, valid, sessionHeaderLine("abc", "/tmp", nil)+"\n"+
		`{"type":"message","id":"1","parentId":null,"timestamp":"2025-01-01T00:00:01Z","message":{"role":"user","content":"hi","timestamp":1}}`+"\n")
	header, entries, err = LoadEntriesFromFile(valid)
	if err != nil || header == nil || header.ID != "abc" || len(entries) != 1 {
		t.Fatalf("valid: %#v %#v %v", header, entries, err)
	}

	mixed := filepath.Join(dir, "mixed.jsonl")
	writeFile(t, mixed, sessionHeaderLine("abc", "/tmp", nil)+"\n"+"not valid json\n"+
		`{"type":"message","id":"1","parentId":null,"timestamp":"2025-01-01T00:00:01Z","message":{"role":"user","content":"hi","timestamp":1}}`+"\n")
	if _, entries, _ := LoadEntriesFromFile(mixed); len(entries) != 1 {
		t.Fatalf("mixed entries = %d", len(entries))
	}
}

func TestLoadEntriesFromFileRepairsUnterminatedTail(t *testing.T) {
	dir := t.TempDir()
	valid := filepath.Join(dir, "unterminated.jsonl")
	content := sessionHeaderLine("abc", "/tmp", nil) + "\n" +
		`{"type":"message","id":"1","parentId":null,"timestamp":"2025-01-01T00:00:01Z","message":{"role":"user","content":"hi","timestamp":1}}`
	writeFile(t, valid, content)
	if _, entries, _ := LoadEntriesFromFile(valid); len(entries) != 1 {
		t.Fatalf("entries = %d", len(entries))
	}
	if got := readFile(t, valid); got != content+"\n" {
		t.Fatalf("repaired file = %q", got)
	}

	malformedTail := filepath.Join(dir, "malformed-tail.jsonl")
	content = sessionHeaderLine("abc", "/tmp", nil) + "\n" + `{"type":"message"`
	writeFile(t, malformedTail, content)
	if _, entries, _ := LoadEntriesFromFile(malformedTail); len(entries) != 0 {
		t.Fatalf("entries = %d", len(entries))
	}
	if got := readFile(t, malformedTail); got != content+"\n" {
		t.Fatalf("repaired = %q", got)
	}

	notSession := filepath.Join(dir, "invalid.jsonl")
	content = `{"type":"message","id":"1"}`
	writeFile(t, notSession, content)
	if header, _, _ := LoadEntriesFromFile(notSession); header != nil {
		t.Fatal("expected invalid")
	}
	if got := readFile(t, notSession); got != content {
		t.Fatalf("file changed = %q", got)
	}
}

func TestOpenReadsCwdFromPrefixedHeader(t *testing.T) {
	dir := t.TempDir()
	sessionDir := t.TempDir()
	for _, test := range []struct {
		name   string
		prefix string
		id     string
	}{
		{"leading blank lines", "\n  \n", "leading-blank"},
		{"leading malformed lines", "not json\n{broken json\n", "leading-malformed"},
		{"a multi-buffer header", "", strings.Repeat("a", 8192)},
	} {
		t.Run(test.name, func(t *testing.T) {
			storedCwd := filepath.Join(dir, "stored-project")
			file := filepath.Join(sessionDir, "header.jsonl")
			writeFile(t, file, test.prefix+sessionHeaderLine(test.id, storedCwd, new(3))+"\n")
			session, err := Open(file, sessionDir, "")
			if err != nil {
				t.Fatal(err)
			}
			if session.GetSessionID() != test.id || session.GetCwd() != storedCwd {
				t.Fatalf("session = %q cwd = %q", session.GetSessionID(), session.GetCwd())
			}
		})
	}
}

func TestFindMostRecentSession(t *testing.T) {
	dir := t.TempDir()
	if _, ok := FindMostRecentSession(dir, ""); ok {
		t.Fatal("empty directory should have no session")
	}
	if _, ok := FindMostRecentSession(filepath.Join(dir, "nonexistent"), ""); ok {
		t.Fatal("missing directory should have no session")
	}
	writeFile(t, filepath.Join(dir, "file.txt"), "hello")
	if _, ok := FindMostRecentSession(dir, ""); ok {
		t.Fatal("non-jsonl should be ignored")
	}
	invalid := filepath.Join(dir, "invalid.jsonl")
	writeFile(t, invalid, `{"type":"message"}`+"\n")
	if _, ok := FindMostRecentSession(dir, ""); ok {
		t.Fatal("invalid header should be ignored")
	}

	older := filepath.Join(dir, "older.jsonl")
	newer := filepath.Join(dir, "newer.jsonl")
	writeFile(t, older, sessionHeaderLine("old", "/tmp", new(3))+"\n")
	writeFile(t, newer, sessionHeaderLine("new", "/tmp", new(3))+"\n")
	if err := os.Chtimes(older, time.Now().Add(-time.Hour), time.Now().Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}
	if got, ok := FindMostRecentSession(dir, ""); !ok || got != newer {
		t.Fatalf("most recent = %q ok=%v", got, ok)
	}

	projectA := filepath.Join(dir, "a")
	projectB := filepath.Join(dir, "b")
	fileA := filepath.Join(dir, "a.jsonl")
	fileB := filepath.Join(dir, "b.jsonl")
	writeFile(t, fileA, sessionHeaderLine("a", projectA, new(3))+"\n")
	writeFile(t, fileB, sessionHeaderLine("b", projectB, new(3))+"\n")
	if got, ok := FindMostRecentSession(dir, projectA); !ok || got != fileA {
		t.Fatalf("filter cwd = %q", got)
	}
	if got, ok := FindMostRecentSession(dir, projectB); !ok || got != fileB {
		t.Fatalf("filter cwd = %q", got)
	}
}

func TestSessionFileCreation(t *testing.T) {
	dir := t.TempDir()
	session, err := Create(dir, dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := session.AppendModelChange("anthropic", "claude-sonnet-4-5"); err != nil {
		t.Fatal(err)
	}
	if _, err := session.AppendThinkingLevelChange("off"); err != nil {
		t.Fatal(err)
	}
	if pathExists(session.GetSessionFile()) {
		t.Fatal("setup-only session should not create a file")
	}

	if _, err := session.AppendMessage(userMsg("first question")); err != nil {
		t.Fatal(err)
	}
	file := session.GetSessionFile()
	if !pathExists(file) {
		t.Fatal("first user message should create the file")
	}
	if got := readSessionFileRoles(t, file); !equalStrings(got, []string{"session", "model_change", "thinking_level_change", "user"}) {
		t.Fatalf("roles = %v", got)
	}

	reopened, err := Open(file, dir, "")
	if err != nil {
		t.Fatal(err)
	}
	if messages := reopened.BuildSessionContext().Messages; len(messages) != 1 {
		t.Fatalf("reopened messages = %d", len(messages))
	}
}

func TestSessionFileAppendsWithoutRewrite(t *testing.T) {
	dir := t.TempDir()
	session, err := Create(dir, dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := session.AppendMessage(userMsg("first question")); err != nil {
		t.Fatal(err)
	}
	if _, err := session.AppendCustomEntry("preset-state", map[string]any{"name": "plan"}); err != nil {
		t.Fatal(err)
	}
	if _, err := session.AppendMessage(assistantMsg("first answer")); err != nil {
		t.Fatal(err)
	}
	if got := readSessionFileRoles(t, session.GetSessionFile()); !equalStrings(got, []string{"session", "user", "custom", "assistant"}) {
		t.Fatalf("roles = %v", got)
	}
}

func TestSetSessionFileCorrupted(t *testing.T) {
	dir := t.TempDir()
	empty := filepath.Join(dir, "empty.jsonl")
	writeFile(t, empty, "")
	session, err := Open(empty, dir, "")
	if err != nil {
		t.Fatal(err)
	}
	if session.GetSessionID() == "" || session.GetHeader() == nil {
		t.Fatal("expected new header")
	}
	lines := strings.Split(strings.TrimSpace(readFile(t, empty)), "\n")
	if len(lines) != 1 || !strings.Contains(lines[0], `"type":"session"`) {
		t.Fatalf("file = %q", readFile(t, empty))
	}

	noHeader := filepath.Join(dir, "no-header.jsonl")
	original := `{"type":"message","id":"abc","parentId":"orphaned","timestamp":"2025-01-01T00:00:00Z","message":{"role":"assistant","content":"test"}}` + "\n"
	writeFile(t, noHeader, original)
	if _, err := Open(noHeader, dir, ""); err == nil || !strings.Contains(err.Error(), "not a valid pi session") {
		t.Fatalf("error = %v", err)
	}
	if got := readFile(t, noHeader); got != original {
		t.Fatalf("file changed = %q", got)
	}
}

func TestCustomSessionID(t *testing.T) {
	session := newMemory(t, "/project")
	if _, err := session.NewSession(&NewSessionOptions{ID: new("my-custom-id")}); err != nil {
		t.Fatal(err)
	}
	if session.GetSessionID() != "my-custom-id" {
		t.Fatalf("id = %q", session.GetSessionID())
	}
	if header := session.GetHeader(); header == nil || header.ID != "my-custom-id" {
		t.Fatalf("header = %#v", header)
	}
	if session.GetSessionFile() != "" {
		t.Fatal("in-memory session should have no file")
	}

	for _, invalid := range []string{"", "-abc", "abc-", "_abc", "abc_", ".abc", "abc.", "abc/def", "abc def"} {
		other := newMemory(t, "/project")
		if _, err := other.NewSession(&NewSessionOptions{ID: new(invalid)}); err == nil || !strings.Contains(err.Error(), "Session id must be non-empty") {
			t.Fatalf("id %q error = %v", invalid, err)
		}
	}

	generated := newMemory(t, "/project")
	if _, err := generated.NewSession(nil); err != nil {
		t.Fatal(err)
	}
	if !uuidV7Pattern.MatchString(generated.GetSessionID()) {
		t.Fatalf("generated id = %q", generated.GetSessionID())
	}

	dir := t.TempDir()
	created, err := Create(dir, dir, &NewSessionOptions{ID: new("created-session-id")})
	if err != nil {
		t.Fatal(err)
	}
	if created.GetSessionID() != "created-session-id" {
		t.Fatalf("id = %q", created.GetSessionID())
	}
	if pathExists(created.GetSessionFile()) {
		t.Fatal("file should not exist yet")
	}
	if match, _ := regexp.MatchString(`^\d{4}-\d{2}-\d{2}T\d{2}-\d{2}-\d{2}-\d{3}Z_created-session-id\.jsonl$`, filepath.Base(created.GetSessionFile())); !match {
		t.Fatalf("file name = %q", filepath.Base(created.GetSessionFile()))
	}
}

func TestInMemoryPreloadedEntries(t *testing.T) {
	source := newMemory(t, "/project")
	mustAppendMessage(t, source, userMsg("hello"))
	if _, err := source.AppendModelChange("anthropic", "claude-opus-4-5"); err != nil {
		t.Fatal(err)
	}
	mustAppendMessage(t, source, userMsg("again"))
	stored := source.GetEntries()

	session, err := InMemory("/project", nil, nil, stored)
	if err != nil {
		t.Fatal(err)
	}
	if len(session.GetEntries()) != len(stored) {
		t.Fatalf("entries = %d", len(session.GetEntries()))
	}
	lastID := stored[len(stored)-1].Base().ID
	appended, err := session.AppendMessage(userMsg("continued"))
	if err != nil {
		t.Fatal(err)
	}
	if leaf := session.GetLeafID(); leaf == nil || *leaf != appended {
		t.Fatalf("leaf = %v", leaf)
	}
	if ptrValue(session.GetEntry(appended).Base().ParentID) != lastID {
		t.Fatalf("parent = %v", session.GetEntry(appended).Base().ParentID)
	}
	if session.IsPersisted() || session.GetSessionFile() != "" {
		t.Fatal("in-memory session must not persist")
	}
}

func TestInMemoryHeaderAmongEntries(t *testing.T) {
	header := model.NewSessionHeader("stored-session", "2026-01-01T00:00:00Z", "/stored")
	session, err := InMemory("/project", &NewSessionOptions{ID: new("ignored")}, &header, nil)
	if err != nil {
		t.Fatal(err)
	}
	if session.GetSessionID() != "stored-session" || session.GetHeader() == nil || session.GetHeader().Cwd != "/stored" {
		t.Fatalf("id = %q header = %#v", session.GetSessionID(), session.GetHeader())
	}
}

func TestHeaderlessEntriesAdoptedWithoutMigration(t *testing.T) {
	entries := []model.SessionEntry{msgEntry("abc12345", nil, userMsg("hello"))}
	session, err := InMemory("/project", &NewSessionOptions{ID: new("restored-session")}, nil, entries)
	if err != nil {
		t.Fatal(err)
	}
	if session.GetSessionID() != "restored-session" {
		t.Fatalf("id = %q", session.GetSessionID())
	}
	if session.GetHeader() == nil || session.GetHeader().Cwd != "/project" {
		t.Fatalf("header = %#v", session.GetHeader())
	}
	if len(session.GetEntries()) != 1 || session.GetEntries()[0].Base().ID != "abc12345" {
		t.Fatalf("entries = %#v", session.GetEntries())
	}
}

func TestRejectsUnsupportedVersions(t *testing.T) {
	for _, version := range []int{1, 2, 4} {
		header := model.SessionHeader{Type: "session", ID: "old", Timestamp: "2025-01-01T00:00:00Z", Cwd: "/project", Version: new(version)}
		if _, err := InMemory("/project", nil, &header, nil); err == nil || !strings.Contains(err.Error(), "unsupported session version") {
			t.Fatalf("version %d error = %v", version, err)
		}
	}
	header := model.SessionHeader{Type: "session", ID: "old", Timestamp: "2025-01-01T00:00:00Z", Cwd: "/project"}
	if _, err := InMemory("/project", nil, &header, nil); err == nil || !strings.Contains(err.Error(), "unsupported session version") {
		t.Fatalf("missing version error = %v", err)
	}
}

func TestListAndContinueRecentFilterCwd(t *testing.T) {
	root := t.TempDir()
	dirA := filepath.Join(root, "a")
	dirB := filepath.Join(root, "b")
	projectA := filepath.Join(root, "project-a")
	projectB := filepath.Join(root, "project-b")

	sessionA, err := Create(projectA, dirA, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := sessionA.AppendMessage(userMsg("from A")); err != nil {
		t.Fatal(err)
	}
	sessionB, err := Create(projectB, dirB, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := sessionB.AppendMessage(userMsg("from B")); err != nil {
		t.Fatal(err)
	}

	sessions, err := List(context.Background(), projectA, dirA, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(sessions) != 1 || sessions[0].Path != sessionA.GetSessionFile() {
		t.Fatalf("list A = %#v", sessions)
	}
	all, err := ListAllProjects(context.Background(), root, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 2 {
		t.Fatalf("all = %d", len(all))
	}
	continued, err := ContinueRecent(projectA, dirA)
	if err != nil {
		t.Fatal(err)
	}
	if continued.GetSessionFile() != sessionA.GetSessionFile() {
		t.Fatalf("continued = %q", continued.GetSessionFile())
	}
	if found, ok := FindByID(projectB, sessionB.GetSessionID(), dirB); !ok || found != sessionB.GetSessionFile() {
		t.Fatalf("FindByID = %q ok=%v", found, ok)
	}
}

func TestListCancellation(t *testing.T) {
	dir := t.TempDir()
	for range 2 {
		session, err := Create(filepath.Join(dir, "p"), dir, nil)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := session.AppendMessage(userMsg("hello")); err != nil {
			t.Fatal(err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := ListAllProjects(ctx, dir, nil); err == nil {
		t.Fatal("expected cancellation error")
	}
}

func readSessionFileRoles(t *testing.T, file string) []string {
	t.Helper()
	var roles []string
	for line := range strings.SplitSeq(readFile(t, file), "\n") {
		parsed, ok := parseSessionLine(line)
		if !ok {
			continue
		}
		if parsed.header != nil {
			roles = append(roles, "session")
			continue
		}
		if message, ok := parsed.entry.(*model.SessionMessageEntry); ok {
			roles = append(roles, string(message.Message.MessageRole()))
			continue
		}
		roles = append(roles, parsed.entry.Base().Type)
	}
	return roles
}

func equalStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}
