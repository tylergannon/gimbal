package agent

import (
	"strings"
	"testing"

	"github.com/tylergannon/gimbal/internal/pi/model"
)

func validatePlain(t *testing.T, valueSchema *model.Schema, input any) (map[string]any, error) {
	t.Helper()
	tool := model.Tool{Name: "echo", Description: "Echo", Parameters: model.Object(model.Prop("value", valueSchema))}
	return ValidateToolArguments(tool, model.ToolCall{ID: "tool-1", Name: "echo", Arguments: map[string]any{"value": input}})
}

func mustValue(t *testing.T, valueSchema *model.Schema, input any) any {
	t.Helper()
	args, err := validatePlain(t, valueSchema, input)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	return args["value"]
}

func TestValidateToolArgumentsCoercesPrimitives(t *testing.T) {
	cases := []struct {
		name   string
		schema *model.Schema
		input  any
		want   any
	}{
		{"number from string", model.Number(), "42", float64(42)},
		{"number from true", model.Number(), true, float64(1)},
		{"number from null", model.Number(), nil, float64(0)},
		{"integer from string", model.Integer(), "42", float64(42)},
		{"boolean from true string", model.Boolean(), "true", true},
		{"boolean from false string", model.Boolean(), "false", false},
		{"boolean from 1", model.Boolean(), 1, true},
		{"boolean from 0", model.Boolean(), 0, false},
		{"string from null", model.String(), nil, ""},
		{"string from true", model.String(), true, "true"},
		{"null from empty string", &model.Schema{Type: "null"}, "", nil},
		{"null from zero", &model.Schema{Type: "null"}, 0, nil},
		{"null from false", &model.Schema{Type: "null"}, false, nil},
		{"nullable number", &model.Schema{Type: "number", Nullable: true}, nil, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := mustValue(t, tc.schema, tc.input)
			if !jsonValueEqual(got, tc.want) {
				t.Fatalf("got %#v, want %#v", got, tc.want)
			}
		})
	}
}

func TestValidateToolArgumentsRejectsInvalidCoercions(t *testing.T) {
	cases := []struct {
		name   string
		schema *model.Schema
		input  any
	}{
		{"boolean from 1 string", model.Boolean(), "1"},
		{"boolean from 0 string", model.Boolean(), "0"},
		{"null from null string", &model.Schema{Type: "null"}, "null"},
		{"integer from fraction string", model.Integer(), "42.1"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := validatePlain(t, tc.schema, tc.input); err == nil {
				t.Fatalf("expected validation error")
			} else if !strings.Contains(err.Error(), "Validation failed") {
				t.Fatalf("error = %v, want a Validation failed message", err)
			}
		})
	}
}

func TestValidateToolArgumentsDropsOptionalNulls(t *testing.T) {
	nullableUnion := &model.Schema{AnyOf: []*model.Schema{model.String(), {Type: "null"}}}
	parameters := model.Object(
		model.Prop("path", model.String()),
		model.Opt("offset", model.Number()),
		model.Opt("nullable", nullableUnion),
		model.Prop("metadata", model.Object(model.Opt("enabled", model.Boolean()))),
	)
	tool := model.Tool{Name: "echo", Description: "Echo", Parameters: parameters}
	args, err := ValidateToolArguments(tool, model.ToolCall{
		ID: "tool-1", Name: "echo",
		Arguments: map[string]any{"path": "file.txt", "offset": nil, "nullable": nil, "metadata": map[string]any{"enabled": nil}},
	})
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if _, present := args["offset"]; present {
		t.Fatalf("optional null offset was not dropped: %v", args)
	}
	if value, present := args["nullable"]; !present || value != nil {
		t.Fatalf("nullable null was not preserved: %v", args)
	}
	metadata, _ := args["metadata"].(map[string]any)
	if _, present := metadata["enabled"]; present {
		t.Fatalf("nested optional null was not dropped: %v", args)
	}
}

func TestValidateToolArgumentsCoercesAnyOf(t *testing.T) {
	nullable := &model.Schema{AnyOf: []*model.Schema{model.Number(), {Type: "null"}}}
	got := mustValue(t, nullable, "42")
	if !jsonValueEqual(got, float64(42)) {
		t.Fatalf("got %#v, want 42", got)
	}
}

func TestValidateToolArgumentsPreservesMatchingUnionArm(t *testing.T) {
	oneOf := &model.Schema{OneOf: []*model.Schema{model.Number(), {Type: "null"}}}
	got := mustValue(t, oneOf, nil)
	if got != nil {
		t.Fatalf("got %#v, want nil", got)
	}
}

func TestValidateToolCallReportsMissingTool(t *testing.T) {
	_, err := ValidateToolCall(nil, model.ToolCall{ID: "tool-1", Name: "missing"})
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("err = %v, want not found", err)
	}
}

func TestValidateToolArgumentsAcceptsNullableArray(t *testing.T) {
	arraySchema := &model.Schema{Type: "array", Nullable: true, Items: model.String()}
	got := mustValue(t, arraySchema, nil)
	if got != nil {
		t.Fatalf("got %#v, want nil", got)
	}
}
