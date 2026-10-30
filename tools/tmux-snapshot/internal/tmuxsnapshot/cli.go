package tmuxsnapshot

import (
	"context"
	"fmt"
)

func (a *App) printUsage() {
	fmt.Fprintln(a.Stderr, "Usage:")
	fmt.Fprintln(a.Stderr, "  tmux-snapshot save")
	fmt.Fprintln(a.Stderr, "  tmux-snapshot restore")
}

func (a *App) fatal(err error) int {
	fmt.Fprintf(a.Stderr, "tmux-snapshot: %s\n", err)
	return 1
}

func (a *App) Main(ctx context.Context, args []string) int {
	if len(args) != 1 {
		a.printUsage()
		return 2
	}

	switch args[0] {
	case "save":
		path, err := a.Save(ctx)
		if err != nil {
			return a.fatal(err)
		}
		fmt.Fprintln(a.Stdout, path)
		return 0
	case "restore":
		if err := a.Restore(ctx); err != nil {
			return a.fatal(err)
		}
		return 0
	default:
		a.printUsage()
		return 2
	}
}
