# Security

The agent enforces a layered security model.

## Permissions

Every tool call passes through the `PermissionEngine`:

- **Modes**: `ask`, `allow`, `deny`
- **Path restrictions**: `allowed_paths` / `denied_paths` in config
- **Tool restrictions**: `allowed_tools` / `denied_tools`
- **Network scope**: only explicitly allowed domains

## Secret redaction

Tool output is scanned for API-key patterns before being returned to the model. The engine runs before tool results are appended to the context.

## Environment protection

The shell runner refuses to inject protected environment variables (`AGENT_API_KEY`, `SSH_AUTH_SOCK`, AWS credentials, etc.).

## Audit log

Every permission decision is written to `~/.agent/audit/YYYY-MM-DD.jsonl` with outcome, target, and timestamp.

## Sandboxing

Path restrictions keep the agent confined to configured directories. The CI mode (`--ci`) disables interactive confirmation and requires explicit `allowed_tools`.
