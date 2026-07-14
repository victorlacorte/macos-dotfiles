package agentpicker

import (
	"context"
	"errors"
	"strings"
)

func (a *App) option(ctx context.Context, name, fallback string) string {
	if value := a.tmux(ctx, "show-option", "-gqv", name); value != "" {
		return value
	}
	return fallback
}

// SplitShellWords accepts quoting and backslash escapes used by tmux fzf
// options. It deliberately performs no expansion, and rejects command
// substitution syntax instead of evaluating it.
func SplitShellWords(value string) ([]string, error) {
	var words []string
	var word strings.Builder
	quote := rune(0)
	escaped, started := false, false
	runes := []rune(value)
	for i, r := range runes {
		if escaped {
			word.WriteRune(r)
			escaped, started = false, true
			continue
		}
		if quote != '\'' && r == '\\' {
			escaped, started = true, true
			continue
		}
		if quote == 0 && (r == '\'' || r == '"') {
			quote, started = r, true
			continue
		}
		if quote == r {
			quote = 0
			continue
		}
		if quote == 0 && (r == ' ' || r == '\t' || r == '\n') {
			if started {
				words = append(words, word.String())
				word.Reset()
				started = false
			}
			continue
		}
		if r == '`' || (r == '$' && i+1 < len(runes) && runes[i+1] == '(') {
			return nil, errors.New("command substitution is not supported")
		}
		word.WriteRune(r)
		started = true
	}
	if escaped || quote != 0 {
		return nil, errors.New("unterminated quote or escape")
	}
	if started {
		words = append(words, word.String())
	}
	return words, nil
}

func shellQuote(value string) string {
	if value == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

func getenv(key string) string {
	// Kept behind a function to make all provider environment dependencies
	// explicit in one place.
	return envLookup(key)
}
