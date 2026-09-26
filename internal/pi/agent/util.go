package agent

import (
	"fmt"
	"time"
)

func timeNowMillis() int64 {
	return time.Now().UnixNano() / int64(time.Millisecond)
}

// panicMessage renders a recovered panic value into a message string, matching
// pi's `error instanceof Error ? error.message : String(error)`.
func panicMessage(value any) string {
	if err, ok := value.(error); ok {
		return err.Error()
	}
	return fmt.Sprintf("%v", value)
}
