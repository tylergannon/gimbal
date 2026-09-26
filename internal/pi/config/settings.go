package config

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/tylergannon/gimbal/internal/pi/model"
)

// Settings is one scope's settings object. It is deliberately a plain map so
// that unknown keys written by a newer pi survive an unrelated update.
type Settings map[string]any

// SettingsScope identifies which settings file a value belongs to.
type SettingsScope string

const (
	// SettingsScopeGlobal is the per-user settings file.
	SettingsScopeGlobal SettingsScope = "global"
	// SettingsScopeProject is the per-project settings file.
	SettingsScopeProject SettingsScope = "project"
)

// SettingsError records a failed settings load or write.
type SettingsError struct {
	Scope SettingsScope
	Path  string
	Err   error
}

func (e SettingsError) Error() string {
	if e.Path != "" {
		return fmt.Sprintf("invalid %s settings %s: %v", e.Scope, e.Path, e.Err)
	}
	return fmt.Sprintf("invalid %s settings: %v", e.Scope, e.Err)
}

// SettingsStorage is the read-modify-write backend behind a SettingsManager.
type SettingsStorage interface {
	WithLock(scope SettingsScope, fn func(current []byte) ([]byte, error)) error
}

// FileSettingsStorage reads and writes the global and project settings files.
type FileSettingsStorage struct {
	GlobalPath  string
	ProjectPath string
}

// NewFileSettingsStorage returns file storage for a project cwd and agent dir.
func NewFileSettingsStorage(cwd, agentDir string) *FileSettingsStorage {
	resolvedCwd := ResolvePath(cwd, ".")
	resolvedAgentDir := ResolvePath(agentDir, ".")
	return &FileSettingsStorage{
		GlobalPath:  filepath.Join(resolvedAgentDir, "settings.json"),
		ProjectPath: filepath.Join(resolvedCwd, ConfigDirName, "settings.json"),
	}
}

func (s *FileSettingsStorage) path(scope SettingsScope) string {
	if scope == SettingsScopeProject {
		return s.ProjectPath
	}
	return s.GlobalPath
}

// WithLock runs fn against the current file contents and writes the returned
// bytes when they are non-nil. Reading never creates the file's directory.
func (s *FileSettingsStorage) WithLock(scope SettingsScope, fn func(current []byte) ([]byte, error)) error {
	path := s.path(scope)
	_, statErr := os.Stat(path)
	if statErr != nil {
		next, err := fn(nil)
		if err != nil {
			return err
		}
		if next == nil {
			return nil
		}
		return withFileLock(settingsLockContext(), path, func() error {
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				return err
			}
			return os.WriteFile(path, next, 0o644)
		})
	}
	return withFileLock(settingsLockContext(), path, func() error {
		current, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		next, err := fn(current)
		if err != nil {
			return err
		}
		if next == nil {
			return nil
		}
		return os.WriteFile(path, next, 0o644)
	})
}

// InMemorySettingsStorage is a storage backend with no file I/O.
type InMemorySettingsStorage struct {
	Global  []byte
	Project []byte
}

// WithLock runs fn against the in-memory contents and stores the result.
func (s *InMemorySettingsStorage) WithLock(scope SettingsScope, fn func(current []byte) ([]byte, error)) error {
	current := s.Global
	if scope == SettingsScopeProject {
		current = s.Project
	}
	next, err := fn(current)
	if err != nil {
		return err
	}
	if next == nil {
		return nil
	}
	if scope == SettingsScopeProject {
		s.Project = next
	} else {
		s.Global = next
	}
	return nil
}

// CreateOptions are the options accepted when creating a SettingsManager.
type CreateOptions struct {
	// ProjectTrusted controls whether project settings are read and written.
	// It defaults to true.
	ProjectTrusted *bool
}

// SettingsManager loads, merges and persists global and project settings.
type SettingsManager struct {
	storage         SettingsStorage
	globalSettings  Settings
	projectSettings Settings
	settings        Settings
	projectTrusted  bool

	modifiedFields              map[string]bool
	modifiedNestedFields        map[string]map[string]bool
	modifiedProjectFields       map[string]bool
	modifiedProjectNestedFields map[string]map[string]bool

	globalSettingsLoadError  error
	projectSettingsLoadError error

	errors        []SettingsError
	settingsPaths map[SettingsScope]string
}

// NewSettingsManager loads settings from cwd/.pi/settings.json and
// agentDir/settings.json. An empty agentDir uses GetAgentDir.
func NewSettingsManager(cwd, agentDir string, options CreateOptions) *SettingsManager {
	if agentDir == "" {
		agentDir = GetAgentDir()
	}
	resolvedCwd := ResolvePath(cwd, ".")
	resolvedAgentDir := ResolvePath(agentDir, ".")
	storage := NewFileSettingsStorage(resolvedCwd, resolvedAgentDir)
	return newSettingsManagerFromStorage(storage, options, map[SettingsScope]string{
		SettingsScopeGlobal:  storage.GlobalPath,
		SettingsScopeProject: storage.ProjectPath,
	})
}

// NewInMemorySettingsManager returns a manager backed by no files.
func NewInMemorySettingsManager(settings Settings, options CreateOptions) *SettingsManager {
	storage := &InMemorySettingsStorage{}
	initial := migrateSettings(normalizeSettings(settings))
	data, _ := encodeSettings(initial)
	_ = storage.WithLock(SettingsScopeGlobal, func([]byte) ([]byte, error) { return data, nil })
	return newSettingsManagerFromStorage(storage, options, nil)
}

// NewSettingsManagerFromStorage builds a manager over an arbitrary storage.
func NewSettingsManagerFromStorage(storage SettingsStorage, options CreateOptions) *SettingsManager {
	return newSettingsManagerFromStorage(storage, options, nil)
}

func newSettingsManagerFromStorage(storage SettingsStorage, options CreateOptions, paths map[SettingsScope]string) *SettingsManager {
	projectTrusted := true
	if options.ProjectTrusted != nil {
		projectTrusted = *options.ProjectTrusted
	}
	globalSettings, globalErr := tryLoadSettings(storage, SettingsScopeGlobal, projectTrusted)
	projectSettings, projectErr := tryLoadSettings(storage, SettingsScopeProject, projectTrusted)
	manager := &SettingsManager{
		storage:                     storage,
		globalSettings:              globalSettings,
		projectSettings:             projectSettings,
		projectTrusted:              projectTrusted,
		modifiedFields:              map[string]bool{},
		modifiedNestedFields:        map[string]map[string]bool{},
		modifiedProjectFields:       map[string]bool{},
		modifiedProjectNestedFields: map[string]map[string]bool{},
		globalSettingsLoadError:     globalErr,
		projectSettingsLoadError:    projectErr,
		settingsPaths:               paths,
	}
	if globalErr != nil {
		manager.recordError(SettingsScopeGlobal, globalErr)
	}
	if projectErr != nil {
		manager.recordError(SettingsScopeProject, projectErr)
	}
	manager.settings = deepMergeSettings(globalSettings, projectSettings)
	return manager
}

func tryLoadSettings(storage SettingsStorage, scope SettingsScope, projectTrusted bool) (Settings, error) {
	settings, err := loadSettings(storage, scope, projectTrusted)
	if err != nil {
		return Settings{}, err
	}
	return settings, nil
}

