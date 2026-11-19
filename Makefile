.PHONY: test install-agent-picker install-tmux-snapshot install-orchestrate-journal test-orchestrate-journal stow-agents install-skills test-skills

test:
	go -C tools/agent-picker test ./...
	go -C tools/tmux-snapshot test ./...
	$(MAKE) test-skills
	sh tests/journal_append_test.sh
	$(MAKE) test-orchestrate-journal

install-agent-picker:
	GOBIN=$(HOME)/.local/bin go -C tools/agent-picker install ./cmd/agent-picker

install-tmux-snapshot:
	GOBIN=$(HOME)/.local/bin go -C tools/tmux-snapshot install ./cmd/tmux-snapshot

# Links the checkout's entry point: Node strips the TypeScript types at run time.
install-orchestrate-journal:
	install -d "$(HOME)/.local/bin"
	ln -sf "$(CURDIR)/tools/orchestrate-journal/src/cli.ts" "$(HOME)/.local/bin/orchestrate-journal"

tools/orchestrate-journal/node_modules: tools/orchestrate-journal/package.json tools/orchestrate-journal/package-lock.json
	npm ci --prefix tools/orchestrate-journal

test-orchestrate-journal: tools/orchestrate-journal/node_modules
	cd tools/orchestrate-journal && npm exec -- tsc --noEmit && npm test

# Keep authored skills as folded directory links: Cursor CLI's slash menu skips
# a SKILL.md whose real path is outside the skills root. Claude Code reads
# ~/.claude/skills instead of ~/.agents/skills, so link them there too.
stow-agents:
	stow --delete --no-folding --target="$(HOME)" agents
	if [ -d "$(HOME)/.agents/skills" ]; then \
		find "$(HOME)/.agents/skills" -mindepth 1 -depth -type d -empty -delete; \
	fi
	stow --no-folding --ignore='^\.agents/skills/.+' --target="$(HOME)" agents
	mkdir -p "$(HOME)/.agents/skills"
	stow --dir=agents/.agents --target="$(HOME)/.agents/skills" skills
	mkdir -p "$(HOME)/.claude/skills"
	stow --dir=agents/.agents --target="$(HOME)/.claude/skills" skills

install-skills:
	install -d "$(HOME)/.local/bin" "$(HOME)/.local/share/macos-dotfiles/skills"
	install -m 755 scripts/skills "$(HOME)/.local/bin/skills"
	install -m 644 skills/catalog.json "$(HOME)/.local/share/macos-dotfiles/skills/catalog.json"
	rm -rf "$(HOME)/.local/share/macos-dotfiles/skills/overlays"
	cp -R skills/overlays "$(HOME)/.local/share/macos-dotfiles/skills/overlays"

test-skills:
	sh tests/skills_test.sh
