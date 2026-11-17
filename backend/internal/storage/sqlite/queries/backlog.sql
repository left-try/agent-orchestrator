-- Project-scoped backlog items created from unimplemented orchestrator discussions.

-- name: InsertBacklogItem :exec
INSERT INTO backlog_items (
    id, project_id, orchestrator_session_id, title, summary, prompt, status, spawned_session_id, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: SelectBacklogItemByID :one
SELECT *
FROM backlog_items
WHERE id = ?;

-- name: SelectBacklogItemsByProject :many
SELECT *
FROM backlog_items
WHERE project_id = ?
ORDER BY CASE WHEN status = 'open' THEN 0 ELSE 1 END, updated_at DESC, id DESC;

-- name: SelectAllBacklogItems :many
SELECT *
FROM backlog_items
ORDER BY CASE WHEN status = 'open' THEN 0 ELSE 1 END, updated_at DESC, id DESC;

-- name: UpdateBacklogItem :one
UPDATE backlog_items
SET title = ?, summary = ?, prompt = ?, status = ?, updated_at = ?
WHERE id = ?
RETURNING *;

-- name: MarkBacklogItemImplemented :one
UPDATE backlog_items
SET status = 'implemented', spawned_session_id = ?, updated_at = ?
WHERE id = ?
RETURNING *;

-- name: DeleteBacklogItemByID :execrows
DELETE FROM backlog_items
WHERE id = ?;
