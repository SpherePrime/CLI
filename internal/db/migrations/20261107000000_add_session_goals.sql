-- +goose Up
CREATE TABLE session_goals (
    session_id TEXT PRIMARY KEY REFERENCES sessions(id) ON DELETE CASCADE,
    text TEXT NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('active', 'achieved')),
    summary TEXT NOT NULL DEFAULT '',
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
);

-- +goose Down
DROP TABLE session_goals;
