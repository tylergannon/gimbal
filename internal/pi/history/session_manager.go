package history

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/tylergannon/gimbal/internal/pi/files"
	"github.com/tylergannon/gimbal/internal/pi/model"
)

const (
	sessionReadBufferSize       = 1024 * 1024
	sessionHeaderReadBufferSize = 4096
	// maxSessionHeaderScanBytes bounds synchronous header discovery while
	// allowing large cwd and custom metadata fields.
	maxSessionHeaderScanBytes = 1024 * 1024
)

var sessionIDPattern = regexp.MustCompile(`^[A-Za-z0-9](?:[A-Za-z0-9._-]*[A-Za-z0-9])?$`)

// NewSessionOptions configures a new session.
type NewSessionOptions struct {
	// ID is an explicit session id. A nil ID generates a UUIDv7.
	ID *string
	// ParentSession is the path of the session this one was forked from.
	ParentSession string
}

// AssertValidSessionID rejects ids that cannot name a session.
func AssertValidSessionID(id string) error {
	if !sessionIDPattern.MatchString(id) {
		//nolint:staticcheck // upstream error text is part of the contract
		return errors.New("Session id must be non-empty, contain only alphanumeric characters, '-', '_', and '.', and start and end with an alphanumeric character")
	}
	return nil
}

// SessionManager manages conversation sessions as append-only trees stored in
// JSONL files.
//
// Each session entry has an id and parentId forming a tree structure. The
// leaf pointer tracks the current position. Appending creates a child of the
// current leaf. Branching moves the leaf to an earlier entry, allowing new
// branches without modifying history.
type SessionManager struct {
	mu sync.Mutex

	sessionID   string
	sessionFile string
	sessionDir  string
	cwd         string
	persist     bool
	flushed     bool

	header  *model.SessionHeader
	entries []model.SessionEntry

	byID                map[string]model.SessionEntry
	labelsByID          map[string]string
	labelTimestampsByID map[string]string
	leafID              *string
}

// Create starts a new persisted session.
//
// sessionDir must name the Gimbal-owned directory where session files are
// written. An empty directory is refused rather than falling back to the
// user's Pi home.
func Create(cwd, sessionDir string, options *NewSessionOptions) (*SessionManager, error) {
	if sessionDir == "" {
		return nil, errors.New("history: create session: session directory is required")
	}
	sm := &SessionManager{
		cwd:        resolvePath(cwd),
		sessionDir: normalizePath(sessionDir),
		persist:    true,
	}
	if err := os.MkdirAll(sm.sessionDir, 0o755); err != nil {
		return nil, err
	}
	if _, err := sm.newSession(options); err != nil {
		return nil, err
	}
	return sm, nil
}

// InMemory starts a session with no file persistence.
//
// A header and entries carried from outside the filesystem are adopted
// verbatim; when entries carry no header, one is created from options.
func InMemory(cwd string, options *NewSessionOptions, header *model.SessionHeader, entries []model.SessionEntry) (*SessionManager, error) {
	if cwd == "" {
		wd, err := os.Getwd()
		if err != nil {
			return nil, err
		}
		cwd = wd
	}
	sm := &SessionManager{
		cwd:        resolvePath(cwd),
		sessionDir: "",
		persist:    false,
	}
	if err := sm.loadEntries(header, entries, options); err != nil {
		return nil, err
	}
	return sm, nil
}

// Open opens a specific session file.
//
// cwdOverride, when non-empty, replaces the cwd stored in the session header.
// When sessionDir is empty the file's parent directory is used.
func Open(path, sessionDir, cwdOverride string) (*SessionManager, error) {
	resolvedPath := resolvePath(path)
	cwd := cwdOverride
	if cwd == "" && pathExists(resolvedPath) {
		if header, err := readSessionHeaderBounded(resolvedPath); err == nil && header != nil {
			cwd = header.Cwd
		}
	}
	if cwd == "" {
		wd, err := os.Getwd()
		if err != nil {
			return nil, err
		}
		cwd = wd
	}
	dir := sessionDir
	if dir == "" {
		dir = filepath.Dir(resolvedPath)
	}
	sm := &SessionManager{
		cwd:        resolvePath(cwd),
		sessionDir: normalizePath(dir),
		persist:    true,
	}
	if err := sm.setSessionFile(resolvedPath); err != nil {
		return nil, err
	}
	return sm, nil
}

