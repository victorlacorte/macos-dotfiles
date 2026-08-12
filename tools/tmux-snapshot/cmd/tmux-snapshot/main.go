package main

import (
	"context"
	"os"

	"github.com/victorlacorte/macos-dotfiles/tools/tmux-snapshot/internal/tmuxsnapshot"
)

func main() {
	os.Exit(tmuxsnapshot.NewApp().Main(context.Background(), os.Args[1:]))
}