func loadSettings(storage SettingsStorage, scope SettingsScope, projectTrusted bool) (Settings, error) {
	if scope == SettingsScopeProject && !projectTrusted {
		return Settings{}, nil
	}
	var content []byte
	if err := storage.WithLock(scope, func(current []byte) ([]byte, error) {
		content = append([]byte(nil), current...)
		return nil, nil
	}); err != nil {
		return Settings{}, err
	}
	if len(bytes.TrimSpace(content)) == 0 {
		return Settings{}, nil
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(StripBOM(string(content))), &parsed); err != nil {
		return Settings{}, err
	}
	return migrateSettings(Settings(parsed)), nil
}

func normalizeSettings(settings Settings) Settings {
	if settings == nil {
		return Settings{}
	}
	data, err := json.Marshal(settings)
	if err != nil {
		out := Settings{}
		maps.Copy(out, settings)
		return out
	}
	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		out = map[string]any{}
	}
	return Settings(out)
}

func migrateSettings(settings Settings) Settings {
	if settings == nil {
		settings = Settings{}
	}
	if _, hasSteering := settings["steeringMode"]; !hasSteering {
		if queueMode, ok := settings["queueMode"]; ok {
			settings["steeringMode"] = queueMode
			delete(settings, "queueMode")
		}
	}
	if _, hasTransport := settings["transport"]; !hasTransport {
		if websockets, ok := settings["websockets"].(bool); ok {
			if websockets {
				settings["transport"] = string(model.TransportWebSocket)
			} else {
				settings["transport"] = string(model.TransportSSE)
			}
			delete(settings, "websockets")
		}
	}
	if skills, ok := settings["skills"].(map[string]any); ok {
		if enable, present := skills["enableSkillCommands"]; present {
			if _, exists := settings["enableSkillCommands"]; !exists {
				settings["enableSkillCommands"] = enable
			}
		}
		if dirs, ok := skills["customDirectories"].([]any); ok && len(dirs) > 0 {
			settings["skills"] = dirs
		} else {
			delete(settings, "skills")
		}
	}
	if retry, ok := settings["retry"].(map[string]any); ok {
		if maxDelay, present := retry["maxDelayMs"]; present {
			provider, _ := retry["provider"].(map[string]any)
			if provider == nil {
				provider = map[string]any{}
			}
			if _, exists := provider["maxRetryDelayMs"]; !exists {
				provider["maxRetryDelayMs"] = maxDelay
			}
			retry["provider"] = provider
			delete(retry, "maxDelayMs")
		}
	}
	return settings
}

func encodeSettings(settings Settings) ([]byte, error) {
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(settings); err != nil {
		return nil, err
	}
	return bytes.TrimRight(buffer.Bytes(), "\n"), nil
}

func deepMergeSettings(base, overrides Settings) Settings {
	return Settings(deepMergeObjects(map[string]any(base), map[string]any(overrides)))
}

func deepMergeObjects(base, overrides map[string]any) map[string]any {
	result := make(map[string]any, len(base)+len(overrides))
	maps.Copy(result, base)
	for key, overrideValue := range overrides {
		if overrideValue == nil {
			continue
		}
		baseValue, baseIsObject := result[key].(map[string]any)
		overrideObject, overrideIsObject := overrideValue.(map[string]any)
		if baseIsObject && overrideIsObject {
			result[key] = deepMergeObjects(baseValue, overrideObject)
			continue
		}
		result[key] = overrideValue
	}
	return result
}

func cloneSettings(settings Settings) Settings {
	clone := make(Settings, len(settings))
	maps.Copy(clone, settings)
	return clone
}

// GetGlobalSettings returns a copy of the global settings.
func (m *SettingsManager) GetGlobalSettings() Settings { return cloneSettings(m.globalSettings) }

// GetProjectSettings returns a copy of the project settings.
func (m *SettingsManager) GetProjectSettings() Settings { return cloneSettings(m.projectSettings) }

// IsProjectTrusted reports whether project settings may be read and written.
func (m *SettingsManager) IsProjectTrusted() bool { return m.projectTrusted }

// SetProjectTrusted changes project trust, reloading or dropping project
// settings as appropriate.
func (m *SettingsManager) SetProjectTrusted(trusted bool) {
	if m.projectTrusted == trusted {
		return
	}
	m.projectTrusted = trusted
	m.modifiedProjectFields = map[string]bool{}
	m.modifiedProjectNestedFields = map[string]map[string]bool{}

	if !trusted {
		m.projectSettings = Settings{}
		m.projectSettingsLoadError = nil
		m.settings = deepMergeSettings(m.globalSettings, m.projectSettings)
		return
	}
	projectSettings, err := tryLoadSettings(m.storage, SettingsScopeProject, trusted)
	m.projectSettings = projectSettings
	m.projectSettingsLoadError = err
	if err != nil {
		m.recordError(SettingsScopeProject, err)
	}
	m.settings = deepMergeSettings(m.globalSettings, m.projectSettings)
}

// Reload rereads both settings files and clears session modifications.
func (m *SettingsManager) Reload() {
	globalSettings, globalErr := tryLoadSettings(m.storage, SettingsScopeGlobal, m.projectTrusted)
	if globalErr == nil {
		m.globalSettings = globalSettings
		m.globalSettingsLoadError = nil
	} else {
		m.globalSettingsLoadError = globalErr
		m.recordError(SettingsScopeGlobal, globalErr)
	}

	m.modifiedFields = map[string]bool{}
	m.modifiedNestedFields = map[string]map[string]bool{}
	m.modifiedProjectFields = map[string]bool{}
	m.modifiedProjectNestedFields = map[string]map[string]bool{}

	projectSettings, projectErr := tryLoadSettings(m.storage, SettingsScopeProject, m.projectTrusted)
	if projectErr == nil {
		m.projectSettings = projectSettings
		m.projectSettingsLoadError = nil
	} else {
		m.projectSettingsLoadError = projectErr
		m.recordError(SettingsScopeProject, projectErr)
	}
	m.settings = deepMergeSettings(m.globalSettings, m.projectSettings)
}

// ApplyOverrides applies additional in-memory overrides on top of the merged
// settings. They are not persisted and not tracked as modified.
func (m *SettingsManager) ApplyOverrides(overrides Settings) {
	m.settings = deepMergeSettings(m.settings, overrides)
}

// Flush waits for pending writes. Writes are synchronous, so this is a no-op.
func (m *SettingsManager) Flush() {}

// DrainErrors returns and clears the accumulated settings errors.
func (m *SettingsManager) DrainErrors() []SettingsError {
	drained := append([]SettingsError(nil), m.errors...)
	m.errors = nil
	return drained
}

func (m *SettingsManager) markModified(field string, nested ...string) {
	m.modifiedFields[field] = true
	if len(nested) > 0 {
		if m.modifiedNestedFields[field] == nil {
			m.modifiedNestedFields[field] = map[string]bool{}
		}
		for _, key := range nested {
			m.modifiedNestedFields[field][key] = true
		}
	}
}

func (m *SettingsManager) markProjectModified(field string, nested ...string) {
	m.modifiedProjectFields[field] = true
	if len(nested) > 0 {
		if m.modifiedProjectNestedFields[field] == nil {
			m.modifiedProjectNestedFields[field] = map[string]bool{}
		}
		for _, key := range nested {
			m.modifiedProjectNestedFields[field][key] = true
		}
	}
}

func (m *SettingsManager) assertProjectTrustedForWrite() error {
	if !m.projectTrusted {
		return fmt.Errorf("project is not trusted; refusing to write project settings")
	}
	return nil
}

func (m *SettingsManager) recordError(scope SettingsScope, err error) {
	path := ""
	if m.settingsPaths != nil {
		path = m.settingsPaths[scope]
	}
	m.errors = append(m.errors, SettingsError{Scope: scope, Path: path, Err: err})
}

