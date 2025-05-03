# My macOS dotfiles

```sh
brew install pinentry-mac
```

## Neovim config

- How to easily validate new configs?

```sh
XDG_CONFIG_HOME=./nvim-0.11/ nvim nvim-0.11/nvim/init.lua
```

- Add `nvim_lspconfig`. Validate extending default props

- Add Mason so LSPs are not maintained manually. Uninstall `lua-language-server` added via brew (to avoid duplicating the code)

## TODO

- Attempt to move back lsp from the `after` dir
