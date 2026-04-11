package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/ochanuco/marume/internal/cli"
)

// main wires OS signals into the CLI runner and maps returned errors to exit codes.
func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	args := os.Args[1:]
	if err := cli.Run(ctx, args, os.Stdin, os.Stdout, os.Stderr); err != nil {
		if errors.Is(err, context.Canceled) {
			os.Exit(0)
		}
		if cli.JSONErrorsEnabled(args) {
			if writeErr := cli.WriteErrorJSON(os.Stderr, err); writeErr != nil {
				fmt.Fprintln(os.Stderr, err)
			}
		} else {
			fmt.Fprintln(os.Stderr, err)
		}
		os.Exit(cli.ExitCode(err))
	}
}