func (m *SettingsManager) clearModifiedScope(scope SettingsScope) {
	if scope == SettingsScopeGlobal {
		m.modifiedFields = map[string]bool{}
		m.modifiedNestedFields = map[string]map[string]bool{}
		return
	}
	m.modifiedProjectFields = map[string]bool{}
	m.modifiedProjectNestedFields = map[string]map[string]bool{}
}

func cloneNestedFields(source map[string]map[string]bool) map[string]map[string]bool {
	clone := make(map[string]map[string]bool, len(source))
	for key, value := range source {
		nested := make(map[string]bool, len(value))
		for nestedKey := range value {
			nested[nestedKey] = true
		}
		clone[key] = nested
	}
	return clone
}

func (m *SettingsManager) persistScopedSettings(scope SettingsScope, snapshot Settings, modifiedFields map[string]bool, modifiedNestedFields map[string]map[string]bool) error {
	return m.storage.WithLock(scope, func(current []byte) ([]byte, error) {
		currentFile := Settings{}
		if len(bytes.TrimSpace(current)) > 0 {
			var parsed map[string]any
			if err := json.Unmarshal([]byte(StripBOM(string(current))), &parsed); err != nil {
				return nil, err
			}
			currentFile = migrateSettings(Settings(parsed))
		}
		merged := cloneSettings(currentFile)
		for field := range modifiedFields {
			value, present := snapshot[field]
			if !present {
				continue
			}
			nested, hasNested := modifiedNestedFields[field]
			if !hasNested {
				merged[field] = value
				continue
			}
			baseNested, _ := currentFile[field].(map[string]any)
			inMemoryNested, _ := value.(map[string]any)
			mergedNested := cloneMap(baseNested)
			for nestedKey := range nested {
				if inMemoryNested == nil {
					delete(mergedNested, nestedKey)
					continue
				}
				mergedNested[nestedKey] = inMemoryNested[nestedKey]
			}
			merged[field] = mergedNested
		}
		return encodeSettings(merged)
	})
}

func cloneMap(source map[string]any) map[string]any {
	clone := make(map[string]any, len(source))
	maps.Copy(clone, source)
	return clone
}

func (m *SettingsManager) save() {
	m.settings = deepMergeSettings(m.globalSettings, m.projectSettings)
	if m.globalSettingsLoadError != nil {
		return
	}
	snapshot := cloneSettings(m.globalSettings)
	modifiedFields := map[string]bool{}
	for field := range m.modifiedFields {
		modifiedFields[field] = true
	}
	modifiedNested := cloneNestedFields(m.modifiedNestedFields)
	if err := m.persistScopedSettings(SettingsScopeGlobal, snapshot, modifiedFields, modifiedNested); err != nil {
		m.recordError(SettingsScopeGlobal, err)
	}
	m.clearModifiedScope(SettingsScopeGlobal)
}

func (m *SettingsManager) saveProjectSettings(settings Settings) {
	if err := m.assertProjectTrustedForWrite(); err != nil {
		m.recordError(SettingsScopeProject, err)
		return
	}
	m.projectSettings = cloneSettings(settings)
	m.settings = deepMergeSettings(m.globalSettings, m.projectSettings)
	if m.projectSettingsLoadError != nil {
		return
	}
	snapshot := cloneSettings(m.projectSettings)
	modifiedFields := map[string]bool{}
	for field := range m.modifiedProjectFields {
		modifiedFields[field] = true
	}
	modifiedNested := cloneNestedFields(m.modifiedProjectNestedFields)
	if err := m.persistScopedSettings(SettingsScopeProject, snapshot, modifiedFields, modifiedNested); err != nil {
		m.recordError(SettingsScopeProject, err)
	}
	m.clearModifiedScope(SettingsScopeProject)
}

func (m *SettingsManager) updateProjectSettings(field string, update func(Settings)) error {
	if err := m.assertProjectTrustedForWrite(); err != nil {
		return err
	}
	projectSettings := cloneSettings(m.projectSettings)
	update(projectSettings)
	m.markProjectModified(field)
	m.saveProjectSettings(projectSettings)
	return nil
}

func (m *SettingsManager) setGlobal(field string, value any) {
	if value == nil {
		delete(m.globalSettings, field)
	} else {
		m.globalSettings[field] = value
	}
	m.markModified(field)
	m.save()
}

func (m *SettingsManager) setGlobalNested(field, nested string, value any) {
	object, _ := m.globalSettings[field].(map[string]any)
	if object == nil {
		object = map[string]any{}
	}
	if value == nil {
		delete(object, nested)
	} else {
		object[nested] = value
	}
	m.globalSettings[field] = object
	m.markModified(field, nested)
	m.save()
}

func (m *SettingsManager) field(field string) (any, bool) {
	value, ok := m.settings[field]
	return value, ok && value != nil
}

func (m *SettingsManager) stringField(field string) (string, bool) {
	value, ok := m.field(field)
	if !ok {
		return "", false
	}
	str, ok := value.(string)
	return str, ok
}

func (m *SettingsManager) boolField(field string, fallback bool) bool {
	value, ok := m.field(field)
	if !ok {
		return fallback
	}
	b, ok := value.(bool)
	if !ok {
		return fallback
	}
	return b
}

func intFieldValue(value any) (int, bool) {
	switch typed := value.(type) {
	case float64:
		return int(typed), typed == float64(int(typed))
	case int:
		return typed, true
	case json.Number:
		number, err := typed.Int64()
		if err != nil {
			return 0, false
		}
		return int(number), true
	}
	return 0, false
}

func (m *SettingsManager) intField(field string, fallback int) int {
	value, ok := m.field(field)
	if !ok {
		return fallback
	}
	if number, ok := intFieldValue(value); ok {
		return number
	}
	return fallback
}

func (m *SettingsManager) stringSlice(field string) ([]string, bool) {
	value, ok := m.field(field)
	if !ok {
		return nil, false
	}
	switch typed := value.(type) {
	case []any:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			if str, ok := item.(string); ok {
				out = append(out, str)
			}
		}
		return out, true
	case []string:
		return append([]string(nil), typed...), true
	}
	return nil, false
}

// GetLastChangelogVersion returns the last seen changelog version.
func (m *SettingsManager) GetLastChangelogVersion() (string, bool) {
	return m.stringField("lastChangelogVersion")
}

// SetLastChangelogVersion records the last seen changelog version.
func (m *SettingsManager) SetLastChangelogVersion(version string) {
	m.setGlobal("lastChangelogVersion", version)
}

// GetSessionDir returns the configured session directory with ~ expanded.
func (m *SettingsManager) GetSessionDir() (string, bool) {
	sessionDir, ok := m.stringField("sessionDir")
	if !ok || sessionDir == "" {
		return sessionDir, ok
	}
	return NormalizePath(sessionDir), true
}

// GetDefaultProvider returns the configured default provider.
func (m *SettingsManager) GetDefaultProvider() (string, bool) {
	return m.stringField("defaultProvider")
}

// GetDefaultModel returns the configured default model id.
func (m *SettingsManager) GetDefaultModel() (string, bool) { return m.stringField("defaultModel") }

// SetDefaultProvider records the default provider.
func (m *SettingsManager) SetDefaultProvider(provider string) {
	m.setGlobal("defaultProvider", provider)
}

// SetDefaultModel records the default model id.
func (m *SettingsManager) SetDefaultModel(modelID string) { m.setGlobal("defaultModel", modelID) }

