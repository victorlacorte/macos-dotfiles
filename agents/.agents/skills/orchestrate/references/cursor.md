# Cursor adapter

Use the [saved role defaults](subagents.md), preserving the exact model ID and its encoded effort. Map the role to the saved agent type only when the active spawn surface exposes that field. Do not append effort to a model ID or send a separate effort override unless the active syntax supports it.

Confirm the active tools support the assigned work. A configured `explore` type does not by itself establish access to web research, shell commands, or check-running tools. Use the available capabilities and report limitations; do not claim a capability is absent based on the type name alone. Report any model or type override imposed by the client. See the [Cursor subagents documentation](https://cursor.com/docs/subagents) for supported custom-agent fields.
