// Command modelcatalog refreshes everything Gimbal generates from provider
// model catalogs: the private price table (models.dev), the model aliases,
// and the Diffusion Router's model limits (the router's own catalog).
//
// Run it with `just models`. The router catalog needs DIFFUSION_API_KEY.
// Every alias family is listed in `families`; the newest catalog version of
// each becomes its unversioned alias, so a new model needs no edit here.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"go/format"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	catalogURL   = "https://models.dev/api.json"
	diffusionURL = "https://router.diffusion.io/v1/models"
)

var providers = []string{"anthropic", "openai", "google"}

// family is one alias series: the models of a provider whose IDs match
// pattern, with the version captured by its single group. A dash between
// version digits, as in claude-opus-5-5, reads as a dot.
type family struct {
	name, provider string
	pattern        *regexp.Regexp
}

var families = []family{
	{"haiku", "anthropic", regexp.MustCompile(`^claude-haiku-(\d+(?:-\d)?)$`)},
	{"sonnet", "anthropic", regexp.MustCompile(`^claude-sonnet-(\d+(?:-\d)?)$`)},
	{"opus", "anthropic", regexp.MustCompile(`^claude-opus-(\d+(?:-\d)?)$`)},
	{"fable", "anthropic", regexp.MustCompile(`^claude-fable-(\d+(?:-\d)?)$`)},
	{"luna", "openai", regexp.MustCompile(`^gpt-(\d+(?:\.\d+)?)-luna$`)},
	{"terra", "openai", regexp.MustCompile(`^gpt-(\d+(?:\.\d+)?)-terra$`)},
	{"sol", "openai", regexp.MustCompile(`^gpt-(\d+(?:\.\d+)?)-sol$`)},
	{"astra", "openai", regexp.MustCompile(`^gpt-(\d+(?:\.\d+)?)-astra$`)},
	{"gpt", "openai", regexp.MustCompile(`^gpt-(\d+(?:\.\d+)?)-sol$`)},
	{"deepseek-flash", "diffusion", regexp.MustCompile(`^deepseek-(\d+(?:\.\d+)?)-flash$`)},
	{"glm", "diffusion", regexp.MustCompile(`^glm-(\d+(?:\.\d+)?)$`)},
	{"glm-flash", "diffusion", regexp.MustCompile(`^glm-(\d+(?:\.\d+)?)-flash$`)},
	{"glm-vision", "diffusion", regexp.MustCompile(`^glm-(\d+(?:\.\d+)?)-vision$`)},
}

type catalog map[string]provider

type provider struct {
	Models map[string]model `json:"models"`
}

type model struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	ReleaseDate string `json:"release_date"`
	LastUpdated string `json:"last_updated"`
	Cost        *cost  `json:"cost"`
}

type cost struct {
	Input      *float64 `json:"input"`
	Output     *float64 `json:"output"`
	CacheRead  *float64 `json:"cache_read"`
	CacheWrite *float64 `json:"cache_write"`
}

type row struct {
	Provider    string
	ID          string
	Name        string
	ReleaseDate string
	LastUpdated string
	Input       float64
	Output      float64
	CacheRead   float64
	CacheWrite  float64
}

// diffusionModel is one router model with the metadata Gimbal needs. The
// router's response is a `data` array; only chat models are kept.
type diffusionModel struct {
	ID              string   `json:"id"`
	ContextWindow   int      `json:"context_window"`
	MaxInputTokens  int      `json:"max_input_tokens"`
	MaxOutputTokens int      `json:"max_output_tokens"`
	Capabilities    []string `json:"capabilities"`
}

type diffusionCatalog struct {
	Data []diffusionModel `json:"data"`
}

// alias is one family member: the catalog version and its native model ID.
type alias struct {
	Family, Provider, Version, Native string
}

