package openai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/tylergannon/gimbal/internal/pi/model"
	"github.com/tylergannon/gimbal/internal/pi/wire"
)

// The Chat Completions streaming entry points, ported from
// openai-completions.ts stream/streamSimple (upstream d6af72e1).

// streamChunk is the subset of a ChatCompletionChunk the provider reads.
type streamChunk struct {
	ID      string         `json:"id"`
	Model   string         `json:"model"`
	Choices []streamChoice `json:"choices"`
	Usage   *rawUsage      `json:"usage"`
}

type streamChoice struct {
	Delta        streamDelta `json:"delta"`
	FinishReason *string     `json:"finish_reason"`
	Usage        *rawUsage   `json:"usage"`
}

type streamDelta struct {
	Content          *string           `json:"content"`
	ReasoningContent *string           `json:"reasoning_content"`
	Reasoning        *string           `json:"reasoning"`
	ReasoningText    *string           `json:"reasoning_text"`
	ToolCalls        []toolCallDelta   `json:"tool_calls"`
	ReasoningDetails []json.RawMessage `json:"reasoning_details"`
}

type toolCallDelta struct {
	Index    *float64       `json:"index"`
	ID       string         `json:"id"`
	Function *functionDelta `json:"function"`
	Custom   *customDelta   `json:"custom"`
}

type functionDelta struct {
	Name      *string `json:"name"`
	Arguments *string `json:"arguments"`
}

type customDelta struct {
	Name  *string `json:"name"`
	Input *string `json:"input"`
}

// streamBlock accumulates one assistant content block while a stream runs.
type streamBlock struct {
	kind              string // "text", "thinking", "toolCall"
	text              strings.Builder
	thinking          strings.Builder
	thinkingSignature string
	toolID            string
	toolName          string
	args              map[string]any
	partialArgs       strings.Builder
	grammar           *grammarInputBuffer
	index             float64
	hasIndex          bool
}

func (b *streamBlock) content() model.Content {
	switch b.kind {
	case "text":
		return model.TextContent{Text: b.text.String()}
	case "thinking":
		return model.ThinkingContent{Thinking: b.thinking.String(), ThinkingSignature: b.thinkingSignature}
	default:
		args := b.args
		if args == nil {
			args = map[string]any{}
		}
		return model.ToolCall{ID: b.toolID, Name: b.toolName, Arguments: args}
	}
}

// StreamSimple maps unified simple options to a Chat Completions request and
// streams it. It satisfies model.StreamFunction.
func StreamSimple(ctx context.Context, m *model.Model, transcript model.TranscriptContext, options *model.SimpleStreamOptions) (model.AssistantMessageEventChannel, error) {
	var apiKey string
	var env model.ProviderEnv
	if options != nil {
		apiKey = options.APIKey
		env = options.Env
	}
	var headers model.ProviderHeaders
	if options != nil {
		headers = options.Headers
	}
	if _, err := resolveClientAPIKey(m.Provider, apiKey, headers, env); err != nil {
		return nil, err
	}
	base := wire.BuildBaseOptions(m, transcript, options, "")
	o := &Options{StreamOptions: base}
	if options != nil {
		if options.ToolChoice != "" {
			o.ToolChoice = string(options.ToolChoice)
		}
		if options.Reasoning != "" {
			clamped := clampThinkingLevel(m, model.ModelThinkingLevel(options.Reasoning))
			if clamped != model.ModelThinkingLevel(model.ThinkingOff) {
				o.ReasoningEffort = string(clamped)
			}
		}
		o.ThinkingBudgets = options.ThinkingBudgets
	}
	return Stream(ctx, m, transcript, o), nil
}