// SetDefaultModelAndProvider records both the default provider and model id.
func (m *SettingsManager) SetDefaultModelAndProvider(provider, modelID string) {
	m.globalSettings["defaultProvider"] = provider
	m.globalSettings["defaultModel"] = modelID
	m.markModified("defaultProvider")
	m.markModified("defaultModel")
	m.save()
}

// GetSteeringMode returns the steering queue mode, defaulting to one-at-a-time.
func (m *SettingsManager) GetSteeringMode() string {
	if value, ok := m.stringField("steeringMode"); ok {
		return value
	}
	return "one-at-a-time"
}

// SetSteeringMode records the steering queue mode.
func (m *SettingsManager) SetSteeringMode(mode string) { m.setGlobal("steeringMode", mode) }

// GetFollowUpMode returns the follow-up queue mode, defaulting to one-at-a-time.
func (m *SettingsManager) GetFollowUpMode() string {
	if value, ok := m.stringField("followUpMode"); ok {
		return value
	}
	return "one-at-a-time"
}

// SetFollowUpMode records the follow-up queue mode.
func (m *SettingsManager) SetFollowUpMode(mode string) { m.setGlobal("followUpMode", mode) }

// GetThemeSetting returns the raw theme setting, including auto light/dark IDs.
func (m *SettingsManager) GetThemeSetting() (string, bool) { return m.stringField("theme") }

// GetTheme returns the fixed theme name, or "" for auto themes with a slash.
func (m *SettingsManager) GetTheme() (string, bool) {
	theme, ok := m.stringField("theme")
	if !ok || strings.Contains(theme, "/") {
		return "", false
	}
	return theme, true
}

// SetTheme records the theme.
func (m *SettingsManager) SetTheme(theme string) { m.setGlobal("theme", theme) }

// GetDefaultThinkingLevel returns the configured default thinking level.
func (m *SettingsManager) GetDefaultThinkingLevel() (model.ThinkingLevel, bool) {
	value, ok := m.stringField("defaultThinkingLevel")
	if !ok {
		return "", false
	}
	return model.ThinkingLevel(value), true
}

// SetDefaultThinkingLevel records the default thinking level.
func (m *SettingsManager) SetDefaultThinkingLevel(level model.ThinkingLevel) {
	m.setGlobal("defaultThinkingLevel", string(level))
}

// GetModelThinkingLevel returns a per-model thinking-level override.
func (m *SettingsManager) GetModelThinkingLevel(provider, modelID string) (model.ThinkingLevel, bool) {
	levels, ok := m.settings["modelThinkingLevels"].(map[string]any)
	if !ok {
		return "", false
	}
	value, ok := levels[provider+"/"+modelID].(string)
	if !ok {
		return "", false
	}
	return model.ThinkingLevel(value), true
}

// GetAllModelThinkingLevels returns every per-model thinking-level override.
func (m *SettingsManager) GetAllModelThinkingLevels() map[string]model.ThinkingLevel {
	levels, ok := m.settings["modelThinkingLevels"].(map[string]any)
	if !ok {
		return map[string]model.ThinkingLevel{}
	}
	out := make(map[string]model.ThinkingLevel, len(levels))
	for key, value := range levels {
		if str, ok := value.(string); ok {
			out[key] = model.ThinkingLevel(str)
		}
	}
	return out
}

// SetModelThinkingLevel records a per-model thinking-level override.
func (m *SettingsManager) SetModelThinkingLevel(provider, modelID string, level model.ThinkingLevel) {
	levels, _ := m.globalSettings["modelThinkingLevels"].(map[string]any)
	if levels == nil {
		levels = map[string]any{}
	}
	levels[provider+"/"+modelID] = string(level)
	m.globalSettings["modelThinkingLevels"] = levels
	m.markModified("modelThinkingLevels")
	m.save()
}

// RemoveModelThinkingLevel removes a per-model thinking-level override.
func (m *SettingsManager) RemoveModelThinkingLevel(provider, modelID string) {
	levels, ok := m.globalSettings["modelThinkingLevels"].(map[string]any)
	if !ok {
		return
	}
	delete(levels, provider+"/"+modelID)
	if len(levels) == 0 {
		delete(m.globalSettings, "modelThinkingLevels")
	}
	m.markModified("modelThinkingLevels")
	m.save()
}

// GetTransport returns the preferred transport, defaulting to auto.
func (m *SettingsManager) GetTransport() model.Transport {
	if value, ok := m.stringField("transport"); ok {
		return model.Transport(value)
	}
	return model.TransportAuto
}

// SetTransport records the preferred transport.
func (m *SettingsManager) SetTransport(transport model.Transport) {
	m.setGlobal("transport", string(transport))
}

// GetCompactionEnabled reports whether auto-compaction is enabled.
func (m *SettingsManager) GetCompactionEnabled() bool {
	compaction, _ := m.settings["compaction"].(map[string]any)
	if compaction == nil {
		return true
	}
	if enabled, ok := compaction["enabled"].(bool); ok {
		return enabled
	}
	return true
}

// SetCompactionEnabled records whether auto-compaction is enabled.
func (m *SettingsManager) SetCompactionEnabled(enabled bool) {
	m.setGlobalNested("compaction", "enabled", enabled)
}

func (m *SettingsManager) compactionTokenSetting(field string, modelID ...string) (int, error) {
	compaction, _ := m.settings["compaction"].(map[string]any)
	var ordinary any
	if compaction != nil {
		ordinary = compaction[field]
	}
	if ordinary != nil {
		number, ok := intFieldValue(ordinary)
		if !ok || number < 0 {
			return 0, fmt.Errorf("invalid compaction.%s setting: %v. Expected a non-negative safe integer", field, ordinary)
		}
	}

	var override any
	if compaction != nil && len(modelID) > 0 && modelID[0] != "" {
		overrides, _ := compaction["modelOverrides"].(map[string]any)
		if overrides != nil {
			entry, _ := overrides[modelID[0]].(map[string]any)
			if entry != nil {
				override = entry[field]
			}
		}
	}
	if override != nil {
		number, ok := intFieldValue(override)
		if !ok || number < 0 {
			return 0, fmt.Errorf("invalid compaction.modelOverrides[%q].%s setting: %v. Expected a non-negative safe integer", modelID[0], field, override)
		}
		return number, nil
	}
	if ordinary != nil {
		number, _ := intFieldValue(ordinary)
		return number, nil
	}
	if field == "reserveTokens" {
		return 16384, nil
	}
	return 20000, nil
}

// GetCompactionReserveTokens returns the reserved-token budget.
func (m *SettingsManager) GetCompactionReserveTokens(modelID string) (int, error) {
	return m.compactionTokenSetting("reserveTokens", modelID)
}

// GetCompactionKeepRecentTokens returns the kept-recent-token budget.
func (m *SettingsManager) GetCompactionKeepRecentTokens(modelID string) (int, error) {
	return m.compactionTokenSetting("keepRecentTokens", modelID)
}

// CompactionSettingsValue is the resolved compaction configuration.
type CompactionSettingsValue struct {
	Enabled          bool
	ReserveTokens    int
	KeepRecentTokens int
}

// GetCompactionSettings resolves all three compaction values.
func (m *SettingsManager) GetCompactionSettings(modelID string) (CompactionSettingsValue, error) {
	reserve, err := m.GetCompactionReserveTokens(modelID)
	if err != nil {
		return CompactionSettingsValue{}, err
	}
	keepRecent, err := m.GetCompactionKeepRecentTokens(modelID)
	if err != nil {
		return CompactionSettingsValue{}, err
	}
	return CompactionSettingsValue{Enabled: m.GetCompactionEnabled(), ReserveTokens: reserve, KeepRecentTokens: keepRecent}, nil
}

