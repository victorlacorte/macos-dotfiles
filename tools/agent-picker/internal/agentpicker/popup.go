package agentpicker

import (
	"context"
	"fmt"
	"strings"
	"time"
)

func (a *App) Popup(ctx context.Context, provider, client string) {
	if _, err := a.Runner.LookPath("tmux"); err != nil {
		fmt.Fprintln(a.Stderr, "agent-picker: tmux is required")
		return
	}
	claudePrefix := a.option(ctx, "@claude_agent_session_prefix", "claude-")
	codexPrefix := a.option(ctx, "@codex_agent_session_prefix", "codex-")
	width := a.option(ctx, "@agent_popup_width", "90%")
	height := a.option(ctx, "@agent_popup_height", "90%")
	parentOption := "@agent_parent"

	mySession := sessionForClient(a.tmux(ctx, "list-clients", "-F", "#{client_name} #{session_name}"), client)
	dedicated := (provider == "all" || provider == "claude") && strings.HasPrefix(mySession, claudePrefix) ||
		(provider == "all" || provider == "codex") && strings.HasPrefix(mySession, codexPrefix)
	host := client
	if dedicated {
		_, _ = a.run(ctx, "tmux", "detach-client", "-s", mySession)
		for i := 0; i < 100; i++ {
			clients := a.tmux(ctx, "list-clients", "-F", "#{session_name}")
			if !hasLine(clients, mySession) {
				break
			}
			a.Clock.Sleep(50 * time.Millisecond)
		}
		host = a.tmux(ctx, "show-options", "-gqv", parentOption)
	} else if host != "" {
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

func sessionForClient(clients, name string) string {
	for _, line := range strings.Split(clients, "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[0] == name {
			return fields[1]
		}
	}
	return ""
}

func hasLine(lines, value string) bool {
	for _, line := range strings.Split(lines, "\n") {
		if line == value {
			return true
		}
	}
	return false
}
