package resources

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/tylergannon/gimbal/internal/pi/config"
)

// ProjectTrustStoreEntry is one trust decision and the path it applies to.
type ProjectTrustStoreEntry struct {
	Path     string
	Decision bool
}

// ProjectTrustUpdate is one write to the trust store. A nil Decision removes
// the entry.
type ProjectTrustUpdate struct {
	Path     string
	Decision *bool
}

// ProjectTrustOption is one answer to the trust prompt.
type ProjectTrustOption struct {
	Label        string
	Trusted      bool
	Updates      []ProjectTrustUpdate
	SavedPath    string
	HasSavedPath bool
}

// trustRequiringProjectConfigResources are the project .pi entries that make
// project resources trust-requiring.
var trustRequiringProjectConfigResources = []string{
	"settings.json",
	"extensions",
	"skills",
	"prompts",
	"themes",
	"SYSTEM.md",
	"APPEND_SYSTEM.md",
}

// ProjectTrustStore persists project trust decisions in agentDir/trust.json.
type ProjectTrustStore struct {
	trustPath string
}

// NewProjectTrustStore returns a store rooted at agentDir.
func NewProjectTrustStore(agentDir string) *ProjectTrustStore {
	return &ProjectTrustStore{trustPath: filepath.Join(resolvePath(agentDir, "."), "trust.json")}
}

// Get returns the nearest decision for cwd, or nil when none is recorded.
func (s *ProjectTrustStore) Get(cwd string) *bool {
	entry, ok := s.GetEntry(cwd)
	if !ok {
		return nil
	}
	decision := entry.Decision
	return &decision
}

// GetEntry returns the nearest trust entry to cwd.
func (s *ProjectTrustStore) GetEntry(cwd string) (ProjectTrustStoreEntry, bool) {
	var entry ProjectTrustStoreEntry
	found := false
	_ = s.withLock(func() error {
		data, err := readTrustFile(s.trustPath)
		if err != nil {
			return err
		}
		entry, found = findNearestTrustEntry(data, cwd)
		return nil
	})
	return entry, found
}

// Set records one decision for cwd.
func (s *ProjectTrustStore) Set(cwd string, decision *bool) {
	s.SetMany([]ProjectTrustUpdate{{Path: cwd, Decision: decision}})
}

// SetMany records several decisions in one transaction.
func (s *ProjectTrustStore) SetMany(updates []ProjectTrustUpdate) {
	_ = s.withLock(func() error {
		data, err := readTrustFile(s.trustPath)
		if err != nil {
			return err
		}
		for _, update := range updates {
			key := normalizeCwd(update.Path)
			if update.Decision == nil {
				delete(data, key)
			} else {
				data[key] = update.Decision
			}
		}
		return writeTrustFile(s.trustPath, data)
	})
}

// withLock runs fn while holding the trust store lock.
func (s *ProjectTrustStore) withLock(fn func() error) error {
	if err := os.MkdirAll(filepath.Dir(s.trustPath), 0o755); err != nil {
		return err
	}
	lockPath := s.trustPath + ".lock"
	deadline := time.Now().Add(10 * time.Second)
	for {
		file, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
		if err == nil {
			_ = file.Close()
			defer func() { _ = os.Remove(lockPath) }()
			return fn()
		}
		if !os.IsExist(err) {
			return err
		}
		if time.Now().After(deadline) {
			return errTrustLockTimeout
		}
		time.Sleep(20 * time.Millisecond)
	}
}

var errTrustLockTimeout = &trustLockError{}

type trustLockError struct{}

func (*trustLockError) Error() string { return "timed out acquiring trust store lock" }

func normalizeCwd(cwd string) string {
	return canonicalizePath(resolvePath(cwd, "."))
}

// findNearestTrustEntry walks up from cwd until it finds a decision.
func findNearestTrustEntry(data map[string]*bool, cwd string) (ProjectTrustStoreEntry, bool) {
	currentDir := normalizeCwd(cwd)
	for {
		if decision, ok := data[currentDir]; ok && decision != nil {
			return ProjectTrustStoreEntry{Path: currentDir, Decision: *decision}, true
		}
		parentDir := filepath.Dir(currentDir)
		if parentDir == currentDir {
			return ProjectTrustStoreEntry{}, false
		}
		currentDir = parentDir
	}
}

