package projectfiles

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

func TestConfiguredR2AndTemporaryLinks(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := os.MkdirAll(filepath.Join(home, ".gimbal"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".gimbal", "config.json"), []byte(`{"uploadArtifact":{"provider":"r2","r2":{"accountId":"account","accessKeyId":"key","secretAccessKey":"secret","bucket":"configured"}}}`), 0600); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"GIMBAL_R2_ACCOUNT_ID", "GIMBAL_R2_ACCESS_KEY_ID", "GIMBAL_R2_SECRET_ACCESS_KEY", "GIMBAL_R2_BUCKET"} {
		t.Setenv(name, "")
	}
	_, bucket, e := client()
	if e != nil || bucket != "configured" {
		t.Fatalf("configured bucket: %q %v", bucket, e)
	}
	t.Setenv("GIMBAL_R2_BUCKET", "override")
	_, bucket, e = client()
	if e != nil || bucket != "override" {
		t.Fatal("environment precedence")
	}
	link, e := ViewURL(context.Background(), Document{Bucket: "configured", Key: "projects/p/object/report.txt"})
	if e != nil {
		t.Fatal(e)
	}
	if !bytes.Contains([]byte(link), []byte("X-Amz-Expires=900")) || !bytes.Contains([]byte(link), []byte("projects/p/object/report.txt")) {
		t.Fatal("missing temporary object link")
	}
}
func TestR2OriginalBytesAndObjectIdentity(t *testing.T) {
	if os.Getenv("GIMBAL_TEST_R2") != "1" {
		t.Skip("explicit live R2 check")
	}
	ctx := context.Background()
	dir := t.TempDir()
	fixtures := map[string][]byte{"notes.md": []byte("# Workspace reference\n\nOriginal project document.\n"), "notes.txt": []byte("Project transcript reference.\n")}
	for name, data := range fixtures {
		f, e := Upload(ctx, dir, "workspace-check", name, data)
		if e != nil {
			t.Fatal(e)
		}
		c, _, e := client()
		if e != nil {
			t.Fatal(e)
		}
		defer func() {
			if _, err := c.DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: aws.String(f.Bucket), Key: aws.String(f.Key)}); err != nil {
				t.Error(err)
			}
		}()
		metadata, e := Get(dir, f.ID)
		if e != nil || metadata.Key != f.Key || metadata.Bucket != f.Bucket {
			t.Fatal("lost object identity")
		}
		body, _, _, e := Body(ctx, metadata, "")
		if e != nil {
			t.Fatal(e)
		}
		actual, e := io.ReadAll(body)
		if err := body.Close(); err != nil {
			t.Fatal(err)
		}
		if e != nil || !bytes.Equal(actual, data) {
			t.Fatal("changed original bytes")
		}
		link, e := ViewURL(ctx, metadata)
		if e != nil {
			t.Fatal(e)
		}
		response, e := http.Get(link)
		if e != nil {
			t.Fatal(e)
		}
		actual, e = io.ReadAll(response.Body)
		if err := response.Body.Close(); err != nil {
			t.Fatal(err)
		}
		if response.StatusCode != 200 || e != nil || !bytes.Equal(actual, data) {
			t.Fatal("temporary link did not serve original bytes")
		}
	}
	records, e := List(dir)
	if e != nil || len(records) != len(fixtures) {
		t.Fatal("lost project association")
	}
	_, e = json.Marshal(records)
	if e != nil {
		t.Fatal(e)
	}
}
