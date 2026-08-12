.PHONY: test install-agent-picker install-tmux-snapshot

test:
	go -C tools/agent-picker test ./...
	go -C tools/tmux-snapshot test ./...

install-agent-picker:
	GOBIN=$(HOME)/.local/bin go -C tools/agent-picker install ./cmd/agent-picker

install-tmux-snapshot:
	GOBIN=$(HOME)/.local/bin go -C tools/tmux-snapshot install ./cmd/tmux-snapshot
