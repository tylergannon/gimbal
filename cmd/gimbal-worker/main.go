package main

import (
	"context"
	"log"
	"os/signal"
	"syscall"

	"github.com/tylergannon/gimbal/internal/execution"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()
	if err := execution.RunWorker(ctx); err != nil && ctx.Err() == nil {
		log.Fatal(err)
	}
}
