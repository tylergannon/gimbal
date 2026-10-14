package web

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tylergannon/devalue/v5"
	generated "github.com/tylergannon/gimbal/internal/skgo"
)

// A native caller has no browser Referrer. Explicit project identity must survive
// a different displayed project, retries, and replacement of the server instance.
func TestRecordingNativeCommandsSurviveInstanceReplacement(t *testing.T) {
	base := t.TempDir()
	a := filepath.Join(base, "a")
	b := filepath.Join(base, "b")
	for _, p := range []string{a, b} {
		if err := os.Mkdir(p, 0755); err != nil {
			t.Fatal(err)
		}
	}
	dir := filepath.Join(base, "instance")
	instance, err := NewInstance(t.Context(), dir, []string{a, b}, WithPort(0))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { instance.cancel(nil); instance.Wait() })
	owner, err := instance.Owner.Project(a)
	if err != nil {
		t.Fatal(err)
	}
	other, err := instance.Owner.Project(b)
	if err != nil {
		t.Fatal(err)
	}
	id := owner.ID()
	call := func(name string, input any) {
		t.Helper()
		remoteID := ""
		for _, r := range generated.Remotes() {
			if r.Module() == "src/lib/transcript.remote.ts" && strings.HasSuffix(r.ID(), "/"+name) {
				remoteID = r.ID()
			}
		}
		if remoteID == "" {
			t.Fatalf("remote %s missing", name)
		}
		encoded, e := devalue.Stringify(input)
		if e != nil {
			t.Fatal(e)
		}
		body, e := json.Marshal(map[string]any{"payload": base64.RawURLEncoding.EncodeToString([]byte(encoded))})
		if e != nil {
			t.Fatal(e)
		}
		origin := "http://" + instance.address
		request, e := http.NewRequest(http.MethodPost, origin+"/_app/remote/"+remoteID, bytes.NewReader(body))
		if e != nil {
			t.Fatal(e)
		}
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("Origin", origin)
		// A displayed project must not change the owner specified by the native command.
		request.Header.Set("Referer", origin+"/projects/"+other.ID()+"/recordings")
		response, e := http.DefaultClient.Do(request)
		if e != nil {
			t.Fatal(e)
		}
		defer func() { _ = response.Body.Close() }()
		raw, e := io.ReadAll(response.Body)
		if e != nil {
			t.Fatal(e)
		}
		var result struct {
			Type string `json:"type"`
		}
		if e = json.Unmarshal(raw, &result); e != nil {
			t.Fatal(e)
		}
		if response.StatusCode != 200 || result.Type != "result" {
			t.Fatalf("%s: status=%d body=%s", name, response.StatusCode, raw)
		}
	}
	for n := range 31 {
		call("openSession", map[string]any{"project": id, "sessionID": fmt.Sprintf("history-%02d", n), "source": "synthetic", "requirements": false})
		call("closeSession", map[string]any{"project": id, "sessionID": fmt.Sprintf("history-%02d", n), "sequence": 0})
	}
	call("markRequirements", map[string]any{"project": id, "id": "history-00"})
	marked, err := owner.Recordings().Get("history-00")
	if err != nil || !marked.Requirements {
		t.Fatalf("mark remote did not persist requirements: %+v %v", marked, err)
	}
	call("openSession", map[string]any{"project": id, "sessionID": "requirements", "source": "synthetic", "requirements": true})
	requirements, err := owner.Recordings().Get("requirements")
	if err != nil || !requirements.Requirements {
		t.Fatalf("requirements Open lost flag: %+v %v", requirements, err)
	}
	call("openSession", map[string]any{"project": id, "sessionID": "active", "source": "synthetic", "requirements": false})
	ordinary, err := owner.Recordings().Get("active")
	if err != nil || ordinary.Requirements {
		t.Fatalf("ordinary Open unexpectedly marked: %+v %v", ordinary, err)
	}
	call("activateSession", map[string]any{"project": id, "sessionID": "active"})
	update := map[string]any{"project": id, "sessionID": "active", "sequence": 1, "finalized": []any{}, "provisional": []any{map[string]any{"startMs": 0, "endMs": 500, "text": "incorrect"}}}
	call("acceptTranscript", update)
	call("acceptTranscript", update)
	update = map[string]any{"project": id, "sessionID": "active", "sequence": 2, "finalized": []any{map[string]any{"startMs": 0, "endMs": 500, "text": "Corrected."}}, "provisional": []any{}}
	call("acceptTranscript", update)
	if len(other.Recordings().List("").Items) != 0 {
		t.Fatal("displayed project received another project's transcript")
	}
	instance.cancel(nil)
	instance.Wait()
	instance, err = NewInstance(t.Context(), dir, nil, WithPort(0))
	if err != nil {
		t.Fatal(err)
	}
	owner, err = instance.Owner.Project(a)
	if err != nil {
		t.Fatal(err)
	}
	saved, err := owner.Recordings().Get("active")
	if err != nil {
		t.Fatal(err)
	}
	if saved.Status != "interrupted" || saved.Sequence != 2 || len(saved.Finalized) != 1 || saved.Finalized[0].Text != "Corrected." || len(saved.Provisional) != 0 {
		t.Fatalf("recovered %+v", saved)
	}
	first := owner.Recordings().List("")
	second := owner.Recordings().List(first.Next)
	if len(first.Items) != 25 || len(second.Items) != 8 {
		t.Fatalf("history pages: %d %d", len(first.Items), len(second.Items))
	}
}
