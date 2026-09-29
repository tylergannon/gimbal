package claimaudit

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const StateDir = ".semantic-index"

type Block struct {
	ID      string `json:"id"`
	File    string `json:"file"`
	Line    int    `json:"line"`
	Heading string `json:"heading"`
	Text    string `json:"text"`
	Digest  string `json:"digest"`
}

type Inventory struct {
	Revision string            `json:"revision"`
	Blocks   []Block           `json:"blocks"`
	Files    map[string]string `json:"files"`
	Sources  map[string]string `json:"sources"`
	Unknown  []string          `json:"unknown,omitempty"`
	Extract  []string          `json:"extract"`
}

type Reference struct {
	Path      string `json:"path"`
	StartLine int    `json:"start_line"`
	EndLine   int    `json:"end_line"`
	Quote     string `json:"quote,omitempty"`
}

type Occurrence struct {
	BlockID string `json:"block_id"`
	Text    string `json:"text"`
}

type Claim struct {
	ID                string       `json:"id"`
	Text              string       `json:"text"`
	Scope             string       `json:"scope"`
	Kind              string       `json:"kind"`
	Occurrences       []Occurrence `json:"occurrences"`
	References        []Reference  `json:"references"`
	Disposition       string       `json:"disposition,omitempty"`
	DispositionDigest string       `json:"disposition_digest,omitempty"`
	Review            string       `json:"review,omitempty"`
}

func statePath(dir, name string) string { return filepath.Join(dir, StateDir, name) }
func digest(v []byte) string            { sum := sha256.Sum256(v); return hex.EncodeToString(sum[:]) }
func markFree(s string) string {
	lines := strings.Split(s, "\n")
	out := lines[:0]
	for _, line := range lines {
		if strings.HasPrefix(line, "<!-- gimbal-audit ") || strings.HasPrefix(line, "> **Gimbal audit: ") {
			continue
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}

// Prepare inventories all research index prose and all original source files.
// Claims for unchanged blocks remain in claims.jsonl; changed blocks are assigned
// to the curator in inventory.json. No model calls occur here.
func Prepare(dir string) (Inventory, error) {
	var inv Inventory
	inv.Sources = map[string]string{}
	inv.Files = map[string]string{}
	previous := Inventory{}
	if data, err := os.ReadFile(statePath(dir, "inventory.json")); err == nil {
		_ = json.Unmarshal(data, &previous)
	}
	old := map[string]string{}
	for _, b := range previous.Blocks {
		old[b.ID] = b.Digest
	}
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("research input contains symlink %s", path)
		}
		if d.IsDir() {
			if path != dir && filepath.Base(path) == StateDir {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		parts := strings.Split(filepath.ToSlash(rel), "/")
		if len(parts) > 1 && parts[len(parts)-2] == "sources" || strings.Contains("/"+filepath.ToSlash(rel), "/sources/") {
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			inv.Sources[rel] = digest(data)
			return nil
		}
		if filepath.Base(path) != "INDEX.md" && !strings.Contains(filepath.ToSlash(rel), "/clips/") {
			inv.Unknown = append(inv.Unknown, rel)
			return nil
		}
		if filepath.Ext(path) != ".md" && filepath.Ext(path) != ".txt" {
			inv.Unknown = append(inv.Unknown, rel)
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		text := markFree(string(data))
		inv.Files[rel] = digest([]byte(text))
		scanner := bufio.NewScanner(strings.NewReader(text))
		scanner.Buffer(make([]byte, 64*1024), 10<<20)
		line, start := 0, 0
		heading := ""
		var block []string
		flush := func() {
			if len(block) == 0 {
				return
			}
			content := strings.Join(block, "\n")
			id := digest([]byte(rel + "\x00" + fmt.Sprint(start) + "\x00" + content))[:16]
			b := Block{ID: id, File: rel, Line: start, Heading: heading, Text: content, Digest: digest([]byte(heading + "\x00" + content))}
			inv.Blocks = append(inv.Blocks, b)
			if old[id] != b.Digest {
				inv.Extract = append(inv.Extract, id)
			}
			block = nil
		}
		for scanner.Scan() {
			line++
			s := scanner.Text()
			if strings.TrimSpace(s) == "" {
				flush()
				continue
			}
			if len(block) == 0 {
				start = line
			}
			if strings.HasPrefix(s, "#") {
				flush()
				heading = s
				start = line
			}
			block = append(block, s)
		}
		flush()
		return scanner.Err()
	})
	if err != nil {
		return inv, err
	}
	sort.Slice(inv.Blocks, func(i, j int) bool {
		if inv.Blocks[i].File == inv.Blocks[j].File {
			return inv.Blocks[i].Line < inv.Blocks[j].Line
		}
		return inv.Blocks[i].File < inv.Blocks[j].File
	})
	sort.Strings(inv.Unknown)
	sort.Strings(inv.Extract)
	if len(inv.Blocks) == 0 {
		return inv, fmt.Errorf("no index blocks under %s", dir)
	}
	if len(inv.Unknown) > 0 {
		return inv, fmt.Errorf("unknown research file roles: %s", strings.Join(inv.Unknown, ", "))
	}
	if err := os.MkdirAll(statePath(dir, ""), 0o755); err != nil {
		return inv, err
	}
	claims, err := ReadClaims(dir)
	if err != nil && !os.IsNotExist(err) {
		return inv, err
	}
	current := map[string]string{}
	for _, b := range inv.Blocks {
		current[b.ID] = b.Digest
	}
	kept := claims[:0]
	for _, c := range claims {
		occurrences := c.Occurrences[:0]
		for _, o := range c.Occurrences {
			if previousDigest, known := old[o.BlockID]; known && previousDigest != current[o.BlockID] {
				// A previously inventoried block changed or disappeared. Unknown
				// anchors stay visible so validation can request curator repair.
				continue
			}
			occurrences = append(occurrences, o)
		}
		if len(occurrences) > 0 || len(c.Occurrences) == 0 {
			c.Occurrences = occurrences
			kept = append(kept, c)
		}
	}
	if err := writeClaims(dir, kept); err != nil {
		return inv, err
	}
	inv.Revision = revision(inv, kept)
	if err := writeJSON(statePath(dir, "inventory.json"), inv); err != nil {
		return inv, err
	}
	return inv, nil
}

func revision(inv Inventory, claims []Claim) string {
	b, _ := json.Marshal(struct {
		Blocks  []Block
		Files   map[string]string
		Sources map[string]string
		Claims  []Claim
	}{inv.Blocks, inv.Files, inv.Sources, claims})
	return digest(b)
}

func ReadClaims(dir string) ([]Claim, error) {
	f, err := os.Open(statePath(dir, "claims.jsonl"))
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	s := bufio.NewScanner(f)
	s.Buffer(make([]byte, 64*1024), 10<<20)
	var out []Claim
	for s.Scan() {
		if strings.TrimSpace(s.Text()) == "" {
			continue
		}
		var c Claim
		if err := json.Unmarshal(s.Bytes(), &c); err != nil {
			return nil, &ClaimsError{Err: fmt.Errorf("claims.jsonl line %d: %w", len(out)+1, err)}
		}
		out = append(out, c)
	}
	return out, s.Err()
}
func writeClaims(dir string, claims []Claim) error {
	var b strings.Builder
	for _, c := range claims {
		line, err := json.Marshal(c)
		if err != nil {
			return err
		}
		b.Write(line)
		b.WriteByte('\n')
	}
	return os.WriteFile(statePath(dir, "claims.jsonl"), []byte(b.String()), 0o644)
}
func writeJSON(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
