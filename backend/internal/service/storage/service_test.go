package storage

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

type fakeMetadata struct {
	projects []ProjectInfo
	sessions []SessionInfo
}

func (f fakeMetadata) ListStorageProjects(_ context.Context) ([]ProjectInfo, error) {
	return f.projects, nil
}

func (f fakeMetadata) ListStorageSessions(_ context.Context) ([]SessionInfo, error) {
	return f.sessions, nil
}

func writeBytes(t *testing.T, path string, n int) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
	if err := os.WriteFile(path, make([]byte, n), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func TestUsageAndSafeCleanup(t *testing.T) {
	ctx := context.Background()
	dataDir := t.TempDir()

	writeBytes(t, filepath.Join(dataDir, "ao.db"), 4096)
	writeBytes(t, filepath.Join(dataDir, "worktrees", "proj-a", "proj-a-1", "file.txt"), 1000)
	writeBytes(t, filepath.Join(dataDir, "worktrees", "proj-a", "proj-a-2", "file.txt"), 2000)
	writeBytes(t, filepath.Join(dataDir, "worktrees", "proj-a", "proj-a-dirty", "file.txt"), 3000)
	writeBytes(t, filepath.Join(dataDir, "artifacts", "proj-a-1", "report.md"), 500)
	writeBytes(t, filepath.Join(dataDir, "artifact-previews", "thumb.png"), 250)
	writeBytes(t, filepath.Join(dataDir, "attachments", "img.png"), 125)

	meta := fakeMetadata{
		projects: []ProjectInfo{{ID: "proj-a", Name: "Project Alpha"}},
		sessions: []SessionInfo{
			{ID: "proj-a-1", ProjectID: "proj-a", Terminated: false},
			{ID: "proj-a-2", ProjectID: "proj-a", Terminated: true},
			{ID: "proj-a-dirty", ProjectID: "proj-a", Terminated: true},
		},
	}

	dirtyDir := filepath.Join(dataDir, "worktrees", "proj-a", "proj-a-dirty")
	svc := New(dataDir, meta, WithDirtyChecker(func(_ context.Context, dir string) (bool, error) {
		return filepath.Clean(dir) == filepath.Clean(dirtyDir), nil
	}))

	usage, err := svc.Usage(ctx)
	if err != nil {
		t.Fatalf("Usage: %v", err)
	}
	if usage.DatabaseBytes != 4096 {
		t.Fatalf("DatabaseBytes = %d, want 4096", usage.DatabaseBytes)
	}
	if usage.WorktreesBytes != 6000 || usage.WorktreeCount != 3 {
		t.Fatalf("Worktrees = %d bytes / %d count, want 6000 / 3", usage.WorktreesBytes, usage.WorktreeCount)
	}
	if usage.ArtifactsBytes != 750 {
		t.Fatalf("ArtifactsBytes = %d, want 750", usage.ArtifactsBytes)
	}
	if usage.AttachmentsBytes != 125 {
		t.Fatalf("AttachmentsBytes = %d, want 125", usage.AttachmentsBytes)
	}
	if len(usage.Projects) != 1 || usage.Projects[0].ProjectName != "Project Alpha" {
		t.Fatalf("Projects = %+v, want Project Alpha", usage.Projects)
	}
	if usage.Projects[0].ActiveWorktreeCount != 1 || usage.Projects[0].ReclaimableWorktreeCount != 2 {
		t.Fatalf("project worktree counts = %+v, want 1 active / 2 reclaimable", usage.Projects[0])
	}

	res, err := svc.Cleanup(ctx, CleanupRequest{IncludeArtifactPreviews: true})
	if err != nil {
		t.Fatalf("Cleanup: %v", err)
	}
	if res.RemovedWorktrees != 1 || res.RemovedArtifactPreviews != 1 {
		t.Fatalf("Cleanup removed = %+v, want 1 worktree and 1 preview", res)
	}
	if res.SkippedActive != 1 || res.SkippedDirty != 1 {
		t.Fatalf("Cleanup skipped = %+v, want 1 active and 1 dirty", res)
	}
	if res.FreedBytes != 2250 {
		t.Fatalf("FreedBytes = %d, want 2250", res.FreedBytes)
	}
	// Verify dirty and active worktrees survived.
	if _, err := os.Stat(dirtyDir); err != nil {
		t.Fatalf("dirty worktree was removed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dataDir, "worktrees", "proj-a", "proj-a-1")); err != nil {
		t.Fatalf("active worktree was removed: %v", err)
	}
}

func TestPolicyPersistenceAndValidation(t *testing.T) {
	ctx := context.Background()
	svc := New(t.TempDir(), nil, WithClock(func() time.Time { return time.Unix(1700000000, 0) }))

	got, err := svc.GetPolicy(ctx)
	if err != nil {
		t.Fatalf("GetPolicy: %v", err)
	}
	if got.AutoCleanupEnabled || got.RetentionDays != defaultRetentionDays {
		t.Fatalf("default policy = %+v", got)
	}

	if _, err := svc.UpdatePolicy(ctx, StoragePolicy{AutoCleanupEnabled: true, RetentionDays: 0}); err == nil {
		t.Fatal("expected error for retentionDays=0")
	}

	want := StoragePolicy{AutoCleanupEnabled: true, RetentionDays: 30, MaxWorktreesPerProject: 10}
	updated, err := svc.UpdatePolicy(ctx, want)
	if err != nil || updated != want {
		t.Fatalf("UpdatePolicy = %+v, %v; want %+v", updated, err, want)
	}
	reloaded, err := svc.GetPolicy(ctx)
	if err != nil || reloaded != want {
		t.Fatalf("reloaded policy = %+v, %v; want %+v", reloaded, err, want)
	}
}
