package storage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	policyFileName        = "storage-policy.json"
	defaultRetentionDays  = 14
	minRetentionDays      = 1
	maxRetentionDays      = 365
	maxWorktreesPerProjHi = 500
)

// ProjectInfo describes a registered project for storage attribution.
type ProjectInfo struct {
	ID   string
	Name string
}

// SessionInfo describes a session for active vs reclaimable worktree detection.
type SessionInfo struct {
	ID           string
	ProjectID    string
	WorktreePath string
	Terminated   bool
	UpdatedAt    time.Time
}

// MetadataProvider supplies live project and session metadata to the storage service.
type MetadataProvider interface {
	ListStorageProjects(ctx context.Context) ([]ProjectInfo, error)
	ListStorageSessions(ctx context.Context) ([]SessionInfo, error)
}

// StoragePolicy controls automatic cleanup of old, clean, terminated worktrees.
type StoragePolicy struct {
	AutoCleanupEnabled     bool `json:"autoCleanupEnabled"`
	RetentionDays          int  `json:"retentionDays"`
	MaxWorktreesPerProject int  `json:"maxWorktreesPerProject"`
}

// DefaultPolicy returns the conservative default storage policy (auto-cleanup off).
func DefaultPolicy() StoragePolicy {
	return StoragePolicy{
		AutoCleanupEnabled:     false,
		RetentionDays:          defaultRetentionDays,
		MaxWorktreesPerProject: 0,
	}
}

// ProjectStorageUsage summarizes disk usage for a single project.
type ProjectStorageUsage struct {
	ProjectID                string `json:"projectId"`
	ProjectName              string `json:"projectName"`
	WorktreesBytes           int64  `json:"worktreesBytes"`
	WorktreeCount            int    `json:"worktreeCount"`
	ActiveWorktreeCount      int    `json:"activeWorktreeCount"`
	ReclaimableWorktreeCount int    `json:"reclaimableWorktreeCount"`
	ReclaimableBytes         int64  `json:"reclaimableBytes"`
	ArtifactsBytes           int64  `json:"artifactsBytes"`
	TotalBytes               int64  `json:"totalBytes"`
}

// UsageSnapshot summarizes disk usage across the AO data directory.
type UsageSnapshot struct {
	DataDir          string                `json:"dataDir"`
	TotalBytes       int64                 `json:"totalBytes"`
	DatabaseBytes    int64                 `json:"databaseBytes"`
	WorktreesBytes   int64                 `json:"worktreesBytes"`
	WorktreeCount    int                   `json:"worktreeCount"`
	ArtifactsBytes   int64                 `json:"artifactsBytes"`
	ArtifactCount    int                   `json:"artifactCount"`
	AttachmentsBytes int64                 `json:"attachmentsBytes"`
	OtherBytes       int64                 `json:"otherBytes"`
	ReclaimableBytes int64                 `json:"reclaimableBytes"`
	Projects         []ProjectStorageUsage `json:"projects"`
	Policy           StoragePolicy         `json:"policy"`
}

// CleanupRequest scopes a manual or scheduled storage cleanup run.
type CleanupRequest struct {
	ProjectID               string `json:"projectId,omitempty"`
	IncludeArtifactPreviews bool   `json:"includeArtifactPreviews"`
	OlderThanDays           int    `json:"olderThanDays,omitempty"`
}

// CleanupResult reports what a cleanup pass removed and skipped.
type CleanupResult struct {
	RemovedWorktrees        int   `json:"removedWorktrees"`
	RemovedArtifactPreviews int   `json:"removedArtifactPreviews"`
	FreedBytes              int64 `json:"freedBytes"`
	SkippedDirty            int   `json:"skippedDirty"`
	SkippedActive           int   `json:"skippedActive"`
}

// DirtyChecker reports whether a worktree directory has uncommitted git changes.
type DirtyChecker func(ctx context.Context, dir string) (bool, error)

// Service computes disk usage under AO's data directory and safely cleans up
// unused, clean worktrees and preview caches.
type Service struct {
	dataDir    string
	metadata   MetadataProvider
	isDirty    DirtyChecker
	now        func() time.Time
	policyMu   sync.Mutex
	cleanupMu  sync.Mutex
}

// Option customizes a Service instance.
type Option func(*Service)

// WithDirtyChecker overrides git dirty-tree inspection (useful in unit tests).
func WithDirtyChecker(fn DirtyChecker) Option {
	return func(s *Service) {
		if fn != nil {
			s.isDirty = fn
		}
	}
}

