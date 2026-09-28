# Expert Team A2A Protocol

Reasonix expert teams use the A2A 1.0 data model over an in-process custom
binding. The binding is identified by:

```text
urn:reasonix:a2a:binding:in-process:v1
```

The implementation follows the A2A separation between communication and task
outputs:

- `Message` and `Part` carry lead-to-member instructions.
- `Task` and `TaskStatus` carry lifecycle state.
- `Artifact` and `Part` carry the member's final output and generated files.
- Background work returns `TASK_STATE_WORKING` immediately and later returns
  exactly one terminal state (`TASK_STATE_COMPLETED`, `TASK_STATE_FAILED`, or
  `TASK_STATE_CANCELED`) with the same task and context identifiers. The task ID
  is allocated before dispatch, including for ephemeral child sessions, so a
  terminal response can never leave the original working task orphaned.

## Topology

The required extension is:

```text
urn:reasonix:a2a:extension:lead-member-only:v1
```

Only this directed topology is valid:

```text
lead expert ──SendMessage──> configured member
lead expert <──Task/Artifact── configured member
```

The following routes are forbidden:

```text
member ─X─> member
member ─X─> lead as a new request
member ─X─> another task/parallel_tasks endpoint
```

Enforcement is layered:

1. The lead runtime creates the A2A envelope; the model does not author routing
   metadata.
2. The recipient must match the configured `member_name`.
3. Runtime validation requires `senderRole=lead`,
   `recipientRole=member`, and `mode=lead-member-only`.
4. Child registries exclude `task`, `parallel_tasks`, and other delegation
   tools, so a member has no peer-messaging endpoint.

## SendMessage request

The in-process transport uses the A2A JSON-RPC binding:

```json
{
  "jsonrpc": "2.0",
  "id": "message-...",
  "method": "SendMessage",
  "params": {
    "message": {
      "messageId": "message-...",
      "contextId": "parent-session",
      "role": "ROLE_USER",
      "parts": [
        {
          "text": "member-specific delegated task",
          "mediaType": "text/plain"
        }
      ],
      "extensions": [
        "urn:reasonix:a2a:extension:lead-member-only:v1"
      ]
    },
    "configuration": {
      "acceptedOutputModes": [
        "text/plain",
        "application/vnd.reasonix.workspace-file+json"
      ]
    },
    "metadata": {
      "protocolVersion": "1.0",
      "protocolBinding": "urn:reasonix:a2a:binding:in-process:v1",
      "topology": {
        "mode": "lead-member-only",
        "senderRole": "lead",
        "senderName": "lead-expert",
        "recipientRole": "member",
        "recipientName": "configured member name"
      }
    }
  }
}
```

For a background task, `configuration.returnImmediately` is `true`.

## Task response

Successful member execution returns an A2A `Task`:

```json
{
  "jsonrpc": "2.0",
  "id": "message-...",
  "result": {
    "task": {
      "id": "server-generated-task-id",
      "contextId": "parent-session",
      "status": {
        "state": "TASK_STATE_COMPLETED",
        "timestamp": "RFC3339 timestamp"
      },
      "artifacts": [
        {
          "artifactId": "artifact-...",
          "name": "final-answer",
          "parts": [
            {
              "text": "member result",
              "mediaType": "text/plain"
            }
          ]
        },
        {
          "artifactId": "artifact-...",
          "name": "report.pdf",
          "parts": [
            {
              "data": {
                "path": "canonical workspace-contained absolute path",
                "size": 1234
              },
              "filename": "report.pdf",
              "mediaType": "application/pdf"
            }
          ]
        }
      ]
    }
  }
}
```

Workspace file artifacts are derived from successful writer receipts and
runtime-observed workspace deltas rather than trusted from model prose. Only
actual regular files inside the owning workspace are included in the structured
handoff. The lead must link the actual file path in its final response; a file
request in user prose is not a separate host final-readiness condition.

Failed and cancelled executions return the same `Task` envelope with a terminal
status and an agent status message containing the execution error. They do not
return partial workspace files as successful artifacts.

Reference: [A2A Protocol specification](https://github.com/a2aproject/A2A/blob/main/docs/specification.md).
