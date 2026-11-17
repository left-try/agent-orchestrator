-- Summary: persist customizable Kanban board settings in app_settings and add project-scoped backlog_items for unimplemented orchestrator discussions.
-- +goose Up
-- +goose StatementBegin
ALTER TABLE app_settings ADD COLUMN kanban_config TEXT NOT NULL DEFAULT '';
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TABLE backlog_items (
    id                      TEXT PRIMARY KEY,
    project_id              TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    orchestrator_session_id TEXT NOT NULL DEFAULT '',
    title                   TEXT NOT NULL,
    summary                 TEXT NOT NULL DEFAULT '',
    prompt                  TEXT NOT NULL DEFAULT '',
    status                  TEXT NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'implemented')),
    spawned_session_id      TEXT NOT NULL DEFAULT '',
    created_at              TIMESTAMP NOT NULL,
    updated_at              TIMESTAMP NOT NULL
);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE INDEX idx_backlog_items_project_status_updated
    ON backlog_items (project_id, status, updated_at DESC);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_backlog_items_project_status_updated;
-- +goose StatementEnd

-- +goose StatementBegin
DROP TABLE IF EXISTS backlog_items;
-- +goose StatementEnd

-- +goose StatementBegin
ALTER TABLE app_settings DROP COLUMN kanban_config;
-- +goose StatementEnd