// BranchSummarySettingsValue is the resolved branch-summary configuration.
type BranchSummarySettingsValue struct {
	ReserveTokens int
	SkipPrompt    bool
}

// GetBranchSummarySettings resolves the branch-summary configuration.
func (m *SettingsManager) GetBranchSummarySettings() BranchSummarySettingsValue {
	branch, _ := m.settings["branchSummary"].(map[string]any)
	reserve := 16384
	if branch != nil {
		if number, ok := intFieldValue(branch["reserveTokens"]); ok {
			reserve = number
		}
	}
	return BranchSummarySettingsValue{ReserveTokens: reserve, SkipPrompt: m.GetBranchSummarySkipPrompt()}
}

// GetBranchSummarySkipPrompt reports whether the branch-summary prompt is skipped.
func (m *SettingsManager) GetBranchSummarySkipPrompt() bool {
	branch, _ := m.settings["branchSummary"].(map[string]any)
	if branch == nil {
		return false
	}
	skip, _ := branch["skipPrompt"].(bool)
	return skip
}

// GetRetryEnabled reports whether agent retry is enabled.
func (m *SettingsManager) GetRetryEnabled() bool {
	retry, _ := m.settings["retry"].(map[string]any)
	if retry == nil {
		return true
	}
	enabled, ok := retry["enabled"].(bool)
	if !ok {
		return true
	}
	return enabled
}

// SetRetryEnabled records whether agent retry is enabled.
func (m *SettingsManager) SetRetryEnabled(enabled bool) {
	m.setGlobalNested("retry", "enabled", enabled)
}

// RetrySettingsValue is the resolved agent retry configuration.
type RetrySettingsValue struct {
	Enabled         bool
	MaxRetries      int
	BaseDelayMs     int
	MaxAgentDelayMs int
}

// GetRetrySettings resolves the agent retry configuration.
func (m *SettingsManager) GetRetrySettings() RetrySettingsValue {
	retry, _ := m.settings["retry"].(map[string]any)
	value := RetrySettingsValue{Enabled: m.GetRetryEnabled(), MaxRetries: 3, BaseDelayMs: 2000, MaxAgentDelayMs: 60000}
	if retry == nil {
		return value
	}
	if number, ok := intFieldValue(retry["maxRetries"]); ok {
		value.MaxRetries = number
	}
	if number, ok := intFieldValue(retry["baseDelayMs"]); ok {
		value.BaseDelayMs = number
	}
	if number, ok := intFieldValue(retry["maxAgentDelayMs"]); ok {
		value.MaxAgentDelayMs = number
	}
	return value
}

// ProviderRetrySettingsValue is the resolved provider retry configuration.
type ProviderRetrySettingsValue struct {
	TimeoutMs       *int
	MaxRetries      *int
	MaxRetryDelayMs int
}

// GetProviderRetrySettings resolves the provider retry configuration.
func (m *SettingsManager) GetProviderRetrySettings() ProviderRetrySettingsValue {
	value := ProviderRetrySettingsValue{MaxRetryDelayMs: 60000}
	retry, _ := m.settings["retry"].(map[string]any)
	if retry == nil {
		return value
	}
	provider, _ := retry["provider"].(map[string]any)
	if provider == nil {
		return value
	}
	if number, ok := intFieldValue(provider["timeoutMs"]); ok {
		value.TimeoutMs = &number
	}
	if number, ok := intFieldValue(provider["maxRetries"]); ok {
		value.MaxRetries = &number
	}
	if number, ok := intFieldValue(provider["maxRetryDelayMs"]); ok {
		value.MaxRetryDelayMs = number
	}
	return value
}

// GetHTTPIdleTimeoutMs resolves the HTTP idle timeout, defaulting to five
// minutes.
func (m *SettingsManager) GetHTTPIdleTimeoutMs() (int, error) {
	value, present := m.field("httpIdleTimeoutMs")
	if !present {
		return DefaultHTTPIdleTimeoutMs, nil
	}
	timeout, ok := ParseHTTPIdleTimeoutMs(value)
	if !ok {
		return 0, fmt.Errorf("invalid httpIdleTimeoutMs setting: %v", value)
	}
	return timeout, nil
}

// SetHTTPIdleTimeoutMs records the HTTP idle timeout.
func (m *SettingsManager) SetHTTPIdleTimeoutMs(timeoutMs int) error {
	if timeoutMs < 0 {
		return fmt.Errorf("invalid httpIdleTimeoutMs setting: %d", timeoutMs)
	}
	m.setGlobal("httpIdleTimeoutMs", timeoutMs)
	return nil
}

// CacheWarmingMode is pi's cache-warming profile.
type CacheWarmingMode string

const (
	CacheWarmingOff       CacheWarmingMode = "off"
	CacheWarmingStreaming CacheWarmingMode = "streaming"
	CacheWarmingIdle      CacheWarmingMode = "idle"
)

// GetCacheWarmingMode returns the globally configured cache-warming mode.
func (m *SettingsManager) GetCacheWarmingMode() CacheWarmingMode {
	mode, ok := m.globalSettings["cacheWarming"].(string)
	if !ok {
		return CacheWarmingStreaming
	}
	switch CacheWarmingMode(mode) {
	case CacheWarmingOff, CacheWarmingStreaming, CacheWarmingIdle:
		return CacheWarmingMode(mode)
	}
	return CacheWarmingStreaming
}

// SetCacheWarmingMode records the cache-warming mode.
func (m *SettingsManager) SetCacheWarmingMode(mode CacheWarmingMode) {
	m.setGlobal("cacheWarming", string(mode))
}

// GetWebSocketConnectTimeoutMs returns the WebSocket connect timeout.
func (m *SettingsManager) GetWebSocketConnectTimeoutMs() (int, bool, error) {
	value, present := m.field("websocketConnectTimeoutMs")
	if !present {
		return 0, false, nil
	}
	timeout, ok := ParseHTTPIdleTimeoutMs(value)
	if !ok {
		return 0, false, fmt.Errorf("invalid websocketConnectTimeoutMs setting: %v", value)
	}
	return timeout, true, nil
}

// GetHideThinkingBlock reports whether the thinking block is hidden.
func (m *SettingsManager) GetHideThinkingBlock() bool {
	return m.boolField("hideThinkingBlock", false)
}

// SetHideThinkingBlock records whether the thinking block is hidden.
func (m *SettingsManager) SetHideThinkingBlock(hide bool) { m.setGlobal("hideThinkingBlock", hide) }

// GetShowCacheMissNotices reports whether cache-miss notices are shown.
func (m *SettingsManager) GetShowCacheMissNotices() bool {
	return m.boolField("showCacheMissNotices", false)
}

// SetShowCacheMissNotices records whether cache-miss notices are shown.
func (m *SettingsManager) SetShowCacheMissNotices(show bool) {
	m.setGlobal("showCacheMissNotices", show)
}

// GetExternalEditorCommand returns the configured external editor, falling
// back to VISUAL/EDITOR and then the platform default.
func (m *SettingsManager) GetExternalEditorCommand() string {
	if editor, ok := m.stringField("externalEditor"); ok && strings.TrimSpace(editor) != "" {
		return editor
	}
	if editor := os.Getenv("VISUAL"); editor != "" {
		return editor
	}
	if editor := os.Getenv("EDITOR"); editor != "" {
		return editor
	}
	return externalEditorDefault(runtime.GOOS)
}