// Stream streams a Chat Completions request. The returned stream completes on
// a done or error event; the final assistant message is available from
// Result().
func Stream(ctx context.Context, m *model.Model, transcript model.TranscriptContext, options *Options) *wire.AssistantMessageEventStream {
	if options == nil {
		options = &Options{}
	}
	c := getCompat(m)
	normalized := model.ResolveTranscript(transcript, c.SupportsMidConvoSystemMessages)
	stream := wire.NewAssistantMessageEventStream()

	runCtx := ctx
	if options.TimeoutMs > 0 {
		var cancel context.CancelFunc
		runCtx, cancel = context.WithTimeout(ctx, time.Duration(options.TimeoutMs)*time.Millisecond)
		defer cancel()
	}

	go func() {
		output := &model.AssistantMessage{
			Content:    model.ContentList{},
			Api:        m.Api,
			Provider:   m.Provider,
			Model:      m.ID,
			StopReason: model.StopPending,
			Timestamp:  time.Now().UnixMilli(),
		}

		var thinkBlock *streamBlock
		var streamedReasoningDetails []json.RawMessage
		var order []*streamBlock

		materialize := func() {
			content := make(model.ContentList, len(order))
			for i, b := range order {
				content[i] = b.content()
			}
			output.Content = content
		}
		applyStreamedReasoningDetails := func() {
			if thinkBlock == nil || len(streamedReasoningDetails) == 0 {
				return
			}
			thinkBlock.thinkingSignature = marshalOpenAIReasoningDetails(streamedReasoningDetails)
			materialize()
		}
		snapshot := func() *model.AssistantMessage {
			copied := output.Clone()
			return &copied
		}
		fail := func(err error) {
			applyStreamedReasoningDetails()
			if ctx.Err() != nil {
				output.StopReason = model.StopAborted
			} else {
				output.StopReason = model.StopError
			}
			output.ErrorMessage = err.Error()
			stream.Push(model.AssistantMessageEvent{Type: model.EventError, Reason: output.StopReason, Error: snapshot()})
			stream.End()
		}

		apiKey, err := resolveClientAPIKey(m.Provider, options.APIKey, options.Headers, options.Env)
		if err != nil {
			fail(err)
			return
		}
		grammarProps, err := createGrammarToolInputProperties(model.GetDeclaredTools(normalized.Messages), c.SupportsOpenAIGrammarTools)
		if err != nil {
			fail(err)
			return
		}
		params, err := buildParams(m, normalized, options, c, resolveCacheRetention(options.CacheRetention, options.Env), grammarProps)
		if err != nil {
			fail(err)
			return
		}
		body := any(params)
		if options.OnPayload != nil {
			next, err := options.OnPayload(body, m)
			if err != nil {
				fail(err)
				return
			}
			if next != nil {
				body = next
			}
		}
		payload, err := json.Marshal(body)
		if err != nil {
			fail(err)
			return
		}

		baseURL := m.BaseURL
		if baseURL == "" {
			baseURL = "https://api.openai.com/v1"
		}
		url := strings.TrimRight(baseURL, "/") + "/chat/completions"
		headers := buildHeaders(m, c, options, apiKey)
		client := options.HTTPClient
		if client == nil {
			client = http.DefaultClient
		}

		resp, err := wire.RetryProviderRequest(runCtx, func() (*http.Response, error) {
			request, reqErr := http.NewRequestWithContext(runCtx, http.MethodPost, url, bytes.NewReader(payload))
			if reqErr != nil {
				return nil, reqErr
			}
			request.Header = headers.Clone()
			response, sendErr := client.Do(request)
			if sendErr != nil {
				return nil, sendErr
			}
			if response.StatusCode < 200 || response.StatusCode >= 300 {
				bodyBytes, _ := io.ReadAll(io.LimitReader(response.Body, 1<<20))
				_ = response.Body.Close()
				return nil, &wire.ProviderHTTPError{
					Err:     formatHTTPError("OpenAI", response.StatusCode, bodyBytes),
					Status:  response.StatusCode,
					Headers: response.Header,
				}
			}
			return response, nil
		}, wire.ProviderRetryOptions{MaxRetries: options.MaxRetries, MaxRetryDelayMs: options.MaxRetryDelayMs})
		if err != nil {
			if errors.Is(err, wire.ErrProviderAborted) && ctx.Err() == nil {
				fail(errors.New("Request timed out")) //nolint:staticcheck // user-facing message
				return
			}
			fail(err)
			return
		}
		defer func() { _ = resp.Body.Close() }()
		if options.OnResponse != nil {
			if err := options.OnResponse(model.ProviderResponse{Status: resp.StatusCode, Headers: flattenHeaders(resp.Header)}, m); err != nil {
				fail(err)
				return
			}
		}

		stream.Push(model.AssistantMessageEvent{Type: model.EventStart, Partial: snapshot()})

		var textBlock *streamBlock
		toolBlocksByIndex := map[float64]*streamBlock{}
		toolBlocksByID := map[string]*streamBlock{}
		indexOf := func(b *streamBlock) int {
			for i, candidate := range order {
				if candidate == b {
					return i
				}
			}
			return -1
		}
		ensureTextBlock := func() *streamBlock {
			if textBlock == nil {
				textBlock = &streamBlock{kind: "text"}
				order = append(order, textBlock)
				materialize()
				stream.Push(model.AssistantMessageEvent{Type: model.EventTextStart, ContentIndex: indexOf(textBlock), Partial: snapshot()})
			}
			return textBlock
		}
		ensureThinkingBlock := func(signature string) *streamBlock {
			if thinkBlock == nil {
				thinkBlock = &streamBlock{kind: "thinking", thinkingSignature: signature}
				order = append(order, thinkBlock)
				materialize()
				stream.Push(model.AssistantMessageEvent{Type: model.EventThinkingStart, ContentIndex: indexOf(thinkBlock), Partial: snapshot()})
			}
			return thinkBlock
		}
		grammarInput := func(b *streamBlock) string {
			if b.grammar == nil {
				return ""
			}
			value, _ := b.args[b.grammar.Property].(string)
			return value
		}
		startGrammarBuffer := func(b *streamBlock) {
			property, ok := grammarProps[b.toolName]
			if !ok {
				property = "input"
			}
			b.grammar = newGrammarInputBuffer(property)
			b.args = map[string]any{property: ""}
		}
		ensureToolCallBlock := func(tc toolCallDelta) *streamBlock {
			name := toolCallName(tc)
			var block *streamBlock
			if tc.Index != nil {
				block = toolBlocksByIndex[*tc.Index]
			}
			if block == nil && tc.ID != "" {
				block = toolBlocksByID[tc.ID]
			}
			isCustomCall := tc.Custom != nil && tc.Function == nil
			if block == nil {
				block = &streamBlock{kind: "toolCall", toolID: tc.ID, toolName: name, args: map[string]any{}}
				if isCustomCall {
					startGrammarBuffer(block)
				}
				if tc.Index != nil {
					block.hasIndex = true
					block.index = *tc.Index
					toolBlocksByIndex[*tc.Index] = block
				}
				if tc.ID != "" {
					toolBlocksByID[tc.ID] = block
				}
				order = append(order, block)
				materialize()
				stream.Push(model.AssistantMessageEvent{Type: model.EventToolCallStart, ContentIndex: indexOf(block), Partial: snapshot()})
			}
			if tc.Index != nil && !block.hasIndex {
				block.hasIndex = true
				block.index = *tc.Index
				toolBlocksByIndex[*tc.Index] = block
			}
			if tc.ID != "" {
				toolBlocksByID[tc.ID] = block
			}
			if block.toolName == "" && name != "" {
				block.toolName = name
			}
			if isCustomCall && block.grammar == nil {
				startGrammarBuffer(block)
			}
			return block
		}

		hasFinishReason := false
		onProviderStreamEvent := options.OnProviderStreamEvent
		err = readSSE(runCtx, resp.Body, func(data []byte) error {
			if onProviderStreamEvent != nil {
				var generic any
				if json.Unmarshal(data, &generic) == nil {
					if err := onProviderStreamEvent(generic, m); err != nil {
						return err
					}
				}
			}
			var raw map[string]json.RawMessage
			if json.Unmarshal(data, &raw) == nil {
				if errorValue, present := raw["error"]; present && rawTruthy(errorValue) {
					return chunkError(errorValue)
				}
			}
			var chunk streamChunk
			if err := json.Unmarshal(data, &chunk); err != nil {
				return err
			}
			if output.ResponseID == "" && chunk.ID != "" {
				output.ResponseID = chunk.ID
			}
			if chunk.Model != "" && chunk.Model != m.ID && output.ResponseModel == "" {
				output.ResponseModel = chunk.Model
			}
			if chunk.Usage != nil {
				output.Usage = parseChunkUsage(chunk.Usage, m)
			}
			if len(chunk.Choices) == 0 {
				return nil
			}
			choice := chunk.Choices[0]
			if chunk.Usage == nil && choice.Usage != nil {
				output.Usage = parseChunkUsage(choice.Usage, m)
			}
			if choice.FinishReason != nil && *choice.FinishReason != "" {
				output.RawStopReason = *choice.FinishReason
				stopReason, errorMessage := mapStopReason(*choice.FinishReason)
				output.StopReason = stopReason
				if errorMessage != "" {
					output.ErrorMessage = errorMessage
				}
				hasFinishReason = true
			}

			delta := choice.Delta
			if delta.Content != nil && *delta.Content != "" {
				block := ensureTextBlock()
				block.text.WriteString(*delta.Content)
				materialize()
				stream.Push(model.AssistantMessageEvent{Type: model.EventTextDelta, ContentIndex: indexOf(block), Delta: *delta.Content, Partial: snapshot()})
			}

			reasoningField, reasoningValue := firstReasoningDelta(delta)
			if reasoningValue != "" {
				if m.Provider == "opencode-go" && reasoningField == "reasoning" {
					reasoningField = "reasoning_content"
				}
				block := ensureThinkingBlock(reasoningField)
				if block.thinkingSignature == "" {
					block.thinkingSignature = reasoningField
				}
				block.thinking.WriteString(reasoningValue)
				materialize()
				stream.Push(model.AssistantMessageEvent{Type: model.EventThinkingDelta, ContentIndex: indexOf(block), Delta: reasoningValue, Partial: snapshot()})
			}

			for _, tc := range delta.ToolCalls {
				block := ensureToolCallBlock(tc)
				if block.toolID == "" && tc.ID != "" {
					block.toolID = tc.ID
					toolBlocksByID[tc.ID] = block
				}
				if name := toolCallName(tc); block.toolName == "" && name != "" {
					block.toolName = name
				}
				deltaText := ""
				switch {
				case tc.Function != nil && tc.Function.Arguments != nil && *tc.Function.Arguments != "":
					deltaText = *tc.Function.Arguments
					block.partialArgs.WriteString(deltaText)
					block.args = wire.ParseStreamingJSON(block.partialArgs.String())
				case tc.Custom != nil && tc.Custom.Input != nil && *tc.Custom.Input != "" && block.grammar != nil:
					nextInput := grammarInput(block) + *tc.Custom.Input
					jsonDelta, err := appendGrammarToolInputJSONDelta(block.grammar, block.grammar.Property, nextInput, false)
					if err != nil {
						return err
					}
					block.args = map[string]any{block.grammar.Property: nextInput}
					deltaText = jsonDelta
				}
				materialize()
				stream.Push(model.AssistantMessageEvent{Type: model.EventToolCallDelta, ContentIndex: indexOf(block), Delta: deltaText, Partial: snapshot()})
			}

			for _, detail := range delta.ReasoningDetails {
				if !isOpenAIReasoningDetail(detail) {
					continue
				}
				ensureThinkingBlock("")
				streamedReasoningDetails = appendOpenAIReasoningDetail(streamedReasoningDetails, detail)
			}
			return nil
		})
		if err != nil {
			fail(err)
			return
		}

		materialize()
		for _, block := range order {
			switch block.kind {
			case "text":
				stream.Push(model.AssistantMessageEvent{Type: model.EventTextEnd, ContentIndex: indexOf(block), Content: block.text.String(), Partial: snapshot()})
			case "thinking":
				applyStreamedReasoningDetails()
				stream.Push(model.AssistantMessageEvent{Type: model.EventThinkingEnd, ContentIndex: indexOf(block), Content: block.thinking.String(), Partial: snapshot()})
			case "toolCall":
				if block.grammar != nil {
					jsonDelta, err := appendGrammarToolInputJSONDelta(block.grammar, block.grammar.Property, grammarInput(block), true)
					if err != nil {
						fail(err)
						return
					}
					if jsonDelta != "" {
						materialize()
						stream.Push(model.AssistantMessageEvent{Type: model.EventToolCallDelta, ContentIndex: indexOf(block), Delta: jsonDelta, Partial: snapshot()})
					}
				} else {
					block.args = wire.ParseStreamingJSON(block.partialArgs.String())
				}
				materialize()
				toolCall, _ := block.content().(model.ToolCall)
				stream.Push(model.AssistantMessageEvent{Type: model.EventToolCallEnd, ContentIndex: indexOf(block), ToolCall: &toolCall, Partial: snapshot()})
			}
		}

		if ctx.Err() != nil || output.StopReason == model.StopAborted {
			fail(errors.New("Request was aborted")) //nolint:staticcheck // pi's exact message
			return
		}
		if !hasFinishReason && !c.SupportsFinishReason {
			output.StopReason = model.StopStop
			for _, block := range output.Content {
				if _, ok := block.(model.ToolCall); ok {
					output.StopReason = model.StopToolUse
					break
				}
			}
		}
		if output.StopReason == model.StopError {
			message := output.ErrorMessage
			if message == "" {
				message = "Provider returned an error stop reason"
			}
			fail(errors.New(message))
			return
		}
		if (c.SupportsFinishReason && !hasFinishReason) || output.StopReason == model.StopPending {
			fail(errors.New("Stream ended without finish_reason"))
			return
		}
		stream.Push(model.AssistantMessageEvent{Type: model.EventDone, Reason: output.StopReason, Message: snapshot()})
		stream.End()
	}()

	return stream
}

