-- +goose Up
CREATE TABLE agent_runs (
    workspace_path TEXT NOT NULL,
    run_id TEXT NOT NULL,
    session_id TEXT NOT NULL,
    request_fingerprint BLOB NOT NULL,
    process_id TEXT NOT NULL,
    workspace_instance_id TEXT NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('running', 'completed', 'failed', 'cancelled', 'interrupted')),
    accepted_at TEXT NOT NULL,
    finished_at TEXT,
    error TEXT NOT NULL DEFAULT '',
    message_id TEXT NOT NULL DEFAULT '',
    response_text TEXT NOT NULL DEFAULT '',
    PRIMARY KEY (workspace_path, run_id)
);
CREATE INDEX agent_runs_workspace_accepted ON agent_runs(workspace_path, accepted_at);
CREATE INDEX agent_runs_workspace_session ON agent_runs(workspace_path, session_id);

-- +goose Down
DROP TABLE agent_runs;
