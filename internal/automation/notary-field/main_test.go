package main

import (
	"strings"
	"testing"
)

func TestNotaryResponse(t *testing.T) {
	for _, test := range []struct {
		name, input, want string
		invalid           bool
	}{
		{"status", `{"id":"submission","status":"Accepted"}`, "Accepted", false}, {"id", `{"id":"submission","status":"Invalid"}`, "submission", false}, {"id", `{"status":"Invalid"}`, "", false}, {"status", `garbage`, "", true}, {"status", `{"status":4}`, "", true}, {"unexpected", `{}`, "", true},
	} {
		t.Run(test.name+test.input, func(t *testing.T) {
			got, err := field(strings.NewReader(test.input), test.name)
			if (err != nil) != test.invalid || got != test.want {
				t.Fatalf("got%q error%v", got, err)
			}
		})
	}
}
