.PHONY: test install-agent-picker install-tmux-snapshot install-md-view build-md-view-css verify-md-view-css

test:
	go -C tools/agent-picker test ./...
	go -C tools/tmux-snapshot test ./...
	go -C tools/md-view test ./...
	$(MAKE) verify-md-view-css

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
	install -m 644 tools/md-view/pandoc/styles/md-view.css "$(HOME)/.local/share/md-view/pandoc/styles/md-view.css"

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
