#!/bin/sh
# Cursor postToolUse/postToolUseFailure hook: appends attempt.finished to the
# orchestrate task journal, through journal-append, for Task calls whose brief
# has the attempt header. Always exits 0 with an empty response so it can never
# block the agent.

input=$(cat)
printf '{}\n'

command -v jq >/dev/null 2>&1 || exit 0

report=$(printf '%s' "$input" | jq -r '
  select(.tool_name == "Task")
  | (.tool_input | if type == "string" then (fromjson? // {}) else . end)
  | .prompt // empty
  | split("\n")
  | map(select(startswith("Report: ")))
  | first // empty
  | ltrimstr("Report: ")' 2>/dev/null) || exit 0

case $report in
  /*/.scratch/*/reports/*.md) ;;
  *) exit 0 ;;
esac

task_dir=${report%/reports/*}
attempt_id=$(basename -- "$report" .md)
[ -d "$task_dir" ] || exit 0

data=$(printf '%s' "$input" | jq -c '
  (.tool_input | if type == "string" then (fromjson? // {}) else . end) as $in
  | (.tool_output | if type == "string" then (fromjson? // {}) else (. // {}) end) as $out
  | (if .hook_event_name == "postToolUseFailure" then null else ($out.isBackground // false) end) as $background
  | (if $background == true then null else ($out.durationMs // .duration) end) as $duration
  | ($in.prompt | split("\n") | map(select(startswith("Role: "))) | first // null | if . then ltrimstr("Role: ") else null end) as $role
  | {
      role: $role,
      native_child_id: ($out.agentId // null),
      child_type: ($in.subagent_type // null),
      requested_model: ($in.model // null),
      harness_status: (if .hook_event_name == "postToolUseFailure"
        then (if .is_interrupt then "interrupted" else (.failure_type // "error") end)
        else ($out.status // null) end),
      error_message: (.error_message // null),
      background: $background,
      started_at: (if $background == true then (now | todate)
        elif $duration then ((now - ($duration / 1000)) | todate) else null end),
      duration_ms: $duration,
      parent_conversation_id: (.conversation_id // null),
      child_transcript_path: (if (.transcript_path | type) == "string" and $out.agentId
        then ((.transcript_path | sub("/[^/]+$"; "")) + "/subagents/" + $out.agentId + ".jsonl")
        else null end),
      harness_version: (.cursor_version // null)
    }' 2>/dev/null) || exit 0

append=$(dirname "$(dirname "$(realpath "$0")")")/scripts/journal-append
printf '%s' "$data" | "$append" "$task_dir" attempt.finished --writer hook --attempt "$attempt_id" >/dev/null 2>&1
exit 0
