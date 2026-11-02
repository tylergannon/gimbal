package main

import (
	"os"
	"slices"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

type candidateWorkflowStep struct {
	ID   string            `yaml:"id"`
	Uses string            `yaml:"uses"`
	Run  string            `yaml:"run"`
	With map[string]string `yaml:"with"`
	Env  map[string]string `yaml:"env"`
}

func TestWorkflowAssemblesWithoutPublicationAndConsumesTheFixedArtifact(t *testing.T) {
	raw, err := os.ReadFile("../../../.github/workflows/release.yml")
	if err != nil {
		t.Fatal(err)
	}
	var workflow struct {
		On struct {
			Dispatch struct {
				Inputs map[string]struct {
					Default string `yaml:"default"`
				} `yaml:"inputs"`
			} `yaml:"workflow_dispatch"`
		} `yaml:"on"`
		Jobs map[string]struct {
			If      string                  `yaml:"if"`
			Needs   yaml.Node               `yaml:"needs"`
			Outputs map[string]string       `yaml:"outputs"`
			Steps   []candidateWorkflowStep `yaml:"steps"`
		} `yaml:"jobs"`
	}
	if err := yaml.Unmarshal(raw, &workflow); err != nil {
		t.Fatal(err)
	}
	if workflow.On.Dispatch.Inputs["publish"].Default != "true" {
		t.Fatal("current publication default changed")
	}
	candidate, publish := workflow.Jobs["candidate"], workflow.Jobs["publish"]
	var candidateNeeds, publishNeeds []string
	if err := candidate.Needs.Decode(&candidateNeeds); err != nil {
		t.Fatal(err)
	}
	if err := publish.Needs.Decode(&publishNeeds); err != nil {
		t.Fatal(err)
	}
	if candidate.If != "" || !slices.Contains(candidateNeeds, "linux-cli") || !slices.Contains(candidateNeeds, "macos-release") {
		t.Fatal("build-only candidate must assemble all platform outputs without a publish condition")
	}
	if publish.If != "inputs.publish" || !slices.Contains(publishNeeds, "qualify") || !slices.Contains(publishNeeds, "candidate") {
		t.Fatal("publisher must preserve the source gate and consume an assembled candidate")
	}
	var upload, assemble candidateWorkflowStep
	for _, step := range candidate.Steps {
		if step.ID == "upload" {
			upload = step
		}
		if step.ID == "assemble" {
			assemble = step
		}
	}
	if !strings.HasPrefix(upload.Uses, "actions/upload-artifact@") || upload.With["name"] != candidate.Outputs["artifact-name"] || !strings.Contains(upload.With["name"], "github.run_id") || !strings.Contains(upload.With["name"], "github.run_attempt") || upload.With["path"] != "candidate/" || upload.With["retention-days"] != "2" {
		t.Fatal("candidate upload must identify one run/attempt, include the complete directory and retain the current lifetime")
	}
	for _, name := range []string{"artifact-id", "artifact-digest", "artifact-url"} {
		if candidate.Outputs[name] != "${{ steps.upload.outputs."+name+" }}" {
			t.Fatalf("immutable upload %s is not exposed", name)
		}
	}
	if !strings.Contains(assemble.Run, `git rev-parse "refs/tags/$CANDIDATE_TAG^{commit}"`) || !strings.Contains(assemble.Run, "assemble-candidate") || !strings.Contains(assemble.Run, "GITHUB_OUTPUT") {
		t.Fatal("assembly must bind the checked source/tag and record its producing attempt")
	}
	var downloads, verifies int
	for _, step := range publish.Steps {
		if strings.HasPrefix(step.Uses, "actions/checkout@") && step.With["repository"] == "" && step.With["ref"] != "${{ github.sha }}" {
			t.Fatal("publisher must use immutable source")
		}
		if strings.HasPrefix(step.Uses, "actions/download-artifact@") {
			downloads++
			if step.With["artifact-ids"] != "${{ needs.candidate.outputs.artifact-id }}" || step.With["merge-multiple"] != "true" || step.With["name"] != "" || step.With["pattern"] != "" {
				t.Fatal("publisher must consume the fixed artifact ID directly into dist")
			}
		}
		if strings.Contains(step.Run, "verify-candidate") {
			verifies++
			if step.Env["CANDIDATE_RUN_ATTEMPT"] != "${{ needs.candidate.outputs.run-attempt }}" || candidate.Outputs["run-attempt"] != "${{ steps.assemble.outputs.run-attempt }}" {
				t.Fatal("publish retries must verify the candidate's producing attempt")
			}
		}
		if strings.Contains(step.Run, "sha256sum") || strings.Contains(step.Run, "assemble-candidate") {
			t.Fatal("publisher must not reconstruct fixed candidate content")
		}
	}
	if downloads != 1 || verifies != 1 {
		t.Fatal("publisher needs one fixed-artifact download and identity/content verification")
	}
}
