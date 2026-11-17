package backlog

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/aoagents/agent-orchestrator/backend/internal/domain"
	"github.com/aoagents/agent-orchestrator/backend/internal/httpd/apierr"
	"github.com/aoagents/agent-orchestrator/backend/internal/ports"
)

// Service manages project backlog items captured from unimplemented orchestrator discussions.
type Service struct {
	store    Store
	sessions Sessions
	newID    func() string
	now      func() time.Time
}

// Deps configures a Service.
type Deps struct {
	Store    Store
	Sessions Sessions
	NewID    func() string
	Now      func() time.Time
}

// New constructs a backlog Service.
func New(d Deps) *Service {
	if d.NewID == nil {
		d.NewID = func() string { return "bl_" + uuid.NewString() }
	}
	if d.Now == nil {
		d.Now = func() time.Time { return time.Now().UTC() }
	}
	return &Service{
		store:    d.Store,
		sessions: d.Sessions,
		newID:    d.NewID,
		now:      d.Now,
	}
}

// Create stores a new open backlog item for a project.
func (s *Service) Create(ctx context.Context, input CreateInput) (domain.BacklogItem, error) {
	if s == nil || s.store == nil {
		return domain.BacklogItem{}, errors.New("backlog: store is required")
	}
	projectID := domain.ProjectID(strings.TrimSpace(string(input.ProjectID)))
	if projectID == "" {
		return domain.BacklogItem{}, apierr.Invalid("INVALID_PROJECT_ID", "Project id is required", nil)
	}
	title := strings.TrimSpace(input.Title)
	summary := strings.TrimSpace(input.Summary)
	prompt := strings.TrimSpace(input.Prompt)
	if prompt == "" {
		if summary != "" {
			prompt = summary
		} else {
			prompt = title
		}
	}
	now := s.now().UTC()
	item := domain.BacklogItem{
		ID:                    domain.BacklogItemID(s.newID()),
		ProjectID:             projectID,
		OrchestratorSessionID: strings.TrimSpace(string(input.OrchestratorSessionID)),
		Title:                 title,
		Summary:               summary,
		Prompt:                prompt,
		Status:                domain.BacklogStatusOpen,
		CreatedAt:             now,
		UpdatedAt:             now,
	}
	if err := item.Validate(); err != nil {
		return domain.BacklogItem{}, invalidBacklogError(err)
	}
	if err := s.store.InsertBacklogItem(ctx, item); err != nil {
		return domain.BacklogItem{}, storeError(err)
	}
	return item, nil
}

// List returns backlog items, optionally scoped to one project.
func (s *Service) List(ctx context.Context, projectID domain.ProjectID) ([]domain.BacklogItem, error) {
	if s == nil || s.store == nil {
		return nil, errors.New("backlog: store is required")
	}
	if strings.TrimSpace(string(projectID)) == "" {
		return s.store.SelectAllBacklogItems(ctx)
	}
	return s.store.SelectBacklogItemsByProject(ctx, projectID)
}

// Get returns a backlog item by ID.
func (s *Service) Get(ctx context.Context, id domain.BacklogItemID) (domain.BacklogItem, error) {
	if s == nil || s.store == nil {
		return domain.BacklogItem{}, errors.New("backlog: store is required")
	}
	if strings.TrimSpace(string(id)) == "" {
		return domain.BacklogItem{}, apierr.Invalid("INVALID_BACKLOG_ID", "Backlog item id is required", nil)
	}
	item, ok, err := s.store.SelectBacklogItemByID(ctx, id)
	if err != nil {
		return domain.BacklogItem{}, storeError(err)
	}
	if !ok {
		return domain.BacklogItem{}, apierr.NotFound("BACKLOG_ITEM_NOT_FOUND", "Unknown backlog item")
	}
	return item, nil
}

// Update modifies a backlog item's title, summary, prompt, or status.
func (s *Service) Update(ctx context.Context, id domain.BacklogItemID, input UpdateInput) (domain.BacklogItem, error) {
	if s == nil || s.store == nil {
		return domain.BacklogItem{}, errors.New("backlog: store is required")
	}
	if strings.TrimSpace(string(id)) == "" {
		return domain.BacklogItem{}, apierr.Invalid("INVALID_BACKLOG_ID", "Backlog item id is required", nil)
	}
	existing, ok, err := s.store.SelectBacklogItemByID(ctx, id)
	if err != nil {
		return domain.BacklogItem{}, storeError(err)
	}
	if !ok {
		return domain.BacklogItem{}, apierr.NotFound("BACKLOG_ITEM_NOT_FOUND", "Unknown backlog item")
	}

	title := existing.Title
	if t := strings.TrimSpace(input.Title); t != "" {
		title = t
	}
	summary := existing.Summary
	if input.Summary != "" {
		summary = strings.TrimSpace(input.Summary)
	}
	prompt := existing.Prompt
	if p := strings.TrimSpace(input.Prompt); p != "" {
		prompt = p
	}
	status := existing.Status
	if input.Status != "" {
		status = input.Status
	}

	updated := domain.BacklogItem{
		ID:                    existing.ID,
		ProjectID:             existing.ProjectID,
		OrchestratorSessionID: existing.OrchestratorSessionID,
		Title:                 title,
		Summary:               summary,
		Prompt:                prompt,
		Status:                status,
		SpawnedSessionID:      existing.SpawnedSessionID,
		CreatedAt:             existing.CreatedAt,
		UpdatedAt:             s.now().UTC(),
	}
	if err := updated.Validate(); err != nil {
		return domain.BacklogItem{}, invalidBacklogError(err)
	}
	saved, ok, err := s.store.UpdateBacklogItem(ctx, updated)
	if err != nil {
		return domain.BacklogItem{}, storeError(err)
	}
	if !ok {
		return domain.BacklogItem{}, apierr.NotFound("BACKLOG_ITEM_NOT_FOUND", "Unknown backlog item")
	}
	return saved, nil
}

