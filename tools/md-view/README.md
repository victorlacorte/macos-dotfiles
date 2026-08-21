# md-view

`md-view` is a small compiled macOS Markdown preview command. It uses Pandoc
for the Markdown AST and HTML writer, adds a local presentation filter, then
either opens the generated HTML with `open` or writes it for another program.

## Installation

From the repository root:

```sh
mise install pandoc
make install-md-view
```

Go 1.22 or newer is required to build the command. The executable is installed
at `~/.local/bin/md-view`. Its Pandoc defaults,
filter, include, and stylesheet are installed under
`~/.local/share/md-view`. The installed executable also works directly from
the repository while developing, without depending on the current directory.

## Development

Run package tests or invoke a temporary development binary from the repository
root:

```sh
go -C tools/md-view test ./...
MD_VIEW_DATA_DIR="$PWD/tools/md-view" go -C tools/md-view run ./cmd/md-view FILE.md
```

`go run` places its executable outside the repository, so set
`MD_VIEW_DATA_DIR` explicitly as shown. A built or installed binary discovers
the repository-relative or installed asset directory automatically.

## CLI

```sh
md-view FILE.md
md-view render FILE.md --output OUTPUT.html
```

The default command validates the input, renders it to a deterministic file
under `${TMPDIR:-/tmp}/md-view`, and opens that file with macOS `open`. The
cache name is derived from the absolute input path, so viewing the same file
reuses its previous preview. Pandoc writes a temporary sibling first and the
CLI atomically replaces the cached file only after a successful render.

`render` is the non-opening boundary for scripts and future application code.
It requires `--output` (or `-o`) and an existing parent directory. The output
path may be relative or absolute, may contain spaces or Unicode, and must not
resolve to the input Markdown file. The command prints the resolved output
path after success.

Inputs must be regular, readable files with a Markdown-like extension such as
`.md` or `.markdown`. The original source is never rewritten. The
`MD_VIEW_DATA_DIR` environment variable is available for shell tests and
development overrides; it is not needed for a normal repository or installed
invocation.

## Rendering architecture

Pandoc is configured by `pandoc/defaults.yaml` with this reader:

```text
gfm+sourcepos+yaml_metadata_block+fenced_divs+bracketed_spans+attributes-raw_html
```

Pandoc 3.10.2 was the tested version. The planned GFM combination needed the
small `+attributes` addition so explicit heading and fenced-code identifiers
are parsed by that version. `filters` is the Pandoc defaults-file key for the
Lua filter, even though the corresponding command-line option is
`--lua-filter`.

The filter in `pandoc/filters/md-view.lua` has three responsibilities:

1. It changes fenced code blocks with the `mermaid` class into safe `.mermaid`
   containers whose source text is escaped by Pandoc's HTML writer. Their
   identifier, classes, safe data/ARIA attributes, and `data-pos` are kept.
2. It wraps each semantic Pandoc table in `.table-scroll` while retaining the
   original table, caption, alignment, and source metadata.
3. Because Pandoc can represent disabled raw HTML tags as raw inline/block
   nodes, it converts those nodes to ordinary text before HTML writing. This
   prevents source HTML from becoming live markup.

The defaults file embeds the stylesheet, CSS, and local images into one
standalone HTML document. Relative resources are resolved from the source
file's directory, even though the output is elsewhere. The only intentional
external resource is the exact Mermaid browser bundle:

```text
https://cdn.jsdelivr.net/npm/mermaid@11.12.1/dist/mermaid.min.js
```

The static include initializes Mermaid after the document is ready, uses
`securityLevel: "strict"`, selects a light or dark theme from the system
preference, and handles each diagram independently. A bad diagram gets a
visible error while the rest of the document remains readable.

The preview stylesheet starts as `pandoc/styles/md-view.src.css`. Tailwind CSS
4.3.3 and `@tailwindcss/typography` compile that into `pandoc/styles/md-view.css`,
which is the file Pandoc embeds and `make install-md-view` copies. Pandoc writes
plain HTML, so the source file applies the `prose` styles to `body` instead of
stamping utility classes onto the document.

Regenerate the committed CSS from the repository root after editing the source:

```sh
make build-md-view-css
```

`make test` compiles again into a temp file and fails if the committed CSS
differs. The result uses a 76rem container, the system sans stack, GitHub-like
link colors, and the md-view rules for wide tables, Mermaid, callouts,
dark-mode code spans, and print. Pandoc still injects its own highlight
stylesheet. Dark mode overrides those span colors here, rather than setting
`highlight-style` in the defaults file.

## Source positions and trust boundary

`sourcepos` is the initial extension seam for a future annotation application.
Pandoc emits `data-pos` attributes on source-backed headings, paragraphs,
inline elements, lists, tables, code containers, and other elements where the
writer has a position. The Lua filter carries the source position to Mermaid
containers and table wrappers. These are source ranges, not persistent
annotation IDs.

The input is treated as local Markdown, but source-provided raw HTML is still
disabled and escaped. Mermaid attributes are restricted to the identifier,
classes, data/ARIA attributes, and a small set of descriptive attributes.
The static browser JavaScript and CSS are trusted application assets. Markdown
links remain ordinary links, and Mermaid itself is loaded from the pinned
jsDelivr URL, so Mermaid rendering needs browser network access.

## Future annotation boundary

The viewer does not implement annotations. The original Markdown remains the
canonical source. A future annotation record should combine a document
revision or hash, heading path or ID, Pandoc source range, exact selected
quote, and optional prefix/suffix context. DOM-only selectors, generated node
IDs, and CSS classes are not sufficient persistent identifiers.

The preferred first export is a second Markdown file with the unchanged body
and annotation records in YAML metadata. Inline spans or fenced divs can be a
derived format later. Section and block annotations should come before exact
selections spanning multiple inline constructs, which may require a
concrete-syntax parser alongside Pandoc.

## Tests

Unit tests cover parsing, argument order, quoting, atomic failure behavior, and
browser-launch rules. An integration test renders the representative fixture
with real Pandoc and checks source positions, table structure, Mermaid source,
embedded CSS, embedded local images, raw-HTML handling, and the pinned
external script.

Run the Go tests from the module, or run all repository tests:

```sh
go -C tools/md-view test ./...
make test
```

The fixture intentionally includes repeated headings and phrases, lists,
blockquote content, a fenced div, narrow and wide tables, flowchart and
sequence Mermaid diagrams, an invalid diagram, a relative SVG, Unicode,
source HTML, and a path containing spaces.
