package controllers

import (
	"context"
	"net/http"
	"net/url"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/aoagents/agent-orchestrator/backend/internal/domain"
	"github.com/aoagents/agent-orchestrator/backend/internal/httpd/apispec"
	"github.com/aoagents/agent-orchestrator/backend/internal/httpd/envelope"
	backlogsvc "github.com/aoagents/agent-orchestrator/backend/internal/service/backlog"
)

// BacklogService is the controller-facing project backlog contract.
type BacklogService interface {
	Create(ctx context.Context, input backlogsvc.CreateInput) (domain.BacklogItem, error)
	List(ctx context.Context, projectID domain.ProjectID) ([]domain.BacklogItem, error)
	Get(ctx context.Context, id domain.BacklogItemID) (domain.BacklogItem, error)
	Update(ctx context.Context, id domain.BacklogItemID, input backlogsvc.UpdateInput) (domain.BacklogItem, error)
	Delete(ctx context.Context, id domain.BacklogItemID) error
	Implement(ctx context.Context, id domain.BacklogItemID, input backlogsvc.ImplementInput) (backlogsvc.ImplementResult, error)
}

// BacklogController owns the Kanban backlog REST routes.
type BacklogController struct {
	Svc BacklogService
}

// Register mounts the backlog REST routes.
func (c *BacklogController) Register(r chi.Router) {
	r.Get("/backlog", c.listAll)
	r.Get("/projects/{projectId}/backlog", c.listByProject)
	r.Post("/projects/{projectId}/backlog", c.create)
	r.Get("/backlog/{backlogId}", c.get)
	r.Patch("/backlog/{backlogId}", c.update)
	r.Delete("/backlog/{backlogId}", c.delete)
	r.Post("/backlog/{backlogId}/implement", c.implement)
}

func (c *BacklogController) listAll(w http.ResponseWriter, r *http.Request) {
	if c.Svc == nil {
		apispec.NotImplemented(w, r, "GET", "/api/v1/backlog")
		return
	}
	projectID := domain.ProjectID(strings.TrimSpace(r.URL.Query().Get("projectId")))
	items, err := c.Svc.List(r.Context(), projectID)
	if err != nil {
		envelope.WriteError(w, r, err)
		return
	}
	envelope.WriteJSON(w, http.StatusOK, ListBacklogResponse{Items: backlogItemResponses(items)})
}

func (c *BacklogController) listByProject(w http.ResponseWriter, r *http.Request) {
	if c.Svc == nil {
		apispec.NotImplemented(w, r, "GET", "/api/v1/projects/{projectId}/backlog")
		return
	}
	projectID := domain.ProjectID(chi.URLParam(r, "projectId"))
	items, err := c.Svc.List(r.Context(), projectID)
	if err != nil {
		envelope.WriteError(w, r, err)
		return
	}
	envelope.WriteJSON(w, http.StatusOK, ListBacklogResponse{Items: backlogItemResponses(items)})
}

func (c *BacklogController) create(w http.ResponseWriter, r *http.Request) {
	if c.Svc == nil {
		apispec.NotImplemented(w, r, "POST", "/api/v1/projects/{projectId}/backlog")
		return
	}
	var req CreateBacklogItemRequest
	if !decodeCueBody(w, r, &req, 128<<10) {
		return
	}
	projectID := domain.ProjectID(chi.URLParam(r, "projectId"))
	item, err := c.Svc.Create(r.Context(), backlogsvc.CreateInput{
		ProjectID:             projectID,
		OrchestratorSessionID: domain.SessionID(strings.TrimSpace(req.OrchestratorSessionID)),
		Title:                 req.Title,
		Summary:               req.Summary,
		Prompt:                req.Prompt,
	})
	if err != nil {
		envelope.WriteError(w, r, err)
		return
	}
	envelope.WriteJSON(w, http.StatusCreated, BacklogItemEnvelope{Item: backlogItemResponse(item)})
}

