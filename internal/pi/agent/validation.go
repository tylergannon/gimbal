package agent

import (
	"encoding/json"
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"

	"github.com/tylergannon/gimbal/internal/pi/model"
)

// This file ports packages/ai/src/utils/validation.ts: tool arguments are
// cloned, nulls for optional non-nullable properties are dropped, primitive
// values are coerced to a schema's declared types, and the result is validated
// against the schema. Errors are formatted so the model can correct itself.

// ValidateToolCall finds a tool by name and validates the call's arguments.
func ValidateToolCall(tools []model.AgentTool, toolCall model.ToolCall) (map[string]any, error) {
	for _, tool := range tools {
		if tool.Name == toolCall.Name {
			return ValidateToolArguments(tool.AsTool(), toolCall)
		}
	}
	return nil, fmt.Errorf("Tool %q not found", toolCall.Name) //nolint:staticcheck // pi's exact error message
}

// ValidateToolArguments validates and coerces tool-call arguments against the
// tool's schema, returning the validated arguments or a pi-formatted error.
func ValidateToolArguments(tool model.Tool, toolCall model.ToolCall) (map[string]any, error) {
	args, _ := deepCopyValue(toolCall.Arguments).(map[string]any)
	if args == nil {
		args = map[string]any{}
	}
	normalizeOptionalNulls(args, tool.Parameters)
	if coerced, ok := coerceWithJSONSchema(args, tool.Parameters).(map[string]any); ok {
		args = coerced
	}

	if errs := validateValue(tool.Parameters, args, ""); len(errs) > 0 {
		lines := make([]string, len(errs))
		for i, e := range errs {
			lines[i] = "  - " + e
		}
		received, _ := json.MarshalIndent(toolCall.Arguments, "", "  ")
		return nil, fmt.Errorf("Validation failed for tool %q:\n%s\n\nReceived arguments:\n%s", //nolint:staticcheck // pi's exact error message
			toolCall.Name, strings.Join(lines, "\n"), string(received))
	}
	return args, nil
}

func deepCopyValue(v any) any {
	switch t := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(t))
		for k, e := range t {
			out[k] = deepCopyValue(e)
		}
		return out
	case []any:
		out := make([]any, len(t))
		for i, e := range t {
			out[i] = deepCopyValue(e)
		}
		return out
	case json.RawMessage:
		return append(json.RawMessage(nil), t...)
	default:
		return v
	}
}

func schemaTypes(s *model.Schema) []string {
	if s == nil {
		return nil
	}
	var types []string
	if s.Type != "" {
		types = append(types, s.Type)
	}
	if s.Nullable {
		types = append(types, "null")
	}
	return types
}

func matchesJSONType(value any, schemaType string) bool {
	switch schemaType {
	case "number":
		return isNumber(value)
	case "integer":
		f, ok := numberValue(value)
		return ok && math.Trunc(f) == f
	case "boolean":
		_, ok := value.(bool)
		return ok
	case "string":
		_, ok := value.(string)
		return ok
	case "null":
		return value == nil
	case "array":
		_, ok := value.([]any)
		return ok
	case "object":
		_, ok := value.(map[string]any)
		return ok
	default:
		return false
	}
}

func isNumber(value any) bool {
	_, ok := numberValue(value)
	return ok
}

func numberValue(value any) (float64, bool) {
	switch t := value.(type) {
	case float64:
		return t, true
	case float32:
		return float64(t), true
	case int:
		return float64(t), true
	case int64:
		return float64(t), true
	case json.Number:
		f, err := t.Float64()
		return f, err == nil
	default:
		return 0, false
	}
}

