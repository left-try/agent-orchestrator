package daemon

import (
	"context"

	"github.com/aoagents/agent-orchestrator/backend/internal/domain"
	storagesvc "github.com/aoagents/agent-orchestrator/backend/internal/service/storage"
)

type storageMetadataStore interface {
	ListProjects(ctx context.Context) ([]domain.ProjectRecord, error)
	ListSessions(ctx context.Context, project domain.ProjectID) ([]domain.SessionRecord, error)
}

type storageMetadataAdapter struct {
	store storageMetadataStore
}

func (a storageMetadataAdapter) ListStorageProjects(ctx context.Context) ([]storagesvc.ProjectInfo, error) {
	projects, err := a.store.ListProjects(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]storagesvc.ProjectInfo, 0, len(projects))
	for _, p := range projects {
		name := p.DisplayName
		if name == "" {
			name = p.ID
		}
		out = append(out, storagesvc.ProjectInfo{
			ID:   p.ID,
			Name: name,
		})
	}
	return out, nil
}

func (a storageMetadataAdapter) ListStorageSessions(ctx context.Context) ([]storagesvc.SessionInfo, error) {
	sessions, err := a.store.ListSessions(ctx, "")
	if err != nil {
		return nil, err
	}
	out := make([]storagesvc.SessionInfo, 0, len(sessions))
	for _, s := range sessions {
		out = append(out, storagesvc.SessionInfo{
			ID:           string(s.ID),
			ProjectID:    string(s.ProjectID),
			WorktreePath: s.Metadata.WorkspacePath,
			Terminated:   s.IsTerminated,
			UpdatedAt:    s.UpdatedAt,
		})
	}
	return out, nil
}
