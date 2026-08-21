package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/victorlacorte/macos-dotfiles/tools/md-view/internal/mdview"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGHUP, syscall.SIGTERM)
	status := mdview.NewApp().Main(ctx, os.Args[1:])
	stop()
	os.Exit(status)
}