// schemaAccepts reports whether a value satisfies the schema's type,
// combinators and const/enum constraints. It mirrors the subset of JSON Schema
// the tool validators rely on.
func schemaAccepts(s *model.Schema, value any) bool {
	if s == nil {
		return true
	}
	if s.HasConst {
		return jsonValueEqual(s.Const, value)
	}
	if len(s.Enum) > 0 {
		for _, e := range s.Enum {
			if jsonValueEqual(e, value) {
				return true
			}
		}
		return false
	}
	for _, sub := range s.AllOf {
		if !schemaAccepts(sub, value) {
			return false
		}
	}
	if len(s.AnyOf) > 0 {
		matched := false
		for _, sub := range s.AnyOf {
			if schemaAccepts(sub, value) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	if len(s.OneOf) > 0 {
		count := 0
		for _, sub := range s.OneOf {
			if schemaAccepts(sub, value) {
				count++
			}
		}
		if count != 1 {
			return false
		}
	}
	types := schemaTypes(s)
	if len(types) == 0 {
		return true
	}
	if value == nil {
		return slices.Contains(types, "null")
	}
	for _, t := range types {
		if matchesJSONType(value, t) {
			return true
		}
	}
	return false
}

// validateValue returns pi-formatted validation problems, one per line without
// the leading "  - ".
func validateValue(s *model.Schema, value any, path string) []string {
	if s == nil {
		return nil
	}
	var errs []string
	formatPath := func(base, key string) string {
		if base == "" {
			return key
		}
		return base + "." + key
	}

	if s.HasConst && !jsonValueEqual(s.Const, value) {
		errs = append(errs, pathOrRoot(path)+": must equal the declared const")
		return errs
	}
	if len(s.Enum) > 0 {
		found := false
		for _, e := range s.Enum {
			if jsonValueEqual(e, value) {
				found = true
				break
			}
		}
		if !found {
			errs = append(errs, pathOrRoot(path)+": must be one of the declared values")
			return errs
		}
	}

	for _, sub := range s.AllOf {
		errs = append(errs, validateValue(sub, value, path)...)
	}
	if len(s.AnyOf) > 0 {
		matched := false
		for _, sub := range s.AnyOf {
			if schemaAccepts(sub, value) {
				matched = true
				break
			}
		}
		if !matched {
			errs = append(errs, pathOrRoot(path)+": must match at least one allowed schema")
		}
	}
	if len(s.OneOf) > 0 {
		count := 0
		for _, sub := range s.OneOf {
			if schemaAccepts(sub, value) {
				count++
			}
		}
		if count != 1 {
			errs = append(errs, pathOrRoot(path)+": must match exactly one allowed schema")
		}
	}

	types := schemaTypes(s)
	if len(types) == 0 {
		return errs
	}
	matched := false
	for _, t := range types {
		if (value == nil && t == "null") || matchesJSONType(value, t) {
			matched = true
			break
		}
	}
	if !matched {
		errs = append(errs, pathOrRoot(path)+": expected "+strings.Join(types, " or "))
		return errs
	}

	if object, ok := value.(map[string]any); ok && containsType(types, "object") {
		for _, name := range s.Required {
			if _, present := object[name]; !present {
				errs = append(errs, formatPath(path, name)+": must have required property "+strconv.Quote(name))
			}
		}
		for _, name := range s.OrderedProperties() {
			propValue, present := object[name]
			if !present {
				continue
			}
			errs = append(errs, validateValue(s.Properties[name], propValue, formatPath(path, name))...)
		}
	}
	if arr, ok := value.([]any); ok && containsType(types, "array") && s.Items != nil {
		for i, item := range arr {
			errs = append(errs, validateValue(s.Items, item, fmt.Sprintf("%s[%d]", pathOrEmpty(path), i))...)
		}
	}
	return errs
}

func containsType(types []string, want string) bool {
	return slices.Contains(types, want)
}

func pathOrRoot(path string) string {
	if path == "" {
		return "root"
	}
	return path
}

func pathOrEmpty(path string) string {
	return path
}

func jsonValueEqual(a, b any) bool {
	ra, errA := json.Marshal(a)
	rb, errB := json.Marshal(b)
	if errA != nil || errB != nil {
		return false
	}
	return string(ra) == string(rb)
}

// normalizeOptionalNulls treats null as omission: strict constrained sampling
// forces the model to emit every property, so optional properties come back as
// explicit nulls. A null value for a present property is deleted when the
// property is not required and its schema does not accept null.
func normalizeOptionalNulls(value any, schema *model.Schema) {
	if arr, ok := value.([]any); ok {
		if schema != nil && schema.Items != nil {
			for _, item := range arr {
				normalizeOptionalNulls(item, schema.Items)
			}
		}
		return
	}
	obj, ok := value.(map[string]any)
	if !ok || schema == nil || schema.Properties == nil {
		return
	}
	required := make(map[string]bool, len(schema.Required))
	for _, key := range schema.Required {
		required[key] = true
	}
	for key, propertySchema := range schema.Properties {
		v, present := obj[key]
		if !present || propertySchema == nil {
			continue
		}
		_, refIsString := propertySchema.Extra["$ref"].(string)
		if v == nil && !required[key] && !refIsString && !schemaAccepts(propertySchema, nil) {
			delete(obj, key)
		} else {
			normalizeOptionalNulls(v, propertySchema)
		}
	}
}

func coercePrimitiveByType(value any, schemaType string) any {
	switch schemaType {
	case "number":
		if value == nil {
			return float64(0)
		}
		if s, ok := value.(string); ok && strings.TrimSpace(s) != "" {
			if parsed, err := strconv.ParseFloat(strings.TrimSpace(s), 64); err == nil {
				return parsed
			}
		}
		if b, ok := value.(bool); ok {
			if b {
				return float64(1)
			}
			return float64(0)
		}
		return value
	case "integer":
		if value == nil {
			return float64(0)
		}
		if s, ok := value.(string); ok && strings.TrimSpace(s) != "" {
			if parsed, err := strconv.ParseFloat(strings.TrimSpace(s), 64); err == nil && math.Trunc(parsed) == parsed {
				return parsed
			}
		}
		if b, ok := value.(bool); ok {
			if b {
				return float64(1)
			}
			return float64(0)
		}
		return value
	case "boolean":
		if value == nil {
			return false
		}
		if s, ok := value.(string); ok {
			if s == "true" {
				return true
			}
			if s == "false" {
				return false
			}
		}
		if f, ok := numberValue(value); ok {
			if f == 1 {
				return true
			}
			if f == 0 {
				return false
			}
		}
		return value
	case "string":
		if value == nil {
			return ""
		}
		switch t := value.(type) {
		case float64:
			return strconv.FormatFloat(t, 'f', -1, 64)
		case int:
			return strconv.Itoa(t)
		case int64:
			return strconv.FormatInt(t, 10)
		case json.Number:
			return t.String()
		case bool:
			return strconv.FormatBool(t)
		default:
			return value
		}
	case "null":
		if s, ok := value.(string); ok && s == "" {
			return nil
		}
		if b, ok := value.(bool); ok && !b {
			return nil
		}
		if f, ok := numberValue(value); ok && f == 0 {
			return nil
		}
		return value
	default:
		return value
	}
}

func applySchemaObjectCoercion(value map[string]any, schema *model.Schema) {
	defined := make(map[string]bool, len(schema.Properties))
	for key, propertySchema := range schema.Properties {
		defined[key] = true
		if propValue, present := value[key]; present {
			value[key] = coerceWithJSONSchema(propValue, propertySchema)
		}
	}
	if schema.AdditionalSchema != nil {
		for key, propertyValue := range value {
			if defined[key] {
				continue
			}
			value[key] = coerceWithJSONSchema(propertyValue, schema.AdditionalSchema)
		}
	}
}

func applySchemaArrayCoercion(value []any, schema *model.Schema) {
	if schema.Items == nil {
		return
	}
	for i := range value {
		value[i] = coerceWithJSONSchema(value[i], schema.Items)
	}
}

func coerceWithUnionSchema(value any, schemas []*model.Schema) any {
	for _, schema := range schemas {
		if schemaAccepts(schema, value) {
			return value
		}
	}
	for _, schema := range schemas {
		candidate := coerceWithJSONSchema(deepCopyValue(value), schema)
		if schemaAccepts(schema, candidate) {
			return candidate
		}
	}
	return value
}

func coerceWithJSONSchema(value any, schema *model.Schema) any {
	if schema == nil {
		return value
	}
	next := value
	for _, nested := range schema.AllOf {
		next = coerceWithJSONSchema(next, nested)
	}
	if len(schema.AnyOf) > 0 {
		next = coerceWithUnionSchema(next, schema.AnyOf)
	}
	if len(schema.OneOf) > 0 {
		next = coerceWithUnionSchema(next, schema.OneOf)
	}

	types := schemaTypes(schema)
	matchesUnionMember := len(types) > 1
	if matchesUnionMember {
		matchesUnionMember = false
		for _, t := range types {
			if (next == nil && t == "null") || matchesJSONType(next, t) {
				matchesUnionMember = true
				break
			}
		}
	}
	if len(types) > 0 && !matchesUnionMember {
		for _, t := range types {
			candidate := coercePrimitiveByType(next, t)
			if !jsonValueEqual(candidate, next) {
				next = candidate
				break
			}
		}
	}

	if object, ok := next.(map[string]any); ok && containsType(types, "object") {
		applySchemaObjectCoercion(object, schema)
	}
	if arr, ok := next.([]any); ok && containsType(types, "array") {
		applySchemaArrayCoercion(arr, schema)
	}
	return next
}
