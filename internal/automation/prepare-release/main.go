// Prepare release atomically reserves checked main source and its next patch tag.
package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
)

type preparer struct {
	root string
	git  func(...string) (string, error)
}

func newPreparer(root string) preparer {
	return preparer{root: root, git: func(args ...string) (string, error) {
		command := exec.Command("git", args...)
		command.Dir = root
		raw, err := command.Output()
		if err != nil {
			var exit *exec.ExitError
			if errors.As(err, &exit) {
				return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, exit.Stderr)
			}
			return "", err
		}
		return strings.TrimSpace(string(raw)), nil
	}}
}

var stableTag = regexp.MustCompile(`^v[0-9]+\.[0-9]+\.[0-9]+$`)

func version(tag string) ([3]int, error) {
	var result [3]int
	fields := strings.Split(strings.TrimPrefix(tag, "v"), ".")
	if len(fields) != 3 {
		return result, fmt.Errorf("invalid version %s", tag)
	}
	for i, field := range fields {
		value, err := strconv.Atoi(field)
		if err != nil {
			return result, err
		}
		result[i] = value
	}
	return result, nil
}
func (p preparer) prepare(source, remote string) (string, string, error) {
	source, err := p.git("rev-parse", source+"^{commit}")
	if err != nil {
		return "", "", err
	}
	if _, err := p.git("fetch", remote, "main", "--tags"); err != nil {
		return "", "", err
	}
	raw, err := p.git("tag", "--list")
	if err != nil {
		return "", "", err
	}
	tags := []string{}
	for line := range strings.SplitSeq(raw, "\n") {
		if stableTag.MatchString(line) {
			if _, err := version(line); err != nil {
				return "", "", err
			}
			tags = append(tags, line)
		}
	}
	slices.SortFunc(tags, func(a, b string) int {
		av, _ := version(a)
		bv, _ := version(b)
		for i := range av {
			if av[i] < bv[i] {
				return -1
			}
			if av[i] > bv[i] {
				return 1
			}
		}
		return 0
	})
	for _, tag := range slices.Backward(tags) {
		sha, err := p.git("rev-parse", tag+"^{commit}")
		if err != nil {
			return "", "", err
		}
		message, err := p.git("show", "-s", "--format=%B", sha)
		if err != nil {
			return "", "", err
		}
		if sha == source || slices.Contains(strings.Split(message, "\n"), "Release-Source: "+source) {
			if tag == tags[len(tags)-1] {
				return tag, sha, nil
			}
			return "", "", nil
		}
	}
	latest, err := p.git("rev-parse", remote+"/main")
	if err != nil {
		return "", "", err
	}
	if latest != source {
		return "", "", nil
	}
	if len(tags) == 0 {
		return "", "", errors.New("no stable version tag exists; establish the initial version first")
	}
	previous := tags[len(tags)-1]
	if _, err := p.git("merge-base", "--is-ancestor", previous, source); err != nil {
		return "", "", err
	}
	number, err := version(previous)
	if err != nil {
		return "", "", err
	}
	tag := fmt.Sprintf("v%d.%d.%d", number[0], number[1], number[2]+1)
	if _, err := p.git("checkout", "--detach", source); err != nil {
		return "", "", err
	}
	status, err := p.git("status", "--porcelain")
	if err != nil {
		return "", "", err
	}
	if status != "" {
		return "", "", errors.New("release checkout must be clean")
	}
	changelog := filepath.Join(p.root, "CHANGELOG.md")
	rawBytes, err := os.ReadFile(changelog)
	if err != nil {
		return "", "", err
	}
	text := string(rawBytes)
	heading := "## [Unreleased]\n"
	start := strings.Index(text, heading)
	if start < 0 {
		return "", "", errors.New("changelog has no Unreleased section")
	}
	start += len(heading)
	end := strings.Index(text[start:], "\n## [")
	if end < 0 {
		return "", "", errors.New("changelog has no previous release section")
	}
	end += start
	entries := strings.TrimSpace(text[start:end])
	if entries == "" {
		subjects, err := p.git("log", "--format=%s", previous+".."+source)
		if err != nil {
			return "", "", err
		}
		entries = "### Changed\n\n- " + strings.Join(strings.Split(subjects, "\n"), "\n- ")
	}
	text = text[:start] + "\n## [" + tag[1:] + "] - " + time.Now().UTC().Format(time.DateOnly) + "\n\n" + entries + "\n" + text[end:]
	compare := "https://github.com/tylergannon/gimbal/compare/"
	unreleased := regexp.MustCompile(`(?m)^\[Unreleased\]: .*?$`)
	location := unreleased.FindStringIndex(text)
	if location != nil {
		text = text[:location[0]] + "[Unreleased]: " + compare + tag + "...main\n[" + tag[1:] + "]: " + compare + previous + "..." + tag + text[location[1]:]
	}
	if err := os.WriteFile(changelog, []byte(text), 0644); err != nil {
		return "", "", err
	}
	if _, err := p.git("add", "CHANGELOG.md"); err != nil {
		return "", "", err
	}
	if _, err := p.git("-c", "core.hooksPath=/dev/null", "commit", "-m", "chore: release "+tag+"\n\nRelease-Source: "+source); err != nil {
		return "", "", err
	}
	sha, err := p.git("rev-parse", "HEAD")
	if err != nil {
		return "", "", err
	}
	if _, err := p.git("tag", "-a", tag, "-m", "Gimbal "+tag+"\n\nRelease-Source: "+source); err != nil {
		return "", "", err
	}
	if _, pushErr := p.git("push", "--atomic", remote, "HEAD:refs/heads/main", "refs/tags/"+tag); pushErr != nil {
		if _, err := p.git("tag", "-d", tag); err != nil {
			return "", "", err
		}
		if _, err := p.git("fetch", remote, "main", "--tags"); err != nil {
			return "", "", err
		}
		current, err := p.git("rev-parse", remote+"/main")
		if err != nil {
			return "", "", err
		}
		if current != source {
			return "", "", nil
		}
		return "", "", fmt.Errorf("atomic release push failed; no release was dispatched: %w", pushErr)
	}
	return tag, sha, nil
}
func execute(args []string) error {
	if len(args) != 1 {
		return errors.New("usage: prepare-release SOURCE")
	}
	root, err := os.Getwd()
	if err != nil {
		return err
	}
	tag, sha, err := newPreparer(root).prepare(args[0], "origin")
	if err != nil {
		return err
	}
	result := fmt.Sprintf("tag=%s\nsha=%s\n", tag, sha)
	if destination := os.Getenv("GITHUB_OUTPUT"); destination != "" {
		file, err := os.OpenFile(destination, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
		if err != nil {
			return err
		}
		_, writeErr := file.WriteString(result)
		closeErr := file.Close()
		if err := errors.Join(writeErr, closeErr); err != nil {
			return err
		}
	}
	fmt.Print(result)
	return nil
}
func main() {
	if err := execute(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
