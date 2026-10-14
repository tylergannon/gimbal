// Manage the target project's chapter ledger.
package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

var chapterID = regexp.MustCompile(`^CHAPTER-(\d{4})$`)
var fields = []string{"id", "title", "status", "doc", "created", "updated", "summary", "sprint_ids"}

type chapter struct {
	ID      string   `yaml:"id"`
	Title   string   `yaml:"title"`
	Status  string   `yaml:"status"`
	Doc     string   `yaml:"doc"`
	Created string   `yaml:"created"`
	Updated string   `yaml:"updated"`
	Summary string   `yaml:"summary"`
	Sprints []string `yaml:"sprint_ids"`
}
type ledger struct {
	Chapters []chapter `yaml:"chapters"`
}

func load(path string) (ledger, error) {
	var data ledger
	bytes, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		data.Chapters = []chapter{}
		return data, nil
	}
	if err != nil {
		return data, err
	}
	err = yaml.Unmarshal(bytes, &data)
	return data, err
}
func save(path string, data ledger) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	bytes, err := yaml.Marshal(data)
	if err != nil {
		return err
	}
	return os.WriteFile(path, bytes, 0644)
}
func nextID(data ledger) string {
	n := 0
	for _, entry := range data.Chapters {
		var value int
		if chapterID.MatchString(entry.ID) {
			if _, err := fmt.Sscanf(entry.ID, "CHAPTER-%d", &value); err == nil && value > n {
				n = value
			}
		}
	}
	return fmt.Sprintf("CHAPTER-%04d", n+1)
}
func find(data ledger, id string) *chapter {
	for i := range data.Chapters {
		if data.Chapters[i].ID == id {
			return &data.Chapters[i]
		}
	}
	return nil
}

// Options may follow positional arguments, matching the documented command interface.
func options(args []string) (map[string][]string, []string, error) {
	opts := map[string][]string{}
	var positional []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if strings.HasPrefix(arg, "--") {
			key := strings.TrimPrefix(arg, "--")
			if !strings.Contains("|project|id|status|doc|summary|sprint-id|", "|"+key+"|") {
				return nil, nil, fmt.Errorf("unknown option %s", arg)
			}
			if i+1 == len(args) {
				return nil, nil, fmt.Errorf("%s requires a value", arg)
			}
			i++
			opts[key] = append(opts[key], args[i])
		} else {
			positional = append(positional, arg)
		}
	}
	return opts, positional, nil
}
func value(opts map[string][]string, key, defaultValue string) string {
	items := opts[key]
	if len(items) > 0 {
		return items[len(items)-1]
	}
	return defaultValue
}
func run(args []string, out io.Writer) error {
	for _, arg := range args {
		if arg == "--help" || arg == "-h" {
			_, err := fmt.Fprintln(out, "chapter --project ROOT next-id | add TITLE --doc PATH --summary TEXT | list | get ID")
			return err
		}
	}
	opts, pos, err := options(args)
	if err != nil {
		return err
	}
	if len(pos) == 0 {
		return fmt.Errorf("commands: next-id, add TITLE --doc PATH --summary TEXT, list, get ID; --project ROOT is required")
	}
	project := value(opts, "project", "")
	if project == "" {
		return fmt.Errorf("--project is required because go run -C changes directory")
	}
	root, err := filepath.Abs(project)
	if err != nil {
		return err
	}
	info, err := os.Stat(root)
	if err != nil || !info.IsDir() {
		return fmt.Errorf("project directory does not exist: %s", root)
	}
	path := filepath.Join(root, "docs/chapters/ledger.yaml")
	data, err := load(path)
	if err != nil {
		return err
	}
	status := value(opts, "status", "")
	if status != "" && !strings.Contains("|active|paused|done|abandoned|", "|"+status+"|") {
		return fmt.Errorf("invalid status %s", status)
	}
	switch pos[0] {
	case "next-id":
		_, err = fmt.Fprintln(out, nextID(data))
		return err
	case "add":
		if len(pos) != 2 {
			return fmt.Errorf("add requires a title")
		}
		if _, ok := opts["doc"]; !ok {
			return fmt.Errorf("--doc is required")
		}
		if _, ok := opts["summary"]; !ok {
			return fmt.Errorf("--summary is required")
		}
		id := value(opts, "id", nextID(data))
		if !chapterID.MatchString(id) {
			return fmt.Errorf("chapter id must look like CHAPTER-0001")
		}
		if find(data, id) != nil {
			return fmt.Errorf("%s already exists", id)
		}
		timestamp := time.Now().UTC().Format(time.RFC3339)
		sprints := opts["sprint-id"]
		if sprints == nil {
			sprints = []string{}
		}
		data.Chapters = append(data.Chapters, chapter{id, pos[1], value(opts, "status", "active"), value(opts, "doc", ""), timestamp, timestamp, value(opts, "summary", ""), sprints})
		if err := save(path, data); err != nil {
			return err
		}
		_, err = fmt.Fprintln(out, id)
		return err
	case "list":
		for _, entry := range data.Chapters {
			if status == "" || entry.Status == status {
				if _, err := fmt.Fprintf(out, "%s  %s  %s\n", entry.ID, entry.Status, entry.Title); err != nil {
					return err
				}
			}
		}
		return nil
	case "get":
		if len(pos) != 2 {
			return fmt.Errorf("get requires an ID")
		}
		entry := find(data, pos[1])
		if entry == nil {
			return fmt.Errorf("%s not found", pos[1])
		}
		values := []string{entry.ID, entry.Title, entry.Status, entry.Doc, entry.Created, entry.Updated, entry.Summary, strings.Join(entry.Sprints, ", ")}
		for i, key := range fields {
			if _, err := fmt.Fprintf(out, "%s: %s\n", key, values[i]); err != nil {
				return err
			}
		}
		return nil
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
