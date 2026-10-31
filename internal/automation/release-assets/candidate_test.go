package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var testCandidate = candidateIdentity{
	Repository: "tylergannon/gimbal", SourceSHA: strings.Repeat("a", 40),
	Tag: "v0.20.2", RunID: 41243685402, RunAttempt: 2,
}

func candidateFixture(t *testing.T) (string, string) {
	t.Helper()
	base := t.TempDir()
	input, output := filepath.Join(base, "input"), filepath.Join(base, "candidate")
	for _, name := range candidatePayloads(testCandidate) {
		writeAsset(t, input, name, "opaque signed package bytes: "+name)
	}
	zip := candidatePayloads(testCandidate)[3]
	info, err := os.Stat(filepath.Join(input, zip))
	if err != nil {
		t.Fatal(err)
	}
	feed := fmt.Sprintf(`<rss xmlns:sparkle="http://www.andymatuschak.org/xml-namespaces/sparkle"><channel><item><sparkle:shortVersionString>0.20.2</sparkle:shortVersionString><enclosure url="https://github.com/tylergannon/gimbal/releases/download/v0.20.2/%s" length="%d" sparkle:edSignature="opaque existing signature"/></item></channel></rss>`, filepath.Base(zip), info.Size())
	writeAsset(t, input, "macos/appcast.xml", feed)
	return input, output
}

func assembledFixture(t *testing.T) (string, candidateManifest) {
	t.Helper()
	input, output := candidateFixture(t)
	manifest, err := assembleCandidate(input, output, testCandidate)
	if err != nil {
		t.Fatal(err)
	}
	return output, manifest
}

func saveCandidateManifest(t *testing.T, directory string, manifest candidateManifest) {
	t.Helper()
	raw, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	writeAsset(t, directory, "candidate.json", string(raw))
}

func TestCandidatePreservesPayloadBytesAndBindsFinalPublicationFiles(t *testing.T) {
	input, output := candidateFixture(t)
	manifest, err := assembleCandidate(input, output, testCandidate)
	if err != nil {
		t.Fatal(err)
	}
	if manifest.SchemaVersion != 1 || manifest.candidateIdentity != testCandidate || len(manifest.Files) != 7 {
		t.Fatalf("manifest identity/file set: %+v", manifest)
	}
	for _, entry := range manifest.Files {
		raw := []byte(readAsset(t, output, entry.Name))
		digest := sha256.Sum256(raw)
		if entry.SHA256 != hex.EncodeToString(digest[:]) {
			t.Fatalf("wrong digest for %s", entry.Name)
		}
		if entry.Name != "checksums.txt" && string(raw) != readAsset(t, input, entry.Name) {
			t.Fatalf("payload was rewritten: %s", entry.Name)
		}
	}
	checksums := readAsset(t, output, "checksums.txt")
	if strings.Count(checksums, "\n") != 5 || !strings.Contains(checksums, "  ./gimbal_0.20.2_linux_amd64.tar.gz\n") || !strings.Contains(checksums, "  macos/Gimbal-0.20.2-macos-arm64.dmg\n") {
		t.Fatalf("publication checksums: %s", checksums)
	}
	if _, err := verifyCandidate(output, testCandidate); err != nil {
		t.Fatal(err)
	}
}

func TestCandidateRejectsEachMissingOrTamperedPublicationFile(t *testing.T) {
	for _, name := range append(candidatePayloads(testCandidate), "checksums.txt", "candidate.json") {
		for _, missing := range []bool{true, false} {
			t.Run(fmt.Sprintf("%s/missing=%v", name, missing), func(t *testing.T) {
				directory, _ := assembledFixture(t)
				if missing {
					if err := os.Remove(filepath.Join(directory, name)); err != nil {
						t.Fatal(err)
					}
				} else {
					writeAsset(t, directory, name, "tampered")
				}
				if _, err := verifyCandidate(directory, testCandidate); err == nil {
					t.Fatal("missing/tampered candidate accepted")
				}
			})
		}
	}
}

func TestCandidateRejectsMismatchedIdentityAndManifestShapes(t *testing.T) {
	for _, name := range []string{"repository", "source", "tag", "run", "attempt", "schema", "duplicate", "missing", "traversal", "absolute", "backslash", "bad digest", "unknown field", "trailing data"} {
		t.Run(name, func(t *testing.T) {
			directory, manifest := assembledFixture(t)
			switch name {
			case "repository":
				manifest.Repository = "other/gimbal"
			case "source":
				manifest.SourceSHA = strings.Repeat("b", 40)
			case "tag":
				manifest.Tag = "v0.20.3"
			case "run":
				manifest.RunID++
			case "attempt":
				manifest.RunAttempt++
			case "schema":
				manifest.SchemaVersion++
			case "duplicate":
				manifest.Files[1] = manifest.Files[0]
			case "missing":
				manifest.Files = manifest.Files[:6]
			case "traversal":
				manifest.Files[0].Name = "../outside"
			case "absolute":
				manifest.Files[0].Name = "/tmp/outside"
			case "backslash":
				manifest.Files[0].Name = `macos\outside`
			case "bad digest":
				manifest.Files[0].SHA256 = "abc"
			}
			saveCandidateManifest(t, directory, manifest)
			if name == "unknown field" {
				writeAsset(t, directory, "candidate.json", strings.Replace(readAsset(t, directory, "candidate.json"), `"schema_version":1`, `"schema_version":1,"qualified":true`, 1))
			}
			if name == "trailing data" {
				writeAsset(t, directory, "candidate.json", readAsset(t, directory, "candidate.json")+`{}`)
			}
			if _, err := verifyCandidate(directory, testCandidate); err == nil {
				t.Fatal("mismatched/ambiguous manifest accepted")
			}
		})
	}
}

