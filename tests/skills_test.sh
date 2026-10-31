#!/bin/sh

set -eu

test_dir=$(CDPATH='' cd -- "$(dirname -- "$0")" && pwd) || exit 1
repo_root=$(CDPATH='' cd -- "$test_dir/.." && pwd) || exit 1
skills=$repo_root/scripts/skills

tmpdir=$(mktemp -d) || exit 1
trap 'rm -rf "$tmpdir"' EXIT

HOME=$tmpdir/home
mkdir -p "$HOME"
export HOME

GH_LOG=$tmpdir/gh.log
: > "$GH_LOG"
export GH_LOG

err=$tmpdir/stderr.txt
out=$tmpdir/stdout.txt

stub_bin=$tmpdir/bin
mkdir -p "$stub_bin"

cat > "$stub_bin/gh" <<'EOF'
#!/bin/sh
set -eu
: "${GH_LOG:=/dev/null}"
printf '%s\n' "$*" >> "$GH_LOG"

[ "${1-}" = skill ] || exit 1
cmd=$2
shift 2

if [ "$cmd" = install ]; then
  [ "$#" -ge 2 ] || exit 1
  selector=$2
  shift 2
  dest=""
  while [ "$#" -gt 0 ]; do
    case "$1" in
      --dir)
        dest=$2
        shift 2
        ;;
      *) exit 1 ;;
    esac
  done
  [ -n "$dest" ] || exit 1
  name=$(basename -- "$selector")
  mkdir -p "$dest/$name"
  printf '%s\n' "${STUB_SKILL_CONTENT:-upstream}" > "$dest/$name/SKILL.md"
  if [ -n "${STUB_UPSTREAM_FILE-}" ]; then
    mkdir -p "$dest/$name/$(dirname -- "$STUB_UPSTREAM_FILE")"
    printf '%s\n' "upstream-file" > "$dest/$name/$STUB_UPSTREAM_FILE"
  fi
  exit 0
fi

if [ "$cmd" = update ]; then
  dest=""
  dry_run=0
  while [ "$#" -gt 0 ]; do
    case "$1" in
      --dir)
        dest=$2
        shift 2
        ;;
      --dry-run)
        dry_run=1
        shift
        ;;
      *) break ;;
    esac
  done
  [ -n "$dest" ] || exit 1
  if [ "$dry_run" -eq 1 ]; then
    exit 0
  fi
  if [ "${STUB_UPDATE_NOOP:-}" = 1 ]; then
    exit 0
  fi
  for name in "$@"; do
    rm -rf "$dest/$name"
    mkdir -p "$dest/$name"
    printf '%s\n' "updated" > "$dest/$name/SKILL.md"
    if [ -n "${STUB_UPSTREAM_FILE-}" ]; then
      mkdir -p "$dest/$name/$(dirname -- "$STUB_UPSTREAM_FILE")"
      printf '%s\n' "upstream-file" > "$dest/$name/$STUB_UPSTREAM_FILE"
    fi
  done
  exit 0
fi

exit 1
EOF
chmod +x "$stub_bin/gh"

PATH="$stub_bin:$PATH"
export PATH

catalog_dir=$tmpdir/catalog
mkdir -p "$catalog_dir"
cat > "$catalog_dir/catalog.json" <<'EOF'
{
  "schemaVersion": 1,
  "skills": {
    "alpha": {
      "repository": "example/skills",
      "selector": "alpha"
    },
    "beta": {
      "repository": "example/skills",
      "selector": "beta"
    }
  }
}
EOF
export SKILLS_CATALOG="$catalog_dir/catalog.json"

fail_test() {
  printf 'FAIL: %s\n' "$*" >&2
  exit 1
}

expect_status() {
  want=$1
  shift
  got=0
  "$@" || got=$?
  [ "$got" -eq "$want" ] || fail_test "expected exit $want from $*, got $got"
}

expect_file() {
  path=$1
  expected=$2
  [ -f "$path" ] || fail_test "missing $path"
  got=$(cat "$path")
  [ "$got" = "$expected" ] || fail_test "$path: expected '$expected', got '$got'"
}

