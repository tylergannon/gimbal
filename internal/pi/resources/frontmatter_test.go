package resources

import (
	"strings"
	"testing"
)

func TestParseFrontmatter(t *testing.T) {
	frontmatter, body, err := ParseFrontmatter("---\nname: valid-skill\ndescription: A valid skill.\n---\nBody text\n")
	if err != nil {
		t.Fatalf("ParseFrontmatter: %v", err)
	}
	if frontmatter["name"] != "valid-skill" || frontmatter["description"] != "A valid skill." {
		t.Errorf("frontmatter = %+v", frontmatter)
	}
	if body != "Body text" {
		t.Errorf("body = %q", body)
	}
}

func TestParseFrontmatterNoHeader(t *testing.T) {
	frontmatter, body, err := ParseFrontmatter("Just content\n")
	if err != nil {
		t.Fatalf("ParseFrontmatter: %v", err)
	}
	if len(frontmatter) != 0 {
		t.Errorf("frontmatter = %+v", frontmatter)
	}
	if body != "Just content\n" {
		t.Errorf("body = %q", body)
	}
}

func TestParseFrontmatterNormalizesNewlinesAndBOM(t *testing.T) {
	content := "\ufeff---\r\nname: x\r\n---\r\nBody"
	frontmatter, body, err := ParseFrontmatter(content)
	if err != nil {
		t.Fatalf("ParseFrontmatter: %v", err)
	}
	if frontmatter["name"] != "x" {
		t.Errorf("frontmatter = %+v", frontmatter)
	}
	if body != "Body" {
		t.Errorf("body = %q", body)
	}
}

func TestParseFrontmatterInvalidYAML(t *testing.T) {
	_, _, err := ParseFrontmatter("---\ndescription: [unclosed bracket\n---\nBody")
	if err == nil {
		t.Fatal("expected an error for invalid YAML")
	}
	if !strings.Contains(err.Error(), "yaml") {
		t.Logf("error = %v", err)
	}
}

func TestStripFrontmatter(t *testing.T) {
	body := StripFrontmatter("---\nname: x\n---\nBody")
	if body != "Body" {
		t.Errorf("body = %q", body)
	}
}
