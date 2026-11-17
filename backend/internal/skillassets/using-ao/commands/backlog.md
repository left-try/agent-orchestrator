# ao backlog

Save, inspect, update, implement, or delete Kanban backlog items capturing unimplemented discussions.

```text
ao backlog list [--project <id>] [--json]
ao backlog add --title <title> [--summary <summary>] [--prompt <prompt>] [--project <id>] [--json]
ao backlog update <backlog-id> [--title <title>] [--summary <summary>] [--prompt <prompt>] [--status open|implemented] [--json]
ao backlog implement <backlog-id> [--harness <agent>] [--model <id>] [--mode chat|tui] [--json]
ao backlog delete <backlog-id>
```

When an orchestrator wraps up a substantive unimplemented discussion and the human starts a new topic, it may occasionally ask once whether to save that discussion to the Kanban Backlog, then run `ao backlog add` if approved.