reset_home() {
  rm -rf "$HOME/.agents" "$HOME/.cursor"
  mkdir -p "$HOME/.agents/skills" "$HOME/.cursor/skills"
  : > "$GH_LOG"
  unset STUB_UPSTREAM_FILE
  unset STUB_SKILL_CONTENT
  unset STUB_UPDATE_NOOP
}

reset_overlays() {
  rm -rf "$catalog_dir/overlays"
  mkdir -p "$catalog_dir/overlays/alpha/agents"
  printf '%s\n' "overlay-a" > "$catalog_dir/overlays/alpha/agents/openai.yaml"
}

alpha_dir=$HOME/.agents/skills/alpha
beta_dir=$HOME/.agents/skills/beta

printf 'a. install --all on clean home\n'
reset_overlays
reset_home
expect_status 0 "$skills" install --all
expect_file "$alpha_dir/SKILL.md" "upstream"
expect_file "$alpha_dir/agents/openai.yaml" "overlay-a"
expect_file "$alpha_dir/.skills-overlay" "agents/openai.yaml"
[ ! -f "$beta_dir/.skills-overlay" ] || fail_test "a: beta should not have a marker"
expect_file "$beta_dir/SKILL.md" "upstream"
n=0
while IFS= read -r line || [ -n "$line" ]; do
  [ -n "$line" ] || continue
  n=$((n + 1))
  printf '%s\n' "$line" | grep -q -- '--dir' || fail_test "a: missing --dir: $line"
done < "$GH_LOG"
[ "$n" -eq 2 ] || fail_test "a: expected 2 gh install lines, got $n: $(cat "$GH_LOG")"

printf 'b. install alpha is skipped when already installed\n'
reset_overlays
reset_home
expect_status 0 "$skills" install alpha
: > "$GH_LOG"
got=0
"$skills" install alpha >"$out" || got=$?
[ "$got" -eq 0 ] || fail_test "b: expected exit 0, got $got"
grep -q 'already installed' "$out" || fail_test "b: stdout missing already installed: $(cat "$out")"
[ ! -s "$GH_LOG" ] || fail_test "b: gh was invoked: $(cat "$GH_LOG")"

printf 'c. install --all only installs missing skills\n'
reset_overlays
reset_home
expect_status 0 "$skills" install alpha
: > "$GH_LOG"
expect_status 0 "$skills" install --all
gh_line=$(cat "$GH_LOG")
printf '%s\n' "$gh_line" | grep -q beta || fail_test "c: gh missing beta: $gh_line"
printf '%s\n' "$gh_line" | grep -Eq '(^| )alpha( |$)' && fail_test "c: gh installed alpha again: $gh_line"
n=0
while IFS= read -r line || [ -n "$line" ]; do
  [ -n "$line" ] || continue
  n=$((n + 1))
done < "$GH_LOG"
[ "$n" -eq 1 ] || fail_test "c: expected 1 gh install line, got $n: $gh_line"

printf 'd. existing dir without SKILL.md is refused\n'
reset_overlays
reset_home
mkdir -p "$alpha_dir"
: > "$GH_LOG"
got=0
"$skills" install alpha 2>/dev/null || got=$?
[ "$got" -eq 2 ] || fail_test "d: expected exit 2, got $got"
[ ! -s "$GH_LOG" ] || fail_test "d: gh was invoked: $(cat "$GH_LOG")"

printf 'e. update swap then noop reapply overlay edits\n'
reset_overlays
reset_home
expect_status 0 "$skills" install alpha
printf '%s\n' "overlay-b" > "$catalog_dir/overlays/alpha/agents/openai.yaml"
expect_status 0 "$skills" update alpha
expect_file "$alpha_dir/SKILL.md" "updated"
expect_file "$alpha_dir/agents/openai.yaml" "overlay-b"
expect_file "$alpha_dir/.skills-overlay" "agents/openai.yaml"
printf '%s\n' "overlay-c" > "$catalog_dir/overlays/alpha/agents/openai.yaml"
STUB_UPDATE_NOOP=1
export STUB_UPDATE_NOOP
expect_status 0 "$skills" update alpha
expect_file "$alpha_dir/agents/openai.yaml" "overlay-c"
expect_file "$alpha_dir/.skills-overlay" "agents/openai.yaml"
unset STUB_UPDATE_NOOP

