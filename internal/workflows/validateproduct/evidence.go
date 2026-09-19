package validateproduct

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// Resolve symlinks before checking containment, including links in parent directories.
func evidencePath(dir, path string) (string, error) {
	root, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(absolute(dir, path))
	if err != nil {
		return "", err
	}
	relative, err := filepath.Rel(root, resolved)
	if err != nil || !filepath.IsLocal(relative) {
		return "", fmt.Errorf("evidence is outside this feature: %s", path)
	}
	return resolved, nil
}

func fileDigest(path string) ([sha256.Size]byte, error) {
	var digest [sha256.Size]byte
	file, err := os.Open(path)
	if err != nil {
		return digest, err
	}
	defer func() { _ = file.Close() }()
	info, err := file.Stat()
	if err != nil {
		return digest, err
	}
	if !info.Mode().IsRegular() || info.Size() == 0 {
		return digest, fmt.Errorf("missing or empty evidence: %s", path)
	}
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return digest, err
	}
	copy(digest[:], hash.Sum(nil))
	return digest, nil
}

func snapshotEvidence(dir string, paths []string) (map[string][sha256.Size]byte, error) {
	originals := make(map[string][sha256.Size]byte, len(paths))
	for _, path := range paths {
		resolved, err := evidencePath(dir, path)
		if err != nil {
			return nil, err
		}
		digest, err := fileDigest(resolved)
		if err != nil {
			return nil, err
		}
		originals[resolved] = digest
	}
	return originals, nil
}

func unchanged(path string, want [sha256.Size]byte) error {
	got, err := fileDigest(path)
	if err != nil {
		return err
	}
	if got != want {
		return fmt.Errorf("file was modified: %s", path)
	}
	return nil
}

func verdictEvidence(dir string, originals map[string][sha256.Size]byte, paths []string) ([]string, error) {
	// Check every original, even files the validator omitted from its citations.
	for path, digest := range originals {
		if err := unchanged(path, digest); err != nil {
			return nil, err
		}
	}
	evidence := make([]string, 0, len(paths))
	for _, path := range paths {
		resolved, err := evidencePath(dir, path)
		if err != nil {
			return nil, err
		}
		if _, ok := originals[resolved]; !ok {
			return nil, fmt.Errorf("citation is not original feature evidence: %s", path)
		}
		evidence = append(evidence, resolved)
	}
	return evidence, nil
}
