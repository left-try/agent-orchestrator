package controllers

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/aoagents/agent-orchestrator/backend/internal/httpd/apispec"
	"github.com/aoagents/agent-orchestrator/backend/internal/httpd/envelope"
	storagesvc "github.com/aoagents/agent-orchestrator/backend/internal/service/storage"
)

// StorageService is the narrow daemon contract needed by StorageController.
type StorageService interface {
	Usage(ctx context.Context) (storagesvc.UsageSnapshot, error)
	UpdatePolicy(ctx context.Context, policy storagesvc.StoragePolicy) (storagesvc.StoragePolicy, error)
	Cleanup(ctx context.Context, req storagesvc.CleanupRequest) (storagesvc.CleanupResult, error)
}

// StorageController serves disk usage inspection, auto-cleanup policy, and safe cleanup routes.
type StorageController struct {
	Svc StorageService
}

// Register mounts the storage management routes on the /api/v1 router.
func (c *StorageController) Register(r chi.Router) {
	r.Get("/storage/usage", c.usage)
	r.Patch("/storage/policy", c.updatePolicy)
	r.Post("/storage/cleanup", c.cleanup)
}

func (c *StorageController) usage(w http.ResponseWriter, r *http.Request) {
	if c.Svc == nil {
		apispec.NotImplemented(w, r, http.MethodGet, "/api/v1/storage/usage")
		return
	}
	snap, err := c.Svc.Usage(r.Context())
	if err != nil {
		envelope.WriteError(w, r, err)
		return
	}
	envelope.WriteJSON(w, http.StatusOK, toStorageUsageResponse(snap))
}

func (c *StorageController) updatePolicy(w http.ResponseWriter, r *http.Request) {
	if c.Svc == nil {
		apispec.NotImplemented(w, r, http.MethodPatch, "/api/v1/storage/policy")
		return
	}
	var req UpdateStoragePolicyRequest
	if !decodeConversationBody(w, r, &req) {
		return
	}
	updated, err := c.Svc.UpdatePolicy(r.Context(), storagesvc.StoragePolicy{
		AutoCleanupEnabled:     req.AutoCleanupEnabled,
		RetentionDays:          req.RetentionDays,
		MaxWorktreesPerProject: req.MaxWorktreesPerProject,
	})
	if err != nil {
		envelope.WriteAPIError(w, r, http.StatusBadRequest, "validation", "STORAGE_POLICY_INVALID", err.Error(), nil)
		return
	}
	envelope.WriteJSON(w, http.StatusOK, StoragePolicyDTO{
		AutoCleanupEnabled:     updated.AutoCleanupEnabled,
		RetentionDays:          updated.RetentionDays,
		MaxWorktreesPerProject: updated.MaxWorktreesPerProject,
	})
}

func (c *StorageController) cleanup(w http.ResponseWriter, r *http.Request) {
	if c.Svc == nil {
		apispec.NotImplemented(w, r, http.MethodPost, "/api/v1/storage/cleanup")
		return
	}
	var req StorageCleanupRequest
	if r.Body != nil && r.ContentLength != 0 {
		if !decodeConversationBody(w, r, &req) {
			return
		}
	}
	res, err := c.Svc.Cleanup(r.Context(), storagesvc.CleanupRequest{
		ProjectID:               req.ProjectID,
		IncludeArtifactPreviews: req.IncludeArtifactPreviews,
		OlderThanDays:           req.OlderThanDays,
	})
	if err != nil {
		envelope.WriteError(w, r, err)
		return
	}
	envelope.WriteJSON(w, http.StatusOK, StorageCleanupResponse{
		RemovedWorktrees:        res.RemovedWorktrees,
		RemovedArtifactPreviews: res.RemovedArtifactPreviews,
		FreedBytes:              res.FreedBytes,
		SkippedDirty:            res.SkippedDirty,
		SkippedActive:           res.SkippedActive,
	})
}

func toStorageUsageResponse(snap storagesvc.UsageSnapshot) StorageUsageResponse {
	projects := make([]ProjectStorageUsageDTO, 0, len(snap.Projects))
	for _, p := range snap.Projects {
		projects = append(projects, ProjectStorageUsageDTO{
			ProjectID:                p.ProjectID,
			ProjectName:              p.ProjectName,
			WorktreesBytes:           p.WorktreesBytes,
			WorktreeCount:            p.WorktreeCount,
			ActiveWorktreeCount:      p.ActiveWorktreeCount,
			ReclaimableWorktreeCount: p.ReclaimableWorktreeCount,
			ReclaimableBytes:         p.ReclaimableBytes,
			ArtifactsBytes:           p.ArtifactsBytes,
			TotalBytes:               p.TotalBytes,
		})
	}
	return StorageUsageResponse{
		DataDir:          snap.DataDir,
		TotalBytes:       snap.TotalBytes,
		DatabaseBytes:    snap.DatabaseBytes,
		WorktreesBytes:   snap.WorktreesBytes,
		WorktreeCount:    snap.WorktreeCount,
		ArtifactsBytes:   snap.ArtifactsBytes,
		ArtifactCount:    snap.ArtifactCount,
		AttachmentsBytes: snap.AttachmentsBytes,
		OtherBytes:       snap.OtherBytes,
		ReclaimableBytes: snap.ReclaimableBytes,
		Projects:         projects,
		Policy: StoragePolicyDTO{
			AutoCleanupEnabled:     snap.Policy.AutoCleanupEnabled,
			RetentionDays:          snap.Policy.RetentionDays,
			MaxWorktreesPerProject: snap.Policy.MaxWorktreesPerProject,
		},
	}
}