printf 'f. noop update replaces overlay files\n'
reset_overlays
reset_home
expect_status 0 "$skills" install alpha
rm -rf "$catalog_dir/overlays/alpha"
mkdir -p "$catalog_dir/overlays/alpha"
printf '%s\n' "overlay-notes" > "$catalog_dir/overlays/alpha/notes.txt"
STUB_UPDATE_NOOP=1
export STUB_UPDATE_NOOP
expect_status 0 "$skills" update alpha
[ ! -e "$alpha_dir/agents/openai.yaml" ] || fail_test "f: old overlay file still present"
expect_file "$alpha_dir/notes.txt" "overlay-notes"
expect_file "$alpha_dir/.skills-overlay" "notes.txt"
unset STUB_UPDATE_NOOP

printf 'g. deleting overlay dir strips overlay files\n'
reset_overlays
reset_home
expect_status 0 "$skills" install alpha
rm -rf "$catalog_dir/overlays/alpha"
STUB_UPDATE_NOOP=1
export STUB_UPDATE_NOOP
expect_status 0 "$skills" update alpha
[ ! -e "$alpha_dir/agents/openai.yaml" ] || fail_test "g: overlay file still present"
[ ! -e "$alpha_dir/.skills-overlay" ] || fail_test "g: overlay marker still present"
expect_file "$alpha_dir/SKILL.md" "upstream"
unset STUB_UPDATE_NOOP

printf 'h. upstream file wins over overlay on update\n'
reset_overlays
reset_home
expect_status 0 "$skills" install alpha
STUB_UPSTREAM_FILE=agents/openai.yaml
export STUB_UPSTREAM_FILE
got=0
"$skills" update alpha 2>"$err" || got=$?
[ "$got" -eq 0 ] || fail_test "h: update alpha failed with $got"
expect_file "$alpha_dir/agents/openai.yaml" "upstream-file"
grep -q 'provided upstream' "$err" || fail_test "h: missing skip warning on stderr"
if [ -f "$alpha_dir/.skills-overlay" ] && grep -Fxq 'agents/openai.yaml' "$alpha_dir/.skills-overlay"; then
  fail_test "h: marker lists upstream file"
fi
STUB_UPDATE_NOOP=1
export STUB_UPDATE_NOOP
got=0
"$skills" update alpha 2>"$err" || got=$?
[ "$got" -eq 0 ] || fail_test "h: noop update failed with $got"
expect_file "$alpha_dir/agents/openai.yaml" "upstream-file"
unset STUB_UPSTREAM_FILE
unset STUB_UPDATE_NOOP

printf 'i. update --dry-run leaves mutated overlay\n'
reset_overlays
reset_home
expect_status 0 "$skills" install alpha
printf '%s\n' "mutated" > "$alpha_dir/agents/openai.yaml"
: > "$GH_LOG"
expect_status 0 "$skills" update alpha --dry-run
grep -q -- '--dry-run' "$GH_LOG" || fail_test "i: gh was not called with --dry-run"
expect_file "$alpha_dir/agents/openai.yaml" "mutated"

printf 'j. update --all skips missing catalog skills\n'
reset_overlays
reset_home
expect_status 0 "$skills" install alpha
: > "$GH_LOG"
got=0
"$skills" update --all 2>"$err" || got=$?
[ "$got" -eq 0 ] || fail_test "j: expected exit 0, got $got"
gh_line=$(cat "$GH_LOG")
printf '%s\n' "$gh_line" | grep -q -- '--dir' || fail_test "j: gh missing --dir: $gh_line"
printf '%s\n' "$gh_line" | grep -Eq '(^| )--all( |$)' && fail_test "j: gh received --all: $gh_line"
printf '%s\n' "$gh_line" | grep -Eq '(^| )alpha( |$)' || fail_test "j: gh missing alpha: $gh_line"
printf '%s\n' "$gh_line" | grep -Eq '(^| )beta( |$)' && fail_test "j: gh received beta: $gh_line"
grep -q 'skipping beta' "$err" || fail_test "j: stderr missing skip notice: $(cat "$err")"

printf 'k. update --all with none installed\n'
reset_overlays
reset_home
: > "$GH_LOG"
got=0
"$skills" update --all >"$out" || got=$?
[ "$got" -eq 0 ] || fail_test "k: expected exit 0, got $got"
[ ! -s "$GH_LOG" ] || fail_test "k: gh was invoked: $(cat "$GH_LOG")"

