package agentpicker

import (
	"context"
	"crypto/sha256"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode"
)

const processFormat = "pid=,ppid=,pgid=,tpgid=,tty=,stat=,lstart=,comm="

type process struct {
	pid, ppid, pgid, tpgid int
	tty, state, command    string
	started                time.Time
}

func (p process) foreground() bool {
	return p.pid > 1 && p.pgid > 0 && p.pgid == p.tpgid &&
		p.tty != "??" && p.tty != "?" && p.tty != "" &&
		!strings.ContainsAny(p.state, "TtXZ") && !p.started.IsZero()
}

func (p process) identity() string {
	if p.started.IsZero() {
		return ""
	}
	return fmt.Sprintf("%x", sha256.Sum256([]byte(fmt.Sprintf("%d\x00%d\x00%d\x00%s\x00%s\x00%d",
		p.pid, p.ppid, p.pgid, p.tty, p.command, p.started.Unix()))))
}

func (a *App) processInventory(ctx context.Context) map[int]process {
	return a.readProcesses(ctx, "-Ao", processFormat)
}

func (a *App) readProcesses(ctx context.Context, args ...string) map[int]process {
	out, err := a.Runner.Run(ctx, Command{Name: "ps", Args: args, Env: map[string]string{"LC_ALL": "C", "TZ": "UTC"}})
	if err != nil {
		return nil
	}
	return parseProcesses(out)
}

func parseProcesses(out string) map[int]process {
	processes := make(map[int]process)
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 12 {
			continue
		}
		pid, e1 := strconv.Atoi(fields[0])
		ppid, e2 := strconv.Atoi(fields[1])
		pgid, e3 := strconv.Atoi(fields[2])
		tpgid, e4 := strconv.Atoi(fields[3])
		started, e5 := time.Parse("Mon Jan 2 15:04:05 2006", strings.Join(fields[6:11], " "))
		if e1 != nil || e2 != nil || e3 != nil || e4 != nil || e5 != nil {
			continue
		}
		// Keep embedded spaces in executable paths after the metadata columns.
		command := line
		for i := 0; i < 11; i++ {
			command = strings.TrimLeftFunc(command, unicode.IsSpace)
			command = command[strings.IndexFunc(command, unicode.IsSpace):]
		}
		processes[pid] = process{pid: pid, ppid: ppid, pgid: pgid, tpgid: tpgid,
			tty: strings.TrimPrefix(fields[4], "/dev/"), state: fields[5],
			started: started, command: strings.TrimLeftFunc(command, unicode.IsSpace)}
	}
	return processes
}

// Walk the entire ancestry inside the foreground job, including shell wrappers.
// Multiple independent matching roots in one job are ambiguous and omitted.
func passiveSessions(inventory *discoveryInventory, matches func(process) bool) (map[string]process, map[int]int) {
	matched := make(map[int]process)
	for pid, p := range inventory.processes {
		if _, ok := inventory.panes[p.tty]; ok && p.foreground() && matches(p) {
			matched[pid] = p
		}
	}
	leaders := make(map[string]process)
	ambiguous := make(map[string]bool)
	for _, p := range matched {
		root := p
		ancestor := p
		for depth := 0; depth < len(inventory.processes); depth++ {
			parent, ok := inventory.processes[ancestor.ppid]
			if !ok || parent.tty != p.tty || parent.pgid != p.pgid || parent.pid == ancestor.pid {
				break
			}
			if _, ok := matched[parent.pid]; ok {
				root = parent
			}
			ancestor = parent
		}
		if prior, ok := leaders[p.tty]; ok && prior.pid != root.pid {
			ambiguous[p.tty] = true
		}
		leaders[p.tty] = root
	}
	for tty := range ambiguous {
		delete(leaders, tty)
	}
	owners := make(map[int]int)
	for pid, p := range inventory.processes {
		leader, ok := leaders[p.tty]
		if !ok || p.pgid != leader.pgid {
			continue
		}
		ancestor := p
		for depth := 0; depth < len(inventory.processes); depth++ {
			if ancestor.pid == leader.pid {
				owners[pid] = leader.pid
				break
			}
			parent, ok := inventory.processes[ancestor.ppid]
			if !ok || parent.tty != p.tty || parent.pgid != p.pgid || parent.pid == ancestor.pid {
				break
			}
			ancestor = parent
		}
	}
	return leaders, owners
}
