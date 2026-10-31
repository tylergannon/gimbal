package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

// A candidate identifies bytes, not product acceptance. The upload artifact's
// own ID and digest are recorded by Actions after upload, outside this content.
type candidateIdentity struct {
	Repository string `json:"repository"`
	SourceSHA  string `json:"source_sha"`
	Tag        string `json:"tag"`
	RunID      uint64 `json:"run_id"`
	RunAttempt uint64 `json:"run_attempt"`
}

type candidateFile struct {
	Name   string `json:"name"`
	SHA256 string `json:"sha256"`
}

type candidateManifest struct {
	SchemaVersion int `json:"schema_version"`
	candidateIdentity
	Files []candidateFile `json:"files"`
}

var candidateRepository = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9-]*/[A-Za-z0-9_][A-Za-z0-9_.-]*$`)
var candidateSHA = regexp.MustCompile(`^[0-9a-f]{40}$`)
var candidateTag = regexp.MustCompile(`^v0\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`)
var candidateDigest = regexp.MustCompile(`^[0-9a-f]{64}$`)

func (id candidateIdentity) validate() error {
	if !candidateRepository.MatchString(id.Repository) || !candidateSHA.MatchString(id.SourceSHA) || !candidateTag.MatchString(id.Tag) || id.RunID == 0 || id.RunAttempt == 0 {
		return errors.New("candidate requires repository owner/name, full lowercase source SHA, v0.x.y tag and positive run ID/attempt")
	}
	return nil
}

// These names match release-cli.sh, release-macos.sh and generate-appcast.sh.
// The first five entries retain the existing publication checksum format.
func candidatePayloads(id candidateIdentity) []string {
	v := strings.TrimPrefix(id.Tag, "v")
	return []string{
		"gimbal_" + v + "_darwin_arm64.tar.gz",
		"gimbal_" + v + "_linux_amd64.tar.gz",
		"gimbal_" + v + "_linux_arm64.tar.gz",
		"macos/Gimbal-" + v + "-macos-arm64.zip",
		"macos/Gimbal-" + v + "-macos-arm64.dmg",
		"macos/appcast.xml",
	}
}

func candidateTree(root *os.Root, names []string) error {
	return fs.WalkDir(root.FS(), ".", func(name string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() && (name == "." || name == "macos") {
			return nil
		}
		if !slices.Contains(names, name) || !entry.Type().IsRegular() {
			return fmt.Errorf("unexpected candidate entry: %s", name)
		}
		return nil
	})
}

func candidateHash(root *os.Root, name string, destination io.Writer) (candidateFile, error) {
	if path.Clean(name) != name || !fs.ValidPath(name) || strings.Contains(name, `\`) {
		return candidateFile{}, fmt.Errorf("unsafe candidate path: %s", name)
	}
	info, err := root.Lstat(name)
	if err != nil {
		return candidateFile{}, err
	}
	if !info.Mode().IsRegular() || info.Size() == 0 {
		return candidateFile{}, fmt.Errorf("candidate file must be nonempty and regular: %s", name)
	}
	file, err := root.Open(name)
	if err != nil {
		return candidateFile{}, err
	}
	hash := sha256.New()
	_, copyErr := io.Copy(io.MultiWriter(hash, destination), file)
	if err := errors.Join(copyErr, file.Close()); err != nil {
		return candidateFile{}, err
	}
	return candidateFile{Name: name, SHA256: hex.EncodeToString(hash.Sum(nil))}, nil
}

func candidateChecksums(files []candidateFile) []byte {
	var text strings.Builder
	for _, file := range files {
		if strings.HasSuffix(file.Name, ".xml") || file.Name == "checksums.txt" {
			continue
		}
		name := file.Name
		if !strings.Contains(name, "/") {
			name = "./" + name
		}
		fmt.Fprintf(&text, "%s  %s\n", file.SHA256, name)
	}
	return []byte(text.String())
}

func candidateAppcast(root *os.Root, id candidateIdentity) error {
	raw, err := root.ReadFile("macos/appcast.xml")
	if err != nil {
		return err
	}
	var feed struct {
		Items []struct {
			Version   string `xml:"http://www.andymatuschak.org/xml-namespaces/sparkle shortVersionString"`
			Enclosure struct {
				URL       string `xml:"url,attr"`
				Length    int64  `xml:"length,attr"`
				Signature string `xml:"http://www.andymatuschak.org/xml-namespaces/sparkle edSignature,attr"`
			} `xml:"enclosure"`
		} `xml:"channel>item"`
	}
	if err := xml.Unmarshal(raw, &feed); err != nil {
		return err
	}
	zip := candidatePayloads(id)[3]
	info, err := root.Stat(zip)
	if err != nil {
		return err
	}
	url := "https://github.com/" + id.Repository + "/releases/download/" + id.Tag + "/" + path.Base(zip)
	if len(feed.Items) != 1 || feed.Items[0].Version != strings.TrimPrefix(id.Tag, "v") || feed.Items[0].Enclosure.URL != url || feed.Items[0].Enclosure.Length != info.Size() || feed.Items[0].Enclosure.Signature == "" {
		return errors.New("appcast must identify this candidate's version, repository, tag and ZIP size and retain its signature")
	}
	return nil
}

func assembleCandidate(input, output string, id candidateIdentity) (manifest candidateManifest, err error) {
	if err := id.validate(); err != nil {
		return manifest, err
	}
	root, err := os.OpenRoot(input)
	if err != nil {
		return manifest, err
	}
	defer func() { _ = root.Close() }()
	names := candidatePayloads(id)
	if err := candidateTree(root, names); err != nil {
		return manifest, err
	}
	if err := candidateAppcast(root, id); err != nil {
		return manifest, err
	}
	// Refuse to overwrite an existing candidate. Failed assembly removes only
	// the new output directory it created, leaving input packages untouched.
	if err := os.Mkdir(output, 0755); err != nil {
		return manifest, err
	}
	defer func() {
		if err != nil {
			_ = os.RemoveAll(output)
		}
	}()
	manifest = candidateManifest{SchemaVersion: 1, candidateIdentity: id}
	for _, name := range names {
		if err := os.MkdirAll(filepath.Join(output, filepath.Dir(name)), 0755); err != nil {
			return manifest, err
		}
		file, err := os.OpenFile(filepath.Join(output, name), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
		if err != nil {
			return manifest, err
		}
		entry, hashErr := candidateHash(root, name, file)
		if err := errors.Join(hashErr, file.Close()); err != nil {
			return manifest, err
		}
		manifest.Files = append(manifest.Files, entry)
	}
	checksums := candidateChecksums(manifest.Files)
	if err := os.WriteFile(filepath.Join(output, "checksums.txt"), checksums, 0644); err != nil {
		return manifest, err
	}
	digest := sha256.Sum256(checksums)
	manifest.Files = append(manifest.Files, candidateFile{Name: "checksums.txt", SHA256: hex.EncodeToString(digest[:])})
	raw, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return manifest, err
	}
	if err := os.WriteFile(filepath.Join(output, "candidate.json"), append(raw, '\n'), 0644); err != nil {
		return manifest, err
	}
	return verifyCandidate(output, id)
}

func verifyCandidate(directory string, expected candidateIdentity) (candidateManifest, error) {
	var manifest candidateManifest
	if err := expected.validate(); err != nil {
		return manifest, err
	}
	root, err := os.OpenRoot(directory)
	if err != nil {
		return manifest, err
	}
	defer func() { _ = root.Close() }()
	names := append(candidatePayloads(expected), "checksums.txt")
	if err := candidateTree(root, append(slices.Clone(names), "candidate.json")); err != nil {
		return manifest, err
	}
	raw, err := root.ReadFile("candidate.json")
	if err != nil {
		return manifest, err
	}
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&manifest); err != nil {
		return manifest, err
	}
	if err := decoder.Decode(new(any)); !errors.Is(err, io.EOF) {
		return manifest, errors.New("candidate manifest contains trailing data")
	}
	if manifest.SchemaVersion != 1 || manifest.candidateIdentity != expected || len(manifest.Files) != len(names) {
		return manifest, errors.New("candidate manifest identity or file set differs from the expected candidate")
	}
	seen := map[string]bool{}
	for _, file := range manifest.Files {
		if !slices.Contains(names, file.Name) || seen[file.Name] || !candidateDigest.MatchString(file.SHA256) {
			return manifest, fmt.Errorf("invalid or duplicate candidate file: %s", file.Name)
		}
		seen[file.Name] = true
		actual, err := candidateHash(root, file.Name, io.Discard)
		if err != nil {
			return manifest, err
		}
		if actual != file {
			return manifest, fmt.Errorf("candidate SHA256 mismatch: %s", file.Name)
		}
	}
	// Check the legacy checksum file too, without rewriting it on consumption.
	checksums, err := root.ReadFile("checksums.txt")
	if err != nil {
		return manifest, err
	}
	ordered := make([]candidateFile, 0, len(names))
	for _, name := range names {
		for _, file := range manifest.Files {
			if file.Name == name {
				ordered = append(ordered, file)
			}
		}
	}
	if string(checksums) != string(candidateChecksums(ordered)) {
		return manifest, errors.New("candidate checksums.txt differs from its publication files")
	}
	return manifest, candidateAppcast(root, expected)
}

func candidateCommand(command string, args []string, stdout io.Writer) error {
	flags := flag.NewFlagSet(command, flag.ContinueOnError)
	var id candidateIdentity
	flags.StringVar(&id.Repository, "repository", "", "Producing repository owner/name")
	flags.StringVar(&id.SourceSHA, "source-sha", "", "Immutable source commit SHA")
	flags.StringVar(&id.Tag, "tag", "", "Existing v0.x.y release tag")
	flags.Uint64Var(&id.RunID, "run-id", 0, "Producing Actions run ID")
	flags.Uint64Var(&id.RunAttempt, "run-attempt", 0, "Producing Actions run attempt")
	input := flags.String("input", "", "Input directory")
	output := flags.String("output", "", "New candidate directory (assembly only)")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 || *input == "" || (command == "assemble-candidate") != (*output != "") {
		return errors.New("use --input and identity flags; assembly also requires a new --output directory")
	}
	var manifest candidateManifest
	var err error
	if command == "assemble-candidate" {
		manifest, err = assembleCandidate(*input, *output, id)
	} else {
		manifest, err = verifyCandidate(*input, id)
	}
	if err != nil {
		return err
	}
	return json.NewEncoder(stdout).Encode(manifest)
}
