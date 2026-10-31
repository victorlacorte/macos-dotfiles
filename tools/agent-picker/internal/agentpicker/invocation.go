package agentpicker

import (
	"path/filepath"
	"strings"
)

func (a *App) interactiveProcess(p process, provider string) bool {
	args, err := a.Runner.ProcessArgs(p.pid)
	return err == nil && interactiveInvocation(provider, args)
}

func (a *App) claudeProcess(p process) bool {
	args, err := a.Runner.ProcessArgs(p.pid)
	if err != nil {
		return false
	}
	base := filepath.Base(p.command)
	if base != "claude" {
		// npm Claude runs under Node. A generic Node process is not sufficient.
		if base != "node" || len(args) < 2 ||
			!strings.HasSuffix(filepath.ToSlash(args[1]), "/@anthropic-ai/claude-code/cli.js") {
			return false
		}
	}
	return interactiveInvocation("claude", args)
}

// Arguments come from the OS with their original boundaries, never shell words
// or ps's flattened command line. Only known noninteractive modes are excluded;
// a free-form positional prompt is valid for all three harnesses.
func interactiveInvocation(provider string, argv []string) bool {
	if len(argv) == 0 || argv[0] == "" {
		return false
	}
	args := argv[1:]
	if len(args) > 0 && (filepath.Base(argv[0]) == "node" || (provider != "codex" && filepath.IsAbs(args[0]))) {
		switch filepath.Ext(args[0]) {
		case ".js", ".cjs", ".mjs":
			args = args[1:]
		}
	}
	valueFlags := map[string]bool{
		"-m": true, "--model": true, "--cwd": true, "--workspace": true,
		"--output-format": true, "--input-format": true,
	}
	switch provider {
	case "codex":
		for _, flag := range []string{"-c", "--config", "-p", "--profile", "-s", "--sandbox", "-a", "--ask-for-approval", "-C", "--cd", "--add-dir", "--enable", "--disable", "-i", "--image", "--local-provider"} {
			valueFlags[flag] = true
		}
	case "claude":
		for _, flag := range []string{"--permission-mode", "--allowedTools", "--allowed-tools", "--disallowedTools", "--disallowed-tools", "--add-dir", "--agent", "--agents", "--system-prompt", "--append-system-prompt", "--mcp-config", "--settings", "--setting-sources", "--session-id", "--effort", "--betas"} {
			valueFlags[flag] = true
		}
	case "cursor":
		for _, flag := range []string{"--resume", "--api-key", "--mode", "--worktree"} {
			valueFlags[flag] = true
		}
	default:
		return false
	}
	mode := ""
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			break
		}
		flag, _, equals := strings.Cut(arg, "=")
		// Claude's resume and worktree flags also open interactive pickers
		// without an argument; do not consume a following option as their value.
		if provider == "claude" && (flag == "--resume" || flag == "-r" || flag == "--worktree" || flag == "-w") {
			if !equals && i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				i++
			}
			continue
		}
		if flag == "--help" || flag == "-h" || flag == "--version" || flag == "-V" || flag == "--list-models" {
			return false
		}
		if provider != "codex" && flag == "-v" {
			return false
		}
		if provider != "codex" && (flag == "--print" || flag == "--bg" || flag == "--background" || flag == "--exec") {
			return false
		}
		if valueFlags[flag] {
			if !equals {
				i++
				if i >= len(args) {
					return false
				}
			}
			continue
		}
		if len(arg) > 2 && arg[0] == '-' && arg[1] != '-' && valueFlags[arg[:2]] {
			continue
		}
		if provider != "codex" && strings.HasPrefix(arg, "-") && !strings.HasPrefix(arg, "--") && strings.Contains(arg[1:], "p") {
			return false
		}
		if strings.HasPrefix(arg, "--type=") || arg == "--worker" {
			return false
		}
		if !strings.HasPrefix(arg, "-") && mode == "" {
			mode = arg
		}
	}
	var excluded []string
	switch provider {
	case "codex":
		excluded = []string{"exec", "e", "review", "login", "logout", "mcp", "mcp-server", "app-server", "app", "completion", "sandbox", "debug", "apply", "a", "cloud", "features", "help"}
	case "claude":
		excluded = []string{"agents", "auth", "config", "doctor", "install", "mcp", "plugin", "plugins", "setup-token", "update", "upgrade", "help"}
	case "cursor":
		excluded = []string{"ls", "list", "login", "logout", "status", "update", "upgrade", "mcp", "models", "install-shell-integration", "uninstall", "help"}
	}
	for _, command := range excluded {
		if mode == command {
			return false
		}
	}
	return true
}
