package transcript

import (
	"errors"
	"strings"
)

const MaxPayloadBytes = 65536
const MaxCanceledSessions = 1024
const MaxSessionBytes = 8 * 1024 * 1024
const MaxInteger = 9007199254740991

type Chunk struct {
	StartMs int64  `json:"startMs"`
	EndMs   int64  `json:"endMs"`
	Text    string `json:"text"`
}
type OpenRequest struct {
	Project   string `json:"project"`
	SessionID string `json:"sessionID"`
	Source    string `json:"source"`
}
type Update struct {
	Project     string  `json:"project"`
	SessionID   string  `json:"sessionID"`
	Sequence    int64   `json:"sequence"`
	Finalized   []Chunk `json:"finalized"`
	Provisional []Chunk `json:"provisional"`
}
type CloseRequest struct {
	Project   string `json:"project"`
	SessionID string `json:"sessionID"`
	Sequence  int64  `json:"sequence"`
}
type Ack struct {
	SessionID string `json:"sessionID"`
	Sequence  int64  `json:"sequence"`
	Closed    bool   `json:"closed"`
}
type Snapshot struct {
	SessionID string `json:"sessionID"`
	Sequence  int64  `json:"sequence"`
	Text      string `json:"text"`
	Source    string `json:"source"`
	Closed    bool   `json:"closed"`
}
type SessionRequest struct {
	Project   string `json:"project"`
	SessionID string `json:"sessionID"`
}

func validate(chunks []Chunk, boundary int64) (int64, string, error) {
	var text strings.Builder
	for _, c := range chunks {
		if c.StartMs < boundary || c.EndMs < c.StartMs || c.EndMs > MaxInteger {
			return 0, "", errors.New("invalid transcript audio range")
		}
		boundary = c.EndMs
		text.WriteString(c.Text)
	}
	return boundary, text.String(), nil
}
