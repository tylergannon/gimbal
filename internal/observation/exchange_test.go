package observation

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tylergannon/polytype"
)

func TestExchangeMarkupIsDerivedFromText(t *testing.T) {
	text := "**Startup** uses `Application.swift`. See [quickstart](https://example.com/docs).\n\n```json\n{\"bold\": \"**literal**\", \"tag\": \"<script>\"}\n```\n\n<script>alert(1)</script>\n\n[unsafe](javascript:alert(1))"
	message := InteractiveMessage{Role: "assistant", Message: text, HTML: polytype.Optional[string]{Value: `<img src=x onerror="alert(1)">`, Present: true}}
	for range 2 { // Also exercise reserialization of a saved checkpoint.
		encoded, err := json.Marshal(renderExchange(InteractiveRow{Messages: []InteractiveMessage{message}}).Messages[0])
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(encoded, &message); err != nil {
			t.Fatal(err)
		}
		for _, want := range []string{"<strong>Startup</strong>", "<code>Application.swift</code>", `href="https://example.com/docs"`, `<pre><code class="language-json">`, "**literal**", "&lt;script&gt;"} {
			if !strings.Contains(message.HTML.Value, want) {
				t.Errorf("markup missing %q: %s", want, message.HTML.Value)
			}
		}
		for _, forbidden := range []string{"<script>", "onerror", "javascript:"} {
			if strings.Contains(message.HTML.Value, forbidden) {
				t.Errorf("markup contains %q: %s", forbidden, message.HTML.Value)
			}
		}
		if message.Message != text {
			t.Fatal("serialization changed literal source text")
		}
	}
}

func TestOldExchangeSnapshotsAndLiveDeltasRenderAtTheBrowserBoundary(t *testing.T) {
	row := InteractiveRow{QuestionID: "question", Messages: []InteractiveMessage{{Role: "assistant", Message: "**Current text**", HTML: polytype.Optional[string]{Present: true, Value: "<script>staleHTML()</script>"}}}}
	store := newStore(nil, "run", "conversation", t.TempDir())
	store.interactions["question"] = &row
	snapshot := store.Snapshot()
	if got := snapshot.Interactions["question"].Messages[0].HTML.Value; got != "<p><strong>Current text</strong></p>\n" {
		t.Fatalf("snapshot markup: %q", got)
	}
	if row.Messages[0].HTML.Value != "<script>staleHTML()</script>" {
		t.Fatal("rendering mutated the retained source")
	}
	// This is an old row frame without safe server markup. The browser sees
	// rendered prose whether the delta is new, replayed or from a remote host.
	delta := Delta{Stream: "stream", Position: 42, Frames: []Frame{{Name: FrameRow, Data: mustMarshal(rowFrame{Table: tableInteractions, Key: "question", Row: row})}}}
	recorder := httptest.NewRecorder()
	if err := writeDelta(recorder, http.NewResponseController(recorder), delta); err != nil {
		t.Fatal(err)
	}
	encoded := strings.Split(strings.Split(recorder.Body.String(), "data: ")[1], "\n\n")[0]
	var delivered Delta
	if err := json.Unmarshal([]byte(encoded), &delivered); err != nil {
		t.Fatal(err)
	}
	var frame struct {
		Key string         `json:"key"`
		Row InteractiveRow `json:"row"`
	}
	if err := json.Unmarshal(delivered.Frames[0].Data, &frame); err != nil {
		t.Fatal(err)
	}
	if delivered.Stream != "stream" || delivered.Position != 42 || frame.Key != "question" || frame.Row.Messages[0].HTML.Value != snapshot.Interactions["question"].Messages[0].HTML.Value {
		t.Fatalf("live delivery changed identity or markup: %+v, %+v", delivered, frame)
	}
	if !strings.Contains(string(delta.Frames[0].Data), "staleHTML") {
		t.Fatal("browser formatting mutated the durable delta")
	}
}

func TestExchangeKeepsUserTextAndJSONLiteral(t *testing.T) {
	for _, text := range []string{`{"text":"**literal** <img src=x>","path":"a_b"}`, "{\n  \"text\": \"a_b\"\n}"} {
		encoded, err := json.Marshal(renderExchange(InteractiveRow{Messages: []InteractiveMessage{{Role: "assistant", Message: text}}}).Messages[0])
		if err != nil {
			t.Fatal(err)
		}
		var message InteractiveMessage
		if err := json.Unmarshal(encoded, &message); err != nil {
			t.Fatal(err)
		}
		if !strings.HasPrefix(message.HTML.Value, "<pre><code>") || strings.Contains(message.HTML.Value, "<strong>") || message.Message != text {
			t.Fatalf("JSON was not kept literal: %+v", message)
		}
	}
	encoded, err := json.Marshal(renderExchange(InteractiveRow{Messages: []InteractiveMessage{{Role: "user", Message: "**literal**", HTML: polytype.Optional[string]{Value: "<strong>untrusted</strong>", Present: true}}}}).Messages[0])
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), `"html"`) {
		t.Fatalf("user message acquired HTML: %s", encoded)
	}
}
