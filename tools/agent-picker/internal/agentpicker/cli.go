package agentpicker

import (
	"context"
	"errors"
	"flag"
	"fmt"
)

const providerUsage = "all|claude|codex"

func (a *App) Main(ctx context.Context, args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(a.Stderr, "agent-picker: missing command")
		a.printUsage()
		return 2
	}

	command := args[0]
	if command != "popup" && command != "select" && command != "list" {
		fmt.Fprintf(a.Stderr, "agent-picker: unknown command %q\n", command)
		a.printUsage()
		return 2
	}

	flags := flag.NewFlagSet(command, flag.ContinueOnError)
	flags.SetOutput(a.Stderr)
	provider := flags.String("provider", "all", "provider to show ("+providerUsage+")")
	flags.Usage = func() { a.printCommandUsage(command, flags) }
	if err := flags.Parse(args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		flags.Usage()
		return 2
	}
	if !validProvider(*provider) {
		fmt.Fprintf(a.Stderr, "agent-picker %s: invalid provider %q (want %s)\n", command, *provider, providerUsage)
		flags.Usage()
		return 2
	}

	positionals := flags.Args()
	switch command {
	case "popup":
		if len(positionals) > 1 {
			fmt.Fprintln(a.Stderr, "agent-picker popup: expected at most one TMUX_CLIENT")
			flags.Usage()
			return 2
		}
		client := ""
		if len(positionals) == 1 {
			client = positionals[0]
		}
		a.Popup(ctx, *provider, client)
	case "select":
		if len(positionals) != 0 {
			fmt.Fprintf(a.Stderr, "agent-picker select: unexpected argument %q\n", positionals[0])
			flags.Usage()
			return 2
		}
		a.Select(ctx, *provider)
	case "list":
		if len(positionals) != 0 {
			fmt.Fprintf(a.Stderr, "agent-picker list: unexpected argument %q\n", positionals[0])
			flags.Usage()
			return 2
		}
		a.List(ctx, *provider)
	}
	return 0
}

func validProvider(provider string) bool {
	return provider == "all" || provider == "claude" || provider == "codex"
}

func (a *App) printUsage() {
	fmt.Fprintln(a.Stderr, "Usage:")
	fmt.Fprintln(a.Stderr, "  agent-picker popup [-provider all|claude|codex] [TMUX_CLIENT]")
	fmt.Fprintln(a.Stderr, "  agent-picker select [-provider all|claude|codex]")
	fmt.Fprintln(a.Stderr, "  agent-picker list [-provider all|claude|codex]")
}

func (a *App) printCommandUsage(command string, flags *flag.FlagSet) {
	switch command {
	case "popup":
		fmt.Fprintln(a.Stderr, "Usage: agent-picker popup [-provider all|claude|codex] [TMUX_CLIENT]")
	default:
		fmt.Fprintf(a.Stderr, "Usage: agent-picker %s [-provider all|claude|codex]\n", command)
	}
	flags.PrintDefaults()
}
