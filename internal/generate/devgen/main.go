// Command devgen supplies Gimbal-owned preparation to skgo dev. It is never
// imported by an application entrypoint.
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/tylergannon/gimbal/internal/devgenerate"
	"github.com/tylergannon/skgo/devgen"
)

func main() {
	root, err := os.Getwd()
	if err == nil {
		var generator *devgenerate.Generator
		generator, err = devgenerate.New(root)
		if err == nil {
			defer func() { _ = generator.Close() }()
			err = devgen.Serve(context.Background(), generator.Generate)
		}
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "gimbal dev generation:", err)
		os.Exit(1)
	}
}