// WithClock overrides the service clock (useful in unit tests).
func WithClock(fn func() time.Time) Option {
	return func(s *Service) {
		if fn != nil {
			s.now = fn
		}
	}
}

// New constructs a storage management service rooted at dataDir.
func New(dataDir string, metadata MetadataProvider, opts ...Option) *Service {
	s := &Service{
		dataDir:  filepath.Clean(dataDir),
		metadata: metadata,
		isDirty:  defaultGitDirtyChecker,
		now:      time.Now,
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// GetPolicy reads the persisted storage policy or returns defaults.
func (s *Service) GetPolicy(_ context.Context) (StoragePolicy, error) {
	s.policyMu.Lock()
	defer s.policyMu.Unlock()
	return s.readPolicyLocked()
}

// UpdatePolicy validates and atomically persists the storage policy.
func (s *Service) UpdatePolicy(_ context.Context, policy StoragePolicy) (StoragePolicy, error) {
	if policy.RetentionDays < minRetentionDays || policy.RetentionDays > maxRetentionDays {
		return StoragePolicy{}, fmt.Errorf("retentionDays must be between %d and %d", minRetentionDays, maxRetentionDays)
	}
	if policy.MaxWorktreesPerProject < 0 || policy.MaxWorktreesPerProject > maxWorktreesPerProjHi {
		return StoragePolicy{}, fmt.Errorf("maxWorktreesPerProject must be between 0 and %d", maxWorktreesPerProjHi)
	}
	s.policyMu.Lock()
	defer s.policyMu.Unlock()
	if err := os.MkdirAll(s.dataDir, 0o750); err != nil {
		return StoragePolicy{}, fmt.Errorf("create data dir: %w", err)
	}
	raw, err := json.MarshalIndent(policy, "", "  ")
	if err != nil {
		return StoragePolicy{}, fmt.Errorf("marshal storage policy: %w", err)
	}
	target := filepath.Join(s.dataDir, policyFileName)
	tmp := target + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return StoragePolicy{}, fmt.Errorf("write storage policy: %w", err)
	}
	if err := os.Rename(tmp, target); err != nil {
		_ = os.Remove(tmp)
		return StoragePolicy{}, fmt.Errorf("commit storage policy: %w", err)
	}
	return policy, nil
}

func (s *Service) readPolicyLocked() (StoragePolicy, error) {
	path := filepath.Join(s.dataDir, policyFileName)
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return DefaultPolicy(), nil
	}
	if err != nil {
		return StoragePolicy{}, fmt.Errorf("read storage policy: %w", err)
	}
	p := DefaultPolicy()
	if err := json.Unmarshal(raw, &p); err != nil {
		return DefaultPolicy(), nil
	}
	if p.RetentionDays < minRetentionDays || p.RetentionDays > maxRetentionDays {
		p.RetentionDays = defaultRetentionDays
	}
	if p.MaxWorktreesPerProject < 0 {
		p.MaxWorktreesPerProject = 0
	}
	return p, nil
}