func (c *BacklogController) get(w http.ResponseWriter, r *http.Request) {
	if c.Svc == nil {
		apispec.NotImplemented(w, r, "GET", "/api/v1/backlog/{backlogId}")
		return
	}
	id, ok := parseBacklogIDParam(w, r)
	if !ok {
		return
	}
	item, err := c.Svc.Get(r.Context(), id)
	if err != nil {
		envelope.WriteError(w, r, err)
		return
	}
	envelope.WriteJSON(w, http.StatusOK, BacklogItemEnvelope{Item: backlogItemResponse(item)})
}

func (c *BacklogController) update(w http.ResponseWriter, r *http.Request) {
	if c.Svc == nil {
		apispec.NotImplemented(w, r, "PATCH", "/api/v1/backlog/{backlogId}")
		return
	}
	id, ok := parseBacklogIDParam(w, r)
	if !ok {
		return
	}
	var req UpdateBacklogItemRequest
	if !decodeCueBody(w, r, &req, 128<<10) {
		return
	}
	item, err := c.Svc.Update(r.Context(), id, backlogsvc.UpdateInput{
		Title:   req.Title,
		Summary: req.Summary,
		Prompt:  req.Prompt,
		Status:  domain.BacklogItemStatus(req.Status),
	})
	if err != nil {
		envelope.WriteError(w, r, err)
		return
	}
	envelope.WriteJSON(w, http.StatusOK, BacklogItemEnvelope{Item: backlogItemResponse(item)})
}

func (c *BacklogController) delete(w http.ResponseWriter, r *http.Request) {
	if c.Svc == nil {
		apispec.NotImplemented(w, r, "DELETE", "/api/v1/backlog/{backlogId}")
		return
	}
	id, ok := parseBacklogIDParam(w, r)
	if !ok {
		return
	}
	if err := c.Svc.Delete(r.Context(), id); err != nil {
		envelope.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (c *BacklogController) implement(w http.ResponseWriter, r *http.Request) {
	if c.Svc == nil {
		apispec.NotImplemented(w, r, "POST", "/api/v1/backlog/{backlogId}/implement")
		return
	}
	id, ok := parseBacklogIDParam(w, r)
	if !ok {
		return
	}
	var req ImplementBacklogItemRequest
	if !decodeCueBody(w, r, &req, 16<<10) {
		return
	}
	res, err := c.Svc.Implement(r.Context(), id, backlogsvc.ImplementInput{
		Harness:       domain.AgentHarness(strings.TrimSpace(req.Harness)),
		Model:         strings.TrimSpace(req.Model),
		InterfaceMode: domain.SessionMode(strings.TrimSpace(req.InterfaceMode)),
		Async:         req.Async,
	})
	if err != nil {
		envelope.WriteError(w, r, err)
		return
	}
	envelope.WriteJSON(w, http.StatusOK, ImplementBacklogItemResponse{
		Item:      backlogItemResponse(res.Item),
		SessionID: string(res.Session.ID),
	})
}

func parseBacklogIDParam(w http.ResponseWriter, r *http.Request) (domain.BacklogItemID, bool) {
	raw, err := url.PathUnescape(chi.URLParam(r, "backlogId"))
	if err != nil || strings.TrimSpace(raw) == "" {
		envelope.WriteAPIError(w, r, http.StatusBadRequest, "bad_request", "INVALID_BACKLOG_ID", "Invalid backlog item id", nil)
		return "", false
	}
	return domain.BacklogItemID(raw), true
}

func backlogItemResponses(in []domain.BacklogItem) []BacklogItemResponse {
	out := make([]BacklogItemResponse, 0, len(in))
	for _, item := range in {
		out = append(out, backlogItemResponse(item))
	}
	return out
}

func backlogItemResponse(item domain.BacklogItem) BacklogItemResponse {
	return BacklogItemResponse{
		ID:                    string(item.ID),
		ProjectID:             string(item.ProjectID),
		OrchestratorSessionID: item.OrchestratorSessionID,
		Title:                 item.Title,
		Summary:               item.Summary,
		Prompt:                item.Prompt,
		Status:                string(item.Status),
		SpawnedSessionID:      item.SpawnedSessionID,
		CreatedAt:             item.CreatedAt,
		UpdatedAt:             item.UpdatedAt,
	}
}
