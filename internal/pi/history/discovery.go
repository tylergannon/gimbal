package history

import (
	"bufio"
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/tylergannon/gimbal/internal/pi/model"
)

const (
	maxConcurrentSessionInfoLoads      = 10
	maxConcurrentSessionDiscoveryLoads = 64
	currentSessionListPublishInterval  = 10
	allSessionListPublishInterval      = 100
)

// SessionListProgress reports session listing progress. partialSessions is
// present only on periodic updates.
type SessionListProgress func(loaded, total int, partialSessions []model.SessionInfo)

// DefaultSessionDir returns the per-cwd session directory under agentDir,
// encoding cwd the way Pi does (--<cwd with separators as dashes>--).
func DefaultSessionDir(cwd, agentDir string) string {
	resolvedCwd := resolvePath(cwd)
	resolvedAgentDir := resolvePath(agentDir)
	return filepath.Join(resolvedAgentDir, "sessions", "--"+encodeCwdSafePath(resolvedCwd)+"--")
}

// EncodeCwdSafePath mirrors Pi's safe-path encoding: strip exactly one leading
// separator, then replace '/', '\\', and ':' with '-'.
func encodeCwdSafePath(resolved string) string {
	if len(resolved) > 0 && (resolved[0] == '/' || resolved[0] == '\\') {
		resolved = resolved[1:]
	}
	return strings.NewReplacer("/", "-", "\\", "-", ":", "-").Replace(resolved)
}

func sessionCwdMatches(cwd, resolvedCwd string) bool {
	if cwd == "" {
		return false
	}
	return resolvePath(cwd) == resolvedCwd
}

// FindMostRecentSession returns the most recently written valid session file
// in sessionDir. When cwd is non-empty only sessions whose header cwd matches
// are considered.
func FindMostRecentSession(sessionDir, cwd string) (string, bool) {
	dir := normalizePath(sessionDir)
	resolvedCwd := ""
	if cwd != "" {
		resolvedCwd = resolvePath(cwd)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", false
	}
	type candidate struct {
		path  string
		mtime time.Time
	}
	var candidates []candidate
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".jsonl") {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		info, err := entry.Info()
		if err != nil {
			return "", false
		}
		candidates = append(candidates, candidate{path: path, mtime: info.ModTime()})
	}
	sort.SliceStable(candidates, func(i, j int) bool { return candidates[i].mtime.After(candidates[j].mtime) })
	for _, candidate := range candidates {
		header := readSessionHeaderForDiscovery(candidate.path)
		if header == nil {
			continue
		}
		if resolvedCwd != "" && !sessionCwdMatches(header.Cwd, resolvedCwd) {
			continue
		}
		return candidate.path, true
	}
	return "", false
}

// List returns the sessions in sessionDir, newest activity first. When cwd is
// non-empty only sessions whose header cwd matches are returned.
func List(ctx context.Context, cwd, sessionDir string, onProgress SessionListProgress) ([]model.SessionInfo, error) {
	dir := normalizePath(sessionDir)
	if resolved := ctx.Err(); resolved != nil {
		return nil, resolved
	}
	if !pathExists(dir) {
		return nil, nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, nil
	}
	var paths []string
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".jsonl") {
			continue
		}
		paths = append(paths, filepath.Join(dir, entry.Name()))
	}
	sort.Sort(sort.Reverse(sort.StringSlice(paths)))
	resolvedCwd := ""
	if cwd != "" {
		resolvedCwd = resolvePath(cwd)
	}
	infos, err := loadSessionInfos(ctx, paths, onProgress, currentSessionListPublishInterval)
	if err != nil {
		return nil, err
	}
	filtered := make([]model.SessionInfo, 0, len(infos))
	for _, info := range infos {
		if resolvedCwd != "" && !sessionCwdMatches(info.Cwd, resolvedCwd) {
			continue
		}
		filtered = append(filtered, info)
	}
	return sortSessionInfos(filtered), nil
}

// ListAllProjects lists every session under a sessions root, whose child
// directories are per-project session directories.
func ListAllProjects(ctx context.Context, sessionsRoot string, onProgress SessionListProgress) ([]model.SessionInfo, error) {
	root := normalizePath(sessionsRoot)
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !pathExists(root) {
		return nil, nil
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, nil
	}
	var dirs []string
	for _, entry := range entries {
		if entry.IsDir() {
			dirs = append(dirs, filepath.Join(root, entry.Name()))
		}
	}
	dirFiles, err := mapWithConcurrency(ctx, dirs, maxConcurrentSessionDiscoveryLoads, func(dir string) []string {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return nil
		}
		var paths []string
		for _, entry := range entries {
			if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".jsonl") {
				paths = append(paths, filepath.Join(dir, entry.Name()))
			}
		}
		return paths
	})
	if err != nil {
		return nil, err
	}
	var paths []string
	for _, group := range dirFiles {
		paths = append(paths, group...)
	}
	sort.Sort(sort.Reverse(sort.StringSlice(paths)))
	infos, err := loadSessionInfos(ctx, paths, onProgress, allSessionListPublishInterval)
	if err != nil {
		return nil, err
	}
	return sortSessionInfos(infos), nil
}

func sortSessionInfos(infos []model.SessionInfo) []model.SessionInfo {
	sort.SliceStable(infos, func(i, j int) bool {
		return parseTime(infos[i].Modified).After(parseTime(infos[j].Modified))
	})
	return infos
}