func externalEditorDefault(goos string) string {
	if goos == "windows" {
		return "notepad"
	}
	return "nano"
}

// GetShellPath returns the configured shell path with ~ expanded.
func (m *SettingsManager) GetShellPath() (string, bool) {
	shellPath, ok := m.stringField("shellPath")
	if !ok || shellPath == "" {
		return shellPath, ok
	}
	return NormalizePath(shellPath), true
}

// SetShellPath records the shell path.
func (m *SettingsManager) SetShellPath(path string) { m.setGlobal("shellPath", path) }

// GetQuietStartup reports whether startup output is suppressed.
func (m *SettingsManager) GetQuietStartup() bool { return m.boolField("quietStartup", false) }

// SetQuietStartup records whether startup output is suppressed.
func (m *SettingsManager) SetQuietStartup(quiet bool) { m.setGlobal("quietStartup", quiet) }

// DefaultProjectTrust is the default answer to the project-trust prompt.
type DefaultProjectTrust string

const (
	ProjectTrustAsk    DefaultProjectTrust = "ask"
	ProjectTrustAlways DefaultProjectTrust = "always"
	ProjectTrustNever  DefaultProjectTrust = "never"
)

// GetDefaultProjectTrust returns the global default project-trust answer.
func (m *SettingsManager) GetDefaultProjectTrust() DefaultProjectTrust {
	value, ok := m.globalSettings["defaultProjectTrust"].(string)
	if !ok {
		return ProjectTrustAsk
	}
	switch DefaultProjectTrust(value) {
	case ProjectTrustAlways, ProjectTrustNever:
		return DefaultProjectTrust(value)
	}
	return ProjectTrustAsk
}

// SetDefaultProjectTrust records the global default project-trust answer.
func (m *SettingsManager) SetDefaultProjectTrust(trust DefaultProjectTrust) {
	m.setGlobal("defaultProjectTrust", string(trust))
}

// GetShellCommandPrefix returns the prefix prepended to every bash command.
func (m *SettingsManager) GetShellCommandPrefix() (string, bool) {
	return m.stringField("shellCommandPrefix")
}

// SetShellCommandPrefix records the bash command prefix.
func (m *SettingsManager) SetShellCommandPrefix(prefix string) {
	m.setGlobal("shellCommandPrefix", prefix)
}

// GetNpmCommand returns the npm command argv.
func (m *SettingsManager) GetNpmCommand() ([]string, bool) { return m.stringSlice("npmCommand") }

// SetNpmCommand records the npm command argv.
func (m *SettingsManager) SetNpmCommand(command []string) {
	if command == nil {
		delete(m.globalSettings, "npmCommand")
		m.markModified("npmCommand")
		m.save()
		return
	}
	m.setGlobal("npmCommand", command)
}

// GetCollapseChangelog reports whether the changelog is collapsed.
func (m *SettingsManager) GetCollapseChangelog() bool { return m.boolField("collapseChangelog", false) }

// SetCollapseChangelog records whether the changelog is collapsed.
func (m *SettingsManager) SetCollapseChangelog(collapse bool) {
	m.setGlobal("collapseChangelog", collapse)
}

// GetEnableInstallTelemetry reports whether install telemetry is enabled.
func (m *SettingsManager) GetEnableInstallTelemetry() bool {
	return m.boolField("enableInstallTelemetry", true)
}

// SetEnableInstallTelemetry records whether install telemetry is enabled.
func (m *SettingsManager) SetEnableInstallTelemetry(enabled bool) {
	m.setGlobal("enableInstallTelemetry", enabled)
}

// GetEnableAnalytics reports whether analytics is enabled.
func (m *SettingsManager) GetEnableAnalytics() bool { return m.boolField("enableAnalytics", false) }

// GetTrackingID returns the analytics tracking identifier.
func (m *SettingsManager) GetTrackingID() (string, bool) { return m.stringField("trackingId") }

// PackageSource is one npm/git package source: either a bare string or an
// object with filtering fields.
type PackageSource struct {
	Source     string
	Object     bool
	Autoload   *bool
	Extensions []string
	Skills     []string
	Prompts    []string
	Themes     []string
}

func (p PackageSource) toSetting() any {
	if !p.Object {
		return p.Source
	}
	object := map[string]any{"source": p.Source}
	if p.Autoload != nil {
		object["autoload"] = *p.Autoload
	}
	if p.Extensions != nil {
		object["extensions"] = p.Extensions
	}
	if p.Skills != nil {
		object["skills"] = p.Skills
	}
	if p.Prompts != nil {
		object["prompts"] = p.Prompts
	}
	if p.Themes != nil {
		object["themes"] = p.Themes
	}
	return object
}

func packageSourceFromSetting(value any) (PackageSource, bool) {
	switch typed := value.(type) {
	case string:
		return PackageSource{Source: typed}, true
	case map[string]any:
		source, _ := typed["source"].(string)
		out := PackageSource{Source: source, Object: true}
		if autoload, ok := typed["autoload"].(bool); ok {
			out.Autoload = &autoload
		}
		out.Extensions = toStringSlice(typed["extensions"])
		out.Skills = toStringSlice(typed["skills"])
		out.Prompts = toStringSlice(typed["prompts"])
		out.Themes = toStringSlice(typed["themes"])
		return out, true
	}
	return PackageSource{}, false
}

func toStringSlice(value any) []string {
	list, ok := value.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(list))
	for _, item := range list {
		if str, ok := item.(string); ok {
			out = append(out, str)
		}
	}
	return out
}

// GetPackages returns the configured package sources.
func (m *SettingsManager) GetPackages() []PackageSource {
	list, ok := m.settings["packages"].([]any)
	if !ok {
		return []PackageSource{}
	}
	out := make([]PackageSource, 0, len(list))
	for _, item := range list {
		if source, ok := packageSourceFromSetting(item); ok {
			out = append(out, source)
		}
	}
	return out
}

// SetPackages records the global package sources.
func (m *SettingsManager) SetPackages(packages []PackageSource) {
	m.setGlobal("packages", packagesToSettings(packages))
}

// SetProjectPackages records the project package sources.
func (m *SettingsManager) SetProjectPackages(packages []PackageSource) error {
	return m.updateProjectSettings("packages", func(settings Settings) {
		settings["packages"] = packagesToSettings(packages)
	})
}

func packagesToSettings(packages []PackageSource) []any {
	out := make([]any, 0, len(packages))
	for _, source := range packages {
		out = append(out, source.toSetting())
	}
	return out
}

// GetExtensionPaths returns the configured local extension paths.
func (m *SettingsManager) GetExtensionPaths() []string {
	paths, _ := m.stringSlice("extensions")
	if paths == nil {
		return []string{}
	}
	return paths
}

// SetExtensionPaths records the global extension paths.
func (m *SettingsManager) SetExtensionPaths(paths []string) { m.setGlobal("extensions", paths) }

// SetProjectExtensionPaths records the project extension paths.
func (m *SettingsManager) SetProjectExtensionPaths(paths []string) error {
	return m.updateProjectSettings("extensions", func(settings Settings) { settings["extensions"] = paths })
}

// GetSkillPaths returns the configured skill paths.
func (m *SettingsManager) GetSkillPaths() []string {
	paths, _ := m.stringSlice("skills")
	if paths == nil {
		return []string{}
	}
	return paths
}