// Usage scans dataDir and returns a breakdown by category and project.
func (s *Service) Usage(ctx context.Context) (UsageSnapshot, error) {
	policy, err := s.GetPolicy(ctx)
	if err != nil {
		return UsageSnapshot{}, err
	}
	projectNames := make(map[string]string)
	activeSessionIDs := make(map[string]bool)
	activeWorktreePaths := make(map[string]bool)
	sessionProject := make(map[string]string)

	if s.metadata != nil {
		if projects, err := s.metadata.ListStorageProjects(ctx); err == nil {
			for _, p := range projects {
				projectNames[p.ID] = p.Name
			}
		}
		if sessions, err := s.metadata.ListStorageSessions(ctx); err == nil {
			for _, sess := range sessions {
				sessionProject[sess.ID] = sess.ProjectID
				if !sess.Terminated {
					activeSessionIDs[sess.ID] = true
					if sess.WorktreePath != "" {
						activeWorktreePaths[filepath.Clean(sess.WorktreePath)] = true
					}
				}
			}
		}
	}

	snap := UsageSnapshot{
		DataDir:  s.dataDir,
		Policy:   policy,
		Projects: []ProjectStorageUsage{},
	}

	byProject := make(map[string]*ProjectStorageUsage)
	ensureProject := func(projectID string) *ProjectStorageUsage {
		if existing, ok := byProject[projectID]; ok {
			return existing
		}
		name := projectNames[projectID]
		if name == "" {
			name = projectID
		}
		entry := &ProjectStorageUsage{
			ProjectID:   projectID,
			ProjectName: name,
		}
		byProject[projectID] = entry
		return entry
	}
	for id := range projectNames {
		ensureProject(id)
	}

	// Database files (ao.db, ao.db-wal, ao.db-shm)
	for _, name := range []string{"ao.db", "ao.db-wal", "ao.db-shm"} {
		if info, err := os.Stat(filepath.Join(s.dataDir, name)); err == nil && !info.IsDir() {
			snap.DatabaseBytes += info.Size()
		}
	}

	// Worktrees: <dataDir>/worktrees/<projectID>/<entry>
	worktreesRoot := filepath.Join(s.dataDir, "worktrees")
	if projDirs, err := os.ReadDir(worktreesRoot); err == nil {
		for _, projDir := range projDirs {
			if ctx.Err() != nil {
				return UsageSnapshot{}, ctx.Err()
			}
			if !projDir.IsDir() {
				continue
			}
			projectID := projDir.Name()
			projUsage := ensureProject(projectID)
			projPath := filepath.Join(worktreesRoot, projectID)
			entries, err := os.ReadDir(projPath)
			if err != nil {
				continue
			}
			for _, entry := range entries {
				if ctx.Err() != nil {
					return UsageSnapshot{}, ctx.Err()
				}
				entryPath := filepath.Join(projPath, entry.Name())
				size := dirSize(ctx, entryPath)
				projUsage.WorktreesBytes += size
				snap.WorktreesBytes += size
				if entry.Name() == "orchestrator" {
					// Count orchestrator subdirs as active worktrees (never auto-reclaimed).
					if sub, err := os.ReadDir(entryPath); err == nil {
						for _, child := range sub {
							if child.IsDir() {
								projUsage.WorktreeCount++
								projUsage.ActiveWorktreeCount++
								snap.WorktreeCount++
							}
						}
					}
					continue
				}
				if !entry.IsDir() {
					continue
				}
				projUsage.WorktreeCount++
				snap.WorktreeCount++
				if activeSessionIDs[entry.Name()] || activeWorktreePaths[filepath.Clean(entryPath)] {
					projUsage.ActiveWorktreeCount++
				} else {
					projUsage.ReclaimableWorktreeCount++
					projUsage.ReclaimableBytes += size
					snap.ReclaimableBytes += size
				}
			}
		}
	}

	// Artifacts: <dataDir>/artifacts/<sessionID> + <dataDir>/artifact-previews
	artifactsRoot := filepath.Join(s.dataDir, "artifacts")
	if artEntries, err := os.ReadDir(artifactsRoot); err == nil {
		for _, entry := range artEntries {
			if ctx.Err() != nil {
				return UsageSnapshot{}, ctx.Err()
			}
			entryPath := filepath.Join(artifactsRoot, entry.Name())
			size := dirSize(ctx, entryPath)
			snap.ArtifactsBytes += size
			snap.ArtifactCount++
			if projID := sessionProject[entry.Name()]; projID != "" {
				ensureProject(projID).ArtifactsBytes += size
			}
		}
	}
	previewsSize := dirSize(ctx, filepath.Join(s.dataDir, "artifact-previews"))
	snap.ArtifactsBytes += previewsSize
	snap.ReclaimableBytes += previewsSize

	// Attachments
	snap.AttachmentsBytes = dirSize(ctx, filepath.Join(s.dataDir, "attachments"))

	// Other top-level entries in dataDir
	knownTop := map[string]bool{
		"ao.db": true, "ao.db-wal": true, "ao.db-shm": true,
		"worktrees": true, "artifacts": true, "artifact-previews": true, "attachments": true,
	}
	if topEntries, err := os.ReadDir(s.dataDir); err == nil {
		for _, entry := range topEntries {
			if knownTop[entry.Name()] {
				continue
			}
			snap.OtherBytes += dirSize(ctx, filepath.Join(s.dataDir, entry.Name()))
		}
	}

	snap.TotalBytes = snap.DatabaseBytes + snap.WorktreesBytes + snap.ArtifactsBytes + snap.AttachmentsBytes + snap.OtherBytes

	projects := make([]ProjectStorageUsage, 0, len(byProject))
	for _, p := range byProject {
		p.TotalBytes = p.WorktreesBytes + p.ArtifactsBytes
		projects = append(projects, *p)
	}
	sort.Slice(projects, func(i, j int) bool {
		if projects[i].TotalBytes != projects[j].TotalBytes {
			return projects[i].TotalBytes > projects[j].TotalBytes
		}
		return projects[i].ProjectName < projects[j].ProjectName
	})
	snap.Projects = projects
	return snap, nil
}