func TestCandidateRefusesInvalidIdentityBeforeCreatingOutput(t *testing.T) {
	for _, name := range []string{"repository", "source", "tag", "run", "attempt"} {
		t.Run(name, func(t *testing.T) {
			input, output := candidateFixture(t)
			id := testCandidate
			switch name {
			case "repository":
				id.Repository = "../gimbal"
			case "source":
				id.SourceSHA = "main"
			case "tag":
				id.Tag = "../v0.20.2"
			case "run":
				id.RunID = 0
			case "attempt":
				id.RunAttempt = 0
			}
			if _, err := assembleCandidate(input, output, id); err == nil {
				t.Fatal("invalid identity accepted")
			}
			if _, err := os.Stat(output); !os.IsNotExist(err) {
				t.Fatalf("invalid identity created output: %v", err)
			}
		})
	}
}

func TestCandidateRejectsBadInputsWithoutLeavingPartialOutput(t *testing.T) {
	for _, name := range []string{"missing", "empty", "wrong version", "extra", "symlink", "directory", "feed version", "feed repository", "feed ZIP", "feed length", "feed signature"} {
		t.Run(name, func(t *testing.T) {
			input, output := candidateFixture(t)
			asset := candidatePayloads(testCandidate)[0]
			full := filepath.Join(input, asset)
			switch name {
			case "missing", "symlink", "directory":
				if err := os.Remove(full); err != nil {
					t.Fatal(err)
				}
				if name == "symlink" {
					if err := os.Symlink(filepath.Join(t.TempDir(), "outside"), full); err != nil {
						t.Fatal(err)
					}
				}
				if name == "directory" {
					if err := os.Mkdir(full, 0700); err != nil {
						t.Fatal(err)
					}
				}
			case "empty":
				writeAsset(t, input, asset, "")
			case "wrong version":
				if err := os.Rename(full, strings.Replace(full, "0.20.2", "0.20.3", 1)); err != nil {
					t.Fatal(err)
				}
			case "extra":
				writeAsset(t, input, "unexpected.tar.gz", "other package")
			default:
				feed := readAsset(t, input, "macos/appcast.xml")
				switch name {
				case "feed version":
					feed = strings.Replace(feed, ">0.20.2<", ">0.20.3<", 1)
				case "feed repository":
					feed = strings.Replace(feed, "tylergannon/gimbal", "other/gimbal", 1)
				case "feed ZIP":
					feed = strings.Replace(feed, "macos-arm64.zip", "other.zip", 1)
				case "feed length":
					feed = strings.Replace(feed, `length="`, `length="999`, 1)
				case "feed signature":
					feed = strings.Replace(feed, "opaque existing signature", "", 1)
				}
				writeAsset(t, input, "macos/appcast.xml", feed)
			}
			if _, err := assembleCandidate(input, output, testCandidate); err == nil {
				t.Fatal("invalid input accepted")
			}
			if _, err := os.Stat(output); !os.IsNotExist(err) {
				t.Fatalf("partial candidate left after failure: %v", err)
			}
		})
	}
}

func TestCandidateRefusesOverwriteAndRejectsRehashedFalseChecksums(t *testing.T) {
	input, output := candidateFixture(t)
	if err := os.Mkdir(output, 0700); err != nil {
		t.Fatal(err)
	}
	writeAsset(t, output, "sentinel", "preserve")
	if _, err := assembleCandidate(input, output, testCandidate); err == nil {
		t.Fatal("existing candidate overwritten")
	}
	if readAsset(t, output, "sentinel") != "preserve" {
		t.Fatal("existing output changed")
	}
	directory, manifest := assembledFixture(t)
	writeAsset(t, directory, "checksums.txt", "misleading checksums\n")
	digest := sha256.Sum256([]byte("misleading checksums\n"))
	manifest.Files[6].SHA256 = hex.EncodeToString(digest[:])
	saveCandidateManifest(t, directory, manifest)
	if _, err := verifyCandidate(directory, testCandidate); err == nil {
		t.Fatal("misleading checksum file accepted after manifest rehash")
	}
}

func TestCandidateCommandOutputAndRejectedArguments(t *testing.T) {
	input, output := candidateFixture(t)
	args := []string{"--repository", testCandidate.Repository, "--source-sha", testCandidate.SourceSHA, "--tag", testCandidate.Tag, "--run-id", "41243685402", "--run-attempt", "2", "--input", input}
	var stdout bytes.Buffer
	if err := candidateCommand("assemble-candidate", append(args, "--output", output), &stdout); err != nil {
		t.Fatal(err)
	}
	var result candidateManifest
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil || result.candidateIdentity != testCandidate {
		t.Fatalf("stdout is not the candidate identity: %s %v", stdout.String(), err)
	}
	args[len(args)-1] = output
	stdout.Reset()
	if err := candidateCommand("verify-candidate", args, &stdout); err != nil {
		t.Fatal(err)
	}
	for _, bad := range [][]string{nil, {"--input", output}, append(append([]string{}, args...), "--output", output), append(append([]string{}, args...), "extra")} {
		stdout.Reset()
		if err := candidateCommand("verify-candidate", bad, &stdout); err == nil || stdout.Len() != 0 {
			t.Fatalf("bad arguments produced success output: %v %q %v", bad, stdout.String(), err)
		}
	}
}
