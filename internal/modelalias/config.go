package modelalias

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Read the per-user overrides on each lookup so a running server sees edits.
func configuredAliases() (map[string]model, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, nil
	}
	path := filepath.Join(home, ".gimbal", "config.json")
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read model aliases from %s: %w", path, err)
	}
	var config struct {
		ModelAliases map[string]string `json:"modelAliases"`
	}
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("parse model aliases in %s: %w", path, err)
	}
	overrides := make(map[string]model, len(config.ModelAliases))
	for alias, target := range config.ModelAliases {
		versions, ok := families[alias]
		if !ok {
			return nil, fmt.Errorf("%s: modelAliases key %q must be an unversioned built-in alias", path, alias)
		}
		if _, isAlias := aliases[target]; isAlias || strings.ContainsAny(target, " \t\r\n\x00") {
			return nil, fmt.Errorf("%s: modelAliases[%q] must name a provider model ID", path, alias)
		}
		resolved, err := resolve(Selection{Name: target}, model{})
		if err != nil {
			return nil, fmt.Errorf("%s: modelAliases[%q]: %w", path, alias, err)
		}
		entry := aliases[alias]
		if resolved.Provider != entry.provider {
			return nil, fmt.Errorf("%s: modelAliases[%q] must name a %s model", path, alias, entry.provider)
		}
		entry.native, entry.version = resolved.Model, ""
		for version, member := range versions {
			if member.native == entry.native {
				entry.version = version
				break
			}
		}
		overrides[alias] = entry
	}
	return overrides, nil
}