// SetSkillPaths records the global skill paths.
func (m *SettingsManager) SetSkillPaths(paths []string) { m.setGlobal("skills", paths) }

// SetProjectSkillPaths records the project skill paths.
func (m *SettingsManager) SetProjectSkillPaths(paths []string) error {
	return m.updateProjectSettings("skills", func(settings Settings) { settings["skills"] = paths })
}

// GetPromptTemplatePaths returns the configured prompt-template paths.
func (m *SettingsManager) GetPromptTemplatePaths() []string {
	paths, _ := m.stringSlice("prompts")
	if paths == nil {
		return []string{}
	}
	return paths
}

// SetPromptTemplatePaths records the global prompt-template paths.
func (m *SettingsManager) SetPromptTemplatePaths(paths []string) { m.setGlobal("prompts", paths) }

// SetProjectPromptTemplatePaths records the project prompt-template paths.
func (m *SettingsManager) SetProjectPromptTemplatePaths(paths []string) error {
	return m.updateProjectSettings("prompts", func(settings Settings) { settings["prompts"] = paths })
}

// GetThemePaths returns the configured theme paths.
func (m *SettingsManager) GetThemePaths() []string {
	paths, _ := m.stringSlice("themes")
	if paths == nil {
		return []string{}
	}
	return paths
}

// SetThemePaths records the global theme paths.
func (m *SettingsManager) SetThemePaths(paths []string) { m.setGlobal("themes", paths) }

// SetProjectThemePaths records the project theme paths.
func (m *SettingsManager) SetProjectThemePaths(paths []string) error {
	return m.updateProjectSettings("themes", func(settings Settings) { settings["themes"] = paths })
}

// GetEnableSkillCommands reports whether skills are registered as commands.
func (m *SettingsManager) GetEnableSkillCommands() bool {
	return m.boolField("enableSkillCommands", true)
}

// SetEnableSkillCommands records whether skills are registered as commands.
func (m *SettingsManager) SetEnableSkillCommands(enabled bool) {
	m.setGlobal("enableSkillCommands", enabled)
}

// GetThinkingBudgets returns the custom thinking-token budgets.
func (m *SettingsManager) GetThinkingBudgets() (map[string]int, bool) {
	budgets, ok := m.settings["thinkingBudgets"].(map[string]any)
	if !ok {
		return nil, false
	}
	out := map[string]int{}
	for key, value := range budgets {
		if number, ok := intFieldValue(value); ok {
			out[key] = number
		}
	}
	return out, true
}

// GetTerminalCapabilityOverrides returns explicit terminal capability values.
func (m *SettingsManager) GetTerminalCapabilityOverrides() map[string]any {
	terminal, _ := m.settings["terminal"].(map[string]any)
	overrides := map[string]any{}
	if terminal == nil {
		return overrides
	}
	switch images := terminal["images"].(type) {
	case string:
		if images == "kitty" || images == "iterm2" {
			overrides["images"] = images
		}
	case bool:
		if !images {
			overrides["images"] = nil
		}
	}
	if trueColor, ok := terminal["trueColor"].(bool); ok {
		overrides["trueColor"] = trueColor
	}
	if hyperlinks, ok := terminal["hyperlinks"].(bool); ok {
		overrides["hyperlinks"] = hyperlinks
	}
	return overrides
}

// GetShowImages reports whether terminal images are shown.
func (m *SettingsManager) GetShowImages() bool {
	terminal, _ := m.settings["terminal"].(map[string]any)
	if terminal == nil {
		return true
	}
	show, ok := terminal["showImages"].(bool)
	if !ok {
		return true
	}
	return show
}

// SetShowImages records whether terminal images are shown.
func (m *SettingsManager) SetShowImages(show bool) { m.setGlobalNested("terminal", "showImages", show) }

// GetImageWidthCells returns the preferred inline image width in cells.
func (m *SettingsManager) GetImageWidthCells() int {
	terminal, _ := m.settings["terminal"].(map[string]any)
	if terminal == nil {
		return 60
	}
	width, ok := intFieldValue(terminal["imageWidthCells"])
	if !ok {
		return 60
	}
	if width < 1 {
		return 1
	}
	return width
}

// SetImageWidthCells records the preferred inline image width in cells.
func (m *SettingsManager) SetImageWidthCells(width int) {
	if width < 1 {
		width = 1
	}
	m.setGlobalNested("terminal", "imageWidthCells", width)
}

// GetClearOnShrink reports whether empty rows are cleared when content shrinks.
func (m *SettingsManager) GetClearOnShrink() bool {
	terminal, _ := m.settings["terminal"].(map[string]any)
	if terminal != nil {
		if clear, ok := terminal["clearOnShrink"].(bool); ok {
			return clear
		}
	}
	return os.Getenv("PI_CLEAR_ON_SHRINK") == "1"
}

// SetClearOnShrink records whether empty rows are cleared when content shrinks.
func (m *SettingsManager) SetClearOnShrink(enabled bool) {
	m.setGlobalNested("terminal", "clearOnShrink", enabled)
}

// GetShowTerminalProgress reports whether terminal progress indicators are shown.
func (m *SettingsManager) GetShowTerminalProgress() bool {
	terminal, _ := m.settings["terminal"].(map[string]any)
	if terminal == nil {
		return false
	}
	show, ok := terminal["showTerminalProgress"].(bool)
	return ok && show
}

// SetShowTerminalProgress records whether terminal progress indicators are shown.
func (m *SettingsManager) SetShowTerminalProgress(enabled bool) {
	m.setGlobalNested("terminal", "showTerminalProgress", enabled)
}

// TuiMode is the interactive renderer mode.
type TuiMode string

const (
	TuiModeRegular    TuiMode = "regular"
	TuiModeFullscreen TuiMode = "fullscreen"
)

// GetTuiMode returns the interactive renderer mode.
func (m *SettingsManager) GetTuiMode() TuiMode {
	if mode, ok := m.stringField("tuiMode"); ok && mode == string(TuiModeFullscreen) {
		return TuiModeFullscreen
	}
	return TuiModeRegular
}

// SetTuiMode records the interactive renderer mode.
func (m *SettingsManager) SetTuiMode(mode TuiMode) { m.setGlobal("tuiMode", string(mode)) }

// FullscreenExitOutput controls what fullscreen mode prints on exit.
type FullscreenExitOutput string

const (
	FullscreenExitTranscript FullscreenExitOutput = "transcript"
	FullscreenExitResumeHint FullscreenExitOutput = "resume-hint"
)

// GetFullscreenExitOutput returns the fullscreen exit output mode.
func (m *SettingsManager) GetFullscreenExitOutput() FullscreenExitOutput {
	if output, ok := m.stringField("fullscreenExitOutput"); ok && output == string(FullscreenExitResumeHint) {
		return FullscreenExitResumeHint
	}
	return FullscreenExitTranscript
}

// SetFullscreenExitOutput records the fullscreen exit output mode.
func (m *SettingsManager) SetFullscreenExitOutput(output FullscreenExitOutput) {
	m.setGlobal("fullscreenExitOutput", string(output))
}

// ScrollViewScrollbar is the fullscreen scrollbar visibility mode.
type ScrollViewScrollbar string

const (
	ScrollbarAuto   ScrollViewScrollbar = "auto"
	ScrollbarAlways ScrollViewScrollbar = "always"
	ScrollbarHidden ScrollViewScrollbar = "hidden"
)

