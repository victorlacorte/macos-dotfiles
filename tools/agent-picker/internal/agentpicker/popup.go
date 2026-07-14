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
	if snapshot := a.snapshot(ctx, provider); len(snapshot.agents) == 0 {
		_, _ = a.run(ctx, "tmux", "display-message", noAgentsMessage(provider))
		return
	}
	width := a.option(ctx, "@agent_popup_width", "90%")
	height := a.option(ctx, "@agent_popup_height", "90%")
	parentOption := "@agent_parent"

	host := client
	if host != "" {
		_, _ = a.run(ctx, "tmux", "set-option", "-g", parentOption, host)
	}

	command := shellQuote(a.Executable) + " select -provider " + shellQuote(provider)
	args := []string{"display-popup"}
	if host != "" {
		args = append(args, "-c", host)
	}
	args = append(args, "-w", width, "-h", height, "-E", command)
	_, _ = a.run(ctx, "tmux", args...)
}
