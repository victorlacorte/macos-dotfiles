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

type providerAdapter struct {
	name    string
	label   string
	collect func(*App, context.Context, *discoveryInventory) []Agent
}

var providerAdapters = []providerAdapter{
	{name: "claude", label: "Claude", collect: (*App).collectClaudeAgents},
	{name: "codex", label: "Codex", collect: (*App).collectCodexAgents},
	{name: "cursor", label: "Cursor", collect: (*App).collectCursorAgents},
}

func providerNames() []string {
	names := make([]string, 0, len(providerAdapters))
	for _, adapter := range providerAdapters {
		names = append(names, adapter.name)
	}
	return names
}

func providerAdapterFor(name string) (providerAdapter, bool) {
	for _, adapter := range providerAdapters {
		if adapter.name == name {
			return adapter, true
		}
	}
	return providerAdapter{}, false
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
	var adapters []providerAdapter
	switch provider {
	case "all":
		adapters = providerAdapters
	default:
		if adapter, ok := providerAdapterFor(provider); ok {
			adapters = []providerAdapter{adapter}
		}
	}
	if len(adapters) == 0 {
		return nil
	}
	inventory := a.startInventory(ctx)
	results := make([][]Agent, len(adapters))
	var wg sync.WaitGroup
	wg.Add(len(adapters))
	for i, adapter := range adapters {
		go func(i int, adapter providerAdapter) {
			defer wg.Done()
			results[i] = adapter.collect(a, ctx, inventory)
		}(i, adapter)
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

func providerLabel(provider string) string {
	if adapter, ok := providerAdapterFor(provider); ok {
		return adapter.label
	}
	return ""
}

func noAgentsMessage(provider string) string {
	if label := providerLabel(provider); label != "" {
		return "agent-picker: no running " + label + " agents found"
	}
	return "agent-picker: no running agents found"
}
