package agentpicker

import (
	"context"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"strconv"
	"strings"
	"time"
)

func (a *App) terminateCommand(ctx context.Context, args []string) int {
	flags := flag.NewFlagSet("terminate", flag.ContinueOnError)
	flags.SetOutput(a.Stderr)
	provider := flags.String("provider", "", "provider of the selected process")
	pane := flags.String("pane", "", "tmux pane ID")
	pid := flags.Int("pid", 0, "selected PID")
	identity := flags.String("identity", "", "process identity from list -identities")
	flags.Usage = func() {
		fmt.Fprintln(a.Stderr, "Usage: agent-picker terminate -provider PROVIDER -pane PANE -pid PID -identity IDENTITY")
	}
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	*provider = strings.TrimSpace(*provider) // fzf's provider display is padded.
	paneNumber, paneErr := strconv.Atoi(strings.TrimPrefix(*pane, "%"))
	decoded, identityErr := hex.DecodeString(*identity)
	if providerLabel(*provider) == "" || *pid <= 1 || !strings.HasPrefix(*pane, "%") || paneErr != nil || paneNumber < 0 ||
		identityErr != nil || len(decoded) != 32 || flags.NArg() != 0 {
		fmt.Fprintln(a.Stderr, "agent-picker terminate: provider, pane, positive PID, and process identity are required")
		flags.Usage()
		return 2
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := a.terminate(ctx, *provider, *pane, *pid, *identity); err != nil {
		message := "agent-picker: " + err.Error()
		fmt.Fprintln(a.Stderr, message)
		messageCtx, cancelMessage := context.WithTimeout(context.WithoutCancel(ctx), time.Second)
		defer cancelMessage()
		a.displayMessage(messageCtx, getenv("AGENT_PICKER_CLIENT"), message)
		return 1
	}
	return 0
}

func (a *App) terminate(ctx context.Context, provider, pane string, pid int, identity string) error {
	changed := errors.New("session changed or is no longer foreground; reload the picker")
	if pid <= 1 || identity == "" || providerLabel(provider) == "" {
		return changed
	}
	// Re-run provider and pane validation, rather than trusting an old fzf row.
	found := false
	for _, agent := range a.Agents(ctx, provider) {
		if agent.PID == pid && agent.Pane == pane && agent.Identity == identity {
			found = true
			break
		}
	}
	if !found {
		return changed
	}
	// Sample again immediately before the signal to narrow the discovery race.
	p, ok := a.readProcesses(ctx, "-p", strconv.Itoa(pid), "-o", processFormat)[pid]
	if !ok || !p.foreground() || p.identity() != identity {
		return changed
	}
	if provider == "claude" {
		if !a.claudeProcess(p) {
			return changed
		}
	} else if !a.interactiveProcess(p, provider) {
		return changed
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := a.Runner.Terminate(pid); err != nil {
		return fmt.Errorf("could not terminate process: %w", err)
	}
	return nil
}
