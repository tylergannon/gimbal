package main

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

var commitHeader = regexp.MustCompile(`(?i)^([a-z]+)(\([^()]+\))?(!)?:\s\S`)
var breakingFooter = regexp.MustCompile(`(?m)^BREAKING[ -]CHANGE:`)

func nextVersion(previous, messages string) (string, error) {
	number, err := version(previous)
	if err != nil {
		return "", err
	}
	if number[0] != 0 {
		return "", errors.New("pre-release Gimbal has no users; major version increments are forbidden")
	}
	bump := 0
	for message := range strings.SplitSeq(messages, "\x00") {
		message = strings.TrimSpace(message)
		if message == "" {
			continue
		}
		header, _, _ := strings.Cut(message, "\n")
		match := commitHeader.FindStringSubmatch(header)
		if (len(match) > 0 && match[3] == "!") || breakingFooter.MatchString(message) {
			return "", fmt.Errorf("pre-release Gimbal has no users; major version increments are forbidden: remove the breaking-change marker from %q", header)
		}
		if len(match) == 0 {
			return "", fmt.Errorf("use a Conventional Commit title (feat: or fix:): %q", header)
		}
		switch strings.ToLower(match[1]) {
		case "feat":
			bump = 2
		case "fix", "perf":
			bump = max(bump, 1)
		}
	}
	switch bump {
	case 2:
		return fmt.Sprintf("v0.%d.0", number[1]+1), nil
	case 1:
		return fmt.Sprintf("v0.%d.%d", number[1], number[2]+1), nil
	default:
		return "", nil
	}
}