func main() {
	prices := flag.String("prices", "internal/observation/model_prices_gen.go", "generated price table")
	aliases := flag.String("aliases", "internal/modelalias/aliases_gen.go", "generated model aliases")
	limits := flag.String("limits", "pi/diffusion_models_gen.go", "generated Diffusion model limits")
	url := flag.String("url", catalogURL, "models.dev catalog URL")
	router := flag.String("router-url", diffusionURL, "Diffusion Router model catalog URL")
	flag.Parse()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	key := strings.TrimSpace(os.Getenv("DIFFUSION_API_KEY"))
	if key == "" {
		fatal(fmt.Errorf("DIFFUSION_API_KEY is required to read the Diffusion Router catalog"))
	}
	raw, err := fetch(ctx, *url, "")
	if err != nil {
		fatal(err)
	}
	routerRaw, err := fetch(ctx, *router, key)
	if err != nil {
		fatal(err)
	}
	rows, ids, err := decode(raw)
	if err != nil {
		fatal(err)
	}
	chat, err := decodeDiffusion(routerRaw)
	if err != nil {
		fatal(err)
	}
	for _, m := range chat {
		ids["diffusion"] = append(ids["diffusion"], m.ID)
	}
	members, err := resolveAliases(ids)
	if err != nil {
		fatal(err)
	}
	outputs := []struct {
		path   string
		render func() ([]byte, error)
	}{
		{*prices, func() ([]byte, error) { return render(rows) }},
		{*aliases, func() ([]byte, error) { return renderAliases(members) }},
		{*limits, func() ([]byte, error) { return renderLimits(chat) }},
	}
	for _, output := range outputs {
		source, err := output.render()
		if err != nil {
			fatal(err)
		}
		if err := os.MkdirAll(filepath.Dir(output.path), 0o755); err != nil {
			fatal(fmt.Errorf("create output directory: %w", err))
		}
		if err := os.WriteFile(output.path, source, 0o644); err != nil {
			fatal(fmt.Errorf("write %s: %w", output.path, err))
		}
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "modelcatalog:", err)
	os.Exit(1)
}

// fetch retries a few times: the Diffusion Router answers 502 intermittently.
func fetch(ctx context.Context, url, bearer string) ([]byte, error) {
	var err error
	for attempt := range 4 {
		var raw []byte
		if raw, err = fetchOnce(ctx, url, bearer); err == nil {
			return raw, nil
		}
		select {
		case <-ctx.Done():
			return nil, err
		case <-time.After(time.Duration(attempt+1) * 5 * time.Second):
		}
	}
	return nil, err
}

func fetchOnce(ctx context.Context, url, bearer string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	response, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download %s: %w", url, err)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download %s: %s", url, response.Status)
	}
	raw, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", url, err)
	}
	return raw, nil
}

// decode returns the priced rows and every model ID per provider, priced or not.
func decode(raw []byte) ([]row, map[string][]string, error) {
	var input catalog
	if err := json.Unmarshal(raw, &input); err != nil {
		return nil, nil, fmt.Errorf("decode catalog: %w", err)
	}

	var rows []row
	ids := map[string][]string{}
	for _, providerID := range providers {
		provider, ok := input[providerID]
		if !ok {
			return nil, nil, fmt.Errorf("catalog does not contain %q", providerID)
		}
		for _, model := range provider.Models {
			if model.ID == "" {
				continue
			}
			ids[providerID] = append(ids[providerID], model.ID)
			if model.Cost == nil || model.Cost.Input == nil || model.Cost.Output == nil {
				continue
			}
			entry := row{
				Provider: providerID, ID: model.ID, Name: model.Name,
				ReleaseDate: model.ReleaseDate, LastUpdated: model.LastUpdated,
				Input: *model.Cost.Input, Output: *model.Cost.Output,
			}
			if model.Cost.CacheRead != nil {
				entry.CacheRead = *model.Cost.CacheRead
			}
			if model.Cost.CacheWrite != nil {
				entry.CacheWrite = *model.Cost.CacheWrite
			}
			rows = append(rows, entry)
		}
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Provider != rows[j].Provider {
			return rows[i].Provider < rows[j].Provider
		}
		return rows[i].ID < rows[j].ID
	})
	return rows, ids, nil
}

