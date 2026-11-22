.PHONY: test install-agent-picker install-tmux-snapshot stow-agents install-skills test-skills

test:
	go -C tools/agent-picker test ./...
	go -C tools/tmux-snapshot test ./...
	$(MAKE) test-skills

install-agent-picker:
	GOBIN=$(HOME)/.local/bin go -C tools/agent-picker install ./cmd/agent-picker

install-tmux-snapshot:
	GOBIN=$(HOME)/.local/bin go -C tools/tmux-snapshot install ./cmd/tmux-snapshot

# --no-folding links files, never directories: harness directories such as
# ~/.claude and ~/.cursor also hold state this package does not own.
stow-agents:
	stow --restow --no-folding --target="$(HOME)" agents

install-skills:
	install -d "$(HOME)/.local/bin" "$(HOME)/.local/share/macos-dotfiles/skills"
	install -m 755 scripts/skills "$(HOME)/.local/bin/skills"
	install -m 644 skills/catalog.json "$(HOME)/.local/share/macos-dotfiles/skills/catalog.json"
	rm -rf "$(HOME)/.local/share/macos-dotfiles/skills/overlays"
	cp -R skills/overlays "$(HOME)/.local/share/macos-dotfiles/skills/overlays"

test-skills:
	sh tests/skills_test.sh