// ContinueRecent continues the most recent session in sessionDir, or starts a
// new one when none exists.
func ContinueRecent(cwd, sessionDir string) (*SessionManager, error) {
	if sessionDir == "" {
		return nil, errors.New("history: continue recent: session directory is required")
	}
	dir := normalizePath(sessionDir)
	if mostRecent, ok := FindMostRecentSession(dir, cwd); ok {
		return Open(mostRecent, dir, "")
	}
	sm := &SessionManager{
		cwd:        resolvePath(cwd),
		sessionDir: dir,
		persist:    true,
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	if _, err := sm.newSession(nil); err != nil {
		return nil, err
	}
	return sm, nil
}

// ForkFrom creates a new session in targetCwd with the full history of the
// source session file.
func ForkFrom(sourcePath, targetCwd, sessionDir string, options *NewSessionOptions) (*SessionManager, error) {
	resolvedSource := resolvePath(sourcePath)
	resolvedTarget := resolvePath(targetCwd)
	header, sourceEntries, err := LoadEntriesFromFile(resolvedSource)
	if err != nil {
		return nil, err
	}
	if header == nil {
		//nolint:staticcheck // upstream error text is part of the contract
		return nil, fmt.Errorf("Cannot fork: source session file is empty or invalid: %s", resolvedSource)
	}
	if sessionDir == "" {
		return nil, errors.New("history: fork session: session directory is required")
	}
	dir := normalizePath(sessionDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}

	newSessionID, err := sessionIDFor(options)
	if err != nil {
		return nil, err
	}
	timestamp := nowISO()
	fileTimestamp := strings.NewReplacer(":", "-", ".", "-").Replace(timestamp)
	newSessionFile := filepath.Join(dir, fileTimestamp+"_"+newSessionID+".jsonl")

	newHeader := model.NewSessionHeader(newSessionID, timestamp, resolvedTarget)
	newHeader.ParentSession = resolvedSource
	f, err := os.OpenFile(newSessionFile, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, err
	}
	if err := writeJSONLine(f, newHeader); err != nil {
		_ = f.Close()
		return nil, err
	}
	for _, entry := range sourceEntries {
		if err := writeJSONLine(f, entry); err != nil {
			_ = f.Close()
			return nil, err
		}
	}
	if err := f.Close(); err != nil {
		return nil, err
	}

	return Open(newSessionFile, dir, "")
}

// FindByID returns the path of the session whose header id matches id.
func FindByID(cwd, id, sessionDir string) (string, bool) {
	if sessionDir == "" {
		return "", false
	}
	dir := normalizePath(sessionDir)
	resolvedCwd := resolvePath(cwd)
	dirEntries, err := os.ReadDir(dir)
	if err != nil {
		return "", false
	}
	for _, entry := range dirEntries {
		if !strings.HasSuffix(entry.Name(), ".jsonl") {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		header := readSessionHeaderForDiscovery(path)
		if header == nil || header.ID != id {
			continue
		}
		if cwd != "" && !sessionCwdMatches(header.Cwd, resolvedCwd) {
			continue
		}
		return path, true
	}
	return "", false
}

// ---------------------------------------------------------------------------
// Session file IO
// ---------------------------------------------------------------------------

// parsedLine is one parsed physical line of a session file.
type parsedLine struct {
	header *model.SessionHeader
	entry  model.SessionEntry
}

// LoadEntriesFromFile parses path into its session header and entries.
//
// It returns (nil, nil, nil) when the file is missing, empty, malformed, or
// does not begin with a valid session header. As upstream does, an
// unterminated final line in an otherwise valid session is terminated with a
// newline so a later append cannot fuse with it.
func LoadEntriesFromFile(path string) (*model.SessionHeader, []model.SessionEntry, error) {
	resolved := normalizePath(path)
	data, err := os.ReadFile(resolved)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil, nil
		}
		return nil, nil, err
	}
	if len(data) == 0 {
		return nil, nil, nil
	}

	lines := parseSessionLines(data)
	if len(lines) == 0 || lines[0].header == nil {
		return nil, nil, nil
	}
	header := lines[0].header
	entries := make([]model.SessionEntry, 0, len(lines)-1)
	for _, line := range lines {
		if line.entry != nil {
			entries = append(entries, line.entry)
		}
	}
	if data[len(data)-1] != '\n' {
		f, err := os.OpenFile(resolved, os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			return nil, nil, err
		}
		if _, err := f.WriteString("\n"); err != nil {
			_ = f.Close()
			return nil, nil, err
		}
		if err := f.Close(); err != nil {
			return nil, nil, err
		}
	}
	return header, entries, nil
}

func parseSessionLines(data []byte) []parsedLine {
	var lines []parsedLine
	for raw := range strings.SplitSeq(string(data), "\n") {
		line, ok := parseSessionLine(raw)
		if ok {
			lines = append(lines, line)
		}
	}
	return lines
}

// parseSessionLine decodes one physical line. It returns false for blank or
// malformed lines, which callers skip.
func parseSessionLine(raw string) (parsedLine, bool) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return parsedLine{}, false
	}
	var head struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal([]byte(trimmed), &head); err != nil {
		return parsedLine{}, false
	}
	switch head.Type {
	case "":
		return parsedLine{}, false
	case "session":
		var header model.SessionHeader
		if err := json.Unmarshal([]byte(trimmed), &header); err != nil {
			return parsedLine{}, false
		}
		return parsedLine{header: &header}, true
	case "context_edit":
		entry, err := parseContextEditLine([]byte(trimmed))
		if err != nil {
			return parsedLine{}, false
		}
		return parsedLine{entry: entry}, true
	default:
		entry, err := model.UnmarshalSessionEntry([]byte(trimmed))
		if err != nil {
			return parsedLine{}, false
		}
		return parsedLine{entry: entry}, true
	}
}

