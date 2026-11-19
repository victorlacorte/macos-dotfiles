# My macOS dotfiles

Each top-level config directory (`git`, `tmux`, `nvim`, `zsh`, ...) is a
[GNU Stow](https://www.gnu.org/software/stow/) package. Stow the ones you want
into `$HOME`. `agents` and `etc` have their own steps, see [Agents](docs/agents.md):

```sh
stow --no --verbose --target=$HOME tmux   # dry run, then drop --no
brew install pinentry-mac tree-sitter-cli fzf
```

## Local tools

Go 1.22 or newer is required. From the repository root:

```sh
make install-agent-picker install-tmux-snapshot   # into ~/.local/bin
make install-orchestrate-journal                  # needs Node, links the checkout
make test
```

Binaries are local build artifacts. Re-run the targets after updating.

## Guides

- [Tmux](docs/tmux.md): sessionizer, agent picker (`prefix u`), snapshots.
- [Agents](docs/agents.md): Codex system config, shared skills, `skills` wrapper.
- [YubiKey](docs/yubikey.md): PIN management with `ykman`.

## Git signing

Pick the `[S]` key from `gpg --list-secret-keys --keyid-format LONG` and put
its full subkey fingerprint in `git/.gitconfig.local` (git-ignored):

```ini
[user]
signingkey = 123abc...!
```

Verify with a throwaway repository:

```sh
tmp=$(mktemp -d)
git -C "$tmp" init
git -C "$tmp" commit --allow-empty -S -m "test signing"
git -C "$tmp" log --show-signature -1
```

## Neovim

Try the config without installing it:

```sh
XDG_CONFIG_HOME=./nvim/.config nvim nvim/.config/nvim/init.lua
```
