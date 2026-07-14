package agentpicker

import (
	"context"
	"fmt"
	"io"
	"strings"
)

type discoveryResult struct {
	empty bool
}

func (a *App) Select(ctx context.Context, provider string) {
	client := getenv("AGENT_PICKER_CLIENT")
	for _, tool := range []string{"tmux", "fzf"} {
		if _, err := a.Runner.LookPath(tool); err == nil {
			continue
		}
		if _, err := a.Runner.LookPath("tmux"); err == nil {
			a.displayMessage(ctx, client, "agent-picker: "+tool+" is required")
		} else {
			fmt.Fprintf(a.Stderr, "agent-picker: %s is required\n", tool)
		}
		return
	}

	options := a.option(ctx, "@agent_fzf_options", "")
	extra, err := SplitShellWords(options)
	if err != nil {
		a.displayMessage(ctx, client, "agent-picker: invalid fzf options: "+err.Error())
		return
	}
	header := "Agents: enter jump, ctrl-x terminate"
	if provider == "claude" {
		header = "Claude agents: enter jump, ctrl-x terminate"
	} else if provider == "codex" {
		header = "Codex agents: enter jump, ctrl-x terminate"
	}
	args := []string{
		"--delimiter=\\t", "--with-nth=2,5,6,7,8", "--reverse", "--cycle",
		"--header=" + header,
		"--preview=tmux capture-pane -ept {2}", "--preview-window=up,70%,follow",
		`--bind=ctrl-x:execute-silent(kill {3})+reload(sleep 0.3; "$AGENT_PICKER" list -provider "$AGENT_PICKER_PROVIDER")`,
	}
	args = append(args, extra...)

	discoveryCtx, cancelDiscovery := context.WithCancel(ctx)
	reader, writer := io.Pipe()
	result := make(chan discoveryResult, 1)
	go func() {
		snapshot := a.snapshot(discoveryCtx, provider)
		empty := len(snapshot.agents) == 0 && discoveryCtx.Err() == nil
		if empty {
			_ = writer.Close()
			result <- discoveryResult{empty: true}
			cancelDiscovery()
			return
		}
		_, writeErr := io.WriteString(writer, snapshot.rows())
		_ = writer.CloseWithError(writeErr)
		result <- discoveryResult{}
	}()

	selected, _ := a.Runner.Run(discoveryCtx, Command{
		Name: "fzf", Args: args, Input: reader, Stderr: a.Stderr,
		Env: map[string]string{
			"FZF_DEFAULT_OPTS": "", "AGENT_PICKER": a.Executable,
			"AGENT_PICKER_PROVIDER": provider,
		},
	})
	cancelDiscovery()
	_ = reader.CloseWithError(context.Canceled)
	discovery := <-result
	if discovery.empty {
		a.displayMessage(ctx, client, noAgentsMessage(provider))
		return
	}

	selected = trimLine(selected)
	if selected == "" {
		return
	}
	fields := strings.Split(selected, "\t")
	if len(fields) < 3 {
		return
	}
	pane := fields[1]
	session := a.tmux(ctx, "display-message", "-p", "-t", pane, "#{session_name}")
	if client != "" {
		_, _ = a.run(ctx, "tmux", "switch-client", "-c", client, "-t", session)
	} else {
		_, _ = a.run(ctx, "tmux", "switch-client", "-t", session)
	}
	_, _ = a.run(ctx, "tmux", "select-window", "-t", pane)
	_, _ = a.run(ctx, "tmux", "select-pane", "-t", pane)
}

func (a *App) displayMessage(ctx context.Context, client, message string) {
	args := []string{"display-message"}
	if client != "" {
		args = append(args, "-c", client)
	}
	args = append(args, message)
	_, _ = a.run(ctx, "tmux", args...)
}