printf 'l. update of missing name is refused\n'
reset_overlays
reset_home
: > "$GH_LOG"
got=0
"$skills" update beta 2>/dev/null || got=$?
[ "$got" -eq 2 ] || fail_test "l: expected exit 2, got $got"
[ ! -s "$GH_LOG" ] || fail_test "l: gh was invoked: $(cat "$GH_LOG")"

printf 'm. skill dir symlink is refused for install and update\n'
reset_overlays
reset_home
ln -s "$tmpdir/elsewhere" "$alpha_dir"
: > "$GH_LOG"
got=0
"$skills" install alpha 2>/dev/null || got=$?
[ "$got" -eq 2 ] || fail_test "m: install expected exit 2, got $got"
[ ! -s "$GH_LOG" ] || fail_test "m: install invoked gh: $(cat "$GH_LOG")"
got=0
"$skills" update alpha 2>/dev/null || got=$?
[ "$got" -eq 2 ] || fail_test "m: update expected exit 2, got $got"
[ ! -s "$GH_LOG" ] || fail_test "m: update invoked gh: $(cat "$GH_LOG")"

printf 'n. orphan overlay dir is refused\n'
reset_overlays
reset_home
mkdir -p "$catalog_dir/overlays/gamma"
: > "$GH_LOG"
got=0
"$skills" install alpha 2>"$err" || got=$?
[ "$got" -eq 2 ] || fail_test "n: expected exit 2, got $got"
grep -q "overlay 'gamma' does not match a catalog skill" "$err" \
  || fail_test "n: missing overlay error: $(cat "$err")"
[ ! -s "$GH_LOG" ] || fail_test "n: gh was invoked: $(cat "$GH_LOG")"
rm -rf "$catalog_dir/overlays/gamma"

printf 'o. --dry-run with install is refused\n'
reset_overlays
reset_home
: > "$GH_LOG"
got=0
"$skills" install alpha --dry-run 2>/dev/null || got=$?
[ "$got" -eq 2 ] || fail_test "o: expected exit 2, got $got"
[ ! -s "$GH_LOG" ] || fail_test "o: gh was invoked: $(cat "$GH_LOG")"

printf 'p. --harness is an unknown option\n'
reset_overlays
reset_home
: > "$GH_LOG"
got=0
"$skills" install alpha --harness cursor 2>/dev/null || got=$?
[ "$got" -eq 2 ] || fail_test "p: expected exit 2, got $got"
[ ! -s "$GH_LOG" ] || fail_test "p: gh was invoked: $(cat "$GH_LOG")"

printf 'q. project scope installs at the git root\n'
reset_overlays
reset_home
project=$tmpdir/proj
mkdir -p "$project/sub"
git -C "$project" init -q
project_phys=$(CDPATH='' cd -- "$project" && pwd -P)
(
  CDPATH='' cd -- "$project_phys/sub" || exit 1
  expect_status 0 "$skills" install alpha --scope project
)
expect_file "$project_phys/.agents/skills/alpha/SKILL.md" "upstream"
expect_file "$project_phys/.agents/skills/alpha/agents/openai.yaml" "overlay-a"
expect_file "$project_phys/.agents/skills/alpha/.skills-overlay" "agents/openai.yaml"
[ ! -e "$alpha_dir/SKILL.md" ] || fail_test "q: installed into home shared dir"

printf 'r. catalog installs and updates preserve linked local skills\n'
reset_overlays
reset_home
authored=$tmpdir/authored-skill
mkdir -p "$authored" "$HOME/.agents/skills/local-files"
printf '%s\n' 'locally authored' > "$authored/SKILL.md"
ln -s "$authored" "$HOME/.agents/skills/local-dir"
ln -s "$authored/SKILL.md" "$HOME/.agents/skills/local-files/SKILL.md"
expect_status 0 "$skills" install --all
expect_status 0 "$skills" update --all --dry-run
expect_status 0 "$skills" update --all
expect_file "$alpha_dir/SKILL.md" "updated"
expect_file "$beta_dir/SKILL.md" "updated"
expect_file "$authored/SKILL.md" 'locally authored'
[ -L "$HOME/.agents/skills/local-dir" ] || fail_test 'r: local directory link replaced'
[ -L "$HOME/.agents/skills/local-files/SKILL.md" ] || fail_test 'r: local file link replaced'
grep -q 'local-dir\|local-files' "$GH_LOG" && fail_test 'r: local skills passed to gh'

