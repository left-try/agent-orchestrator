-- +goose Up
ALTER TABLE sessions ADD COLUMN workspace_folder TEXT NOT NULL DEFAULT '';

ALTER TABLE conversations ADD COLUMN workspace_folder TEXT NOT NULL DEFAULT '';
DROP INDEX idx_conversations_project_scope;
CREATE UNIQUE INDEX idx_conversations_project_scope ON conversations(project_id, workspace_folder) WHERE scope = 'project';

-- +goose Down
DROP INDEX idx_conversations_project_scope;
CREATE UNIQUE INDEX idx_conversations_project_scope ON conversations(project_id) WHERE scope = 'project';
ALTER TABLE conversations DROP COLUMN workspace_folder;
ALTER TABLE sessions DROP COLUMN workspace_folder;