func loadSessionInfos(ctx context.Context, paths []string, onProgress SessionListProgress, publishInterval int) ([]model.SessionInfo, error) {
	infos, err := mapWithConcurrency(ctx, paths, maxConcurrentSessionInfoLoads, func(path string) *model.SessionInfo {
		return buildSessionInfo(ctx, path)
	})
	if err != nil {
		return nil, err
	}
	var out []model.SessionInfo
	for i, info := range infos {
		if info == nil {
			continue
		}
		out = append(out, *info)
		if onProgress != nil {
			loaded := i + 1
			if loaded == 1 || loaded%publishInterval == 0 || loaded == len(paths) {
				onProgress(loaded, len(paths), sortSessionInfos(append([]model.SessionInfo(nil), out...)))
			}
		}
	}
	return out, nil
}

func buildSessionInfo(ctx context.Context, path string) *model.SessionInfo {
	stat, err := os.Stat(path)
	if err != nil {
		if ctx.Err() != nil {
			return nil
		}
		return nil
	}
	file, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer func() { _ = file.Close() }()

	var header *model.SessionHeader
	messageCount := 0
	firstMessage := ""
	var allMessages []string
	name := ""
	var lastActivity int64

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), sessionReadBufferSize)
	for scanner.Scan() {
		if ctx.Err() != nil {
			return nil
		}
		line, ok := parseSessionLine(scanner.Text())
		if !ok {
			continue
		}
		if header == nil {
			if line.header == nil {
				return nil
			}
			header = line.header
			continue
		}
		entry := line.entry
		if entry == nil {
			continue
		}
		if info, ok := entry.(*model.SessionInfoEntry); ok {
			name = strings.TrimSpace(info.Name)
			continue
		}
		message, ok := entry.(*model.SessionMessageEntry)
		if !ok {
			continue
		}
		messageCount++
		if activity := messageActivityTime(message); activity != 0 {
			if activity > lastActivity {
				lastActivity = activity
			}
		}
		if !isTextMessage(message.Message) {
			continue
		}
		text := extractTextContent(message.Message)
		if text == "" {
			continue
		}
		allMessages = append(allMessages, text)
		if firstMessage == "" && message.Message.MessageRole() == model.RoleUser {
			firstMessage = text
		}
	}
	if header == nil {
		return nil
	}
	cwd := header.Cwd
	headerTime := parseTime(header.Timestamp)
	modified := time.Time{}
	switch {
	case lastActivity > 0:
		modified = time.UnixMilli(lastActivity)
	case !headerTime.IsZero():
		modified = headerTime
	default:
		modified = stat.ModTime()
	}
	if firstMessage == "" {
		firstMessage = "(no messages)"
	}
	return &model.SessionInfo{
		Path:              path,
		ID:                header.ID,
		Cwd:               cwd,
		Name:              name,
		ParentSessionPath: header.ParentSession,
		Created:           header.Timestamp,
		Modified:          modified.UTC().Format(time.RFC3339Nano),
		MessageCount:      messageCount,
		FirstMessage:      firstMessage,
		AllMessagesText:   strings.Join(allMessages, " "),
	}
}

func isTextMessage(message model.Message) bool {
	role := message.MessageRole()
	return role == model.RoleUser || role == model.RoleAssistant
}

func extractTextContent(message model.Message) string {
	switch typed := message.(type) {
	case model.UserMessage:
		if text, ok := typed.StringContent(); ok {
			return text
		}
		return textBlocks(typed.Content)
	case model.AssistantMessage:
		return textBlocks(typed.Content)
	default:
		return ""
	}
}

func textBlocks(content model.ContentList) string {
	var texts []string
	for _, block := range content {
		if text, ok := block.(model.TextContent); ok {
			texts = append(texts, text.Text)
		}
	}
	return strings.Join(texts, " ")
}

func messageActivityTime(entry *model.SessionMessageEntry) int64 {
	if !isTextMessage(entry.Message) {
		return 0
	}
	var timestamp int64
	switch typed := entry.Message.(type) {
	case model.UserMessage:
		timestamp = typed.Timestamp
	case model.AssistantMessage:
		timestamp = typed.Timestamp
	}
	if timestamp != 0 {
		return timestamp
	}
	return timestampMillis(entry.Timestamp)
}

func parseTime(value string) time.Time {
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return time.Time{}
	}
	return parsed
}

// mapWithConcurrency applies mapFn to every item with at most limit in flight,
// preserving input order. It stops at the first cancellation.
func mapWithConcurrency[T any, R any](ctx context.Context, items []T, limit int, mapFn func(T) R) ([]R, error) {
	results := make([]R, len(items))
	if len(items) == 0 {
		return results, nil
	}
	if limit > len(items) {
		limit = len(items)
	}
	var (
		mu       sync.Mutex
		next     int
		firstErr error
		wg       sync.WaitGroup
	)
	worker := func() {
		defer wg.Done()
		for {
			if err := ctx.Err(); err != nil {
				mu.Lock()
				if firstErr == nil {
					firstErr = err
				}
				mu.Unlock()
				return
			}
			mu.Lock()
			index := next
			next++
			mu.Unlock()
			if index >= len(items) {
				return
			}
			results[index] = mapFn(items[index])
		}
	}
	wg.Add(limit)
	for i := 0; i < limit; i++ {
		go worker()
	}
	wg.Wait()
	if firstErr != nil {
		return nil, firstErr
	}
	return results, nil
}
