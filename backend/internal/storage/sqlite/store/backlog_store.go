package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/aoagents/agent-orchestrator/backend/internal/domain"
	"github.com/aoagents/agent-orchestrator/backend/internal/storage/sqlite/gen"
)

// InsertBacklogItem persists a new project backlog item. Unknown project IDs
// report domain.ErrProjectUnknown.
func (s *Store) InsertBacklogItem(ctx context.Context, item domain.BacklogItem) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	err := s.qw.InsertBacklogItem(ctx, gen.InsertBacklogItemParams{
		ID:                    item.ID,
		ProjectID:             item.ProjectID,
		OrchestratorSessionID: item.OrchestratorSessionID,
		Title:                 item.Title,
		Summary:               item.Summary,
		Prompt:                item.Prompt,
		Status:                item.Status,
		SpawnedSessionID:      item.SpawnedSessionID,
		CreatedAt:             item.CreatedAt,
		UpdatedAt:             item.UpdatedAt,
	})
	if err != nil {
		if isSQLiteForeignKey(err) {
			return domain.ErrProjectUnknown
		}
		return fmt.Errorf("insert backlog item %s: %w", item.ID, err)
	}
	return nil
}

// SelectBacklogItemByID looks up one backlog item, reporting whether it exists.
func (s *Store) SelectBacklogItemByID(ctx context.Context, id domain.BacklogItemID) (domain.BacklogItem, bool, error) {
	row, err := s.qr.SelectBacklogItemByID(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.BacklogItem{}, false, nil
	}
	if err != nil {
		return domain.BacklogItem{}, false, fmt.Errorf("select backlog item %s: %w", id, err)
	}
	return backlogItemFromGen(row), true, nil
}

// SelectBacklogItemsByProject returns backlog items scoped to a single project.
func (s *Store) SelectBacklogItemsByProject(ctx context.Context, projectID domain.ProjectID) ([]domain.BacklogItem, error) {
	rows, err := s.qr.SelectBacklogItemsByProject(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("select backlog items for project %s: %w", projectID, err)
	}
	out := make([]domain.BacklogItem, 0, len(rows))
	for _, row := range rows {
		out = append(out, backlogItemFromGen(row))
	}
	return out, nil
}

// SelectAllBacklogItems returns backlog items across all projects.
func (s *Store) SelectAllBacklogItems(ctx context.Context) ([]domain.BacklogItem, error) {
	rows, err := s.qr.SelectAllBacklogItems(ctx)
	if err != nil {
		return nil, fmt.Errorf("select all backlog items: %w", err)
	}
	out := make([]domain.BacklogItem, 0, len(rows))
	for _, row := range rows {
		out = append(out, backlogItemFromGen(row))
	}
	return out, nil
}

// UpdateBacklogItem updates editable fields of a backlog item, reporting whether it existed.
func (s *Store) UpdateBacklogItem(ctx context.Context, item domain.BacklogItem) (domain.BacklogItem, bool, error) {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	row, err := s.qw.UpdateBacklogItem(ctx, gen.UpdateBacklogItemParams{
		Title:     item.Title,
		Summary:   item.Summary,
		Prompt:    item.Prompt,
		Status:    item.Status,
		UpdatedAt: item.UpdatedAt,
		ID:        item.ID,
	})
	if errors.Is(err, sql.ErrNoRows) {
		return domain.BacklogItem{}, false, nil
	}
	if err != nil {
		return domain.BacklogItem{}, false, fmt.Errorf("update backlog item %s: %w", item.ID, err)
	}
	return backlogItemFromGen(row), true, nil
}

// MarkBacklogItemImplemented records the worker session spawned from a backlog item.
func (s *Store) MarkBacklogItemImplemented(ctx context.Context, id domain.BacklogItemID, spawnedSessionID domain.SessionID, updatedAt time.Time) (domain.BacklogItem, bool, error) {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	row, err := s.qw.MarkBacklogItemImplemented(ctx, gen.MarkBacklogItemImplementedParams{
		SpawnedSessionID: string(spawnedSessionID),
		UpdatedAt:        updatedAt,
		ID:               id,
	})
	if errors.Is(err, sql.ErrNoRows) {
		return domain.BacklogItem{}, false, nil
	}
	if err != nil {
		return domain.BacklogItem{}, false, fmt.Errorf("mark backlog item %s implemented: %w", id, err)
	}
	return backlogItemFromGen(row), true, nil
}

// DeleteBacklogItemByID deletes a backlog item, reporting whether it existed.
func (s *Store) DeleteBacklogItemByID(ctx context.Context, id domain.BacklogItemID) (bool, error) {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	affected, err := s.qw.DeleteBacklogItemByID(ctx, id)
	if err != nil {
		return false, fmt.Errorf("delete backlog item %s: %w", id, err)
	}
	return affected > 0, nil
}

func backlogItemFromGen(row gen.BacklogItem) domain.BacklogItem {
	return domain.BacklogItem{
		ID:                    row.ID,
		ProjectID:             row.ProjectID,
		OrchestratorSessionID: row.OrchestratorSessionID,
		Title:                 row.Title,
		Summary:               row.Summary,
		Prompt:                row.Prompt,
		Status:                row.Status,
		SpawnedSessionID:      row.SpawnedSessionID,
		CreatedAt:             row.CreatedAt,
		UpdatedAt:             row.UpdatedAt,
	}
}
