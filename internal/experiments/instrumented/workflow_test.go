package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"go.temporal.io/sdk/worker"
)

// Set SPECIMEN_HISTORY to a history exported by temporal workflow show --output json.
func TestRecordedHistoryReplay(t *testing.T) {
	path := os.Getenv("SPECIMEN_HISTORY")
	if path == "" {
		t.Skip("no recorded history supplied")
	}
	replayer, err := worker.NewWorkflowReplayerWithOptions(worker.WorkflowReplayerOptions{DataConverter: dataConverter(stateRoot())})
	if err != nil {
		t.Fatal(err)
	}
	replayer.RegisterWorkflow(ContinuityWorkflow)
	replayer.RegisterWorkflow(FanoutWorkflow)
	replayer.RegisterWorkflow(PlanningWorkflow)
	if err := replayer.ReplayWorkflowHistoryFromJSONFile(nil, path); err != nil {
		t.Fatal(err)
	}
}

func TestControlRejectsCrossOriginBeforeMutation(t *testing.T) {
	for _, path := range []string{"/control/cancel", "/control/steer", "/_app/remote/steer"} {
		t.Run(path, func(t *testing.T) {
			invoked := false
			handler := sameOrigin(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { invoked = true; w.WriteHeader(202) }))
			request := httptest.NewRequest("POST", "http://127.0.0.1:12345"+path, nil)
			request.Header.Set("Origin", "https://unrelated.example")
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if invoked || response.Code != 403 {
				t.Fatalf("cross-origin mutation reached handler: invoked=%v status=%d", invoked, response.Code)
			}
			request.Header.Set("Origin", "http://127.0.0.1:12345")
			response = httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if !invoked || response.Code != 202 {
				t.Fatal("same-origin control was rejected")
			}
		})
	}
}
