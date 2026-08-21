#!/bin/sh

set -eu

test_script_dir=$(CDPATH= cd -P "$(dirname "$0")" && pwd)
tool_dir=$(CDPATH= cd -P "$test_script_dir/.." && pwd)
fixture=$tool_dir/test/fixtures/representative.md
original_path=$PATH

fail() {
  printf 'md-view test failure: %s\n' "$*" >&2
  exit 1
}

assert_file() {
  [ -f "$1" ] || fail "expected file: $1"
}

assert_empty_file() {
  [ ! -s "$1" ] || fail "expected empty file: $1"
}

assert_contains() {
  if ! grep -F -- "$2" "$1" >/dev/null 2>&1; then
    fail "expected $1 to contain: $2"
  fi
}

assert_not_contains() {
  if grep -F -- "$2" "$1" >/dev/null 2>&1; then
    fail "expected $1 not to contain: $2"
  fi
}

assert_equal() {
  [ "$1" = "$2" ] || fail "$3 (expected '$1', got '$2')"
}

count_text() {
  count_needle=$1
  count_file=$2
  awk -v needle="$count_needle" '
    {
      rest = $0
      while ((position = index(rest, needle)) > 0) {
        count++
        rest = substr(rest, position + length(needle))
      }
    }
    END { print count + 0 }
  ' "$count_file"
}

test_tmp=$(mktemp -d "${TMPDIR:-/tmp}/md-view-test.XXXXXX")
test_tmp=$(realpath "$test_tmp")
trap 'rm -rf "$test_tmp"' EXIT HUP INT TERM

cli=$test_tmp/md-view
if ! go -C "$tool_dir" build -o "$cli" ./cmd/md-view; then
  fail 'could not build the Go md-view command'
fi

pandoc_path=
if command -v pandoc >/dev/null 2>&1; then
  pandoc_path=$(command -v pandoc)
elif command -v mise >/dev/null 2>&1; then
  mise_pandoc=$(mise which pandoc 2>/dev/null || true)
  if [ -n "$mise_pandoc" ] && [ -x "$mise_pandoc" ]; then
    pandoc_path=$mise_pandoc
  fi
fi
[ -n "$pandoc_path" ] || fail 'pandoc is required; run mise install pandoc first'

fake_bin=$test_tmp/fake-bin
mkdir -p "$fake_bin"
cp "$tool_dir/test/fakes/pandoc" "$fake_bin/pandoc"
cp "$tool_dir/test/fakes/open" "$fake_bin/open"
chmod 755 "$fake_bin/pandoc" "$fake_bin/open"

source_dir=$test_tmp/'source with spaces and Unicode é'
mkdir -p "$source_dir"
input_path=$source_dir/'representative copy.md'
cp "$fixture" "$input_path"
cp "$tool_dir/test/fixtures/local-image.svg" "$source_dir/local-image.svg"

fake_pandoc_log=$test_tmp/fake-pandoc.log
fake_open_log=$test_tmp/fake-open.log
export MD_VIEW_FAKE_PANDOC_LOG=$fake_pandoc_log
export MD_VIEW_FAKE_OPEN_LOG=$fake_open_log
export MD_VIEW_FAKE_PANDOC_FAIL=0
export MD_VIEW_FAKE_OPEN_STATUS=0

cache_root=$test_tmp/'temporary root'
mkdir -p "$cache_root"
PATH=$fake_bin:$original_path
export PATH

default_stdout=$test_tmp/default.stdout
default_stderr=$test_tmp/default.stderr
if ! MD_VIEW_DATA_DIR="$tool_dir" TMPDIR="$cache_root" "$cli" "$input_path" >"$default_stdout" 2>"$default_stderr"; then
  fail 'default command did not render successfully with fake tools'
