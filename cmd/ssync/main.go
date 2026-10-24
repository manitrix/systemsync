package main

import (
	"context"
	"fmt"
	"github.com/manitrix/systemsync/internal/cli"
	"os"
	"os/signal"
	"syscall"
)

var version = "dev"
var commit = "unknown"
var date = "unknown"

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	root := cli.New(cli.Build{Version: version, Commit: commit, Date: date})
	if e := root.ExecuteContext(ctx); e != nil {
		fmt.Fprintln(os.Stderr, "ssync:", e)
		os.Exit(1)
	}
}
