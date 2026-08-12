package tmuxsnapshot

import (
	"context"
	"fmt"
)

func (a *App) printUsage() {
	fmt.Fprintln(a.Stderr, "Usage:")
	fmt.Fprintln(a.Stderr, "  tmux-snapshot save [FILE]")
	fmt.Fprintln(a.Stderr, "  tmux-snapshot restore [FILE]")
}

func (a *App) fatal(err error) int {
	fmt.Fprintf(a.Stderr, "tmux-snapshot: %s\n", err)
	return 1
}

func (a *App) Main(ctx context.Context, args []string) int {
	if len(args) == 0 {
		a.printUsage()
		return 2
	}

	switch args[0] {
	case "save":
		if len(args) > 2 {
			a.printUsage()
			return 2
		}
		requested := ""
		if len(args) == 2 {
			requested = args[1]
		}
		path, err := a.Save(ctx, requested)
		if err != nil {
			return a.fatal(err)
		}
		fmt.Fprintln(a.Stdout, path)
		return 0
	case "restore":
		if len(args) > 2 {
			a.printUsage()
			return 2
		}
		requested := ""
		if len(args) == 2 {
			requested = args[1]
		}
		path, err := a.resolveSnapshot(requested)
		if err != nil {
			return a.fatal(err)
		}
		if err := a.restoreSnapshot(ctx, path); err != nil {
			return a.fatal(err)
		}
		return 0
	default:
		a.printUsage()
		return 2
	}
}
