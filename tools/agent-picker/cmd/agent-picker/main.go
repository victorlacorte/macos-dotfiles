package main

import (
	"context"
	"os"

	"github.com/victorlacorte/macos-dotfiles/tools/agent-picker/internal/agentpicker"
)

func main() {
	os.Exit(agentpicker.NewApp().Main(context.Background(), os.Args[1:]))
}
