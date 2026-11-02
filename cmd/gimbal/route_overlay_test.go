package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestRouteOverlays(t *testing.T) {
	raw := `[{"Name":"river","Configuration":{"Environments":{"a":"one"},"nested":{"a":1,"b":[1]},"remove":"yes"}},{"Configuration":{"Environments":{"b":"two"},"nested":{"b":[2],"c":true},"remove":null}}]`
	path := filepath.Join(t.TempDir(), "route.json")
	if err := os.WriteFile(path, []byte(raw), 0600); err != nil {
		t.Fatal(err)
	}
	for _, input := range []string{raw, "@" + path} {
		route, err := readRoute(input)
		if err != nil {
			t.Fatal(err)
		}
		var got map[string]any
		if err := json.Unmarshal(route.Configuration, &got); err != nil {
			t.Fatal(err)
		}
		want := map[string]any{"Environments": map[string]any{"a": "one", "b": "two"}, "nested": map[string]any{"a": float64(1), "b": []any{float64(2)}, "c": true}, "remove": nil}
		if route.Name != "river" || !reflect.DeepEqual(got, want) {
			t.Fatalf("route: %+v configuration: %#v", route, got)
		}
	}
	route, err := readRoute(`[{"Name":"first","Configuration":{"old":true}},{"Name":"second","Configuration":{"new":true}}]`)
	if err != nil || string(route.Configuration) != `{"new":true}` {
		t.Fatalf("plugin switch: %+v %v", route, err)
	}
	if _, err := readRoute(`{"Name":"river","Configuration":{}}`); err != nil {
		t.Fatal(err)
	}
	for _, input := range []string{`[]`, `[{} , 1]`, `{"Name":"river"} {}`, `{"Name":"river","unknown":1}`, `{"Name":"river","name":"other"}`} {
		if _, err := readRoute(input); err == nil {
			t.Fatalf("accepted %s", input)
		}
	}
}
