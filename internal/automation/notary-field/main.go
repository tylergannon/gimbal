// Notary field reads the required string from Apple's JSON submission response.
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
)

func field(reader io.Reader, name string) (string, error) {
	if name != "id" && name != "status" {
		return "", errors.New("expected id or status")
	}
	var response map[string]json.RawMessage
	rawResponse, err := io.ReadAll(reader)
	if err != nil {
		return "", err
	}
	if err := json.Unmarshal(rawResponse, &response); err != nil {
		return "", err
	}
	raw, ok := response[name]
	if !ok {
		return "", nil
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return "", err
	}
	return value, nil
}
func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: notary-field id|status < RESPONSE.json")
		os.Exit(1)
	}
	value, err := field(os.Stdin, os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println(value)
}