// decodeDiffusion keeps the router's chat models and requires each to carry
// complete limits, so a partial response cannot silently edit configuration.
func decodeDiffusion(raw []byte) ([]diffusionModel, error) {
	var input diffusionCatalog
	if err := json.Unmarshal(raw, &input); err != nil {
		return nil, fmt.Errorf("decode Diffusion catalog: %w", err)
	}
	var chat []diffusionModel
	for _, m := range input.Data {
		if !slices.Contains(m.Capabilities, "chat") {
			continue
		}
		if m.ID == "" || m.ContextWindow <= 0 || m.MaxInputTokens <= 0 || m.MaxOutputTokens <= 0 {
			return nil, fmt.Errorf("diffusion model %q has incomplete metadata", m.ID)
		}
		chat = append(chat, m)
	}
	if len(chat) == 0 {
		return nil, fmt.Errorf("diffusion catalog lists no chat models")
	}
	sort.Slice(chat, func(i, j int) bool { return chat[i].ID < chat[j].ID })
	return chat, nil
}

// resolveAliases matches every family against its provider's model IDs.
func resolveAliases(ids map[string][]string) ([]alias, error) {
	var members []alias
	for _, f := range families {
		found := 0
		for _, id := range ids[f.provider] {
			match := f.pattern.FindStringSubmatch(id)
			if match == nil {
				continue
			}
			found++
			native := id
			if f.provider == "diffusion" {
				native = "diffusion/" + id
			}
			members = append(members, alias{f.name, f.provider, strings.ReplaceAll(match[1], "-", "."), native})
		}
		if found == 0 {
			return nil, fmt.Errorf("no %s model matches alias family %q", f.provider, f.name)
		}
	}
	sort.Slice(members, func(i, j int) bool {
		if members[i].Family != members[j].Family {
			return members[i].Family < members[j].Family
		}
		return members[i].Version < members[j].Version
	})
	return members, nil
}

func render(rows []row) ([]byte, error) {
	var out bytes.Buffer
	out.WriteString("// Code generated by modelcatalog; DO NOT EDIT.\n\n")
	out.WriteString("package observation\n\n")
	out.WriteString("var modelPrices = []modelPrice{\n")
	for _, row := range rows {
		fmt.Fprintf(&out, "\t{provider: %s, id: %s, name: %s, releaseDate: %s, lastUpdated: %s, input: %s, output: %s, cacheRead: %s, cacheWrite: %s},\n",
			quote(row.Provider), quote(row.ID), quote(row.Name), quote(row.ReleaseDate), quote(row.LastUpdated),
			number(row.Input), number(row.Output), number(row.CacheRead), number(row.CacheWrite))
	}
	out.WriteString("}\n")
	return formatted(&out)
}

// renderAliases writes each family's versions. The package derives the
// unversioned alias from the newest one, so nothing here names "latest".
func renderAliases(members []alias) ([]byte, error) {
	var out bytes.Buffer
	out.WriteString("// Code generated by modelcatalog; DO NOT EDIT.\n\n")
	out.WriteString("package modelalias\n\n")
	out.WriteString("var generatedFamilies = []familyVersion{\n")
	for _, m := range members {
		fmt.Fprintf(&out, "\t{family: %s, provider: %s, version: %s, native: %s},\n",
			quote(m.Family), quote(m.Provider), quote(m.Version), quote(m.Native))
	}
	out.WriteString("}\n")
	return formatted(&out)
}

func renderLimits(models []diffusionModel) ([]byte, error) {
	var out bytes.Buffer
	out.WriteString("// Code generated by modelcatalog; DO NOT EDIT.\n\n")
	out.WriteString("package pi\n\n")
	out.WriteString("// diffusionLimits are the router's advertised limits, used when its live\n// catalog omits a model Gimbal names.\n")
	out.WriteString("var diffusionLimits = map[string]diffusionLimit{\n")
	for _, m := range models {
		fmt.Fprintf(&out, "\t%s: {contextWindow: %d, maxTokens: %d, vision: %t},\n",
			quote(m.ID), m.ContextWindow, m.MaxOutputTokens, slices.Contains(m.Capabilities, "vision"))
	}
	out.WriteString("}\n")
	return formatted(&out)
}

func formatted(out *bytes.Buffer) ([]byte, error) {
	source, err := format.Source(out.Bytes())
	if err != nil {
		return nil, fmt.Errorf("format generated source: %w", err)
	}
	return source, nil
}

func quote(value string) string { return strconv.Quote(value) }

func number(value float64) string { return strconv.FormatFloat(value, 'g', -1, 64) }
