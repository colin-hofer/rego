package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"rego/internal/app"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := app.Run(ctx, os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "rego: %v\n", err)
		os.Exit(1)
	}
}
