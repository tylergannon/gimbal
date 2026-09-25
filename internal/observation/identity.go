package observation

import (
	"encoding/json"

	"github.com/tylergannon/gimbal/internal/sessionstate"
)

// field reads one string out of a decoded JSON object.
func field(value any, key string) string {
	obj, ok := value.(*sessionstate.Obj)
	if !ok || obj == nil {
		return ""
	}
	text, _ := obj.Get(key).(string)
	return text
}

// foldProvenanceLocked keeps the latest native sidecar for each canonical
// message, keyed by the flat normalizedMessageID the root injects after its
// own native-ID mapping. It is current identity and accounting
// availability, not a per-delta audit history: the raw record of every event
// stays in the log.
//
// A sidecar that names no message -- a session-level one, for instance --
// is not folded and is not an error.
func (s *Store) foldProvenanceLocked(script *transcript, ref any, nativeRef json.RawMessage) {
	key := field(ref, "normalizedMessageID")
	if key == "" {
		return
	}
	copied := make(json.RawMessage, len(nativeRef))
	copy(copied, nativeRef)
	script.provenance[key] = copied
}

// decodeRef decodes the placement sidecar. An unparseable sidecar is a
// malformed input the caller reports; it is not silently ignored.
func decodeRef(nativeRef json.RawMessage) (any, error) {
	if len(nativeRef) == 0 {
		return nil, nil
	}
	return sessionstate.DecodeValue(nativeRef)
}
