// Resolve the loaded skill's defaults, user overrides, then project overrides.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

var identifier = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:/-]*$`)
var skillName = regexp.MustCompile(`(?m)^name: (df-[a-z0-9-]+)$`)
var yamlBlock = regexp.MustCompile("(?ms)^```yaml\\n(.*?)^```")

type spec map[string]any

func readYAML(data []byte) (map[string]any, error) {
	var node yaml.Node
	decoder := yaml.NewDecoder(strings.NewReader(string(data)))
	if err := decoder.Decode(&node); err != nil {
		if errors.Is(err, io.EOF) {
			return map[string]any{}, nil
		}
		return nil, err
	}
	var trailing yaml.Node
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("expected one YAML document")
	}
	if len(node.Content) == 0 || node.Content[0].Tag == "!!null" {
		return map[string]any{}, nil
	}
	var unique func(*yaml.Node) error
	unique = func(n *yaml.Node) error {
		if n.Kind == yaml.MappingNode {
			seen := map[string]bool{}
			for i := 0; i < len(n.Content); i += 2 {
				k := n.Content[i]
				if k.Tag != "!!str" || seen[k.Value] {
					return fmt.Errorf("mapping keys must be unique strings")
				}
				seen[k.Value] = true
			}
		}
		for _, child := range n.Content {
			if err := unique(child); err != nil {
				return err
			}
		}
		return nil
	}
	if err := unique(&node); err != nil {
		return nil, err
	}
	var result map[string]any
	if err := node.Decode(&result); err != nil {
		return nil, fmt.Errorf("expected a YAML mapping: %w", err)
	}
	return result, nil
}

func defaults(dir string) (string, map[string]spec, error) {
	text, err := os.ReadFile(filepath.Join(dir, "SKILL.md"))
	if err != nil {
		return "", nil, err
	}
	name := skillName.FindSubmatch(text)
	if name == nil {
		return "", nil, fmt.Errorf("missing skill name")
	}
	var candidates []map[string]spec
	for _, block := range yamlBlock.FindAllSubmatch(text, -1) {
		if !regexp.MustCompile(`(?m)^(model_defaults|agents):`).Match(block[1]) {
			continue
		}
		data, err := readYAML(block[1])
		if err != nil {
			return "", nil, err
		}
		roles := map[string]spec{}
		raw, ok := data["model_defaults"]
		if !ok {
			raw = data["agents"]
		}
		mapping, ok := raw.(map[string]any)
		if !ok {
			return "", nil, fmt.Errorf("expected model defaults mapping")
		}
		if _, exists := data["agents"]; exists {
			main, ok := data["top_level_agent"].(map[string]any)
			if !ok {
				return "", nil, fmt.Errorf("easy loop requires top_level_agent mapping")
			}
			mapping["main"] = main
		}
		for role, value := range mapping {
			fields, ok := value.(map[string]any)
			if !ok {
				return "", nil, fmt.Errorf("%s must be a mapping", role)
			}
			filtered := spec{}
			for key, value := range fields {
				if _, easy := data["agents"]; !easy || key == "model" || key == "reasoning_effort" {
					filtered[key] = value
				}
			}
			roles[role] = filtered
		}
		candidates = append(candidates, roles)
	}
	if len(candidates) != 1 || len(candidates[0]) == 0 {
		return "", nil, fmt.Errorf("expected exactly one model defaults mapping")
	}
	return string(name[1]), candidates[0], nil
}

func validate(fields spec, partial bool) error {
	for key := range fields {
		if key != "model" && key != "reasoning_effort" {
			return fmt.Errorf("expected model and/or reasoning_effort fields")
		}
	}
	model, exists := fields["model"]
	if !partial && !exists {
		return fmt.Errorf("missing model")
	}
	if exists {
		s, ok := model.(string)
		if !ok || !identifier.MatchString(s) {
			return fmt.Errorf("model must be a nonempty model identifier")
		}
	}
	if effort, exists := fields["reasoning_effort"]; exists && effort != nil {
		s, ok := effort.(string)
		if !ok || !strings.Contains("|none|minimal|low|medium|high|xhigh|max|ultra|", "|"+s+"|") {
			return fmt.Errorf("invalid reasoning_effort")
		}
	}
	return nil
}

func invocation(skill, role string, fields spec) (*string, []string, error) {
	model := fields["model"].(string)
	effort, _ := fields["reasoning_effort"].(string)
	if model == "inherit" {
		if (role != "main" && role != "reader") || effort != "" {
			return nil, nil, fmt.Errorf("inherit is only valid for main/reader without effort")
		}
		return nil, []string{}, nil
	}
	cli := ""
	if model == "cli-default" {
		if role != "gemini" || effort != "" {
			return nil, nil, fmt.Errorf("cli-default is only valid for Gemini without effort")
		}
		cli = "agy"
		return &cli, []string{}, nil
	}
	for _, pair := range [][2]string{{"claude-", "claude"}, {"gpt-", "codex"}, {"gemini-", "agy"}} {
		if strings.HasPrefix(model, pair[0]) {
			cli = pair[1]
			break
		}
	}
	if cli == "" {
		return nil, nil, fmt.Errorf("unsupported model prefix: %s", model)
	}
	if strings.HasPrefix(skill, "df-easy-loop-") && cli == "agy" {
		return nil, nil, fmt.Errorf("easy loop supports Claude and Codex models only")
	}
	lanes := map[string]string{"claude": "claude", "codex": "codex", "gemini": "agy"}
	if lane, ok := lanes[role]; ok && lane != cli {
		return nil, nil, fmt.Errorf("model must match selected provider lane")
	}
	allowed := map[string]string{"claude": "|low|medium|high|xhigh|max|", "codex": "|none|minimal|low|medium|high|xhigh|max|ultra|", "agy": "|low|medium|high|"}
	if effort != "" && !strings.Contains(allowed[cli], "|"+effort+"|") {
		return nil, nil, fmt.Errorf("reasoning_effort %s is not supported by %s", effort, cli)
	}
	args := []string{"--model", model}
	if effort != "" {
		if cli == "codex" {
			args = append(args, "-c", `model_reasoning_effort="`+effort+`"`)
		} else {
			args = append(args, "--effort", effort)
		}
	}
	return &cli, args, nil
}

func resolve(dir, project, user string) (map[string]any, error) {
	name, roles, err := defaults(dir)
	if err != nil {
		return nil, err
	}
	sources := map[string]map[string]string{}
	for role, fields := range roles {
		if err := validate(fields, false); err != nil {
			return nil, fmt.Errorf("%s/%s: %w", name, role, err)
		}
		sources[role] = map[string]string{}
		for field := range fields {
			sources[role][field] = "skill default"
		}
	}
	for _, path := range []string{user, filepath.Join(project, ".diffusion/skills/models.yaml")} {
		bytes, err := os.ReadFile(path)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, err
		}
		data, err := readYAML(bytes)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		for key := range data {
			if key != "skills" {
				return nil, fmt.Errorf("%s: expected top-level skills mapping", path)
			}
		}
		raw, exists := data["skills"]
		if !exists {
			continue
		}
		skills, ok := raw.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("expected skills mapping")
		}
		raw, exists = skills[name]
		if !exists {
			continue
		}
		overrides, ok := raw.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("expected role mapping")
		}
		for role, raw := range overrides {
			fields, ok := raw.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("expected model fields")
			}
			target, ok := roles[role]
			if !ok {
				return nil, fmt.Errorf("unknown role %s/%s", name, role)
			}
			if err := validate(fields, true); err != nil {
				return nil, fmt.Errorf("%s: %w", path, err)
			}
			for field, value := range fields {
				target[field] = value
				sources[role][field] = path
			}
		}
	}
	resolved := map[string]any{}
	for role, fields := range roles {
		cli, args, err := invocation(name, role, fields)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", role, err)
		}
		result := map[string]any{"model": fields["model"], "cli": cli, "model_args": args, "sources": sources[role]}
		if value, exists := fields["reasoning_effort"]; exists {
			result["reasoning_effort"] = value
		}
		resolved[role] = result
	}
	return map[string]any{"skill": name, "project_dir": project, "roles": resolved}, nil
}

func absolute(path string) (string, error) {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		path = filepath.Join(home, path[2:])
	}
	return filepath.Abs(path)
}
func run(args []string, out io.Writer) error {
	fs := flag.NewFlagSet("resolve_models", flag.ContinueOnError)
	project := fs.String("project-dir", "", "target project root (required)")
	user := fs.String("user-config", "", "alternate user configuration")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if *project == "" {
		return fmt.Errorf("--project-dir is required because go run -C changes directory")
	}
	root, err := absolute(*project)
	if err != nil {
		return err
	}
	info, err := os.Stat(root)
	if err != nil || !info.IsDir() {
		return fmt.Errorf("project directory does not exist: %s", root)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	if *user == "" {
		*user = filepath.Join(home, ".config/diffusion/skills/models.yaml")
	}
	config, err := absolute(*user)
	if err != nil {
		return err
	}
	dir, err := os.Getwd()
	if err != nil {
		return err
	}
	result, err := resolve(filepath.Clean(filepath.Join(dir, "../..")), root, config)
	if err != nil {
		return err
	}
	encoder := json.NewEncoder(out)
	encoder.SetIndent("", "  ")
	return encoder.Encode(result)
}
func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "Model configuration error:", err)
		os.Exit(2)
	}
}
