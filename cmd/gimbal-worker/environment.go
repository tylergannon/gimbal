package main

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"strings"
)

type environmentConfig struct {
	Set  map[string]string `json:"set"`
	User []string          `json:"user"`
}

func applyEnvironment(path string, lookupUser func(string) (string, error)) error {
	if path == "" {
		return nil
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return errors.New("cannot read worker environment file")
	}
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.DisallowUnknownFields()
	var config *environmentConfig
	if err := decoder.Decode(&config); err != nil || config == nil {
		return errors.New("invalid worker environment JSON")
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return errors.New("invalid worker environment JSON")
	}
	values := make(map[string]string)
	names := make(map[string]bool)
	add := func(name, value string) error {
		key := strings.ToUpper(name)
		if name == "" || strings.ContainsAny(name, "=\x00") || strings.ContainsRune(value, '\x00') || names[key] {
			return errors.New("invalid or duplicate worker environment variable")
		}
		names[key] = true
		values[name] = value
		return nil
	}
	for name, value := range config.Set {
		if err := add(name, value); err != nil {
			return err
		}
	}
	for _, name := range config.User {
		if name == "" || strings.ContainsAny(name, "=\x00") || names[strings.ToUpper(name)] {
			return errors.New("invalid or duplicate worker environment variable")
		}
		value, err := lookupUser(name)
		if err != nil || value == "" {
			return errors.New("required worker user environment import unavailable")
		}
		if err := add(name, value); err != nil {
			return err
		}
	}
	// Resolve and validate everything first. Errors deliberately exclude both
	// parser/registry details and input values, which may contain credentials.
	for name, value := range values {
		if err := os.Setenv(name, value); err != nil {
			return errors.New("cannot apply worker environment")
		}
	}
	return nil
}
