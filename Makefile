.PHONY: test install-agent-picker install-tmux-snapshot install-md-view install-orchestrate-journal test-orchestrate-journal stow-agents install-skills test-skills build-md-view-css verify-md-view-css

test:
	go -C tools/agent-picker test ./...
	go -C tools/tmux-snapshot test ./...
	go -C tools/md-view test ./...
	$(MAKE) verify-md-view-css
	$(MAKE) test-skills
	sh tests/journal_append_test.sh
	$(MAKE) test-orchestrate-journal

install-agent-picker:
	GOBIN=$(HOME)/.local/bin go -C tools/agent-picker install ./cmd/agent-picker

install-tmux-snapshot:
	GOBIN=$(HOME)/.local/bin go -C tools/tmux-snapshot install ./cmd/tmux-snapshot

install-md-view:
	GOBIN=$(HOME)/.local/bin go -C tools/md-view install ./cmd/md-view
	install -d \
		"$(HOME)/.local/bin" \
		"$(HOME)/.local/share/md-view/pandoc/filters" \
		"$(HOME)/.local/share/md-view/pandoc/includes" \
		"$(HOME)/.local/share/md-view/pandoc/styles"
	install -m 644 tools/md-view/pandoc/defaults.yaml "$(HOME)/.local/share/md-view/pandoc/defaults.yaml"
	install -m 644 tools/md-view/pandoc/filters/md-view.lua "$(HOME)/.local/share/md-view/pandoc/filters/md-view.lua"
	install -m 644 tools/md-view/pandoc/includes/mermaid.html "$(HOME)/.local/share/md-view/pandoc/includes/mermaid.html"
	install -m 644 tools/md-view/pandoc/includes/source-copy.html "$(HOME)/.local/share/md-view/pandoc/includes/source-copy.html"
	install -m 644 tools/md-view/pandoc/includes/theme-switcher.html "$(HOME)/.local/share/md-view/pandoc/includes/theme-switcher.html"
	install -m 644 tools/md-view/pandoc/styles/md-view.css "$(HOME)/.local/share/md-view/pandoc/styles/md-view.css"

# Links the checkout's entry point: Node strips the TypeScript types at run time.
install-orchestrate-journal:
	install -d "$(HOME)/.local/bin"
	ln -sf "$(CURDIR)/tools/orchestrate-journal/src/cli.ts" "$(HOME)/.local/bin/orchestrate-journal"

tools/orchestrate-journal/node_modules: tools/orchestrate-journal/package.json tools/orchestrate-journal/package-lock.json
	npm ci --prefix tools/orchestrate-journal

test-orchestrate-journal: tools/orchestrate-journal/node_modules
	cd tools/orchestrate-journal && npm exec -- tsc --noEmit && npm test

# Keep authored skills as folded directory links: Cursor CLI's slash menu skips
# a SKILL.md whose real path is outside the skills root.
stow-agents:
	stow --delete --no-folding --target="$(HOME)" agents
	if [ -d "$(HOME)/.agents/skills" ]; then \
		find "$(HOME)/.agents/skills" -mindepth 1 -depth -type d -empty -delete; \
	fi
	stow --no-folding --ignore='^\.agents/skills/.+' --target="$(HOME)" agents
	mkdir -p "$(HOME)/.agents/skills"
	stow --dir=agents/.agents --target="$(HOME)/.agents/skills" skills

install-skills:
	install -d "$(HOME)/.local/bin" "$(HOME)/.local/share/macos-dotfiles/skills"
	install -m 755 scripts/skills "$(HOME)/.local/bin/skills"
	install -m 644 skills/catalog.json "$(HOME)/.local/share/macos-dotfiles/skills/catalog.json"
	rm -rf "$(HOME)/.local/share/macos-dotfiles/skills/overlays"
	cp -R skills/overlays "$(HOME)/.local/share/macos-dotfiles/skills/overlays"

test-skills:
	sh tests/skills_test.sh

tools/md-view/node_modules: tools/md-view/package.json tools/md-view/package-lock.json
	npm ci --prefix tools/md-view

build-md-view-css: tools/md-view/node_modules
	cd tools/md-view && npm exec -- @tailwindcss/cli \
		-i pandoc/styles/md-view.src.css \
		-o pandoc/styles/md-view.css

verify-md-view-css: tools/md-view/node_modules
	@generated=$$(mktemp) && \
	cd tools/md-view && npm exec -- @tailwindcss/cli \
		-i pandoc/styles/md-view.src.css \
		-o "$$generated" && \
	diff -u pandoc/styles/md-view.css "$$generated" && \
	rm -f "$$generated"
