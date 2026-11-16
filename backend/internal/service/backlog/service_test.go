package backlog

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/aoagents/agent-orchestrator/backend/internal/domain"
	"github.com/aoagents/agent-orchestrator/backend/internal/httpd/apierr"
	"github.com/aoagents/agent-orchestrator/backend/internal/ports"
)

type memBacklogStore struct {
	items map[domain.BacklogItemID]domain.BacklogItem
}

func newMemBacklogStore() *memBacklogStore {
	return &memBacklogStore{items: make(map[domain.BacklogItemID]domain.BacklogItem)}
}

func (s *memBacklogStore) InsertBacklogItem(_ context.Context, item domain.BacklogItem) error {
	s.items[item.ID] = item
	return nil
}

func (s *memBacklogStore) SelectBacklogItemByID(_ context.Context, id domain.BacklogItemID) (domain.BacklogItem, bool, error) {
	item, ok := s.items[id]
	return item, ok, nil
}

func (s *memBacklogStore) SelectBacklogItemsByProject(_ context.Context, projectID domain.ProjectID) ([]domain.BacklogItem, error) {
	var out []domain.BacklogItem
	for _, item := range s.items {
		if item.ProjectID == projectID {
			out = append(out, item)
		}
	}
	return out, nil
}

func (s *memBacklogStore) SelectAllBacklogItems(_ context.Context) ([]domain.BacklogItem, error) {
	var out []domain.BacklogItem
	for _, item := range s.items {
		out = append(out, item)
	}
	return out, nil
}

func (s *memBacklogStore) UpdateBacklogItem(_ context.Context, item domain.BacklogItem) (domain.BacklogItem, bool, error) {
	if _, ok := s.items[item.ID]; !ok {
		return domain.BacklogItem{}, false, nil
	}
	s.items[item.ID] = item
	return item, true, nil
}

func (s *memBacklogStore) MarkBacklogItemImplemented(_ context.Context, id domain.BacklogItemID, spawnedSessionID domain.SessionID, updatedAt time.Time) (domain.BacklogItem, bool, error) {
	item, ok := s.items[id]
	if !ok {
		return domain.BacklogItem{}, false, nil
	}
	item.Status = domain.BacklogStatusImplemented
	item.SpawnedSessionID = string(spawnedSessionID)
	item.UpdatedAt = updatedAt
	s.items[id] = item
	return item, true, nil
}

func (s *memBacklogStore) DeleteBacklogItemByID(_ context.Context, id domain.BacklogItemID) (bool, error) {
	if _, ok := s.items[id]; !ok {
		return false, nil
	}
	delete(s.items, id)
	return true, nil
}

type fakeSessions struct {
	spawned []ports.SpawnConfig
}

func (f *fakeSessions) Spawn(_ context.Context, cfg ports.SpawnConfig) (domain.Session, int, int, error) {
	f.spawned = append(f.spawned, cfg)
	return domain.Session{
		ID:        "ao-worker-42",
		ProjectID: cfg.ProjectID,
		Kind:      cfg.Kind,
	}, 0, 0, nil
}

func TestBacklogLifecycleAndImplement(t *testing.T) {
	ctx := context.Background()
	store := newMemBacklogStore()
	sessions := &fakeSessions{}
	now := time.Date(2026, 11, 15, 10, 0, 0, 0, time.UTC)
	svc := New(Deps{
		Store:    store,
		Sessions: sessions,
		NewID:    func() string { return "bl_1" },
		Now:      func() time.Time { return now },
	})

	created, err := svc.Create(ctx, CreateInput{
		ProjectID:             "ao",
		OrchestratorSessionID: "ao-orchestrator-1",
		Title:                 "Add customizable Kanban columns",
		Summary:               "Allow toggling Done and Backlog columns.",
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.Status != domain.BacklogStatusOpen {
		t.Fatalf("status = %s, want open", created.Status)
	}
	if created.Prompt != "Allow toggling Done and Backlog columns." {
		t.Fatalf("expected default prompt from summary, got %q", created.Prompt)
	}

	updated, err := svc.Update(ctx, created.ID, UpdateInput{
		Title: "Customize Kanban columns",
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Title != "Customize Kanban columns" {
		t.Fatalf("title = %q", updated.Title)
	}

	res, err := svc.Implement(ctx, created.ID, ImplementInput{
		Harness: domain.HarnessClaudeCode,
		Model:   "sonnet",
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Item.Status != domain.BacklogStatusImplemented || res.Item.SpawnedSessionID != "ao-worker-42" {
		t.Fatalf("implemented item = %+v", res.Item)
	}
	if len(sessions.spawned) != 1 || sessions.spawned[0].ParentSessionID != "ao-orchestrator-1" || sessions.spawned[0].Kind != domain.KindWorker {
		t.Fatalf("unexpected spawn configs: %+v", sessions.spawned)
	}

	if err := svc.Delete(ctx, created.ID); err != nil {
		t.Fatal(err)
	}
	var apiErr *apierr.Error
	if _, err := svc.Get(ctx, created.ID); !errors.As(err, &apiErr) || apiErr.Code != "BACKLOG_ITEM_NOT_FOUND" {
		t.Fatalf("expected BACKLOG_ITEM_NOT_FOUND, got %v", err)
	}
}
