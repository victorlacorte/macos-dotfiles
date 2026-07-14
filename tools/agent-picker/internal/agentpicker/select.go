package agentpicker

import (
	"context"
	"fmt"
	"strings"
)

func (a *App) Select(ctx context.Context, provider string) {
	for _, tool := range []string{"tmux", "fzf"} {
		if _, err := a.Runner.LookPath(tool); err == nil {
			continue
		}
		if _, err := a.Runner.LookPath("tmux"); err == nil {
			_, _ = a.run(ctx, "tmux", "display-message", "agent-picker: "+tool+" is required")
		} else {
			fmt.Fprintf(a.Stderr, "agent-picker: %s is required\n", tool)
		}
		return
	}

	options := a.option(ctx, "@agent_fzf_options", "")
	extra, err := SplitShellWords(options)
	if err != nil {
		_, _ = a.run(ctx, "tmux", "display-message", "agent-picker: invalid fzf options: "+err.Error())
		return
	}
	header := "Agents: enter jump, ctrl-x terminate"
	if provider == "claude" {
		header = "Claude agents: enter jump, ctrl-x terminate"
	} else if provider == "codex" {
		header = "Codex agents: enter jump, ctrl-x terminate"
	}
	args := []string{
		"--delimiter=\\t", "--with-nth=2,6,7,8,9", "--reverse", "--cycle",
		"--header=" + header,
		"--preview=tmux capture-pane -ept {2}", "--preview-window=up,70%,follow",
		`--bind=ctrl-x:execute-silent(kill {3})+reload(sleep 0.3; "$AGENT_PICKER" list -provider "$AGENT_PICKER_PROVIDER")`,
	}
	args = append(args, extra...)
	selected, _ := a.Runner.Run(ctx, Command{
		Name: "fzf", Args: args, Input: a.Rows(ctx, provider), Stderr: a.Stderr,
		Env: map[string]string{
			"FZF_DEFAULT_OPTS": "", "AGENT_PICKER": a.Executable,
			"AGENT_PICKER_PROVIDER": provider,
		},
	})
	selected = trimLine(selected)
	if selected == "" {
		return
	}
	fields := strings.Split(selected, "\t")
	if len(fields) < 5 {
		return
	}
	pane, kind, selectedProvider := fields[1], fields[3], fields[4]
	parent := a.tmux(ctx, "show-options", "-gqv", "@agent_parent")
	session := a.tmux(ctx, "display-message", "-p", "-t", pane, "#{session_name}")
	if kind == "loose" {
		if parent != "" {
			_, _ = a.run(ctx, "tmux", "switch-client", "-c", parent, "-t", session)
		} else {
			_, _ = a.run(ctx, "tmux", "switch-client", "-t", session)
		}
		_, _ = a.run(ctx, "tmux", "select-window", "-t", pane)
		_, _ = a.run(ctx, "tmux", "select-pane", "-t", pane)
		return
	}
	origin := a.tmux(ctx, "show-options", "-qv", "-t", session, "@"+selectedProvider+"_agent_origin")
	if origin == "" {
		origin = a.tmux(ctx, "show-options", "-qv", "-t", session, "@agent_origin")
	}
	if origin != "" && parent != "" {
		_, _ = a.run(ctx, "tmux", "switch-client", "-c", parent, "-t", origin)
	}
	_, _ = a.run(ctx, "tmux", "select-window", "-t", pane)
	_, _ = a.run(ctx, "tmux", "select-pane", "-t", pane)
	_, _ = a.Runner.Run(ctx, Command{
		Name: "tmux", Args: []string{"attach-session", "-t", session}, Interactive: true,
	})
}