// Cleanup safely removes clean, non-active worktrees (and optionally preview caches).
// Active sessions and dirty git worktrees are never removed.
func (s *Service) Cleanup(ctx context.Context, req CleanupRequest) (CleanupResult, error) {
	s.cleanupMu.Lock()
	defer s.cleanupMu.Unlock()

	activeSessionIDs := make(map[string]bool)
	activeWorktreePaths := make(map[string]bool)
	if s.metadata != nil {
		if sessions, err := s.metadata.ListStorageSessions(ctx); err == nil {
			for _, sess := range sessions {
				if !sess.Terminated {
					activeSessionIDs[sess.ID] = true
					if sess.WorktreePath != "" {
						activeWorktreePaths[filepath.Clean(sess.WorktreePath)] = true
					}
				}
			}
		}
	}

	var cutoff time.Time
	if req.OlderThanDays > 0 {
		cutoff = s.now().Add(-time.Duration(req.OlderThanDays) * 24 * time.Hour)
	}

	var result CleanupResult
	worktreesRoot := filepath.Join(s.dataDir, "worktrees")
	if projDirs, err := os.ReadDir(worktreesRoot); err == nil {
		for _, projDir := range projDirs {
			if ctx.Err() != nil {
				return result, ctx.Err()
			}
			if !projDir.IsDir() {
				continue
			}
			projectID := projDir.Name()
			if req.ProjectID != "" && req.ProjectID != projectID {
				continue
			}
			projPath := filepath.Join(worktreesRoot, projectID)
			entries, err := os.ReadDir(projPath)
			if err != nil {
				continue
			}
			for _, entry := range entries {
				if ctx.Err() != nil {
					return result, ctx.Err()
				}
				if !entry.IsDir() || entry.Name() == "orchestrator" {
					continue
				}
				entryPath := filepath.Join(projPath, entry.Name())
				if activeSessionIDs[entry.Name()] || activeWorktreePaths[filepath.Clean(entryPath)] {
					result.SkippedActive++
					continue
				}
				if !cutoff.IsZero() {
					if info, err := entry.Info(); err == nil && info.ModTime().After(cutoff) {
						continue
					}
				}
				dirty, err := s.isDirty(ctx, entryPath)
				if err != nil || dirty {
					// Treat probe errors conservatively as dirty; never force-delete.
					result.SkippedDirty++
					continue
				}
				size := dirSize(ctx, entryPath)
				if err := os.RemoveAll(entryPath); err != nil {
					continue
				}
				result.RemovedWorktrees++
				result.FreedBytes += size
			}
		}
	}

	if req.IncludeArtifactPreviews {
		previewsRoot := filepath.Join(s.dataDir, "artifact-previews")
		if entries, err := os.ReadDir(previewsRoot); err == nil {
			for _, entry := range entries {
				if ctx.Err() != nil {
					return result, ctx.Err()
				}
				entryPath := filepath.Join(previewsRoot, entry.Name())
				size := dirSize(ctx, entryPath)
				if err := os.RemoveAll(entryPath); err != nil {
					continue
				}
				result.RemovedArtifactPreviews++
				result.FreedBytes += size
			}
		}
	}

	return result, nil
}

func dirSize(ctx context.Context, root string) int64 {
	info, err := os.Lstat(root)
	if err != nil {
		return 0
	}
	if !info.IsDir() {
		return info.Size()
	}
	var total int64
	_ = filepath.WalkDir(root, func(_ string, d fs.DirEntry, walkErr error) error {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if walkErr != nil || d.IsDir() {
			return nil
		}
		if fi, err := d.Info(); err == nil {
			total += fi.Size()
		}
		return nil
	})
	return total
}

func defaultGitDirtyChecker(ctx context.Context, dir string) (bool, error) {
	// If there is no .git file/directory, it is not a git worktree (safe to clean).
	if _, err := os.Stat(filepath.Join(dir, ".git")); errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	cmd := exec.CommandContext(ctx, "git", "-C", dir, "status", "--porcelain")
	out, err := cmd.Output()
	if err != nil {
		return true, err
	}
	return len(strings.TrimSpace(string(out))) > 0, nil
}