// toolCallName is `function?.name ?? custom?.name`.
func toolCallName(tc toolCallDelta) string {
	if tc.Function != nil && tc.Function.Name != nil && *tc.Function.Name != "" {
		return *tc.Function.Name
	}
	if tc.Custom != nil && tc.Custom.Name != nil {
		return *tc.Custom.Name
	}
	return ""
}

// firstReasoningDelta returns the first non-empty reasoning field and value.
func firstReasoningDelta(delta streamDelta) (string, string) {
	if delta.ReasoningContent != nil && *delta.ReasoningContent != "" {
		return "reasoning_content", *delta.ReasoningContent
	}
	if delta.Reasoning != nil && *delta.Reasoning != "" {
		return "reasoning", *delta.Reasoning
	}
	if delta.ReasoningText != nil && *delta.ReasoningText != "" {
		return "reasoning_text", *delta.ReasoningText
	}
	return "", ""
}

// mapStopReason maps a finish_reason to a stop reason and optional error.
func mapStopReason(reason string) (model.StopReason, string) {
	switch reason {
	case "stop", "end":
		return model.StopStop, ""
	case "length":
		return model.StopLength, ""
	case "function_call", "tool_calls":
		return model.StopToolUse, ""
	case "content_filter":
		return model.StopError, "Provider finish_reason: content_filter"
	case "network_error":
		return model.StopError, "Provider finish_reason: network_error"
	default:
		return model.StopError, fmt.Sprintf("Provider finish_reason: %s", reason)
	}
}

