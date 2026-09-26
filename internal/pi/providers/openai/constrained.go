package openai

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/tylergannon/gimbal/internal/pi/model"
)

// Constrained-sampling helpers used by convertTools, ported from pi
// packages/ai/src/api/constrained-sampling.ts (upstream d6af72e1).

type unsupportedStrictJSONSchemaError struct{ message string }

func (e *unsupportedStrictJSONSchemaError) Error() string { return e.message }

var unsupportedStrictSchemaKeys = []string{
	"$ref", "$defs", "definitions", "allOf", "oneOf", "patternProperties",
	"dependentSchemas", "dependencies", "unevaluatedProperties", "propertyNames",
	"contains", "prefixItems", "not", "if", "then", "else",
}

func isStructuredSchema(schema *model.Schema) bool {
	if schema == nil {
		return false
	}
	if schema.Type == "object" || schema.Type == "array" {
		return true
	}
	return len(schema.Properties) > 0 || schema.Items != nil
}

func schemaAllowsNull(schema *model.Schema) bool {
	if schema == nil {
		return false
	}
	if schema.Type == "null" || schema.Nullable {
		return true
	}
	if schema.HasConst && schema.Const == nil {
		return true
	}
	for _, value := range schema.Enum {
		if value == nil {
			return true
		}
	}
	return slices.ContainsFunc(schema.AnyOf, schemaAllowsNull)
}

func makeSchemaNodeStrict(schema *model.Schema) error {
	if schema == nil {
		return &unsupportedStrictJSONSchemaError{"boolean schemas are unsupported"}
	}
	if len(schema.AllOf) > 0 {
		return &unsupportedStrictJSONSchemaError{"allOf schemas are unsupported"}
	}
	if len(schema.OneOf) > 0 {
		return &unsupportedStrictJSONSchemaError{"oneOf schemas are unsupported"}
	}
	for _, key := range unsupportedStrictSchemaKeys {
		if key == "allOf" || key == "oneOf" {
			continue
		}
		if _, present := schema.Extra[key]; present {
			return &unsupportedStrictJSONSchemaError{fmt.Sprintf("%s schemas are unsupported", key)}
		}
	}

	if len(schema.AnyOf) > 0 {
		for _, variant := range schema.AnyOf {
			if isStructuredSchema(variant) {
				return &unsupportedStrictJSONSchemaError{"object and array unions are unsupported"}
			}
			if err := makeSchemaNodeStrict(variant); err != nil {
				return err
			}
		}
	}
	if schema.Items != nil {
		if err := makeSchemaNodeStrict(schema.Items); err != nil {
			return err
		}
	}

	isObjectSchema := schema.Type == "object"
	if len(schema.Properties) > 0 && !isObjectSchema {
		return &unsupportedStrictJSONSchemaError{"properties require type object"}
	}
	if !isObjectSchema {
		return nil
	}
	if schema.AdditionalSchema != nil {
		return &unsupportedStrictJSONSchemaError{"schema-valued or true additionalProperties is unsupported"}
	}
	if schema.AdditionalAllowed != nil && *schema.AdditionalAllowed {
		return &unsupportedStrictJSONSchemaError{"schema-valued or true additionalProperties is unsupported"}
	}

	requiredSet := map[string]bool{}
	for _, name := range schema.Required {
		requiredSet[name] = true
	}
	propertyNames := schema.OrderedProperties()
	for name := range requiredSet {
		if _, ok := schema.Properties[name]; !ok {
			return &unsupportedStrictJSONSchemaError{"required contains an unknown property"}
		}
	}
	for _, name := range propertyNames {
		property := schema.Properties[name]
		if err := makeSchemaNodeStrict(property); err != nil {
			return err
		}
		if !requiredSet[name] && !schemaAllowsNull(property) {
			schema.Properties[name] = &model.Schema{AnyOf: []*model.Schema{property, {Type: "null"}}}
		}
	}
	schema.Required = propertyNames
	additionalAllowed := false
	schema.AdditionalAllowed = &additionalAllowed
	schema.AdditionalSchema = nil
	return nil
}

