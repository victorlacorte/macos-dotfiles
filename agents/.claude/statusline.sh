#!/bin/sh
# Claude Code status line: "<model> (<effort>) | ctx <N>% used | <used>/<size> tokens".
# Claude Code pipes session JSON on stdin; the context fields are null until
# the first API response of a session (and again right after /compact).
exec jq -r '
  def tokens:
    if . >= 1000000 then "\((. / 100000 | floor) / 10)M"
    else "\(. / 1000 | floor)k" end;

  (.model.display_name // .model.id // "?") as $name
  | (if .effort.level then "\($name) (\(.effort.level))" else $name end) as $model
  | .context_window as $c
  | if $c.used_percentage == null then
      "\($model) | ctx -"
    else
      "\($model) | ctx \($c.used_percentage | floor)% used |\(($c.total_input_tokens + $c.total_output_tokens) | tokens)/\($c.context_window_size | tokens) tokens"
    end
'
