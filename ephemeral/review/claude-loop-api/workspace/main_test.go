package main

import "testing"

func TestExecuteBumpsVersions(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "major", args: []string{"1.2.3", "major"}, want: "2.0.0"},
		{name: "minor", args: []string{"10.20.30", "minor"}, want: "10.21.0"},
		{name: "patch", args: []string{"0.0.0", "patch"}, want: "0.0.1"},
		{name: "leading v and build metadata", args: []string{"v1.2.3+build.7", "patch"}, want: "1.2.4"},
		{name: "prerelease patch finalizes", args: []string{"1.2.3-rc.1+meta", "patch"}, want: "1.2.3"},
		{name: "prerelease minor advances", args: []string{"1.2.3-rc.1", "minor"}, want: "1.3.0"},
		{name: "zero patch minor finalizes", args: []string{"1.2.0-beta.2", "minor"}, want: "1.2.0"},
		{name: "zero lower components major finalizes", args: []string{"2.0.0-alpha.1", "major"}, want: "2.0.0"},
		{name: "release", args: []string{"1.2.3-alpha", "release"}, want: "1.2.3"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := execute(test.args)
			if err != nil {
				t.Fatalf("execute(%q): unexpected error: %v", test.args, err)
			}
			if got != test.want {
				t.Fatalf("execute(%q) = %q, want %q", test.args, got, test.want)
			}
		})
	}
}

func TestExecuteRejectsInvalidInput(t *testing.T) {
	tests := [][]string{
		{"1.2.3", "release"},
		{"1.2", "patch"},
		{"01.2.3", "patch"},
		{"1.2.3-", "patch"},
		{"1.2.3-alpha..1", "patch"},
		{"1.2.3-01", "patch"},
		{"1.2.3+", "patch"},
		{"1.2.3+meta..data", "patch"},
		{"a.b.c", "patch"},
		{"1.2.3", "bump"},
		{"1.2.3"},
		{},
		{"1.2.3", "patch", "extra"},
		{"-1.2.3", "patch"},
	}

	for _, args := range tests {
		t.Run(joinArgs(args), func(t *testing.T) {
			if output, err := execute(args); err == nil {
				t.Fatalf("execute(%q) = %q, nil error; want failure", args, output)
			}
		})
	}
}

func TestParseVersionAcceptsSemverIdentifiers(t *testing.T) {
	for _, input := range []string{
		"0.0.0",
		"v1.2.3-rc.1+build.7",
		"1.2.3-alpha-1+001",
		"999999999999999999999999.0.0",
	} {
		if _, err := parseVersion(input); err != nil {
			t.Errorf("parseVersion(%q): unexpected error: %v", input, err)
		}
	}
}

func joinArgs(args []string) string {
	if len(args) == 0 {
		return "no arguments"
	}
	result := args[0]
	for _, arg := range args[1:] {
		result += " " + arg
	}
	return result
}
