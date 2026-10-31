package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func environmentFile(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "worker environment.json")
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestEnvironmentOverridesAndLiteralUserImport(t *testing.T) {
	t.Setenv("GIMBAL_WORKER_TEST_PATH", "old")
	t.Setenv("GIMBAL_WORKER_TEST_IMPORT", "old")
	path := environmentFile(t, `{"set":{"GIMBAL_WORKER_TEST_PATH":"C:\\Users\\tyler\\bin;%PATH%"},"user":["GIMBAL_WORKER_TEST_IMPORT"]}`)
	err := applyEnvironment(path, func(name string) (string, error) {
		if name != "GIMBAL_WORKER_TEST_IMPORT" {
			t.Fatalf("unexpected import name")
		}
		return "synthetic-value", nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if os.Getenv("GIMBAL_WORKER_TEST_PATH") != `C:\Users\tyler\bin;%PATH%` || os.Getenv("GIMBAL_WORKER_TEST_IMPORT") != "synthetic-value" {
		t.Fatal("explicit environment was not applied literally")
	}
}

func TestEnvironmentRejectsInvalidInputWithoutLeakingOrPartialApplication(t *testing.T) {
	t.Setenv("GIMBAL_WORKER_TEST_KEEP", "original")
	for _, content := range []string{
		`{"set":{"GIMBAL_WORKER_TEST_KEEP":"changed"},"user":["MISSING"]}`,
		`{"set":{"GIMBAL_WORKER_TEST_KEEP":"changed","bad=name":"private-synthetic-value"}}`,
		`{"set":{"GIMBAL_WORKER_TEST_KEEP":"changed","BAD":"private-synthetic-value\u0000"}}`,
		`{"set":{"Path":"one","PATH":"two"}}`,
		`{"set":{"GIMBAL_WORKER_TEST_KEEP":"changed"},"user":["gimbal_worker_test_keep"]}`,
		`{"set":{"GIMBAL_WORKER_TEST_KEEP":"changed"},"user":["EMPTY"]}`,
		`{"set":{"GIMBAL_WORKER_TEST_KEEP":"changed"},"unknown":"private-synthetic-value"}`,
		`{"set":{"GIMBAL_WORKER_TEST_KEEP":123}}`,
		`{"set":{}} {}`, `private-synthetic-value`, `null`,
	} {
		err := applyEnvironment(environmentFile(t, content), func(name string) (string, error) {
			if name == "EMPTY" {
				return "", nil
			}
			return "", errors.New("private-synthetic-value")
		})
		if err == nil {
			t.Fatal("invalid configuration accepted")
		}
		if strings.Contains(err.Error(), "private-synthetic-value") {
			t.Fatal("configuration error leaked a value")
		}
		if os.Getenv("GIMBAL_WORKER_TEST_KEEP") != "original" {
			t.Fatal("invalid file partially changed environment")
		}
	}
	if err := applyEnvironment(filepath.Join(t.TempDir(), "missing"), userEnvironment); err == nil {
		t.Fatal("missing file accepted")
	}
	if err := applyEnvironment("", userEnvironment); err != nil {
		t.Fatal(err)
	}
}