// readTrustFile reads and validates the trust store.
func readTrustFile(path string) (map[string]*bool, error) {
	data := map[string]*bool{}
	content, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return data, nil
		}
		return nil, err
	}
	if len(content) == 0 {
		return data, nil
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(stripBOM(string(content))), &parsed); err != nil {
		return nil, err
	}
	for key, value := range parsed {
		switch typed := value.(type) {
		case bool:
			decision := typed
			data[key] = &decision
		case nil:
			data[key] = nil
		default:
			return nil, &trustFileError{message: "Invalid trust store " + path + ": value for " + key + " must be true, false, or null"}
		}
	}
	return data, nil
}

// writeTrustFile writes the sorted trust store.
func writeTrustFile(path string, data map[string]*bool) error {
	encoded, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	encoded = append(encoded, '\n')
	return os.WriteFile(path, encoded, 0o644)
}

type trustFileError struct{ message string }

func (e *trustFileError) Error() string { return e.message }

// GetProjectTrustParentPath returns the parent of cwd's trust path.
func GetProjectTrustParentPath(cwd string) (string, bool) {
	trustPath := normalizeCwd(cwd)
	parentDir := filepath.Dir(trustPath)
	if parentDir == trustPath {
		return "", false
	}
	return parentDir, true
}

// GetProjectTrustOptions returns the trust prompt choices for cwd.
func GetProjectTrustOptions(cwd string, includeSessionOnly bool) []ProjectTrustOption {
	trustPath := normalizeCwd(cwd)
	yes, no := true, false
	trustOptions := []ProjectTrustOption{
		{
			Label:        "Trust",
			Trusted:      true,
			Updates:      []ProjectTrustUpdate{{Path: trustPath, Decision: &yes}},
			SavedPath:    trustPath,
			HasSavedPath: true,
		},
	}
	if parentPath, ok := GetProjectTrustParentPath(cwd); ok {
		trustOptions = append(trustOptions, ProjectTrustOption{
			Label:   "Trust parent folder (" + parentPath + ")",
			Trusted: true,
			Updates: []ProjectTrustUpdate{
				{Path: parentPath, Decision: &yes},
				{Path: trustPath, Decision: nil},
			},
			SavedPath:    parentPath,
			HasSavedPath: true,
		})
	}
	if includeSessionOnly {
		trustOptions = append(trustOptions, ProjectTrustOption{Label: "Trust (this session only)", Trusted: true})
	}
	trustOptions = append(trustOptions, ProjectTrustOption{
		Label:        "Do not trust",
		Trusted:      false,
		Updates:      []ProjectTrustUpdate{{Path: trustPath, Decision: &no}},
		SavedPath:    trustPath,
		HasSavedPath: true,
	})
	if includeSessionOnly {
		trustOptions = append(trustOptions, ProjectTrustOption{Label: "Do not trust (this session only)", Trusted: false})
	}
	return trustOptions
}

// HasTrustRequiringProjectResources reports whether cwd has project-local
// resources that must be gated by project trust: trust-requiring entries under
// cwd/.pi, or .agents/skills in cwd or an ancestor. The user/global
// ~/.agents/skills directory is ignored even when cwd is $HOME.
func HasTrustRequiringProjectResources(cwd string) bool {
	homeDir := canonicalizePath(resolvePath(config.HomeDir(), "."))
	userAgentsSkillsDir := filepath.Join(homeDir, ".agents", "skills")
	currentDir := canonicalizePath(resolvePath(cwd, "."))

	configDir := filepath.Join(currentDir, config.ConfigDirName)
	for _, entry := range trustRequiringProjectConfigResources {
		if pathExists(filepath.Join(configDir, entry)) {
			return true
		}
	}

	for {
		agentsSkillsDir := filepath.Join(currentDir, ".agents", "skills")
		if agentsSkillsDir != userAgentsSkillsDir && pathExists(agentsSkillsDir) {
			return true
		}
		parentDir := filepath.Dir(currentDir)
		if parentDir == currentDir {
			return false
		}
		currentDir = parentDir
	}
}
