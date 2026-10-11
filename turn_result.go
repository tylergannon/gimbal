package gimbal

import (
	"bytes"
	"encoding/json"
	"encoding/json/jsontext"
	"errors"
	"fmt"

	jsonschema "github.com/santhosh-tekuri/jsonschema/v6"
)

// ErrInvalidOutput identifies an invalid agent answer. Generate re-asks within
// its existing attempt limit; adapters return it without retrying the turn.
var ErrInvalidOutput = errors.New("invalid agent output")

// ValidateTurnResult checks native output without changing its bytes. A nil
// schema requires a JSON string, the final prose message. Native adapters call
// this once before returning a successful turn; forwarding adapters preserve
// the native validation result.
func ValidateTurnResult(schema json.RawMessage, result TurnResult) error {
	if !jsontext.Value(result.Output).IsValid() {
		return fmt.Errorf("%w: response must be one JSON value with unique object names and valid UTF-8", ErrInvalidOutput)
	}
	if len(schema) == 0 {
		var text string
		if bytes.Equal(bytes.TrimSpace(result.Output), []byte("null")) {
			return fmt.Errorf("%w: response must be a JSON string", ErrInvalidOutput)
		}
		if err := json.Unmarshal(result.Output, &text); err != nil {
			return fmt.Errorf("%w: response must be a JSON string: %v", ErrInvalidOutput, err)
		}
		return nil
	}
	var document any
	decoder := json.NewDecoder(bytes.NewReader(schema))
	decoder.UseNumber()
	if err := decoder.Decode(&document); err != nil {
		return fmt.Errorf("gimbal: invalid output schema: %w", err)
	}
	compiler := jsonschema.NewCompiler()
	const schemaURL = "urn:gimbal:turn-output"
	if err := compiler.AddResource(schemaURL, document); err != nil {
		return fmt.Errorf("gimbal: invalid output schema: %w", err)
	}
	compiled, err := compiler.Compile(schemaURL)
	if err != nil {
		return fmt.Errorf("gimbal: invalid output schema: %w", err)
	}
	decoder = json.NewDecoder(bytes.NewReader(result.Output))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidOutput, err)
	}
	if err := compiled.Validate(value); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidOutput, err)
	}
	return nil
}
