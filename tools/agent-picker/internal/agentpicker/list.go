package agentpicker

import (
	"context"
	"fmt"
	"strings"
)

func (a *App) Agents(ctx context.Context, provider string) []Agent {
	now := a.Clock.Now()
	agents := a.collectAgents(ctx, provider)
	SortAgents(agents, now)
	return agents
}

func (a *App) collectAgents(ctx context.Context, provider string) []Agent {
	var providers []Provider
	switch provider {
	case "all":
		providers = []Provider{providerFunc(a.claudeAgents), providerFunc(a.codexAgents)}
	case "claude":
		providers = []Provider{providerFunc(a.claudeAgents)}
	case "codex":
		providers = []Provider{providerFunc(a.codexAgents)}
	default:
		return nil
	}
	var agents []Agent
	for _, provider := range providers {
		agents = append(agents, provider.Agents(ctx)...)
	}
	return agents
}

func (a *App) Rows(ctx context.Context, provider string) string {
	now := a.Clock.Now()
	agents := a.collectAgents(ctx, provider)
	SortAgents(agents, now)
	rows := FormatRows(agents, now)
	if len(rows) == 0 {
		return ""
	}
	return strings.Join(rows, "\n") + "\n"
}

func (a *App) List(ctx context.Context, provider string) {
	fmt.Fprint(a.Stdout, a.Rows(ctx, provider))
}