// rawTruthy reports JavaScript truthiness for a raw JSON value.
func rawTruthy(raw json.RawMessage) bool {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return false
	}
	switch trimmed[0] {
	case 'n', 'f':
		return false
	case 't', '{', '[':
		return true
	case '"':
		return len(trimmed) > 2
	default:
		var number float64
		if json.Unmarshal(trimmed, &number) != nil {
			return false
		}
		return number != 0
	}
}

// chunkError builds an error from a streamed chunk's error member.
func chunkError(raw json.RawMessage) error {
	var object map[string]any
	if json.Unmarshal(raw, &object) == nil {
		if message, ok := object["message"].(string); ok && message != "" {
			return errors.New(message)
		}
	}
	return errors.New(string(raw))
}

// readSSE reads a server-sent-event stream, dispatching each complete data
// payload. A "[DONE]" payload ends the stream.
func readSSE(ctx context.Context, body io.Reader, handle func(data []byte) error) error {
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 0, 64*1024), 16<<20)
	var data []string
	for scanner.Scan() {
		if err := ctx.Err(); err != nil {
			return err
		}
		line := scanner.Text()
		if line == "" {
			if len(data) == 0 {
				continue
			}
			payload := strings.Join(data, "\n")
			data = nil
			if strings.HasPrefix(payload, "[DONE]") {
				return nil
			}
			if err := handle([]byte(payload)); err != nil {
				return err
			}
			continue
		}
		if strings.HasPrefix(line, ":") {
			continue
		}
		field, value, _ := strings.Cut(line, ":")
		if field == "data" {
			data = append(data, strings.TrimPrefix(value, " "))
		}
	}
	return scanner.Err()
}

