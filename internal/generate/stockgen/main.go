package main

import (
	"log"
	"os"

	"github.com/tylergannon/gimble/internal/generate"
)

func main() {
	if len(os.Args) != 2 {
		log.Fatal("usage: stockgen REPOSITORY_ROOT")
	}
	if err := generate.Stock(os.Args[1]); err != nil {
		log.Fatal(err)
	}
}