// parseContextEditLine tolerates the string content form of a replacement,
// which model.ContextEditableContent cannot decode on its own.
func parseContextEditLine(data []byte) (model.SessionEntry, error) {
	var raw struct {
		ID          string          `json:"id"`
		ParentID    *string         `json:"parentId"`
		Timestamp   string          `json:"timestamp"`
		TargetID    string          `json:"targetId"`
		Replacement json.RawMessage `json:"replacement"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	entry := &model.ContextEditEntry{}
	entry.SessionEntryBase = model.SessionEntryBase{
		Type: "context_edit", ID: raw.ID, ParentID: raw.ParentID, Timestamp: raw.Timestamp,
	}
	entry.TargetID = raw.TargetID
	replacement, err := decodeContextEditReplacement(raw.Replacement)
	if err != nil {
		return nil, err
	}
	entry.Replacement = replacement
	return entry, nil
}

func decodeContextEditReplacement(raw json.RawMessage) (*model.ContextEditableContent, error) {
	if len(raw) == 0 || strings.TrimSpace(string(raw)) == "null" {
		return nil, nil
	}
	var wrapper struct {
		Content json.RawMessage `json:"content"`
	}
	if err := json.Unmarshal(raw, &wrapper); err != nil {
		return nil, err
	}
	content := model.ContentList{}
	if len(wrapper.Content) > 0 && wrapper.Content[0] == '"' {
		var text string
		if err := json.Unmarshal(wrapper.Content, &text); err != nil {
			return nil, err
		}
		content = model.ContentList{model.TextContent{Text: text}}
	} else if len(wrapper.Content) > 0 && string(wrapper.Content) != "null" {
		if err := json.Unmarshal(wrapper.Content, &content); err != nil {
			return nil, err
		}
	}
	return &model.ContextEditableContent{Content: content}, nil
}

func readSessionHeaderBounded(path string) (*model.SessionHeader, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	reader := bufio.NewReaderSize(f, sessionHeaderReadBufferSize)
	scanned := 0
	for {
		line, readErr := reader.ReadString('\n')
		scanned += len(line)
		if parsed, ok := parseSessionLine(line); ok {
			return parsed.header, nil
		}
		if scanned > maxSessionHeaderScanBytes {
			if readErr == io.EOF {
				return nil, nil
			}
			return nil, fmt.Errorf("session header exceeds %d-byte scan limit: %s", maxSessionHeaderScanBytes, path)
		}
		if readErr != nil {
			if readErr == io.EOF {
				return nil, nil
			}
			return nil, readErr
		}
	}
}

func readSessionHeaderForDiscovery(path string) *model.SessionHeader {
	header, err := readSessionHeaderBounded(path)
	if err != nil {
		return nil
	}
	return header
}

func writeJSONLine(w io.Writer, value any) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	if _, err := w.Write(raw); err != nil {
		return err
	}
	_, err = w.Write([]byte("\n"))
	return err
}

// ---------------------------------------------------------------------------
// Internal session construction and mutation
// ---------------------------------------------------------------------------

func (sm *SessionManager) newSession(options *NewSessionOptions) (string, error) {
	id, err := sessionIDFor(options)
	if err != nil {
		return "", err
	}
	sm.sessionID = id
	timestamp := nowISO()
	header := model.NewSessionHeader(id, timestamp, sm.cwd)
	if options != nil {
		header.ParentSession = options.ParentSession
	}
	sm.header = &header
	sm.entries = nil
	sm.byID = map[string]model.SessionEntry{}
	sm.labelsByID = map[string]string{}
	sm.labelTimestampsByID = map[string]string{}
	sm.leafID = nil
	sm.flushed = false

	if sm.persist {
		fileTimestamp := strings.NewReplacer(":", "-", ".", "-").Replace(timestamp)
		sm.sessionFile = filepath.Join(sm.sessionDir, fileTimestamp+"_"+id+".jsonl")
	}
	return sm.sessionFile, nil
}

func sessionIDFor(options *NewSessionOptions) (string, error) {
	if options != nil && options.ID != nil {
		if err := AssertValidSessionID(*options.ID); err != nil {
			return "", err
		}
		return *options.ID, nil
	}
	return model.UUIDv7(), nil
}

func (sm *SessionManager) setSessionFile(sessionFile string) error {
	sm.sessionFile = resolvePath(sessionFile)
	if pathExists(sm.sessionFile) {
		header, entries, err := LoadEntriesFromFile(sm.sessionFile)
		if err != nil {
			return err
		}
		if header == nil {
			info, statErr := os.Stat(sm.sessionFile)
			if statErr == nil && info.Size() > 0 {
				//nolint:staticcheck // upstream error text is part of the contract
				return fmt.Errorf("Session file is not a valid pi session: %s", sm.sessionFile)
			}
			explicitPath := sm.sessionFile
			if _, err := sm.newSession(nil); err != nil {
				return err
			}
			sm.sessionFile = explicitPath
			if err := sm.rewriteFile(); err != nil {
				return err
			}
			sm.flushed = true
			return nil
		}
		if err := sm.loadEntries(header, entries, nil); err != nil {
			return err
		}
		sm.flushed = true
		return nil
	}
	explicitPath := sm.sessionFile
	if _, err := sm.newSession(nil); err != nil {
		return err
	}
	sm.sessionFile = explicitPath
	return nil
}

func (sm *SessionManager) loadEntries(header *model.SessionHeader, entries []model.SessionEntry, options *NewSessionOptions) error {
	if header != nil {
		if err := checkSessionVersion(header); err != nil {
			return err
		}
		sm.header = header
		sm.sessionID = header.ID
		sm.entries = entries
	} else {
		if _, err := sm.newSession(options); err != nil {
			return err
		}
		sm.entries = append(sm.entries, entries...)
	}
	sm.buildIndex()
	return nil
}

func checkSessionVersion(header *model.SessionHeader) error {
	version := 0
	if header.Version != nil {
		version = *header.Version
	}
	if version != model.CurrentSessionVersion {
		return fmt.Errorf("history: unsupported session version %d (current is %d): %s", version, model.CurrentSessionVersion, header.ID)
	}
	return nil
}

func (sm *SessionManager) buildIndex() {
	sm.byID = map[string]model.SessionEntry{}
	sm.labelsByID = map[string]string{}
	sm.labelTimestampsByID = map[string]string{}
	sm.leafID = nil
	for _, entry := range sm.entries {
		sm.byID[entry.Base().ID] = entry
		id := entry.Base().ID
		sm.leafID = &id
		if label, ok := entry.(*model.LabelEntry); ok {
			if label.Label != nil {
				sm.labelsByID[label.TargetID] = *label.Label
				sm.labelTimestampsByID[label.TargetID] = label.Timestamp
			} else {
				delete(sm.labelsByID, label.TargetID)
				delete(sm.labelTimestampsByID, label.TargetID)
			}
		}
	}
}

func (sm *SessionManager) rewriteFile() error {
	if !sm.persist || sm.sessionFile == "" {
		return nil
	}
	f, err := os.OpenFile(sm.sessionFile, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	if err := writeJSONLine(f, *sm.header); err != nil {
		return err
	}
	for _, entry := range sm.entries {
		if err := writeJSONLine(f, entry); err != nil {
			return err
		}
	}
	return nil
}

func (sm *SessionManager) hasConversation() bool {
	for _, entry := range sm.entries {
		message, ok := entry.(*model.SessionMessageEntry)
		if !ok {
			continue
		}
		switch message.Message.MessageRole() {
		case model.RoleUser, model.RoleAssistant:
			return true
		}
	}
	return false
}

func (sm *SessionManager) persistEntry(entry model.SessionEntry) error {
	if !sm.persist || sm.sessionFile == "" {
		return nil
	}
	if !sm.flushed {
		if !sm.hasConversation() {
			return nil
		}
		f, err := os.OpenFile(sm.sessionFile, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
		if err != nil {
			return err
		}
		if sm.header != nil {
			if err := writeJSONLine(f, *sm.header); err != nil {
				_ = f.Close()
				return err
			}
		}
		for _, existing := range sm.entries {
			if err := writeJSONLine(f, existing); err != nil {
				_ = f.Close()
				return err
			}
		}
		if err := f.Close(); err != nil {
			return err
		}
		sm.flushed = true
		return nil
	}
	f, err := os.OpenFile(sm.sessionFile, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	if err := writeJSONLine(f, entry); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}

func (sm *SessionManager) generateID() string {
	for range 100 {
		var b [4]byte
		if _, err := rand.Read(b[:]); err != nil {
			panic(err)
		}
		id := hex.EncodeToString(b[:])
		if _, exists := sm.byID[id]; !exists {
			return id
		}
	}
	return randomUUIDv4()
}

func (sm *SessionManager) appendEntry(entry model.SessionEntry) (string, error) {
	sm.entries = append(sm.entries, entry)
	id := entry.Base().ID
	sm.byID[id] = entry
	sm.leafID = &id
	if err := sm.persistEntry(entry); err != nil {
		return "", err
	}
	return id, nil
}

func randomUUIDv4() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	b[6] = 0x40 | b[6]&0x0f
	b[8] = 0x80 | b[8]&0x3f
	h := hex.EncodeToString(b[:])
	return h[0:8] + "-" + h[8:12] + "-" + h[12:16] + "-" + h[16:20] + "-" + h[20:32]
}

// ---------------------------------------------------------------------------
// Public queries
// ---------------------------------------------------------------------------

// IsPersisted reports whether the session writes to disk.
func (sm *SessionManager) IsPersisted() bool {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	return sm.persist
}

// GetCwd returns the session working directory.
func (sm *SessionManager) GetCwd() string {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	return sm.cwd
}

// GetSessionDir returns the session directory.
func (sm *SessionManager) GetSessionDir() string {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	return sm.sessionDir
}

// GetSessionID returns the session id.
func (sm *SessionManager) GetSessionID() string {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	return sm.sessionID
}

// GetSessionFile returns the session file path, or "" for in-memory sessions.
func (sm *SessionManager) GetSessionFile() string {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	return sm.sessionFile
}

// SetSessionFile switches to a different session file.
func (sm *SessionManager) SetSessionFile(sessionFile string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	return sm.setSessionFile(sessionFile)
}

// NewSession replaces the current session with a fresh one.
func (sm *SessionManager) NewSession(options *NewSessionOptions) (string, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	return sm.newSession(options)
}

// GetHeader returns the session header, or nil.
func (sm *SessionManager) GetHeader() *model.SessionHeader {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	if sm.header == nil {
		return nil
	}
	copy := *sm.header
	return &copy
}

// GetEntries returns all entries excluding the header.
func (sm *SessionManager) GetEntries() []model.SessionEntry {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	return append([]model.SessionEntry(nil), sm.entries...)
}

// GetLeafID returns the current leaf id, or nil.
func (sm *SessionManager) GetLeafID() *string {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	if sm.leafID == nil {
		return nil
	}
	id := *sm.leafID
	return &id
}

// GetLeafEntry returns the current leaf entry.
func (sm *SessionManager) GetLeafEntry() model.SessionEntry {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	if sm.leafID == nil {
		return nil
	}
	return sm.byID[*sm.leafID]
}

// GetEntry returns the entry with the given id, or nil.
func (sm *SessionManager) GetEntry(id string) model.SessionEntry {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	return sm.byID[id]
}

// GetChildren returns the direct children of an entry.
func (sm *SessionManager) GetChildren(parentID string) []model.SessionEntry {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	var children []model.SessionEntry
	for _, entry := range sm.entries {
		if entry.Base().ParentID != nil && *entry.Base().ParentID == parentID {
			children = append(children, entry)
		}
	}
	return children
}

// GetLabel returns the label for an entry, or "".
func (sm *SessionManager) GetLabel(id string) (string, bool) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	label, ok := sm.labelsByID[id]
	return label, ok
}

// GetSessionName returns the latest session_info name, if any.
func (sm *SessionManager) GetSessionName() string {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	for _, v := range slices.Backward(sm.entries) {
		info, ok := v.(*model.SessionInfoEntry)
		if !ok {
			continue
		}
		name := strings.TrimSpace(info.Name)
		if name == "" {
			return ""
		}
		return name
	}
	return ""
}

// GetBranch walks from fromID (or the current leaf) to the root.
func (sm *SessionManager) GetBranch(fromID ...string) []model.SessionEntry {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	var start *string
	if len(fromID) > 0 {
		id := fromID[0]
		start = &id
	} else {
		start = sm.leafID
	}
	var path []model.SessionEntry
	var current model.SessionEntry
	if start != nil {
		current = sm.byID[*start]
	}
	for current != nil {
		path = append(path, current)
		parentID := current.Base().ParentID
		if parentID == nil {
			current = nil
			continue
		}
		current = sm.byID[*parentID]
	}
	for i, j := 0, len(path)-1; i < j; i, j = i+1, j-1 {
		path[i], path[j] = path[j], path[i]
	}
	return path
}

// GetTree returns the session as a tree of defensive nodes.
func (sm *SessionManager) GetTree() []model.SessionTreeNode {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	nodeMap := map[string]*model.SessionTreeNode{}
	var roots []*model.SessionTreeNode
	for _, entry := range sm.entries {
		node := &model.SessionTreeNode{Entry: entry}
		if label, ok := sm.labelsByID[entry.Base().ID]; ok {
			value := label
			node.Label = &value
		}
		if timestamp, ok := sm.labelTimestampsByID[entry.Base().ID]; ok {
			value := timestamp
			node.LabelTimestamp = &value
		}
		nodeMap[entry.Base().ID] = node
	}
	for _, entry := range sm.entries {
		node := nodeMap[entry.Base().ID]
		parentID := entry.Base().ParentID
		if parentID == nil || *parentID == entry.Base().ID {
			roots = append(roots, node)
			continue
		}
		parent, ok := nodeMap[*parentID]
		if !ok {
			roots = append(roots, node)
			continue
		}
		parent.Children = append(parent.Children, *node)
	}
	stack := append([]*model.SessionTreeNode(nil), roots...)
	for len(stack) > 0 {
		node := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		sort.SliceStable(node.Children, func(i, j int) bool {
			return timestampMillis(node.Children[i].Entry.Base().Timestamp) < timestampMillis(node.Children[j].Entry.Base().Timestamp)
		})
		for i := range node.Children {
			if child := nodeMap[node.Children[i].Entry.Base().ID]; child != nil {
				stack = append(stack, child)
			}
		}
	}
	out := make([]model.SessionTreeNode, 0, len(roots))
	for _, root := range roots {
		out = append(out, treeNodeValue(root, nodeMap))
	}
	return out
}

func treeNodeValue(node *model.SessionTreeNode, nodeMap map[string]*model.SessionTreeNode) model.SessionTreeNode {
	out := *node
	out.Children = make([]model.SessionTreeNode, 0, len(node.Children))
	for i := range node.Children {
		child := nodeMap[node.Children[i].Entry.Base().ID]
		out.Children = append(out.Children, treeNodeValue(child, nodeMap))
	}
	return out
}

// BuildContextEntries returns the compaction-aware entry list for the current
// leaf.
func (sm *SessionManager) BuildContextEntries() []model.SessionEntry {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	return buildContextEntries(sm.entries, sm.leafID, sm.byID, false)
}

// BuildSessionProjection returns the provenance-preserving model projection.
func (sm *SessionManager) BuildSessionProjection() model.SessionProjection {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	return buildSessionProjection(sm.entries, sm.leafID, sm.byID, false)
}

// BuildSessionContext returns the finalized model context.
func (sm *SessionManager) BuildSessionContext() model.SessionContext {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	projection := buildSessionProjection(sm.entries, sm.leafID, sm.byID, false)
	return model.SessionContext{
		Messages:      projection.Messages,
		ThinkingLevel: projection.ThinkingLevel,
		Model:         projection.Model,
	}
}

// Branch starts a new branch from an earlier entry.
func (sm *SessionManager) Branch(branchFromID string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	if _, ok := sm.byID[branchFromID]; !ok {
		//nolint:staticcheck // upstream error text is part of the contract
		return fmt.Errorf("Entry %s not found", branchFromID)
	}
	id := branchFromID
	sm.leafID = &id
	return nil
}

// ResetLeaf points the leaf before any entry.
func (sm *SessionManager) ResetLeaf() {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.leafID = nil
}

// BranchWithSummary starts a new branch and appends a summary of the abandoned
// path.
func (sm *SessionManager) BranchWithSummary(branchFromID *string, summary string, details any, fromHook bool, usage *model.Usage) (string, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	if branchFromID != nil {
		if _, ok := sm.byID[*branchFromID]; !ok {
			//nolint:staticcheck // upstream error text is part of the contract
			return "", fmt.Errorf("Entry %s not found", *branchFromID)
		}
	}
	fromID := "root"
	if sm.leafID != nil {
		fromID = *sm.leafID
	}
	sm.leafID = branchFromID
	entry := &model.BranchSummaryEntry{
		Type: "branch_summary", ID: sm.generateID(), ParentID: branchFromID, Timestamp: nowISO(),
		FromID:   fromID,
		Summary:  summary,
		Details:  details,
		Usage:    usage,
		FromHook: fromHook,
	}
	return sm.appendEntry(entry)
}

// ---------------------------------------------------------------------------
// Append operations
// ---------------------------------------------------------------------------

// AppendMessage appends a message as a child of the current leaf.
func (sm *SessionManager) AppendMessage(message model.AgentMessage) (string, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	entry := &model.SessionMessageEntry{}
	entry.SessionEntryBase = model.SessionEntryBase{
		Type: "message", ID: sm.generateID(), ParentID: sm.leafID, Timestamp: nowISO(),
	}
	entry.Message = message
	return sm.appendEntry(entry)
}

// AppendThinkingLevelChange appends a thinking-level change.
func (sm *SessionManager) AppendThinkingLevelChange(level string) (string, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	entry := &model.ThinkingLevelChangeEntry{}
	entry.SessionEntryBase = model.SessionEntryBase{
		Type: "thinking_level_change", ID: sm.generateID(), ParentID: sm.leafID, Timestamp: nowISO(),
	}
	entry.ThinkingLevel = level
	return sm.appendEntry(entry)
}

// AppendModelChange appends a model change.
func (sm *SessionManager) AppendModelChange(provider, modelID string) (string, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	entry := &model.ModelChangeEntry{}
	entry.SessionEntryBase = model.SessionEntryBase{
		Type: "model_change", ID: sm.generateID(), ParentID: sm.leafID, Timestamp: nowISO(),
	}
	entry.Provider = provider
	entry.ModelID = modelID
	return sm.appendEntry(entry)
}

// AppendUsage appends model-attributed usage.
func (sm *SessionManager) AppendUsage(kind, provider, name string, usage model.Usage, note string) (*model.UsageEntry, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	entry := &model.UsageEntry{}
	entry.SessionEntryBase = model.SessionEntryBase{
		Type: "usage", ID: sm.generateID(), ParentID: sm.leafID, Timestamp: nowISO(),
	}
	entry.Kind = kind
	entry.Provider = provider
	entry.Model = name
	entry.Usage = usage
	entry.Note = note
	if _, err := sm.appendEntry(entry); err != nil {
		return nil, err
	}
	return entry, nil
}

// AppendCompaction appends a compaction summary.
func (sm *SessionManager) AppendCompaction(summary string, firstKeptEntryID *string, tokensBefore int, details any, fromHook bool, usage *model.Usage) (string, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	timestamp := nowISO()
	id := sm.generateID()
	keptID := id
	if firstKeptEntryID != nil {
		keptID = *firstKeptEntryID
	}
	entry := &model.CompactionEntry{}
	entry.SessionEntryBase = model.SessionEntryBase{
		Type: "compaction", ID: id, ParentID: sm.leafID, Timestamp: timestamp,
	}
	entry.Summary = summary
	entry.FirstKeptEntryID = keptID
	entry.TokensBefore = tokensBefore
	entry.Details = details
	entry.Usage = usage
	entry.FromHook = fromHook
	if systemMessage, ok := model.GetCurrentSystemMessage(buildSessionProjection(sm.entries, sm.leafID, sm.byID, false).Messages); ok {
		message := systemMessage
		message.Timestamp = timestampMillis(timestamp)
		entry.SystemMessage = &message
	}
	return sm.appendEntry(entry)
}

// AppendCustomEntry appends an extension state entry.
func (sm *SessionManager) AppendCustomEntry(customType string, data any) (string, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	entry := &model.CustomEntry{}
	entry.SessionEntryBase = model.SessionEntryBase{
		Type: "custom", ID: sm.generateID(), ParentID: sm.leafID, Timestamp: nowISO(),
	}
	entry.CustomType = customType
	entry.Data = data
	return sm.appendEntry(entry)
}

// AppendSessionInfo appends a session metadata entry.
func (sm *SessionManager) AppendSessionInfo(name string) (string, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sanitized := strings.TrimSpace(regexp.MustCompile(`[\r\n]+`).ReplaceAllString(name, " "))
	entry := &model.SessionInfoEntry{}
	entry.SessionEntryBase = model.SessionEntryBase{
		Type: "session_info", ID: sm.generateID(), ParentID: sm.leafID, Timestamp: nowISO(),
	}
	entry.Name = sanitized
	return sm.appendEntry(entry)
}

// AppendCustomMessageEntry appends an extension message that participates in
// model context.
func (sm *SessionManager) AppendCustomMessageEntry(customType string, content model.ContentList, display bool, details any) (string, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	entry := &model.CustomMessageEntry{}
	entry.SessionEntryBase = model.SessionEntryBase{
		Type: "custom_message", ID: sm.generateID(), ParentID: sm.leafID, Timestamp: nowISO(),
	}
	entry.CustomType = customType
	entry.Content = content
	entry.Display = display
	entry.Details = details
	return sm.appendEntry(entry)
}

// AppendLabelChange sets or clears a label on an entry.
func (sm *SessionManager) AppendLabelChange(targetID string, label *string) (string, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	if _, ok := sm.byID[targetID]; !ok {
		//nolint:staticcheck // upstream error text is part of the contract
		return "", fmt.Errorf("Entry %s not found", targetID)
	}
	entry := &model.LabelEntry{}
	entry.SessionEntryBase = model.SessionEntryBase{
		Type: "label", ID: sm.generateID(), ParentID: sm.leafID, Timestamp: nowISO(),
	}
	entry.TargetID = targetID
	entry.Label = label
	id, err := sm.appendEntry(entry)
	if err != nil {
		return "", err
	}
	if label != nil {
		sm.labelsByID[targetID] = *label
		sm.labelTimestampsByID[targetID] = entry.Timestamp
	} else {
		delete(sm.labelsByID, targetID)
		delete(sm.labelTimestampsByID, targetID)
	}
	return id, nil
}

// AppendContextEdit appends a branch-local edit to an earlier model-visible
// entry. A nil replacement omits the target from context.
func (sm *SessionManager) AppendContextEdit(targetID string, replacement *model.ContextEditableContent) (string, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	target, ok := sm.byID[targetID]
	if !ok {
		//nolint:staticcheck // upstream error text is part of the contract
		return "", fmt.Errorf("Entry %s not found", targetID)
	}
	onBranch := false
	for _, entry := range sm.branchUnlocked() {
		if entry.Base().ID == targetID {
			onBranch = true
			break
		}
	}
	if !onBranch {
		//nolint:staticcheck // upstream error text is part of the contract
		return "", fmt.Errorf("Entry %s is not on the active branch", targetID)
	}
	if !editableContextEntry(target) {
		//nolint:staticcheck // upstream error text is part of the contract
		return "", fmt.Errorf("Entry %s does not contribute editable model content", targetID)
	}
	entry := &model.ContextEditEntry{}
	entry.SessionEntryBase = model.SessionEntryBase{
		Type: "context_edit", ID: sm.generateID(), ParentID: sm.leafID, Timestamp: nowISO(),
	}
	entry.TargetID = targetID
	entry.Replacement = replacement
	return sm.appendEntry(entry)
}

func editableContextEntry(entry model.SessionEntry) bool {
	switch typed := entry.(type) {
	case *model.CustomMessageEntry:
		return true
	case *model.SessionMessageEntry:
		switch typed.Message.MessageRole() {
		case model.RoleUser, model.RoleAssistant, model.RoleToolResult:
			return true
		}
	}
	return false
}

func (sm *SessionManager) branchUnlocked() []model.SessionEntry {
	var path []model.SessionEntry
	current := sm.byID[ptrValue(sm.leafID)]
	for current != nil {
		path = append(path, current)
		parentID := current.Base().ParentID
		if parentID == nil {
			break
		}
		current = sm.byID[*parentID]
	}
	return path
}

// CreateBranchedSession writes a new session containing only the path from
// root to leafID. For in-memory sessions it replaces the current entries and
// returns "".
func (sm *SessionManager) CreateBranchedSession(leafID string) (string, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	previousSessionFile := sm.sessionFile
	path := sm.branchFromUnlocked(leafID)
	if len(path) == 0 {
		//nolint:staticcheck // upstream error text is part of the contract
		return "", fmt.Errorf("Entry %s not found", leafID)
	}

	var pathWithoutLabels []model.SessionEntry
	replacementByLabelID := map[string]string{}
	var pendingLabelIDs []string
	var pathParentID *string
	for _, entry := range path {
		if label, ok := entry.(*model.LabelEntry); ok {
			pendingLabelIDs = append(pendingLabelIDs, label.Base().ID)
			continue
		}
		for _, labelID := range pendingLabelIDs {
			replacementByLabelID[labelID] = entry.Base().ID
		}
		pendingLabelIDs = nil
		var clone model.SessionEntry
		if compaction, ok := entry.(*model.CompactionEntry); ok {
			copied := *compaction
			copied.ParentID = pathParentID
			if compaction.FirstKeptEntryID == compaction.Base().ID {
				copied.FirstKeptEntryID = compaction.Base().ID
			} else if replacement, ok := replacementByLabelID[compaction.FirstKeptEntryID]; ok {
				copied.FirstKeptEntryID = replacement
			}
			clone = &copied
		} else {
			clone = cloneEntryWithParent(entry, pathParentID)
		}
		pathWithoutLabels = append(pathWithoutLabels, clone)
		parentID := entry.Base().ID
		pathParentID = &parentID
	}

	newSessionID := model.UUIDv7()
	timestamp := nowISO()
	fileTimestamp := strings.NewReplacer(":", "-", ".", "-").Replace(timestamp)
	newSessionFile := filepath.Join(sm.sessionDir, fileTimestamp+"_"+newSessionID+".jsonl")

	header := model.NewSessionHeader(newSessionID, timestamp, sm.cwd)
	if sm.persist {
		header.ParentSession = previousSessionFile
	}

	pathEntryIDs := map[string]bool{}
	for _, entry := range pathWithoutLabels {
		pathEntryIDs[entry.Base().ID] = true
	}
	type labelToWrite struct {
		targetID  string
		label     string
		timestamp string
	}
	var labelsToWrite []labelToWrite
	for targetID, label := range sm.labelsByID {
		if pathEntryIDs[targetID] {
			labelsToWrite = append(labelsToWrite, labelToWrite{targetID, label, sm.labelTimestampsByID[targetID]})
		}
	}
	sort.Slice(labelsToWrite, func(i, j int) bool { return labelsToWrite[i].timestamp < labelsToWrite[j].timestamp })

	taken := func(id string) bool {
		_, exists := pathEntryIDs[id]
		return exists
	}
	generate := func() string {
		for range 100 {
			var b [4]byte
			if _, err := rand.Read(b[:]); err != nil {
				panic(err)
			}
			id := hex.EncodeToString(b[:])
			if !taken(id) {
				pathEntryIDs[id] = true
				return id
			}
		}
		return randomUUIDv4()
	}

	var lastID *string
	if len(pathWithoutLabels) > 0 {
		id := pathWithoutLabels[len(pathWithoutLabels)-1].Base().ID
		lastID = &id
	}
	parentID := lastID
	var labelEntries []model.SessionEntry
	for _, item := range labelsToWrite {
		entry := &model.LabelEntry{}
		entry.SessionEntryBase = model.SessionEntryBase{
			Type: "label", ID: generate(), ParentID: parentID, Timestamp: item.timestamp,
		}
		entry.TargetID = item.targetID
		value := item.label
		entry.Label = &value
		labelEntries = append(labelEntries, entry)
		entryParent := entry.Base().ID
		parentID = &entryParent
	}

	sm.header = &header
	sm.entries = append(append([]model.SessionEntry(nil), pathWithoutLabels...), labelEntries...)
	sm.sessionID = newSessionID
	if sm.persist {
		sm.sessionFile = newSessionFile
	}
	sm.buildIndex()

	if sm.persist {
		if sm.hasConversation() {
			if err := sm.rewriteFile(); err != nil {
				return "", err
			}
			sm.flushed = true
		} else {
			sm.flushed = false
		}
		return newSessionFile, nil
	}
	return "", nil
}

func (sm *SessionManager) branchFromUnlocked(leafID string) []model.SessionEntry {
	var path []model.SessionEntry
	current := sm.byID[leafID]
	for current != nil {
		path = append(path, current)
		parentID := current.Base().ParentID
		if parentID == nil {
			break
		}
		current = sm.byID[*parentID]
	}
	for i, j := 0, len(path)-1; i < j; i, j = i+1, j-1 {
		path[i], path[j] = path[j], path[i]
	}
	return path
}

func cloneEntryWithParent(entry model.SessionEntry, parentID *string) model.SessionEntry {
	clone := model.CloneSessionEntry(entry)
	if clone == nil {
		return entry
	}
	setEntryParent(clone, parentID)
	return clone
}

func setEntryParent(entry model.SessionEntry, parentID *string) {
	switch typed := entry.(type) {
	case *model.SessionMessageEntry:
		typed.ParentID = parentID
	case *model.ThinkingLevelChangeEntry:
		typed.ParentID = parentID
	case *model.ModelChangeEntry:
		typed.ParentID = parentID
	case *model.UsageEntry:
		typed.ParentID = parentID
	case *model.CompactionEntry:
		typed.ParentID = parentID
	case *model.BranchSummaryEntry:
		typed.ParentID = parentID
	case *model.CustomEntry:
		typed.ParentID = parentID
	case *model.LabelEntry:
		typed.ParentID = parentID
	case *model.SessionInfoEntry:
		typed.ParentID = parentID
	case *model.CustomMessageEntry:
		typed.ParentID = parentID
	case *model.ContextEditEntry:
		typed.ParentID = parentID
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func normalizePath(path string) string {
	return files.NormalizePath(path, files.PathInputOptions{})
}

func resolvePath(path string) string {
	return files.ResolvePath(path, "", files.PathInputOptions{})
}

func pathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func ptrValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func nowISO() string {
	return time.Now().UTC().Format("2006-01-02T15:04:05.000Z")
}

func timestampMillis(timestamp string) int64 {
	parsed, err := time.Parse(time.RFC3339Nano, timestamp)
	if err != nil {
		return 0
	}
	return parsed.UnixMilli()
}
