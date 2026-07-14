package agentpicker

import (
	"context"
	"fmt"
	"strings"
	"time"
)

type agentSnapshot struct {
	agents []Agent
	now    time.Time
}

func (a *App) snapshot(ctx context.Context, provider string) agentSnapshot {
	now := a.Clock.Now()
	agents := a.collectAgents(ctx, provider)
	SortAgents(agents, now)
	return agentSnapshot{agents: agents, now: now}
}

func (snapshot agentSnapshot) rows() string {
	rows := FormatRows(snapshot.agents, snapshot.now)
	if len(rows) == 0 {
		return ""
	}
	return strings.Join(rows, "\n") + "\n"
}

func (a *App) Agents(ctx context.Context, provider string) []Agent {
	return a.snapshot(ctx, provider).agents
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
	return a.snapshot(ctx, provider).rows()
}

func (a *App) List(ctx context.Context, provider string) {
	fmt.Fprint(a.Stdout, a.Rows(ctx, provider))
}

func noAgentsMessage(provider string) string {
	switch provider {
	case "claude":
		return "agent-picker: no running Claude agents found"
	case "codex":
		return "agent-picker: no running Codex agents found"
	default:
		return "agent-picker: no running agents found"
	}
}