// buildHeaders assembles the request headers: the pi user agent, model
// headers, session-affinity headers and consumer headers, then the API key.
func buildHeaders(m *model.Model, c compat, options *Options, apiKey string) http.Header {
	headers := http.Header{}
	headers.Set("User-Agent", getPiUserAgent())
	headers.Set("Content-Type", "application/json")
	headers.Set("Accept", "text/event-stream")
	mergeProviderHeaders(headers, m.Headers)
	if options.SessionID != "" && c.SendSessionAffinityHeaders &&
		resolveCacheRetention(options.CacheRetention, options.Env) != model.CacheNone {
		if c.SessionAffinityFormat == model.SessionAffinityOpenRouter {
			headers.Set("x-session-id", options.SessionID)
		} else {
			if c.SessionAffinityFormat == model.SessionAffinityOpenAI {
				headers.Set("session_id", options.SessionID)
			}
			headers.Set("x-client-request-id", options.SessionID)
			headers.Set("x-session-affinity", options.SessionID)
		}
	}
	mergeProviderHeaders(headers, options.Headers)
	if apiKey != "" && headers.Get("Authorization") == "" {
		headers.Set("Authorization", "Bearer "+apiKey)
	}
	return headers
}

func mergeProviderHeaders(headers http.Header, source model.ProviderHeaders) {
	for key, value := range source {
		if value == nil {
			headers.Del(key)
			continue
		}
		headers.Set(key, *value)
	}
}

func flattenHeaders(headers http.Header) map[string]string {
	out := map[string]string{}
	for key, values := range headers {
		if len(values) > 0 {
			out[key] = values[0]
		}
	}
	return out
}
