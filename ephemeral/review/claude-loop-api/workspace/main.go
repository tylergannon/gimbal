package main

import (
	"errors"
	"fmt"
	"math/big"
	"os"
	"strings"
)

type version struct {
	major, minor, patch *big.Int
	prerelease          bool
}

func main() {
	output, err := execute(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	fmt.Fprintln(os.Stdout, output)
}

func execute(args []string) (string, error) {
	if len(args) != 2 {
		return "", errors.New("usage: semverbump <version> <major|minor|patch|release>")
	}

	v, err := parseVersion(args[0])
	if err != nil {
		return "", err
	}

	switch args[1] {
	case "major":
		if v.prerelease && v.minor.Sign() == 0 && v.patch.Sign() == 0 {
			return coreString(v), nil
		}
		return bumped(new(big.Int).Add(v.major, big.NewInt(1)), big.NewInt(0), big.NewInt(0)), nil
	case "minor":
		if v.prerelease && v.patch.Sign() == 0 {
			return coreString(v), nil
		}
		return bumped(v.major, new(big.Int).Add(v.minor, big.NewInt(1)), big.NewInt(0)), nil
	case "patch":
		if v.prerelease {
			return coreString(v), nil
		}
		return bumped(v.major, v.minor, new(big.Int).Add(v.patch, big.NewInt(1))), nil
	case "release":
		if !v.prerelease {
			return "", errors.New("release requires a prerelease version")
		}
		return coreString(v), nil
	default:
		return "", fmt.Errorf("unknown bump kind %q", args[1])
	}
}

func bumped(major, minor, patch *big.Int) string {
	return major.String() + "." + minor.String() + "." + patch.String()
}

func coreString(v version) string {
	return bumped(v.major, v.minor, v.patch)
}

func parseVersion(input string) (version, error) {
	var zero version
	if strings.HasPrefix(input, "v") {
		input = input[1:]
	}
	if input == "" {
		return zero, errors.New("invalid version")
	}

	parts := strings.Split(input, "+")
	if len(parts) > 2 || parts[0] == "" {
		return zero, errors.New("invalid version")
	}
	if len(parts) == 2 && !validIdentifiers(parts[1], false) {
		return zero, errors.New("invalid version")
	}

	coreAndPrerelease := strings.SplitN(parts[0], "-", 2)
	core := strings.Split(coreAndPrerelease[0], ".")
	if len(core) != 3 {
		return zero, errors.New("invalid version")
	}
	major, ok := numericIdentifier(core[0])
	if !ok {
		return zero, errors.New("invalid version")
	}
	minor, ok := numericIdentifier(core[1])
	if !ok {
		return zero, errors.New("invalid version")
	}
	patch, ok := numericIdentifier(core[2])
	if !ok {
		return zero, errors.New("invalid version")
	}

	prerelease := false
	if len(coreAndPrerelease) == 2 {
		prerelease = true
		if !validIdentifiers(coreAndPrerelease[1], true) {
			return zero, errors.New("invalid version")
		}
	}

	return version{major: major, minor: minor, patch: patch, prerelease: prerelease}, nil
}

func numericIdentifier(input string) (*big.Int, bool) {
	if input == "" || (len(input) > 1 && input[0] == '0') || !allDigits(input) {
		return nil, false
	}
	n, ok := new(big.Int).SetString(input, 10)
	return n, ok
}

func validIdentifiers(input string, prerelease bool) bool {
	if input == "" {
		return false
	}
	for _, identifier := range strings.Split(input, ".") {
		if identifier == "" {
			return false
		}
		if !allSemverIdentifierChars(identifier) {
			return false
		}
		if prerelease && len(identifier) > 1 && identifier[0] == '0' && allDigits(identifier) {
			return false
		}
	}
	return true
}

func allDigits(input string) bool {
	for i := 0; i < len(input); i++ {
		if input[i] < '0' || input[i] > '9' {
			return false
		}
	}
	return true
}

func allSemverIdentifierChars(input string) bool {
	for i := 0; i < len(input); i++ {
		c := input[i]
		if (c < '0' || c > '9') && (c < 'A' || c > 'Z') && (c < 'a' || c > 'z') && c != '-' {
			return false
		}
	}
	return true
}