fi
assert_empty_file "$default_stdout"
assert_file "$fake_open_log"
assert_equal 1 "$(wc -l <"$fake_open_log" | tr -d ' ')" 'default command should invoke open once'
cached_output=$(sed -n '1p' "$fake_open_log")
case "$cached_output" in
  "$cache_root"/md-view/*.html) ;;
  *) fail "default output was not placed in the TMPDIR cache: $cached_output" ;;
esac
assert_file "$cached_output"
assert_contains "$fake_pandoc_log" "--defaults=$tool_dir/pandoc/defaults.yaml"
assert_contains "$fake_pandoc_log" "--resource-path=$source_dir"
assert_contains "$fake_pandoc_log" "$input_path"
assert_not_contains "$fake_pandoc_log" "$input_path.tmp."
assert_equal 0 "$(find "$cache_root/md-view" -name '*.tmp.*' -print | wc -l | tr -d ' ')" 'temporary render files should be removed'

render_dir=$test_tmp/'output with spaces'
mkdir -p "$render_dir"
render_output=$render_dir/'preview é.html'
printf '%s\n' 'sentinel' >"$fake_open_log"
render_stdout=$test_tmp/render.stdout
if ! MD_VIEW_DATA_DIR="$tool_dir" "$cli" render "$input_path" --output "$render_output" >"$render_stdout"; then
  fail 'render subcommand did not succeed with fake Pandoc'
fi
assert_file "$render_output"
assert_equal "$render_output" "$(sed -n '1p' "$render_stdout")" 'render should print its resolved output path'
assert_equal sentinel "$(sed -n '1p' "$fake_open_log")" 'render subcommand must not invoke open'

render_output_before_input=$render_dir/'before-input.html'
MD_VIEW_DATA_DIR="$tool_dir" "$cli" render --output="$render_output_before_input" "$input_path" >/dev/null
assert_file "$render_output_before_input"
assert_equal sentinel "$(sed -n '1p' "$fake_open_log")" 'option-first render must not invoke open'

if MD_VIEW_DATA_DIR="$tool_dir" "$cli" render "$input_path" --output "$input_path" >"$test_tmp/same.stdout" 2>"$test_tmp/same.stderr"; then
  fail 'render accepted an output path equal to its input'
fi
assert_contains "$test_tmp/same.stderr" 'must not resolve to the input'

if MD_VIEW_DATA_DIR="$tool_dir" "$cli" render "$input_path" --output "$test_tmp/missing/output.html" >"$test_tmp/missing.stdout" 2>"$test_tmp/missing.stderr"; then
  fail 'render accepted a missing output directory'
fi
assert_contains "$test_tmp/missing.stderr" 'output directory does not exist'

invalid_input=$test_tmp/notes.txt
printf '%s\n' 'not a Markdown input' >"$invalid_input"
if MD_VIEW_DATA_DIR="$tool_dir" "$cli" "$invalid_input" >"$test_tmp/invalid.stdout" 2>"$test_tmp/invalid.stderr"; then
  fail 'default command accepted a non-Markdown extension'
fi
assert_contains "$test_tmp/invalid.stderr" 'Markdown extension'

cached_checksum=$(cksum "$cached_output")
printf '%s\n' 'sentinel' >"$fake_open_log"
export MD_VIEW_FAKE_PANDOC_FAIL=1
if MD_VIEW_DATA_DIR="$tool_dir" TMPDIR="$cache_root" "$cli" "$input_path" >"$test_tmp/failure.stdout" 2>"$test_tmp/failure.stderr"; then
  fail 'failed fake Pandoc invocation unexpectedly succeeded'
fi
export MD_VIEW_FAKE_PANDOC_FAIL=0
assert_contains "$test_tmp/failure.stderr" 'fake pandoc parse failure'
assert_equal "$cached_checksum" "$(cksum "$cached_output")" 'failed rendering must preserve the cached preview'
assert_equal sentinel "$(sed -n '1p' "$fake_open_log")" 'failed rendering must not invoke open'
assert_equal 0 "$(find "$cache_root/md-view" -name '*.tmp.*' -print | wc -l | tr -d ' ')" 'failed rendering must remove its temporary file'

helper_commands='realpath tr cksum awk dirname basename mkdir mktemp mv rm'
no_open_bin=$test_tmp/no-open-bin
no_pandoc_bin=$test_tmp/no-pandoc-bin
mkdir -p "$no_open_bin" "$no_pandoc_bin"
for helper in $helper_commands; do
  helper_path=$(command -v "$helper")
  ln -s "$helper_path" "$no_open_bin/$helper"
  ln -s "$helper_path" "$no_pandoc_bin/$helper"
done
ln -s "$fake_bin/pandoc" "$no_open_bin/pandoc"

if PATH="$no_open_bin" MD_VIEW_DATA_DIR="$tool_dir" TMPDIR="$cache_root" "$cli" "$input_path" >"$test_tmp/no-open.stdout" 2>"$test_tmp/no-open.stderr"; then
  fail 'default command succeeded without open'
fi
assert_contains "$test_tmp/no-open.stderr" 'required command not found: open'

if PATH="$no_pandoc_bin" MD_VIEW_DATA_DIR="$tool_dir" TMPDIR="$cache_root" "$cli" "$input_path" >"$test_tmp/no-pandoc.stdout" 2>"$test_tmp/no-pandoc.stderr"; then
  fail 'default command succeeded without Pandoc'
fi
assert_contains "$test_tmp/no-pandoc.stderr" 'required command not found: pandoc'

real_pandoc_dir=$(dirname "$pandoc_path")
PATH=$real_pandoc_dir:$original_path
export PATH

integration_dir=$test_tmp/'integration output'
mkdir -p "$integration_dir"
integration_output=$integration_dir/'representative preview.html'
integration_stdout=$test_tmp/integration.stdout
MD_VIEW_DATA_DIR="$tool_dir" "$cli" render "$input_path" --output "$integration_output" >"$integration_stdout"
assert_file "$integration_output"
assert_equal "$integration_output" "$(sed -n '1p' "$integration_stdout")" 'integration render should print its output path'

assert_contains "$integration_output" '<!DOCTYPE html>'
assert_contains "$integration_output" '<style>'
assert_contains "$integration_output" '.table-scroll'
assert_not_contains "$integration_output" 'md-view.css'
assert_contains "$integration_output" 'data:image/svg+xml'
assert_not_contains "$integration_output" 'local-image.svg'
assert_contains "$integration_output" 'data-pos="'
assert_contains "$integration_output" 'explicit-heading'
assert_contains "$integration_output" 'id="repeated-heading"'
assert_contains "$integration_output" 'class="table-scroll"'
assert_contains "$integration_output" '<table'
assert_contains "$integration_output" '<ul data-pos="'
assert_contains "$integration_output" '<blockquote data-pos="'
assert_contains "$integration_output" 'flowchart TD'
assert_contains "$integration_output" 'sequenceDiagram'
assert_contains "$integration_output" 'Mermaid could not render this diagram'
assert_contains "$integration_output" 'node.textContent = source'
assert_contains "$integration_output" 'div.sourceCode'
assert_contains "$integration_output" '&lt;span'
assert_not_contains "$integration_output" '<span class="unsafe">raw HTML'

script_sources=$(grep -c '<script[^>]*src=' "$integration_output" || true)
assert_equal 1 "$script_sources" 'the generated document should have one external script'
mermaid_url='https://cdn.jsdelivr.net/npm/mermaid@11.12.1/dist/mermaid.min.js'
assert_equal 1 "$(count_text "$mermaid_url" "$integration_output")" 'the Mermaid URL should be pinned exactly once'

if ! awk '/class="mermaid"/ && /data-pos="/ { found = 1 } END { exit found ? 0 : 1 }' "$integration_output"; then
  fail 'rewritten Mermaid blocks must preserve data-pos'
fi
if ! awk '/class="table-scroll"/ && /data-pos="/ { found = 1 } END { exit found ? 0 : 1 }' "$integration_output"; then
  fail 'table wrappers must preserve data-pos'
fi
if ! awk '/id="explicit-heading"/ && /data-pos="/ { found = 1 } END { exit found ? 0 : 1 }' "$integration_output"; then
  fail 'explicit heading identifiers must preserve data-pos'
fi
if ! awk '/id="repeated-heading"/ && /data-pos="/ { found = 1 } END { exit found ? 0 : 1 }' "$integration_output"; then
  fail 'automatic heading identifiers must preserve data-pos'
fi

fixture_html=$(find "$tool_dir/test/fixtures" -name '*.html' -print -quit)
[ -z "$fixture_html" ] || fail "fixture directory received an output artifact: $fixture_html"

printf 'md-view tests passed (%s)\n' "$($pandoc_path --version | sed -n '1p')"
