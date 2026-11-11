#!/bin/sh

set -eu

test_dir=$(CDPATH='' cd -- "$(dirname -- "$0")" && pwd) || exit 1
repo_root=$(CDPATH='' cd -- "$test_dir/.." && pwd) || exit 1
append=$repo_root/agents/.agents/skills/orchestrate/scripts/journal-append

tmpdir=$(mktemp -d) || exit 1
trap 'rm -rf "$tmpdir"' EXIT

task=$tmpdir/20261104-test
mkdir -p "$task"
journal=$task/journal.jsonl
err=$tmpdir/stderr.txt

fail_test() {
  printf 'FAIL: %s\n' "$*" >&2
  exit 1
}

# append <expected-exit> <data> <args...>: runs journal-append with <data> on stdin.
append() {
  want=$1
  data=$2
  shift 2
  got=0
  printf '%s' "$data" | env -u CODEX_THREAD_ID CURSOR_CONVERSATION_ID=conv-1 \
    "$append" "$task" "$@" > "$tmpdir/stdout.txt" 2> "$err" || got=$?
  [ "$got" -eq "$want" ] || fail_test "expected exit $want from $*, got $got: $(cat "$err")"
}

expect_error() {
  grep -qF -- "$1" "$err" || fail_test "expected error '$1', got: $(cat "$err")"
}

expect_lines() {
  got=$(wc -l < "$journal" | tr -d ' ')
  [ "$got" -eq "$1" ] || fail_test "expected $1 journal lines, got $got"
}

expect_jq() {
  jq -e "$1" "$journal" > /dev/null || fail_test "journal does not satisfy: $1"
}

task_started='{"title":"t","outcome":"o","acceptance_criteria":["a"],"derived_from":null}'
reviewed='{"child_status":"complete","decision":"accepted","reason":null,"rationale":"r","correction":null}'
artifact='{"kind":"commit","relationship":"r","locator":"abc"}'
finished='{"outcome":"completed","criteria_met":["a"],"criteria_unmet":[],"open_work_items":[],"next_steps":[],"journal_gaps":[]}'

append 1 "$artifact" artifact.linked
expect_error 'no open session'
[ ! -e "$journal" ] || fail_test 'rejected event created the journal'

append 0 "$task_started" task.started
append 1 "$task_started" task.started
expect_error 'already recorded'

append 0 '{"model":"m","effort":null}' session.started
expect_jq 'select(.type == "session.started") | .data
  | .harness == "cursor" and .conversation_id == "conv-1"
    and (.skill_revision.commit | test("^[0-9a-f]{40}$"))
    and (.skill_revision.dirty | type == "boolean")
    and (.utc_offset | test("^[+-][0-9]{2}:[0-9]{2}$"))'

append 1 '{"child_status":"done","decision":"accepted","reason":"unknown","corrections":"x"}' \
  attempt.reviewed
expect_error 'missing key: rationale'
expect_error 'unknown key: corrections'
expect_error 'child_status must be one of'
expect_error 'reason must be null exactly when decision is accepted'
expect_error 'attempt.reviewed requires --attempt'

append 1 "$reviewed" attempt.reviewed --attempt check
expect_error 'attempt ID must be'
append 1 'not json' artifact.linked
expect_error 'stdin must be a JSON object'
append 1 '{}' usage.recorded
expect_error 'unknown event type'
append 1 '{}' attempt.finished --attempt check-1
expect_error 'parent types are'
expect_lines 2

append 0 "$reviewed" attempt.reviewed --attempt check-1
reviewed_id=$(cat "$tmpdir/stdout.txt")
expect_jq "select(.event_id == \"$reviewed_id\")
  | .writer == \"parent\" and .task_id == \"20261104-test\"
    and .work_item_id == \"check\" and .attempt_id == \"check-1\" and .supersedes == null"

append 1 "$reviewed" attempt.reviewed --attempt check-1
expect_error 'attempt.reviewed for check-1 is already recorded'
append 1 "$reviewed" attempt.reviewed --attempt other-1 --supersedes "$reviewed_id"
expect_error 'names an event of attempt check-1, not other-1'
append 0 "$reviewed" attempt.reviewed --attempt check-1 --supersedes "$reviewed_id"
append 1 "$reviewed" attempt.reviewed --attempt check-1 --supersedes "$reviewed_id"
expect_error 'already replaced'
append 0 '{"child_status":"missing","decision":"blocked","reason":"interrupted","rationale":"r","correction":null}' \
  attempt.reviewed --attempt stopped-1

append 1 "$finished" session.finished --supersedes "$reviewed_id"
expect_error 'names an event of type attempt.reviewed'
append 1 "$finished" session.finished --supersedes missing-id
expect_error 'names no event'

append 0 "$finished" session.finished
finished_id=$(cat "$tmpdir/stdout.txt")
append 1 "$artifact" artifact.linked
expect_error 'no open session'
append 0 "$finished" session.finished --supersedes "$finished_id"
append 0 '{"model":"m","effort":"high"}' session.started
append 0 "$artifact" artifact.linked

expect_lines 9
jq -s -e 'all(.[]; .schema_version == 3
  and (.event_id | test("^[0-9a-f-]{36}$"))
  and (.occurred_at | test("^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9:]{8}Z$")))' "$journal" > /dev/null \
  || fail_test 'envelope fields are malformed'

printf 'ok\n'
