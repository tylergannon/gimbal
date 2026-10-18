// Command gimbal-command accepts one identified command and prints its retained
// descriptor. The independent supervisor continues after this caller exits.
package main

import (
	"fmt"
	"github.com/tylergannon/gimbal/internal/commandexecutor"
	"os"
)

func main() {
	if err := commandexecutor.CLI(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
