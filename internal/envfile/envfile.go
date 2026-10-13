// Package envfile implements the producer output-file contract.
package envfile

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

func Prepare() (string, func(), error) {
	dir, err := os.MkdirTemp("", "gimbal-env-*")
	if err != nil {
		return "", func() {}, err
	}
	return filepath.Join(dir, "values.env"), func() { _ = os.RemoveAll(dir) }, nil
}

var outputName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

func Read(path string, names []string) (map[string]string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("producer output file unavailable: %w", err)
	}
	all := map[string]string{}
	for line := range strings.SplitSeq(string(raw), "\n") {
		line = strings.TrimSuffix(line, "\r")
		if line == "" {
			continue
		}
		name, value, ok := strings.Cut(line, "=")
		if !ok || !outputName.MatchString(name) {
			return nil, fmt.Errorf("malformed producer output")
		}
		if _, exists := all[name]; exists {
			return nil, fmt.Errorf("duplicate producer output %s", name)
		}
		all[name] = value
	}
	values := map[string]string{}
	for _, name := range names {
		value, ok := all[name]
		if !ok {
			return nil, fmt.Errorf("missing declared output %s", name)
		}
		values[name] = value
	}
	return values, nil
}
