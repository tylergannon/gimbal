// CI scope selects affected checks; publication retains one complete main gate.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

type selection struct {
	Kind     string   `json:"kind"`
	Release  bool     `json:"release"`
	Packages []string `json:"go_packages"`
	Web      bool     `json:"web"`
}

func documentation(name string) bool {
	return strings.HasPrefix(name, "skills/") || strings.HasPrefix(name, "docs/") || strings.HasPrefix(name, "ephemeral/") || (!strings.Contains(name, "/") && strings.HasSuffix(name, ".md"))
}
func automation(name string) bool {
	return strings.HasPrefix(name, "scripts/") || strings.HasPrefix(name, ".github/") || strings.HasPrefix(name, "internal/automation/") || strings.HasPrefix(name, ".agents/skills/") || name == "lefthook.yml" || name == "Justfile"
}
func plan(paths []string) selection {
	result := selection{Kind: "documentation", Packages: []string{}}
	for _, name := range paths {
		if name == "" || documentation(name) {
			continue
		}
		result.Release = true
		if result.Kind == "documentation" {
			result.Kind = "automation"
		}
		if !automation(name) {
			result.Kind = "application"
		}
		if strings.HasPrefix(name, "web/") {
			result.Web = true
		}
		if strings.HasSuffix(name, ".go") && !strings.HasPrefix(name, ".agents/skills/") {
			result.Packages = append(result.Packages, "./"+filepath.ToSlash(filepath.Dir(name)))
		}
	}
	if slices.Contains(paths, "go.mod") || slices.Contains(paths, "go.sum") {
		result.Packages = []string{"./..."}
	}
	slices.Sort(result.Packages)
	result.Packages = slices.Compact(result.Packages)
	return result
}
func output(root string, args ...string) (string, error) {
	command := exec.Command(args[0], args[1:]...)
	command.Dir = root
	raw, err := command.Output()
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			return "", fmt.Errorf("%s: %w: %s", strings.Join(args, " "), err, exit.Stderr)
		}
		return "", err
	}
	return string(raw), nil
}
func needsWebAssets(root string, packages []string) (bool, error) {
	if len(packages) == 0 {
		return false, nil
	}
	raw, err := output(root, append([]string{"go", "list", "-deps", "-test", "-e", "-f", "{{.ImportPath}}"}, packages...)...)
	if err != nil {
		return false, err
	}
	for line := range strings.SplitSeq(raw, "\n") {
		fields := strings.Fields(line)
		if len(fields) > 0 && fields[0] == "github.com/tylergannon/gimbal/web" {
			return true, nil
		}
	}
	return false, nil
}

var markdownLink = regexp.MustCompile(`\]\(([^)]+)\)`)

func checkLinks(root string) error {
	root, err := filepath.EvalSymlinks(root)
	if err != nil {
		return err
	}
	return filepath.WalkDir(filepath.Join(root, "skills"), func(source string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !strings.HasSuffix(source, ".md") {
			return nil
		}
		raw, err := os.ReadFile(source)
		if err != nil {
			return err
		}
		for _, match := range markdownLink.FindAllStringSubmatch(string(raw), -1) {
			target := strings.SplitN(match[1], "#", 2)[0]
			if target == "" || strings.Contains(target, "://") || strings.HasPrefix(target, "mailto:") {
				continue
			}
			destination, err := filepath.EvalSymlinks(filepath.Join(filepath.Dir(source), target))
			if err != nil {
				return fmt.Errorf("%s: missing local link %s: %w", source, target, err)
			}
			relative, err := filepath.Rel(root, destination)
			if err != nil || !filepath.IsLocal(relative) {
				return fmt.Errorf("%s: local link escapes repository: %s", source, target)
			}
		}
		return nil
	})
}
func execute(args []string) error {
	flags := flag.NewFlagSet("ci-scope", flag.ContinueOnError)
	base := flags.String("base", "", "Compare changed paths with this ref")
	head := flags.String("head", "HEAD", "Selected revision")
	file := flags.String("plan", "/tmp/gimbal-ci-scope.json", "Scope JSON path")
	runGo := flags.Bool("run-go", false, "Run affected Go tests")
	prerequisites := flags.Bool("go-prerequisites", false, "Select Go test prerequisites")
	links := flags.Bool("check-links", false, "Check instruction links")
	if err := flags.Parse(args); err != nil {
		return err
	}
	root, err := os.Getwd()
	if err != nil {
		return err
	}
	if *links {
		return checkLinks(root)
	}
	if *runGo || *prerequisites {
		raw, err := os.ReadFile(*file)
		if err != nil {
			return err
		}
		var selected selection
		if err := json.Unmarshal(raw, &selected); err != nil {
			return err
		}
		packages := []string{}
		for _, p := range selected.Packages {
			info, err := os.Stat(p)
			if p == "./..." || (err == nil && info.IsDir()) {
				packages = append(packages, p)
			}
		}
		if *prerequisites {
			needed, err := needsWebAssets(root, packages)
			if err != nil {
				return err
			}
			fmt.Printf("web_assets=%t\n", needed)
			return nil
		}
		if len(packages) == 0 {
			return nil
		}
		command := exec.Command("go", append([]string{"test", "-count=1"}, packages...)...)
		command.Stdout = os.Stdout
		command.Stderr = os.Stderr
		return command.Run()
	}
	if *base == "" {
		return errors.New("--base is required to select checks")
	}
	raw, err := output(root, "git", "diff", "--name-only", "-z", *base, *head)
	if err != nil {
		return err
	}
	selected := plan(strings.Split(raw, "\x00"))
	encoded, err := json.Marshal(selected)
	if err != nil {
		return err
	}
	if err := os.WriteFile(*file, encoded, 0600); err != nil {
		return err
	}
	packages, err := json.Marshal(selected.Packages)
	if err != nil {
		return err
	}
	fmt.Printf("kind=%s\nrelease=%t\ngo_packages=%s\nweb=%t\n", selected.Kind, selected.Release, packages, selected.Web)
	return nil
}
func main() {
	if err := execute(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
