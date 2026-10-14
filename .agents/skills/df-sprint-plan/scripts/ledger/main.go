// Manage YAML and TSV sprint ledgers in the target project.
package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

var sprintID = regexp.MustCompile(`^SPRINT-(\d{4})$`)
var chapterID = regexp.MustCompile(`^CHAPTER-\d{4}$`)

const tsvHeader = "sprint_id\ttitle\tstatus\tcreated_at\tupdated_at"

type entry struct {
	ID       string `yaml:"id"`
	Title    string `yaml:"title"`
	Status   string `yaml:"status"`
	Chapter  string `yaml:"chapter,omitempty"`
	Executor string `yaml:"executor,omitempty"`
	Created  string `yaml:"created"`
	Updated  string `yaml:"updated"`
}
type ledger struct {
	path, root string
	isYAML     bool
	entries    []entry
}

func number(id string) (int, error) { return strconv.Atoi(strings.TrimPrefix(id, "SPRINT-")) }
func (l *ledger) canonicalID(id string) (string, error) {
	n, err := number(id)
	if err != nil || n < 0 {
		return "", fmt.Errorf("invalid sprint ID %q", id)
	}
	if l.isYAML {
		return fmt.Sprintf("SPRINT-%04d", n), nil
	}
	if sprintID.MatchString(id) {
		n %= 1000
	}
	return fmt.Sprintf("%03d", n), nil
}
func (l *ledger) statuses() []string {
	if l.isYAML {
		return []string{"planned", "in-progress", "done", "abandoned"}
	}
	return []string{"planned", "in_progress", "completed", "skipped"}
}
func (l *ledger) canonicalStatus(status string) (string, error) {
	aliases := map[string]string{"in_progress": "in-progress", "completed": "done", "skipped": "abandoned"}
	if value, ok := aliases[status]; ok {
		status = value
	}
	if !l.isYAML {
		aliases = map[string]string{"in-progress": "in_progress", "done": "completed", "abandoned": "skipped"}
		if value, ok := aliases[status]; ok {
			status = value
		}
	}
	for _, allowed := range l.statuses() {
		if status == allowed {
			return status, nil
		}
	}
	return "", fmt.Errorf("invalid status: %s; must be one of %v", status, l.statuses())
}
func open(root, path string) (*ledger, error) {
	if path == "" {
		path = filepath.Join(root, "docs/sprints/ledger.yaml")
		if _, err := os.Stat(path); os.IsNotExist(err) {
			tsv := filepath.Join(root, "docs/sprints/ledger.tsv")
			if _, err := os.Stat(tsv); err == nil {
				path = tsv
			}
		}
	} else if !filepath.IsAbs(path) {
		path = filepath.Join(root, path)
	}
	l := &ledger{path: path, root: root, isYAML: strings.HasSuffix(path, ".yaml") || strings.HasSuffix(path, ".yml"), entries: []entry{}}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return l, nil
	}
	if err != nil {
		return nil, err
	}
	if l.isYAML {
		var document struct {
			Sprints []entry `yaml:"sprints"`
		}
		if err := yaml.Unmarshal(data, &document); err != nil {
			return nil, err
		}
		l.entries = document.Sprints
	} else {
		for _, line := range strings.Split(string(data), "\n") {
			if strings.TrimSpace(line) == "" || line == tsvHeader {
				continue
			}
			parts := strings.Split(line, "\t")
			if len(parts) != 5 {
				return nil, fmt.Errorf("invalid TSV line (expected 5 fields): %s", line)
			}
			id, err := l.canonicalID(parts[0])
			if err != nil {
				return nil, err
			}
			l.entries = append(l.entries, entry{ID: id, Title: parts[1], Status: parts[2], Created: parts[3], Updated: parts[4]})
		}
	}
	for _, row := range l.entries {
		if _, err := number(row.ID); err != nil {
			return nil, err
		}
	}
	return l, nil
}
func (l *ledger) save() error {
	sort.SliceStable(l.entries, func(i, j int) bool { a, _ := number(l.entries[i].ID); b, _ := number(l.entries[j].ID); return a < b })
	if err := os.MkdirAll(filepath.Dir(l.path), 0755); err != nil {
		return err
	}
	var data []byte
	if l.isYAML {
		if l.entries == nil {
			l.entries = []entry{}
		}
		var err error
		data, err = yaml.Marshal(struct {
			Sprints []entry `yaml:"sprints"`
		}{l.entries})
		if err != nil {
			return err
		}
	} else {
		lines := []string{tsvHeader}
		for _, row := range l.entries {
			for _, value := range []string{row.Title, row.Status, row.Created, row.Updated} {
				if strings.ContainsAny(value, "\t\r\n") {
					return fmt.Errorf("TSV fields cannot contain tabs or newlines")
				}
			}
			lines = append(lines, strings.Join([]string{row.ID, row.Title, row.Status, row.Created, row.Updated}, "\t"))
		}
		data = []byte(strings.Join(lines, "\n") + "\n")
	}
	return os.WriteFile(l.path, data, 0644)
}
func (l *ledger) find(id string) (*entry, error) {
	canonical, err := l.canonicalID(id)
	if err != nil {
		return nil, err
	}
	for i := range l.entries {
		if l.entries[i].ID == canonical {
			return &l.entries[i], nil
		}
	}
	return nil, nil
}
func timestamp() string { return time.Now().UTC().Format("2006-01-02T15:04:05+00:00") }
func (l *ledger) add(id, title, status, chapter string) (*entry, error) {
	if id == "" {
		n := 0
		for _, row := range l.entries {
			v, _ := number(row.ID)
			if v > n {
				n = v
			}
		}
		id = strconv.Itoa(n + 1)
	}
	id, err := l.canonicalID(id)
	if err != nil {
		return nil, err
	}
	existing, err := l.find(id)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, fmt.Errorf("sprint %s already exists", id)
	}
	status, err = l.canonicalStatus(status)
	if err != nil {
		return nil, err
	}
	if chapter != "" && l.isYAML && !chapterID.MatchString(chapter) {
		return nil, fmt.Errorf("chapter must look like CHAPTER-0001")
	}
	if !l.isYAML {
		chapter = ""
	}
	now := timestamp()
	l.entries = append(l.entries, entry{ID: id, Title: title, Status: status, Chapter: chapter, Created: now, Updated: now})
	return &l.entries[len(l.entries)-1], nil
}
func (l *ledger) selectEntry(status string) *entry {
	canonical, _ := l.canonicalStatus(status)
	var chosen *entry
	for i := range l.entries {
		row := &l.entries[i]
		if row.Status == canonical {
			if chosen == nil {
				chosen = row
			} else if status == "planned" {
				a, _ := number(row.ID)
				b, _ := number(chosen.ID)
				if a < b {
					chosen = row
				}
			}
		}
	}
	return chosen
}
func (l *ledger) sync() (int, error) {
	dir := filepath.Join(l.root, "docs/sprints")
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		dir = filepath.Dir(l.path)
	}
	docs, err := filepath.Glob(filepath.Join(dir, "SPRINT-*.md"))
	if err != nil {
		return 0, err
	}
	added := 0
	for _, path := range docs {
		stem := strings.TrimSuffix(filepath.Base(path), ".md")
		id, err := l.canonicalID(stem)
		if err != nil {
			return added, err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return added, err
		}
		title := "Sprint " + strings.TrimPrefix(stem, "SPRINT-")
		chapter := ""
		lines := strings.Split(string(data), "\n")
		if len(lines) > 40 {
			lines = lines[:40]
		}
		for _, line := range lines {
			if strings.HasPrefix(line, "# ") {
				if _, rest, found := strings.Cut(line, ":"); found && rest != "" {
					title = strings.TrimSpace(rest)
				}
			}
			if strings.HasPrefix(line, "Chapter:") {
				chapter = chapterID.FindString(strings.TrimSpace(strings.TrimPrefix(line, "Chapter:")))
				if chapter == "" {
					chapter = regexp.MustCompile(`CHAPTER-\d{4}`).FindString(line)
				}
			}
		}
		row, err := l.find(id)
		if err != nil {
			return added, err
		}
		if row != nil {
			if l.isYAML && chapter != "" && row.Chapter == "" {
				row.Chapter = chapter
				row.Updated = timestamp()
			}
			continue
		}
		if _, err := l.add(id, title, "planned", chapter); err != nil {
			return added, err
		}
		added++
	}
	return added, nil
}
func options(args []string) (map[string]string, []string, error) {
	opts := map[string]string{}
	var pos []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if strings.HasPrefix(arg, "--") {
			key := arg[2:]
			if !strings.Contains("|project|ledger|status|chapter|", "|"+key+"|") {
				return nil, nil, fmt.Errorf("unknown option %s", arg)
			}
			if i+1 == len(args) {
				return nil, nil, fmt.Errorf("%s requires a value", arg)
			}
			i++
			opts[key] = args[i]
		} else {
			pos = append(pos, arg)
		}
	}
	return opts, pos, nil
}
func printEntry(out io.Writer, l *ledger, row *entry, started bool) error {
	doc := row.ID
	if !l.isYAML {
		doc = "SPRINT-" + doc
	}
	if _, err := fmt.Fprintf(out, "%s\t%s\n  Doc: docs/sprints/%s.md\n", row.ID, row.Title, doc); err != nil {
		return err
	}
	if row.Chapter != "" {
		if _, err := fmt.Fprintln(out, "  Chapter:", row.Chapter); err != nil {
			return err
		}
	}
	if started {
		_, err := fmt.Fprintln(out, "  Started:", row.Updated)
		return err
	}
	return nil
}
func run(args []string, out io.Writer) error {
	for _, arg := range args {
		if arg == "--help" || arg == "-h" {
			_, err := fmt.Fprintln(out, "ledger --project ROOT [--ledger PATH] add TITLE | start ID | complete ID | skip ID | status ID STATUS | set-chapter ID CHAPTER | next | current | stats | list | sync")
			return err
		}
	}
	opts, pos, err := options(args)
	if err != nil {
		return err
	}
	if opts["project"] == "" {
		return fmt.Errorf("--project is required because go run -C changes directory")
	}
	root, err := filepath.Abs(opts["project"])
	if err != nil {
		return err
	}
	info, err := os.Stat(root)
	if err != nil || !info.IsDir() {
		return fmt.Errorf("project directory does not exist: %s", root)
	}
	l, err := open(root, opts["ledger"])
	if err != nil {
		return err
	}
	if len(pos) == 0 {
		return fmt.Errorf("commands: add, start, complete, skip, status, set-chapter, next, current, stats, list, sync")
	}
	switch pos[0] {
	case "add":
		if len(pos) < 2 {
			return fmt.Errorf("add requires TITLE or ID TITLE")
		}
		id, title := "", pos[1]
		if len(pos) > 2 {
			id = pos[1]
			title = strings.Join(pos[2:], " ")
		}
		status := opts["status"]
		if status == "" {
			status = "planned"
		}
		row, err := l.add(id, title, status, opts["chapter"])
		if err != nil {
			return err
		}
		message := fmt.Sprintf("Added sprint %s: %s [%s]", row.ID, row.Title, row.Status)
		if err := l.save(); err != nil {
			return err
		}
		_, err = fmt.Fprintln(out, message)
		return err
	case "start", "complete", "skip", "status", "set-chapter":
		if len(pos) < 2 {
			return fmt.Errorf("%s requires sprint ID", pos[0])
		}
		row, err := l.find(pos[1])
		if err != nil {
			return err
		}
		if row == nil {
			return fmt.Errorf("sprint %s not found", pos[1])
		}
		message := ""
		if pos[0] == "set-chapter" {
			if !l.isYAML {
				return fmt.Errorf("chapter fields require a YAML sprint ledger")
			}
			if len(pos) != 3 {
				return fmt.Errorf("set-chapter requires chapter ID or empty string")
			}
			if pos[2] != "" && !chapterID.MatchString(pos[2]) {
				return fmt.Errorf("chapter must look like CHAPTER-0001, or '' to clear")
			}
			row.Chapter = pos[2]
			message = fmt.Sprintf("Sprint %s chapter updated", pos[1])
		} else {
			status := map[string]string{"start": "in-progress", "complete": "done", "skip": "abandoned"}[pos[0]]
			if pos[0] == "status" {
				if len(pos) != 3 {
					return fmt.Errorf("status requires new status")
				}
				status = pos[2]
			}
			canonical, err := l.canonicalStatus(status)
			if err != nil {
				return err
			}
			row.Status = canonical
			switch pos[0] {
			case "start":
				message = fmt.Sprintf("Sprint %s is now %s", pos[1], canonical)
			case "status":
				message = fmt.Sprintf("Sprint %s status updated to '%s'", pos[1], canonical)
			default:
				message = fmt.Sprintf("Sprint %s marked as %s", pos[1], canonical)
			}
		}
		row.Updated = timestamp()
		if err := l.save(); err != nil {
			return err
		}
		_, err = fmt.Fprintln(out, message)
		return err
	case "next", "current":
		status := "planned"
		if pos[0] == "current" {
			status = "in-progress"
		}
		row := l.selectEntry(status)
		if row == nil {
			if pos[0] == "next" {
				return fmt.Errorf("no planned sprints")
			}
			return fmt.Errorf("no sprint currently in progress")
		}
		return printEntry(out, l, row, pos[0] == "current")
	case "stats":
		if _, err := fmt.Fprintln(out, "Total sprints:", len(l.entries)); err != nil {
			return err
		}
		for _, status := range l.statuses() {
			count := 0
			for _, row := range l.entries {
				if row.Status == status {
					count++
				}
			}
			if _, err := fmt.Fprintf(out, "  %s: %d\n", status, count); err != nil {
				return err
			}
		}
		if row := l.selectEntry("in-progress"); row != nil {
			if _, err := fmt.Fprintf(out, "\nCurrently working on: %s - %s\n", row.ID, row.Title); err != nil {
				return err
			}
		}
		if row := l.selectEntry("planned"); row != nil {
			if _, err := fmt.Fprintf(out, "Next up: %s - %s\n", row.ID, row.Title); err != nil {
				return err
			}
		}
		return nil
	case "list":
		status := ""
		if opts["status"] != "" {
			status, err = l.canonicalStatus(opts["status"])
			if err != nil {
				return err
			}
		}
		for _, row := range l.entries {
			if (status == "" || row.Status == status) && (opts["chapter"] == "" || row.Chapter == opts["chapter"]) {
				chapter := ""
				if row.Chapter != "" {
					chapter = " " + row.Chapter
				}
				if _, err := fmt.Fprintf(out, "%s: %s %s%s\n", row.ID, row.Status, row.Title, chapter); err != nil {
					return err
				}
			}
		}
		return nil
	case "sync":
		added, err := l.sync()
		if err != nil {
			return err
		}
		if err := l.save(); err != nil {
			return err
		}
		_, err = fmt.Fprintf(out, "Synced: %d new sprints added, %d total in ledger\n", added, len(l.entries))
		return err
	default:
		return fmt.Errorf("unknown command %s", pos[0])
	}
}
func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
