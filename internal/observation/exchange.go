package observation

import (
	"bytes"
	"encoding/json"
	"html"
	"slices"

	"github.com/tylergannon/polytype"
	"github.com/yuin/goldmark"
)

// renderExchange uses the conversation transcript's safe Markdown renderer.
// Always derive markup from text; HTML retained in a checkpoint is untrusted.
func renderExchange(row InteractiveRow) InteractiveRow {
	row.Messages = slices.Clone(row.Messages)
	for i := range row.Messages {
		message := &row.Messages[i]
		message.HTML = polytype.Optional[string]{}
		if message.Role != "assistant" {
			continue
		}
		var rendered bytes.Buffer
		if json.Valid([]byte(message.Message)) {
			rendered.WriteString("<pre><code>" + html.EscapeString(message.Message) + "</code></pre>")
		} else if err := goldmark.Convert([]byte(message.Message), &rendered); err != nil {
			continue // The UI falls back to escaped literal text.
		}
		message.HTML = polytype.Optional[string]{Value: rendered.String(), Present: true}
	}
	return row
}

// Old and remote journals can contain pre-renderer rows or supplied HTML.
// Replace only exchange markup when delivering their deltas to the browser.
func renderExchangeDelta(delta Delta) (Delta, error) {
	delta.Frames = slices.Clone(delta.Frames)
	for i, frame := range delta.Frames {
		if frame.Name != FrameRow {
			continue
		}
		var row struct {
			Table string          `json:"table"`
			Key   string          `json:"key"`
			Row   json.RawMessage `json:"row"`
		}
		if err := json.Unmarshal(frame.Data, &row); err != nil {
			return Delta{}, err
		}
		if row.Table != tableInteractions {
			continue
		}
		var exchange InteractiveRow
		if err := json.Unmarshal(row.Row, &exchange); err != nil {
			return Delta{}, err
		}
		delta.Frames[i].Data = mustMarshal(rowFrame{Table: row.Table, Key: row.Key, Row: renderExchange(exchange)})
	}
	return delta, nil
}
