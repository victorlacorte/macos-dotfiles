package agentpicker

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

type Agent struct {
	Provider string
	Pane     string
	PID      int
	Kind     string
	State    string
	Activity time.Time
	Location string
	Path     string
}

type Provider interface {
	Agents(context.Context) []Agent
}

type providerFunc func(context.Context) []Agent

func (provider providerFunc) Agents(ctx context.Context) []Agent { return provider(ctx) }

func rank(agent Agent) int {
	switch {
	case agent.Provider == "claude" && agent.State == "waiting":
		return 0
	case agent.Provider == "claude" && agent.State == "idle":
		return 1
	case agent.Provider == "claude" && agent.State == "working":
		return 3
	default:
		return 2
	}
}

func ageMinutes(now time.Time, activity time.Time) int {
	if activity.IsZero() {
		return 0
	}
	age := int(now.Sub(activity).Minutes())
	if age < 0 {
		return 0
	}
	return age
}

func SortAgents(agents []Agent, now time.Time) {
	sort.SliceStable(agents, func(i, j int) bool {
		a, b := agents[i], agents[j]
		if rank(a) != rank(b) {
			return rank(a) < rank(b)
		}
		if ageMinutes(now, a.Activity) != ageMinutes(now, b.Activity) {
			return ageMinutes(now, a.Activity) < ageMinutes(now, b.Activity)
		}
		if a.Pane != b.Pane {
			return a.Pane < b.Pane
		}
		return a.Location < b.Location
	})
}

func FormatRows(agents []Agent, now time.Time) []string {
	locationWidth, pathWidth := 0, 0
	for _, agent := range agents {
		locationWidth = max(locationWidth, utf8.RuneCountInString(agent.Location))
		pathWidth = max(pathWidth, utf8.RuneCountInString(agent.Path))
	}

	rows := make([]string, 0, len(agents))
	for _, agent := range agents {
		rows = append(rows, formatRow(agent, now, locationWidth, pathWidth))
	}
	return rows
}

func FormatRow(agent Agent, now time.Time) string {
	return formatRow(agent, now, utf8.RuneCountInString(agent.Location), utf8.RuneCountInString(agent.Path))
}

func formatRow(agent Agent, now time.Time, locationWidth, pathWidth int) string {
	ageNumber := ageMinutes(now, agent.Activity)
	age := "-"
	if !agent.Activity.IsZero() {
		age = strconv.Itoa(ageNumber) + "m"
	}
	return fmt.Sprintf("%d\t%s\t%d\t%s\t%s\t%s\t%5s\t%-*s\t%-*s\t%d",
		rank(agent), agent.Pane, agent.PID, agent.Kind, agent.Provider,
		agent.State, age, locationWidth, agent.Location, pathWidth, agent.Path, ageNumber)
}

func trimLine(value string) string { return strings.TrimRight(value, "\r\n") }

func shortenHome(path, home string) string {
	if home != "" && (path == home || strings.HasPrefix(path, home+"/")) {
		return "~" + strings.TrimPrefix(path, home)
	}
	return path
}
