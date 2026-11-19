# Agents

## Stow the agents package

```sh
make stow-agents
```

This stows `agents/` into `$HOME` with `--no-folding` (Claude status line,
Cursor hooks and rules, Codex `AGENTS.md`, shared `AGENTS.md`). Authored skills
are stowed separately, one directory link each, into both `~/.agents/skills`
and `~/.claude/skills`. Cursor's `/` menu skips a `SKILL.md` symlink whose real
path is outside the skills directory, so per-file links do not work. The target
also migrates earlier layouts (a folded `.agents` or `.agents/skills` link, or
per-file skill links).

## Codex system config

`etc/codex/config.toml` is installed as `/etc/codex/config.toml` through the
`etc` Stow package. It adds `~/.codex/plans/` as a writable root for
`workspace-write` sessions, sets the status line, and holds the disabled
`shadcn` and `chrome_devtools` MCP servers. Use `codex ...` directly; no
wrapper adds arguments.

```sh
mkdir -p "$HOME/.codex/plans"
sudo "$(command -v stow)" --target=/etc etc
```

Stow stops if an unmanaged `/etc/codex/config.toml` exists; inspect it before
replacing it. The link is relative, so if the repository moves, remove it and
stow again from the new checkout. Client-specific values such as project trust
belong in the private `~/.codex/config.toml`, which this repository does not
manage. `agents/.codex/AGENTS.md` documents the plan handoff workflow.

## Skills

Install `jq` and the GitHub CLI (`gh skill`), and put `$HOME/.local/bin` on
`PATH`.

```sh
make stow-agents install-skills
skills install --all            # install missing catalog skills
skills update --all --dry-run   # preview upstream changes
skills update --all
skills install unslop --scope project
```

Layout of `~/.agents/skills`, a real directory read by Codex and Cursor:

```text
orchestrate/ -> <checkout>/agents/.agents/skills/orchestrate   authored
babysit/     -> <checkout>/agents/.agents/skills/babysit       authored
reviewable-prs/ -> <checkout>/agents/.agents/skills/reviewable-prs authored
unslop/                                                        downloaded
```

- Authored skills live in `agents/.agents/skills` and are tracked as-is.
  Upstream skills never go there: they come only from `skills/catalog.json`
  and install into `~/.agents/skills`.
- `skills/catalog.json` is the source recipe. `make install-skills` copies the
  wrapper, catalog, and `skills/overlays` into `~/.local`; it downloads nothing.
  Re-run it after editing the catalog. `SKILLS_CATALOG` selects another
  catalog. Running `./scripts/skills` directly uses the checkout catalog.
- `install` adds missing skills only; `update` refreshes installed ones and
  re-applies overlays. Always update through the wrapper: raw `gh skill update`
  replaces the directory and drops overlay files. Review upstream changes.
- Updates use the source metadata recorded at install time. Changing a catalog
  entry's repository or selector does not redirect an installed skill; reinstall it.
- `--all` means the names in the catalog, not every skill upstream.
- Project scope installs into the Git root's `.agents/skills`, even from a
  subdirectory.
- Installs require real `.agents` and `.agents/skills` directories. Links for
  authored skills may coexist; a link occupying a catalog name is refused.

### Overlays

`skills/overlays/<name>/` adds files upstream does not ship (for example
`unslop`'s `agents/openai.yaml`). Never copy upstream files there. Applied
paths are tracked in `.skills-overlay` in the skill directory; overlay files
that upstream starts shipping are skipped with a warning. Removing an overlay
directory and running `skills update NAME` removes its files.

Every installed skill is visible to both harnesses. To keep one out of
automatic use, set `disable-model-invocation: true` in `SKILL.md` (Cursor) or
`policy.allow_implicit_invocation: false` in `agents/openai.yaml` (Codex).

### Migrating from Stow-linked upstream skills

Earlier checkouts linked upstream skills through Stow. If `.agents` or
`.agents/skills` is a directory symlink, run `make stow-agents` first, then
remove dangling links and reinstall. Also delete duplicate catalog skills left
in `~/.cursor/skills`, which Cursor would otherwise list twice.

```sh
make stow-agents
find "$HOME/.agents/skills" -maxdepth 1 -type l ! -exec test -e {} \; -print -delete
skills install --all
```

## Orchestration journals

`make install-orchestrate-journal` links `tools/orchestrate-journal/src/cli.ts`
as `~/.local/bin/orchestrate-journal`:

```sh
orchestrate-journal analyze [--json] [--cursor-dir DIR] journal.jsonl...
```
