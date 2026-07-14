package agentpicker

import (
	"context"
	"fmt"
	"strings"
	"sync"
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
	var collectors []func(context.Context, *discoveryInventory) []Agent
	switch provider {
	case "all":
		collectors = []func(context.Context, *discoveryInventory) []Agent{a.collectClaudeAgents, a.collectCodexAgents}
	case "claude":
		collectors = []func(context.Context, *discoveryInventory) []Agent{a.collectClaudeAgents}
	case "codex":
		collectors = []func(context.Context, *discoveryInventory) []Agent{a.collectCodexAgents}
	default:
		return nil
	}
	inventory := a.startInventory(ctx)
	results := make([][]Agent, len(collectors))
	var wg sync.WaitGroup
	wg.Add(len(collectors))
	for i, collector := range collectors {
		go func() {
			defer wg.Done()
			results[i] = collector(ctx, inventory)
		}()
	}
	wg.Wait()
	inventory.wg.Wait()
	var agents []Agent
	for _, result := range results {
		agents = append(agents, result...)
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
