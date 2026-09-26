package wire

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestRepairJSON(t *testing.T) {
	cases := []struct{ in, want string }{
		{`{"a":"line` + "\n" + `break"}`, `{"a":"line\nbreak"}`},
		{`{"a":"tab` + "\t" + `}"}`, `{"a":"tab\t}"}`},
		{`{"a":"bad\q"}`, `{"a":"bad\\q"}`},
		{`{"a":"ok\n"}`, `{"a":"ok\n"}`},
		{`{"a":"unicode\u0041"}`, `{"a":"unicode\u0041"}`},
		{`{"a":"trailing\`, `{"a":"trailing\\`},
	}
	for _, c := range cases {
		if got := RepairJSON(c.in); got != c.want {
			t.Errorf("RepairJSON(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestParseJSONWithRepair(t *testing.T) {
	type payload struct {
		A string `json:"a"`
	}
	value, err := ParseJSONWithRepair[payload]("{\"a\":\"line\nbreak\"}")
	if err != nil {
		t.Fatalf("ParseJSONWithRepair error = %v", err)
	}
	if value.A != "line\nbreak" {
		t.Fatalf("ParseJSONWithRepair = %q, want line\\nbreak", value.A)
	}
}

// TestParseStreamingJSONTrailingCommaInsideString pins that a trailing comma
// inside an open string is content, not a dangling token.
func TestParseStreamingJSONTrailingCommaInsideString(t *testing.T) {
	cases := []struct {
		in   string
		want map[string]any
	}{
		{`{"a":"x,`, map[string]any{"a": "x,"}},
		{`{"a":"x, `, map[string]any{"a": "x,"}},
		{`{"a":1,`, map[string]any{"a": float64(1)}},
		{`{"a":1, `, map[string]any{"a": float64(1)}},
		{`{"a":"x,","b`, map[string]any{"a": "x,"}},
		{`{"key":`, map[string]any{}},
	}
	for _, c := range cases {
		if got := ParseStreamingJSON(c.in); !reflect.DeepEqual(got, c.want) {
			t.Errorf("ParseStreamingJSON(%q) = %#v want %#v", c.in, got, c.want)
		}
	}
}

// TestParseStreamingJSONDropsDanglingMember mirrors the partial-json behavior
// for members whose value has not started.
func TestParseStreamingJSONDropsDanglingMember(t *testing.T) {
	cases := []struct{ in, want string }{
		{`{"b":1,"a":"x","c":`, `{"b":1,"a":"x"}`},
		{`{"b":1,"a":"x","c" :`, `{"b":1,"a":"x"}`},
		{`{"b":1,"a":"x" , "c": `, `{"b":1,"a":"x"}`},
		{`{"key":`, `{}`},
		{`{"a":{"b":`, `{"a":{}}`},
		{`{"a":{"x":1,"b":`, `{"a":{"x":1}}`},
		{`{"x":1,"a,b":`, `{"x":1}`},
		{`{"x":1,"a\",b":`, `{"x":1}`},
		{`{"x":[1,{"y":`, `{"x":[1,{}]}`},
		{`{"x":"v:","y":`, `{"x":"v:"}`},
		{`{"b":1,"c"`, `{"b":1}`},
		{`{"b":1,"c" `, `{"b":1}`},
		{`{"a":"x"`, `{"a":"x"}`},
		{`{"a":["x"`, `{"a":["x"]}`},
		{`{"a":"x","b":"y"`, `{"a":"x","b":"y"}`},
		{`{"a":{"k":"v"`, `{"a":{"k":"v"}}`},
	}
	for _, c := range cases {
		got := ParseStreamingJSON(c.in)
		want := map[string]any{}
		if err := json.Unmarshal([]byte(c.want), &want); err != nil {
			t.Fatalf("bad want %q: %v", c.want, err)
		}
		if !reflect.DeepEqual(got, want) {
			gotJSON, _ := json.Marshal(got)
			t.Errorf("ParseStreamingJSON(%q) = %s, want %s", c.in, gotJSON, c.want)
		}
	}
}

func TestParseStreamingJSONNonObjectIsEmpty(t *testing.T) {
	for _, in := range []string{"", "   ", "12", `"hello"`, "[1,2]", "null", "not json"} {
		if got := ParseStreamingJSON(in); len(got) != 0 {
			t.Errorf("ParseStreamingJSON(%q) = %#v, want empty object", in, got)
		}
	}
}
