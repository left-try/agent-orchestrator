package backlog

import (
	"context"
	"time"

	"github.com/aoagents/agent-orchestrator/backend/internal/domain"
	"github.com/aoagents/agent-orchestrator/backend/internal/ports"
)

// Store is the persistence surface used by the backlog service.
type Store interface {
	InsertBacklogItem(ctx context.Context, item domain.BacklogItem) error
	SelectBacklogItemByID(ctx context.Context, id domain.BacklogItemID) (domain.BacklogItem, bool, error)
	SelectBacklogItemsByProject(ctx context.Context, projectID domain.ProjectID) ([]domain.BacklogItem, error)
	SelectAllBacklogItems(ctx context.Context) ([]domain.BacklogItem, error)
	UpdateBacklogItem(ctx context.Context, item domain.BacklogItem) (domain.BacklogItem, bool, error)
	MarkBacklogItemImplemented(ctx context.Context, id domain.BacklogItemID, spawnedSessionID domain.SessionID, updatedAt time.Time) (domain.BacklogItem, bool, error)
	DeleteBacklogItemByID(ctx context.Context, id domain.BacklogItemID) (bool, error)
}

// Sessions spawns worker sessions when implementing a backlog item.
type Sessions interface {
	Spawn(ctx context.Context, cfg ports.SpawnConfig) (domain.Session, int, int, error)
}
