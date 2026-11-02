# Cursor adapter

Use the [saved role defaults](subagents.md), preserving the exact model ID and its encoded effort. Map the role to the saved agent type only when the active spawn surface exposes that field. Do not append effort to a model ID or send a separate effort override unless the active syntax supports it.

Do not use the Task tool's `resume` parameter: it continues a finished child with its prior context, while every round requires a fresh child. The Task tool cannot message a running child; a child that needs information ends its attempt as `blocked`.

Confirm the active tools support the assigned work. A configured `explore` type does not by itself establish access to web research, shell commands, or check-running tools. Use the available capabilities and report limitations; do not claim a capability is absent based on the type name alone. A 2026-11-02 probe confirmed that built-in `explore` children can write files and run shell commands; role boundaries such as "do not edit source" are enforced only by the brief. Report any model or type override imposed by the client. See the [Cursor subagents documentation](https://cursor.com/docs/subagents) for supported custom-agent fields.