printf 's. folded skills and agents directory links are refused before gh\n'
reset_overlays
linked_package=$tmpdir/linked-package
mkdir -p "$linked_package/skills"
printf '%s\n' untouched > "$linked_package/skills/keep"
for level in skills agents; do
  reset_home
  if [ "$level" = skills ]; then
    rmdir "$HOME/.agents/skills"
    ln -s "$linked_package/skills" "$HOME/.agents/skills"
  else
    rmdir "$HOME/.agents/skills" "$HOME/.agents"
    ln -s "$linked_package" "$HOME/.agents"
  fi
  for operation in 'install --all' 'update --all' 'update --all --dry-run'; do
    got=0
    # shellcheck disable=SC2086 # Split these fixed test arguments.
    "$skills" $operation >"$out" 2>"$err" || got=$?
    [ "$got" -eq 2 ] || fail_test "s: $level/$operation expected exit 2, got $got"
    grep -q 'make stow-agents' "$err" || fail_test 's: missing migration command'
    [ ! -s "$GH_LOG" ] || fail_test 's: gh invoked through a shared directory link'
    expect_file "$linked_package/skills/keep" untouched
    [ ! -e "$linked_package/skills/alpha" ] || fail_test 's: installed into Stow package'
  done
done

printf 't. project scope refuses a folded agents link\n'
reset_home
rm -rf "$project_phys/.agents"
ln -s "$linked_package" "$project_phys/.agents"
(
  CDPATH='' cd -- "$project_phys/sub" || exit 1
  expect_status 2 "$skills" install alpha --scope project >"$out" 2>"$err"
)
grep -q 'use real .agents and .agents/skills directories' "$err" \
  || fail_test 't: missing project layout guidance'
[ ! -s "$GH_LOG" ] || fail_test 't: gh invoked through a project directory link'

printf 'u. make stow-agents unfolds directory links and coexists with catalog skills\n'
if command -v stow >/dev/null 2>&1; then
  for level in skills agents; do
    stow_home=$tmpdir/stow-home-$level
    mkdir -p "$stow_home"
    if [ "$level" = skills ]; then
      mkdir -p "$stow_home/.agents"
    fi
    stow --dir="$repo_root" --target="$stow_home" agents
    if [ "$level" = skills ]; then
      [ -L "$stow_home/.agents/skills" ] || fail_test 'u: fixture skills directory not folded'
    else
      [ -L "$stow_home/.agents" ] || fail_test 'u: fixture agents directory not folded'
    fi
    expect_status 0 make -s -C "$repo_root" stow-agents HOME="$stow_home"
    [ ! -L "$stow_home/.agents" ] || fail_test 'u: agents directory still folded'
    [ ! -L "$stow_home/.agents/skills" ] || fail_test 'u: skills directory still folded'
    local_skill=$stow_home/.agents/skills/orchestrate/SKILL.md
    [ -L "$local_skill" ] || fail_test 'u: authored skill is not linked'
    : > "$GH_LOG"
    expect_status 0 env HOME="$stow_home" "$skills" install --all
    expect_status 0 env HOME="$stow_home" "$skills" update --all
    expect_file "$stow_home/.agents/skills/alpha/SKILL.md" updated
    expect_file "$stow_home/.agents/skills/beta/SKILL.md" updated
    [ -L "$local_skill" ] || fail_test 'u: catalog operation replaced local link'
    cmp "$local_skill" "$repo_root/agents/.agents/skills/orchestrate/SKILL.md" \
      || fail_test 'u: authored skill changed'
    # Restowing must also preserve catalog skills that are already installed.
    expect_status 0 make -s -C "$repo_root" stow-agents HOME="$stow_home"
    expect_file "$stow_home/.agents/skills/alpha/SKILL.md" updated
  done
else
  printf 'SKIP: Stow migration integration test (stow is not installed)\n'
fi

printf 'ok\n'
