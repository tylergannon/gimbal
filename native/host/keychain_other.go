//go:build !darwin

package main

import "fmt"

type keychainStore struct{ account string }

func (keychainStore) load() ([]byte, error) {
	return nil, fmt.Errorf("desktop login storage requires macOS Keychain")
}
func (keychainStore) save([]byte) error {
	return fmt.Errorf("desktop login storage requires macOS Keychain")
}