// Delete removes a backlog item.
func (s *Service) Delete(ctx context.Context, id domain.BacklogItemID) error {
	if s == nil || s.store == nil {
		return errors.New("backlog: store is required")
	}
	if strings.TrimSpace(string(id)) == "" {
		return apierr.Invalid("INVALID_BACKLOG_ID", "Backlog item id is required", nil)
	}
	ok, err := s.store.DeleteBacklogItemByID(ctx, id)
	if err != nil {
		return err
	}
	if !ok {
		return apierr.NotFound("BACKLOG_ITEM_NOT_FOUND", "Unknown backlog item")
	}
	return nil
}

// Implement spawns a worker session for the backlog item and marks it implemented.
func (s *Service) Implement(ctx context.Context, id domain.BacklogItemID, input ImplementInput) (ImplementResult, error) {
	if s == nil || s.store == nil {
		return ImplementResult{}, errors.New("backlog: store is required")
	}
	if s.sessions == nil {
		return ImplementResult{}, errors.New("backlog: sessions are required")
	}
	if strings.TrimSpace(string(id)) == "" {
		return ImplementResult{}, apierr.Invalid("INVALID_BACKLOG_ID", "Backlog item id is required", nil)
	}
	item, ok, err := s.store.SelectBacklogItemByID(ctx, id)
	if err != nil {
		return ImplementResult{}, storeError(err)
	}
	if !ok {
		return ImplementResult{}, apierr.NotFound("BACKLOG_ITEM_NOT_FOUND", "Unknown backlog item")
	}

	prompt := strings.TrimSpace(item.Prompt)
	if prompt == "" {
		prompt = strings.TrimSpace(item.Summary)
	}
	if prompt == "" {
		prompt = strings.TrimSpace(item.Title)
	}
	displayName := strings.TrimSpace(item.Title)
	if len(displayName) > 100 {
		displayName = strings.TrimSpace(displayName[:100])
	}

	spawned, _, _, err := s.sessions.Spawn(ctx, ports.SpawnConfig{
		ProjectID:           item.ProjectID,
		ParentSessionID:     domain.SessionID(strings.TrimSpace(item.OrchestratorSessionID)),
		Kind:                domain.KindWorker,
		Harness:             input.Harness,
		AgentConfig:         ports.AgentConfig{Model: strings.TrimSpace(input.Model)},
		RequestedMode:       input.InterfaceMode,
		DisplayName:         displayName,
		DisplayNameExplicit: displayName != "",
		KeepDisplayName:     displayName != "",
		Prompt:              prompt,
		Async:               input.Async,
	})
	if err != nil {
		return ImplementResult{}, err
	}

	updated, ok, err := s.store.MarkBacklogItemImplemented(ctx, item.ID, spawned.ID, s.now().UTC())
	if err != nil {
		return ImplementResult{}, err
	}
	if !ok {
		updated = item
		updated.Status = domain.BacklogStatusImplemented
		updated.SpawnedSessionID = string(spawned.ID)
	}
	return ImplementResult{Item: updated, Session: spawned}, nil
}

func storeError(err error) error {
	if errors.Is(err, domain.ErrProjectUnknown) {
		return apierr.NotFound("PROJECT_NOT_FOUND", "Unknown project")
	}
	return err
}

func invalidBacklogError(err error) *apierr.Error {
	switch {
	case errors.Is(err, domain.ErrInvalidBacklogTitle):
		return apierr.Invalid("INVALID_BACKLOG_TITLE", "Backlog title is required and must be at most 160 bytes", nil)
	case errors.Is(err, domain.ErrInvalidBacklogSummary):
		return apierr.Invalid("INVALID_BACKLOG_SUMMARY", "Backlog summary must be at most 4096 bytes", nil)
	case errors.Is(err, domain.ErrInvalidBacklogPrompt):
		return apierr.Invalid("INVALID_BACKLOG_PROMPT", "Backlog prompt must be at most 16384 bytes", nil)
	case errors.Is(err, domain.ErrInvalidBacklogStatus):
		return apierr.Invalid("INVALID_BACKLOG_STATUS", "Backlog status must be open or implemented", nil)
	default:
		return apierr.Invalid("INVALID_BACKLOG_ITEM", err.Error(), nil)
	}
}