// makeStrictJSONSchema converts a tool schema to the strict subset expected by
// provider constrained sampling.
func makeStrictJSONSchema(parameters *model.Schema) (*model.Schema, error) {
	if parameters == nil {
		return nil, &unsupportedStrictJSONSchemaError{"root schema must have type object"}
	}
	cloned := parameters.Clone()
	if err := makeSchemaNodeStrict(cloned); err != nil {
		return nil, err
	}
	if cloned.Type != "object" {
		return nil, &unsupportedStrictJSONSchemaError{"root schema must have type object"}
	}
	return cloned, nil
}

func schemaToAny(parameters *model.Schema) (any, error) {
	if parameters == nil {
		return map[string]any{"type": "object", "properties": map[string]any{}}, nil
	}
	raw, err := json.Marshal(parameters)
	if err != nil {
		return nil, err
	}
	var out any
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// getJSONSchemaToolParameters returns the tool parameters, made strict when the
// tool asked for it and the provider supports strict mode.
func getJSONSchemaToolParameters(tool model.Tool, strict *bool) (*model.Schema, error) {
	if strict != nil && *strict {
		return makeStrictJSONSchema(tool.Parameters)
	}
	return tool.Parameters, nil
}

// resolveJSONSchemaStrictSampling resolves a tool's json_schema constrained
// sampling to its strict flag, or nil for the absent case. A "require"
// strictness that cannot be honored is an error.
func resolveJSONSchemaStrictSampling(tool model.Tool, supportsStrictMode bool) (*bool, error) {
	config := tool.ConstrainedSampling
	if config == nil || config.Type != model.ConstrainedSamplingJSONSchema {
		return nil, nil
	}
	if supportsStrictMode {
		if _, err := makeStrictJSONSchema(tool.Parameters); err == nil {
			return new(true), nil
		} else if config.Strict != model.ConstrainedSamplingRequire {
			return nil, nil
		} else {
			return nil, fmt.Errorf("Tool %q requires JSON-schema constrained sampling, but %s.", tool.Name, err.Error()) //nolint:staticcheck // pi's exact message
		}
	}
	if config.Strict == model.ConstrainedSamplingRequire {
		return nil, fmt.Errorf("Tool %q requires JSON-schema constrained sampling, but strict tools are unsupported.", tool.Name) //nolint:staticcheck // pi's exact message
	}
	return nil, nil
}

// grammarConstrainedSampling is a resolved grammar tool constraint.
type grammarConstrainedSampling struct {
	Format        string
	Definition    string
	InputProperty string
}

func isGrammarConstraint(config *model.ConstrainedSamplingConfig) bool {
	return config != nil && config.Type == model.ConstrainedSamplingGrammar
}

func inferGrammarInputProperty(tool model.Tool) (string, error) {
	schema, _ := schemaToAny(tool.Parameters)
	object, ok := schema.(map[string]any)
	if !ok || object["type"] != "object" {
		return "", errors.New("grammar constrained sampling requires an object parameter schema")
	}
	required, ok := object["required"].([]any)
	if !ok || len(required) != 1 {
		return "", errors.New("grammar constrained sampling requires exactly one required string property")
	}
	inputProperty, ok := required[0].(string)
	if !ok {
		return "", errors.New("grammar constrained sampling requires exactly one required string property")
	}
	properties, _ := object["properties"].(map[string]any)
	property, ok := properties[inputProperty].(map[string]any)
	if !ok {
		return "", fmt.Errorf("grammar constrained sampling requires a properties entry for %s", inputProperty)
	}
	if property["type"] != "string" {
		return "", fmt.Errorf("grammar constrained sampling property %s must have type string", inputProperty)
	}
	return inputProperty, nil
}

// resolveGrammarConstrainedSampling resolves a tool's grammar constraint when
// the provider supports grammar tools.
func resolveGrammarConstrainedSampling(tool model.Tool, supportsOpenAIGrammarTools bool) (*grammarConstrainedSampling, error) {
	config := tool.ConstrainedSampling
	if !isGrammarConstraint(config) {
		return nil, nil
	}
	if !supportsOpenAIGrammarTools {
		return nil, nil
	}
	lark := strings.TrimSpace(config.Variants.OpenAILark)
	regex := strings.TrimSpace(config.Variants.OpenAIRegex)
	hasLark := lark != ""
	hasRegex := regex != ""
	if !hasLark && !hasRegex {
		return nil, fmt.Errorf("Tool %q cannot use grammar constrained sampling: no supported grammar variant was provided.", tool.Name) //nolint:staticcheck // pi's exact message
	}
	inputProperty, err := inferGrammarInputProperty(tool)
	if err != nil {
		return nil, fmt.Errorf("Tool %q cannot use grammar constrained sampling: %s.", tool.Name, err.Error()) //nolint:staticcheck // pi's exact message
	}
	format, definition := "regex", regex
	if hasLark {
		format, definition = "lark", lark
	}
	return &grammarConstrainedSampling{Format: format, Definition: definition, InputProperty: inputProperty}, nil
}

// createGrammarToolInputProperties maps each declared tool to its grammar input
// property.
func createGrammarToolInputProperties(tools []model.Tool, supportsOpenAIGrammarTools bool) (map[string]string, error) {
	properties := map[string]string{}
	for _, tool := range tools {
		grammar, err := resolveGrammarConstrainedSampling(tool, supportsOpenAIGrammarTools)
		if err != nil {
			return nil, err
		}
		if grammar != nil {
			properties[tool.Name] = grammar.InputProperty
		}
	}
	return properties, nil
}

// getGrammarToolInput reads a grammar tool call's input argument.
func getGrammarToolInput(toolName string, arguments map[string]any, inputProperty string) (string, error) {
	input, ok := arguments[inputProperty].(string)
	if !ok {
		return "", fmt.Errorf("Grammar tool call %q requires argument %q to be a string.", toolName, inputProperty) //nolint:staticcheck // pi's exact message
	}
	return input, nil
}

// grammarInputBuffer accumulates a grammar tool call's streamed input.
type grammarInputBuffer struct {
	Input    string
	Started  bool
	Closed   bool
	Property string
}

func newGrammarInputBuffer(property string) *grammarInputBuffer {
	return &grammarInputBuffer{Property: property}
}

// appendGrammarToolInputJSONDelta advances a grammar buffer, returning the
// synthesized JSON delta for the next input.
func appendGrammarToolInputJSONDelta(buffer *grammarInputBuffer, inputProperty, nextInput string, closeInput bool) (string, error) {
	if buffer.Closed {
		if closeInput && nextInput == buffer.Input {
			return "", nil
		}
		return "", fmt.Errorf("grammar tool input for property %q changed after it was closed", inputProperty)
	}
	if !strings.HasPrefix(nextInput, buffer.Input) {
		return "", fmt.Errorf("grammar tool input for property %q changed non-monotonically", inputProperty)
	}
	inputDelta := nextInput[len(buffer.Input):]
	if !closeInput && inputDelta == "" {
		return "", nil
	}
	var delta strings.Builder
	if !buffer.Started {
		key, _ := json.Marshal(inputProperty)
		delta.WriteString("{")
		delta.Write(key)
		delta.WriteString(`:"`)
		buffer.Started = true
	}
	encoded, _ := json.Marshal(inputDelta)
	delta.WriteString(string(encoded[1 : len(encoded)-1]))
	buffer.Input = nextInput
	if closeInput {
		delta.WriteString(`"}`)
		buffer.Closed = true
	}
	return delta.String(), nil
}
