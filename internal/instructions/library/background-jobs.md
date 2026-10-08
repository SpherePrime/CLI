# Managing long-running jobs

Track owned long-running processes with supported lifecycle tools.
Use background capability exposed by bash or agent without inventing parameters. Keep job IDs; inspect agent_jobs and job_output, and job_kill only owned jobs needing termination. Observe server readiness within a deadline before testing and inspect startup logs on failure. Avoid busy polling or long sleeps; continue independent work. Clean up owned processes and report pending jobs or failures.

Detailed references: read_instruction with `source-bash-sleep-no-polling-background-tasks`.

Detailed reference: read_instruction with `source-run-web-server-api-example`.
