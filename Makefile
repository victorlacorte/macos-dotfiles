.PHONY: test install-agent-picker

test:
	go -C tools/agent-picker test ./...

install-agent-picker:
	GOBIN=$(HOME)/.local/bin go -C tools/agent-picker install ./cmd/agent-picker
