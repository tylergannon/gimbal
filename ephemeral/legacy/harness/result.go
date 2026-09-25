package harness

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	jsonschema "github.com/santhosh-tekuri/jsonschema/v6"
)

const outputSchemaURL = "urn:gimbal:harness:output-schema"

// ResultValidator validates native harness output against one caller-supplied
// JSON Schema.
type ResultValidator struct {
	schema *jsonschema.Schema
}

// NewResultValidator compiles the exact supplied schema and verifies that its
// root describes a JSON object.
func NewResultValidator(rawSchema json.RawMessage) (*ResultValidator, error) {
	document, err := decodeOneJSON(rawSchema)
	if err != nil {
		return nil, fmt.Errorf("invalid output schema: %w", err)
	}
	object, ok := document.(map[string]any)
	if !ok {
		return nil, errors.New("output schema must be a JSON object")
	}
	if object["type"] != "object" {
		return nil, errors.New("output schema root type must be object")
	}

	compiler := jsonschema.NewCompiler()
	if err := compiler.AddResource(outputSchemaURL, object); err != nil {
		return nil, fmt.Errorf("invalid output schema: %w", err)
	}
	compiled, err := compiler.Compile(outputSchemaURL)
	if err != nil {
		return nil, fmt.Errorf("invalid output schema: %w", err)
	}
	return &ResultValidator{schema: compiled}, nil
}

// Validate decodes exactly one JSON object, validates it against the schema,
// and returns the compact encoding of what it validated.
func (v *ResultValidator) Validate(rawResult []byte) (json.RawMessage, error) {
	if v == nil || v.schema == nil {
		return nil, errors.New("result validator is not initialized")
	}
	value, err := decodeOneJSON(rawResult)
	if err != nil {
		return nil, fmt.Errorf("invalid harness result: %w", err)
	}
	object, ok := value.(map[string]any)
	if !ok {
		return nil, errors.New("harness result must be a JSON object")
	}
	if err := v.schema.Validate(object); err != nil {
		return nil, fmt.Errorf("harness result does not conform to output schema: %w", err)
	}
	encoded, err := json.Marshal(object)
	if err != nil {
		return nil, fmt.Errorf("encode harness result: %w", err)
	}
	return encoded, nil
}

// ValidateResult compiles rawSchema and validates rawResult against it.
func ValidateResult(rawSchema json.RawMessage, rawResult []byte) (json.RawMessage, error) {
	validator, err := NewResultValidator(rawSchema)
	if err != nil {
		return nil, err
	}
	return validator.Validate(rawResult)
}

// TextResult encodes an assistant's plain text as the JSON string RunTurn
// returns when no output schema was supplied.
func TextResult(text string) (json.RawMessage, error) {
	encoded, err := json.Marshal(text)
	if err != nil {
		return nil, fmt.Errorf("encode assistant text: %w", err)
	}
	return encoded, nil
}

func decodeOneJSON(data []byte) (any, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return nil, fmt.Errorf("multiple JSON values")
		}
		return nil, err
	}
	return value, nil
}
