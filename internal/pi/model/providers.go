package model

import "context"

// StreamFunction is the uniform stream contract of an API implementation
// module. Context carries cancellation, replacing pi's AbortSignal.
type StreamFunction func(ctx context.Context, model *Model, transcript TranscriptContext, options *SimpleStreamOptions) (AssistantMessageEventChannel, error)

// AssistantMessageEventChannel is the stream returned by a StreamFunction. The
// event-stream implementation lives in internal/pi/wire; this declares the
// contract provider adapters satisfy.
type AssistantMessageEventChannel interface {
	// Recv yields the next event, or io.EOF when the stream is finished.
	Recv() (AssistantMessageEvent, error)
	// Close releases the stream.
	Close() error
}

// ProviderStreams is the uniform stream contract of a provider: an API
// implementation plus optional deferred-response methods.
type ProviderStreams interface {
	Stream(ctx context.Context, model *Model, transcript TranscriptContext, options *SimpleStreamOptions) (AssistantMessageEventChannel, error)
}

// ImagesInputContent is content accepted for an image request.
type ImagesInputContent = Content

// ImagesOutputContent is content produced by an image request.
type ImagesOutputContent = Content

// ImagesContext is the input for an image request.
type ImagesContext struct {
	Input []ImagesInputContent `json:"input"`
}

// ImagesStopReason describes why an image request ended.
type ImagesStopReason string

const (
	ImagesStopStop    ImagesStopReason = "stop"
	ImagesStopError   ImagesStopReason = "error"
	ImagesStopAborted ImagesStopReason = "aborted"
)

// AssistantImages is the result of an image request.
type AssistantImages struct {
	Api          ImageApi              `json:"api"`
	Provider     ProviderId            `json:"provider"`
	Model        string                `json:"model"`
	Output       []ImagesOutputContent `json:"output"`
	ResponseID   string                `json:"responseId,omitempty"`
	Usage        *Usage                `json:"usage,omitempty"`
	StopReason   ImagesStopReason      `json:"stopReason"`
	ErrorMessage string                `json:"errorMessage,omitempty"`
	Timestamp    int64                 `json:"timestamp"`
}

// ClassifierQuestion is a classifier question.
type ClassifierQuestion struct {
	Type         string            `json:"type"`
	Instructions string            `json:"instructions"`
	Criteria     map[string]string `json:"criteria,omitempty"`
	CriteriaList []string          `json:"-"`
	CriteriaBool map[string]string `json:"-"`
}

// ClassifierContext is the input for a classifier request.
type ClassifierContext struct {
	State     JsonObject                    `json:"state"`
	Questions map[string]ClassifierQuestion `json:"questions"`
}

// ClassifierAnswer is a classifier answer.
type ClassifierAnswer struct {
	Type          string             `json:"type"`
	Choice        string             `json:"choice,omitempty"`
	Probabilities map[string]float64 `json:"probabilities,omitempty"`
	Confidence    float64            `json:"confidence,omitempty"`
	Score         float64            `json:"score,omitempty"`
	Probability   float64            `json:"probability,omitempty"`
}

// ClassifierStopReason describes why a classifier request ended.
type ClassifierStopReason string

const (
	ClassifierStopStop    ClassifierStopReason = "stop"
	ClassifierStopError   ClassifierStopReason = "error"
	ClassifierStopAborted ClassifierStopReason = "aborted"
)

// ClassifierResult is the result of a classifier request.
type ClassifierResult struct {
	Api          ClassifierApi               `json:"api"`
	Provider     ProviderId                  `json:"provider"`
	Model        string                      `json:"model"`
	Answers      map[string]ClassifierAnswer `json:"answers"`
	StopReason   ClassifierStopReason        `json:"stopReason"`
	ErrorMessage string                      `json:"errorMessage,omitempty"`
	Timestamp    int64                       `json:"timestamp"`
}