// GetFullscreenScrollbar returns the fullscreen scrollbar mode.
func (m *SettingsManager) GetFullscreenScrollbar() ScrollViewScrollbar {
	mode, ok := m.stringField("fullscreenScrollbar")
	if !ok {
		return ScrollbarAuto
	}
	switch ScrollViewScrollbar(mode) {
	case ScrollbarAlways, ScrollbarHidden:
		return ScrollViewScrollbar(mode)
	}
	return ScrollbarAuto
}

// SetFullscreenScrollbar records the fullscreen scrollbar mode.
func (m *SettingsManager) SetFullscreenScrollbar(mode ScrollViewScrollbar) {
	m.setGlobal("fullscreenScrollbar", string(mode))
}

// GetFullscreenCopyOnSelect reports whether selection copies in fullscreen mode.
func (m *SettingsManager) GetFullscreenCopyOnSelect() bool {
	return m.boolField("fullscreenCopyOnSelect", true)
}

// SetFullscreenCopyOnSelect records whether selection copies in fullscreen mode.
func (m *SettingsManager) SetFullscreenCopyOnSelect(enabled bool) {
	m.setGlobal("fullscreenCopyOnSelect", enabled)
}

// GetImageAutoResize reports whether images are auto-resized before sending.
func (m *SettingsManager) GetImageAutoResize() bool {
	images, _ := m.settings["images"].(map[string]any)
	if images == nil {
		return true
	}
	autoResize, ok := images["autoResize"].(bool)
	if !ok {
		return true
	}
	return autoResize
}

// SetImageAutoResize records whether images are auto-resized before sending.
func (m *SettingsManager) SetImageAutoResize(enabled bool) {
	m.setGlobalNested("images", "autoResize", enabled)
}

// GetBlockImages reports whether images are blocked from being sent.
func (m *SettingsManager) GetBlockImages() bool {
	images, _ := m.settings["images"].(map[string]any)
	if images == nil {
		return false
	}
	blocked, _ := images["blockImages"].(bool)
	return blocked
}

// SetBlockImages records whether images are blocked from being sent.
func (m *SettingsManager) SetBlockImages(blocked bool) {
	m.setGlobalNested("images", "blockImages", blocked)
}

// GetEnabledModels returns the model cycling patterns.
func (m *SettingsManager) GetEnabledModels() ([]string, bool) { return m.stringSlice("enabledModels") }

// SetEnabledModels records the model cycling patterns.
func (m *SettingsManager) SetEnabledModels(patterns []string) {
	if patterns == nil {
		delete(m.globalSettings, "enabledModels")
		m.markModified("enabledModels")
		m.save()
		return
	}
	m.setGlobal("enabledModels", patterns)
}

// GetDefaultTools returns the initial built-in tool selection.
func (m *SettingsManager) GetDefaultTools() ([]string, bool) { return m.stringSlice("defaultTools") }

// GetDoubleEscapeAction returns the empty-editor double-escape action.
func (m *SettingsManager) GetDoubleEscapeAction() string {
	if value, ok := m.stringField("doubleEscapeAction"); ok {
		return value
	}
	return "tree"
}

// SetDoubleEscapeAction records the empty-editor double-escape action.
func (m *SettingsManager) SetDoubleEscapeAction(action string) {
	m.setGlobal("doubleEscapeAction", action)
}

// GetTreeFilterMode returns the default /tree filter.
func (m *SettingsManager) GetTreeFilterMode() string {
	mode, ok := m.stringField("treeFilterMode")
	if !ok {
		return "default"
	}
	switch mode {
	case "default", "no-tools", "user-only", "labeled-only", "all":
		return mode
	}
	return "default"
}

// SetTreeFilterMode records the default /tree filter.
func (m *SettingsManager) SetTreeFilterMode(mode string) { m.setGlobal("treeFilterMode", mode) }

// GetShowHardwareCursor reports whether the terminal cursor is shown.
func (m *SettingsManager) GetShowHardwareCursor() bool {
	if value, ok := m.field("showHardwareCursor"); ok {
		if b, ok := value.(bool); ok {
			return b
		}
	}
	return os.Getenv("PI_HARDWARE_CURSOR") == "1"
}

// SetShowHardwareCursor records whether the terminal cursor is shown.
func (m *SettingsManager) SetShowHardwareCursor(enabled bool) {
	m.setGlobal("showHardwareCursor", enabled)
}

// GetEditorPaddingX returns the input editor horizontal padding.
func (m *SettingsManager) GetEditorPaddingX() int { return m.intField("editorPaddingX", 0) }

// SetEditorPaddingX records the input editor horizontal padding.
func (m *SettingsManager) SetEditorPaddingX(padding int) {
	if padding < 0 {
		padding = 0
	}
	if padding > 3 {
		padding = 3
	}
	m.setGlobal("editorPaddingX", padding)
}

// GetOutputPad returns the chat output horizontal padding.
func (m *SettingsManager) GetOutputPad() int {
	if m.intField("outputPad", 1) == 0 {
		return 0
	}
	return 1
}

// SetOutputPad records the chat output horizontal padding.
func (m *SettingsManager) SetOutputPad(padding int) { m.setGlobal("outputPad", padding) }

// GetAutocompleteMaxVisible returns the autocomplete dropdown limit.
func (m *SettingsManager) GetAutocompleteMaxVisible() int {
	return m.intField("autocompleteMaxVisible", 5)
}

// SetAutocompleteMaxVisible records the autocomplete dropdown limit.
func (m *SettingsManager) SetAutocompleteMaxVisible(maxVisible int) {
	if maxVisible < 3 {
		maxVisible = 3
	}
	if maxVisible > 20 {
		maxVisible = 20
	}
	m.setGlobal("autocompleteMaxVisible", maxVisible)
}

// GetCodeBlockIndent returns the markdown code-block indent.
func (m *SettingsManager) GetCodeBlockIndent() string {
	markdown, _ := m.settings["markdown"].(map[string]any)
	if markdown != nil {
		if indent, ok := markdown["codeBlockIndent"].(string); ok {
			return indent
		}
	}
	return "  "
}

// MermaidRenderingMode controls when Mermaid diagrams render.
type MermaidRenderingMode string

const (
	MermaidOff       MermaidRenderingMode = "off"
	MermaidFinal     MermaidRenderingMode = "final"
	MermaidStreaming MermaidRenderingMode = "streaming"
)

// GetMermaidRenderingMode returns the Mermaid rendering mode.
func (m *SettingsManager) GetMermaidRenderingMode() MermaidRenderingMode {
	markdown, _ := m.settings["markdown"].(map[string]any)
	if markdown == nil {
		return MermaidStreaming
	}
	mode, ok := markdown["mermaid"].(string)
	if !ok {
		return MermaidStreaming
	}
	switch MermaidRenderingMode(mode) {
	case MermaidOff, MermaidFinal:
		return MermaidRenderingMode(mode)
	}
	return MermaidStreaming
}

// SetMermaidRenderingMode records the Mermaid rendering mode.
func (m *SettingsManager) SetMermaidRenderingMode(mode MermaidRenderingMode) {
	m.setGlobalNested("markdown", "mermaid", string(mode))
}

// GetWarnings returns the warning toggles.
func (m *SettingsManager) GetWarnings() map[string]any {
	warnings, _ := m.settings["warnings"].(map[string]any)
	if warnings == nil {
		return map[string]any{}
	}
	return cloneMap(warnings)
}

// SetWarnings records the warning toggles.
func (m *SettingsManager) SetWarnings(warnings map[string]any) {
	m.setGlobal("warnings", warnings)
}

func settingsLockContext() context.Context { return context.Background() }
