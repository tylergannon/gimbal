// Release assets transfers generated source and frontend at one exact revision.
package main

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
)

type metadata struct {
	SHA     string   `json:"sha"`
	Deleted []string `json:"deleted"`
}

func git(root string, args ...string) (string, error) {
	command := exec.Command("git", args...)
	command.Dir = root
	raw, err := command.Output()
	return strings.TrimRight(string(raw), "\n"), err
}
func paths(root string, args ...string) ([]string, error) {
	raw, err := git(root, args...)
	if err != nil {
		return nil, err
	}
	return slices.DeleteFunc(strings.Split(raw, "\x00"), func(name string) bool { return name == "" }), nil
}
func pack(root, archive string) error {
	info, err := os.Stat(filepath.Join(root, "web/build/skgo.manifest.json"))
	if err != nil {
		return err
	}
	if info.Size() == 0 {
		return errors.New("frontend manifest is empty")
	}
	changed, err := paths(root, "diff", "--name-only", "-z", "--diff-filter=AM", "HEAD")
	if err != nil {
		return err
	}
	untracked, err := paths(root, "ls-files", "--others", "--exclude-standard", "-z")
	if err != nil {
		return err
	}
	changed = append(changed, untracked...)
	changed = slices.DeleteFunc(changed, func(name string) bool {
		return strings.HasPrefix(name, "ephemeral/") || strings.HasPrefix(name, "docs/")
	})
	sha, err := git(root, "rev-parse", "HEAD")
	if err != nil {
		return err
	}
	deleted, err := paths(root, "diff", "--name-only", "-z", "--diff-filter=D", "HEAD")
	if err != nil {
		return err
	}
	raw, err := json.Marshal(metadata{SHA: sha, Deleted: deleted})
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(archive), 0755); err != nil {
		return err
	}
	file, err := os.Create(archive)
	if err != nil {
		return err
	}
	zip := gzip.NewWriter(file)
	writer := tar.NewWriter(zip)
	writeErr := func() error {
		if err := writer.WriteHeader(&tar.Header{Name: "release-inputs.json", Mode: 0644, Size: int64(len(raw)), Typeflag: tar.TypeReg}); err != nil {
			return err
		}
		if _, err := writer.Write(raw); err != nil {
			return err
		}
		names := append([]string{"web/build"}, changed...)
		for _, name := range []string{".skgo/build.json", "native/Generated.swift"} {
			if _, err := os.Stat(filepath.Join(root, name)); err == nil {
				names = append(names, name)
			} else if !os.IsNotExist(err) {
				return err
			}
		}
		slices.Sort(names)
		names = slices.Compact(names)
		seen := map[string]bool{}
		for _, name := range names {
			if err := filepath.WalkDir(filepath.Join(root, name), func(path string, entry os.DirEntry, walkErr error) error {
				if walkErr != nil {
					return walkErr
				}
				relative, err := filepath.Rel(root, path)
				if err != nil {
					return err
				}
				relative = filepath.ToSlash(relative)
				if seen[relative] {
					return nil
				}
				seen[relative] = true
				info, err := entry.Info()
				if err != nil {
					return err
				}
				link := ""
				if info.Mode()&os.ModeSymlink != 0 {
					link, err = os.Readlink(path)
					if err != nil {
						return err
					}
				}
				header, err := tar.FileInfoHeader(info, link)
				if err != nil {
					return err
				}
				header.Name = relative
				if relative == ".skgo/build.json" {
					header.Name = ".skgo/import.json"
				}
				if err := writer.WriteHeader(header); err != nil {
					return err
				}
				if !info.Mode().IsRegular() {
					return nil
				}
				source, err := os.Open(path)
				if err != nil {
					return err
				}
				_, copyErr := io.Copy(writer, source)
				return errors.Join(copyErr, source.Close())
			}); err != nil {
				return err
			}
		}
		return nil
	}()
	return errors.Join(writeErr, writer.Close(), zip.Close(), file.Close())
}
func visit(archive string, fn func(*tar.Header, *tar.Reader) error) error {
	file, err := os.Open(archive)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()
	zip, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer func() { _ = zip.Close() }()
	reader := tar.NewReader(zip)
	for {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			_, err := io.Copy(io.Discard, zip)
			return err
		}
		if err != nil {
			return err
		}
		if err := fn(header, reader); err != nil {
			return err
		}
	}
}
func localName(name string) (string, error) {
	name = filepath.FromSlash(name)
	if !filepath.IsLocal(name) {
		return "", fmt.Errorf("prepared input escapes the checkout: %s", name)
	}
	return filepath.Clean(name), nil
}
func restore(root, archive string) error {
	checkout, err := os.OpenRoot(root)
	if err != nil {
		return err
	}
	defer func() { _ = checkout.Close() }()
	var meta metadata
	found := false
	// Validate the full archive before deleting or writing any checkout file.
	if err := visit(archive, func(header *tar.Header, reader *tar.Reader) error {
		name, err := localName(header.Name)
		if err != nil {
			return err
		}
		if header.Typeflag != tar.TypeReg && header.Typeflag != tar.TypeDir {
			return errors.New("prepared input is a special file")
		}
		if _, err := checkout.Stat(name); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("prepared input escapes the checkout: %w", err)
		}
		if name == "release-inputs.json" {
			if found || header.Typeflag != tar.TypeReg {
				return errors.New("prepared inputs have duplicate or invalid metadata")
			}
			found = true
			raw, err := io.ReadAll(reader)
			if err != nil {
				return err
			}
			return json.Unmarshal(raw, &meta)
		}
		return nil
	}); err != nil {
		return err
	}
	if !found {
		return errors.New("prepared inputs have no revision metadata")
	}
	sha, err := git(root, "rev-parse", "HEAD")
	if err != nil {
		return err
	}
	if meta.SHA != sha {
		return errors.New("prepared inputs belong to a different release revision")
	}
	for _, name := range meta.Deleted {
		if _, err := localName(name); err != nil {
			return err
		}
		info, err := checkout.Stat(name)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		if err == nil && info.IsDir() {
			return fmt.Errorf("deleted output is a directory: %s", name)
		}
	}
	for _, name := range meta.Deleted {
		if err := checkout.Remove(name); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return visit(archive, func(header *tar.Header, reader *tar.Reader) error {
		name, err := localName(header.Name)
		if err != nil {
			return err
		}
		if name == "release-inputs.json" {
			return nil
		}
		mode := os.FileMode(header.Mode).Perm()
		if header.Typeflag == tar.TypeDir {
			return checkout.MkdirAll(name, mode)
		}
		if err := checkout.MkdirAll(filepath.Dir(name), 0755); err != nil {
			return err
		}
		file, err := checkout.OpenFile(name, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
		if err != nil {
			return err
		}
		if err := file.Chmod(mode); err != nil {
			_ = file.Close()
			return err
		}
		_, copyErr := io.Copy(file, reader)
		return errors.Join(copyErr, file.Close())
	})
}
func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "usage: release-assets pack|restore ARCHIVE")
		os.Exit(1)
	}
	root, err := os.Getwd()
	if err == nil {
		switch os.Args[1] {
		case "pack":
			err = pack(root, os.Args[2])
		case "restore":
			err = restore(root, os.Args[2])
		default:
			err = errors.New("expected pack or restore")
		}
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
