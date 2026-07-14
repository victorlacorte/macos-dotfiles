package agentpicker

import (
	"context"
	"fmt"
)

func (a *App) Popup(ctx context.Context, provider, client string) {
	if _, err := a.Runner.LookPath("tmux"); err != nil {
		fmt.Fprintln(a.Stderr, "agent-picker: tmux is required")
		return
	}
	width := a.option(ctx, "@agent_popup_width", "90%")
	height := a.option(ctx, "@agent_popup_height", "90%")

	command := "AGENT_PICKER_CLIENT=" + shellQuote(client) + " " +
		shellQuote(a.Executable) + " select -provider " + shellQuote(provider)
	args := []string{"display-popup"}
	if client != "" {
		args = append(args, "-c", client)
	}
	args = append(args, "-w", width, "-h", height, "-E", command)
	_, _ = a.run(ctx, "tmux", args...)
}
