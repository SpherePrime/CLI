# Delegating independent work

Delegate independent sizeable work when permitted and useful.
The main agent works inline by default; prefer direct tool calls for reads, searches, edits and checks. Use the agent tool only when work is genuinely independent, large enough to justify a fresh context, or naturally parallel. Give each subagent its objective, context, evidence, file ownership, constraints, acceptance criteria and expected report. Give concurrent writers disjoint scopes. Expect implementation subagents to run relevant tests, builds and checks to verify their own work before reporting back. Do not duplicate a subagent's investigation or redo its work; handle failures and integrate verified results. Respect active user and project instructions for every delegated task.

Detailed references: read_instruction with `source-subagent-delegation-restraint`, `source-writing-subagent-prompts`, `source-tool-usage-subagent-guidance`.
