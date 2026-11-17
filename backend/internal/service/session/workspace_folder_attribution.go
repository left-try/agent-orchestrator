package session

import (
	"context"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/aoagents/agent-orchestrator/backend/internal/domain"
)

const workspaceFolderCacheTTL = 15 * time.Second

type sessionTurnDiffLister interface {
	ListSessionTurnDiffs(ctx context.Context, ids []domain.SessionID) (map[domain.SessionID][]domain.ConversationTurnDiff, error)
}

type sessionWorkspaceFolderBackfiller interface {
	BackfillSessionWorkspaceFolder(ctx context.Context, id domain.SessionID, workspaceFolder string) (bool, error)
}

type workspaceFolderCacheEntry struct {
	folder     string
	repoSig    string
	updatedAt  time.Time
	terminated bool
	checkedAt  time.Time
}

type readyWorkspaceRepo struct {
	name     string
	repoSlug string
	relPath  string
}

func cleanWorkspaceFolderRel(raw string) string {
	if strings.TrimSpace(raw) == "" {
		return ""
	}
	trimmed := filepath.ToSlash(raw)
	trimmed = strings.TrimPrefix(trimmed, "./")
	trimmed = strings.Trim(trimmed, "/")
	if trimmed == "" || trimmed == "." || trimmed == ".." || strings.HasPrefix(trimmed, "../") {
		return ""
	}
	return trimmed
}

func repoSlugFromOriginURL(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	var pathPart string
	if strings.Contains(trimmed, "://") {
		u, err := url.Parse(trimmed)
		if err != nil {
			return ""
		}
		pathPart = u.Path
	} else if colon := strings.Index(trimmed, ":"); colon > 0 && !strings.Contains(trimmed[:colon], "/") {
		pathPart = trimmed[colon+1:]
	} else {
		pathPart = trimmed
	}
	pathPart = strings.Trim(pathPart, "/")
	pathPart = strings.TrimSuffix(pathPart, ".git")
	return strings.ToLower(pathPart)
}

func repoSlugFromPRURL(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return ""
	}
	pathPart := strings.Trim(u.Path, "/")
	if pathPart == "" {
		return ""
	}
	if idx := strings.Index(pathPart, "/-/merge_requests/"); idx > 0 {
		return strings.ToLower(pathPart[:idx])
	}
	if idx := strings.Index(pathPart, "/pull/"); idx > 0 {
		return strings.ToLower(pathPart[:idx])
	}
	if idx := strings.Index(pathPart, "/pulls/"); idx > 0 {
		return strings.ToLower(pathPart[:idx])
	}
	return ""
}

func readyWorkspaceReposForAttribution(repos []domain.WorkspaceRepoRecord) ([]readyWorkspaceRepo, string) {
	ready := make([]readyWorkspaceRepo, 0, len(repos))
	sigParts := make([]string, 0, len(repos))
	for _, repo := range repos {
		if repo.GitStatus.WithDefault() != domain.GitStatusReady {
			continue
		}
		rel := cleanWorkspaceFolderRel(repo.RelativePath)
		if rel == "" {
			continue
		}
		slug := repoSlugFromOriginURL(repo.RepoOriginURL)
		ready = append(ready, readyWorkspaceRepo{
			name:     strings.TrimSpace(repo.Name),
			repoSlug: slug,
			relPath:  rel,
		})
		sigParts = append(sigParts, rel+"="+slug)
	}
	if len(ready) == 0 {
		return nil, ""
	}
	sort.Slice(ready, func(i, j int) bool {
		if len(ready[i].relPath) != len(ready[j].relPath) {
			return len(ready[i].relPath) > len(ready[j].relPath)
		}
		return ready[i].relPath < ready[j].relPath
	})
	sort.Strings(sigParts)
	return ready, strings.Join(sigParts, "\x00")
}

func (s *Service) invalidateWorkspaceFolderCache(id domain.SessionID) {
	if s == nil {
		return
	}
	s.folderCacheMu.Lock()
	delete(s.folderCache, id)
	s.folderCacheMu.Unlock()
}

func (s *Service) cachedWorkspaceFolder(id domain.SessionID, updatedAt time.Time, terminated bool, repoSig string, now time.Time) (string, bool) {
	s.folderCacheMu.Lock()
	defer s.folderCacheMu.Unlock()
	if s.folderCache == nil {
		return "", false
	}
	entry, ok := s.folderCache[id]
	if !ok || entry.repoSig != repoSig || !entry.updatedAt.Equal(updatedAt) {
		return "", false
	}
	if entry.terminated && terminated {
		return entry.folder, true
	}
	if now.Sub(entry.checkedAt) < workspaceFolderCacheTTL {
		return entry.folder, true
	}
	return "", false
}

func (s *Service) storeWorkspaceFolderCache(id domain.SessionID, updatedAt time.Time, terminated bool, repoSig, folder string, now time.Time) {
	s.folderCacheMu.Lock()
	defer s.folderCacheMu.Unlock()
	if s.folderCache == nil {
		s.folderCache = make(map[domain.SessionID]workspaceFolderCacheEntry)
	}
	s.folderCache[id] = workspaceFolderCacheEntry{
		folder:     folder,
		repoSig:    repoSig,
		updatedAt:  updatedAt,
		terminated: terminated,
		checkedAt:  now,
	}
}

type projectWorkspaceRepoSnapshot struct {
	isWorkspace bool
	readyRepos  []readyWorkspaceRepo
	repoSig     string
}

func (s *Service) loadProjectWorkspaceReposSnapshot(ctx context.Context, projectID domain.ProjectID, cache map[domain.ProjectID]projectWorkspaceRepoSnapshot) (projectWorkspaceRepoSnapshot, error) {
	if projectID == "" || s.store == nil {
		return projectWorkspaceRepoSnapshot{}, nil
	}
	if snap, ok := cache[projectID]; ok {
		return snap, nil
	}
	project, ok, err := s.store.GetProject(ctx, string(projectID))
	if err != nil {
		return projectWorkspaceRepoSnapshot{}, err
	}
	if !ok || project.Kind.WithDefault() != domain.ProjectKindWorkspace {
		snap := projectWorkspaceRepoSnapshot{}
		cache[projectID] = snap
		return snap, nil
	}
	repos, err := s.store.ListWorkspaceRepos(ctx, string(projectID))
	if err != nil {
		return projectWorkspaceRepoSnapshot{}, err
	}
	ready, sig := readyWorkspaceReposForAttribution(repos)
	snap := projectWorkspaceRepoSnapshot{
		isWorkspace: len(ready) > 0,
		readyRepos:  ready,
		repoSig:     sig,
	}
	cache[projectID] = snap
	return snap, nil
}

func (s *Service) attributeSessionRecordsWorkspaceFolders(
	ctx context.Context,
	recs []domain.SessionRecord,
	prsBySession map[domain.SessionID][]domain.PRFacts,
) []domain.SessionRecord {
	if s == nil || s.store == nil || len(recs) == 0 {
		return recs
	}
	now := s.now()
	repoCache := make(map[domain.ProjectID]projectWorkspaceRepoSnapshot)
	snaps := make([]projectWorkspaceRepoSnapshot, len(recs))
	out := make([]domain.SessionRecord, len(recs))
	copy(out, recs)

	var uncachedIDs []domain.SessionID
	needInfer := make([]bool, len(recs))
	for i, rec := range out {
		if rec.Kind == domain.KindOrchestrator || rec.Metadata.WorkspaceFolder != "" || rec.ProjectID == "" {
			continue
		}
		snap, err := s.loadProjectWorkspaceReposSnapshot(ctx, rec.ProjectID, repoCache)
		if err != nil || !snap.isWorkspace {
			continue
		}
		snaps[i] = snap
		if cached, hit := s.cachedWorkspaceFolder(rec.ID, rec.UpdatedAt, rec.IsTerminated, snap.repoSig, now); hit {
			if cached != "" {
				out[i].Metadata.WorkspaceFolder = cached
			}
			continue
		}
		needInfer[i] = true
		uncachedIDs = append(uncachedIDs, rec.ID)
	}
	if len(uncachedIDs) == 0 {
		return out
	}

	var diffsBySession map[domain.SessionID][]domain.ConversationTurnDiff
	if lister, ok := s.store.(sessionTurnDiffLister); ok {
		if diffs, err := lister.ListSessionTurnDiffs(ctx, uncachedIDs); err == nil {
			diffsBySession = diffs
		}
	}

	backfiller, canBackfill := s.store.(sessionWorkspaceFolderBackfiller)
	for i, rec := range out {
		if !needInfer[i] {
			continue
		}
		snap := snaps[i]
		inferred := s.inferSessionWorkspaceFolder(ctx, rec, snap.readyRepos, diffsBySession[rec.ID], prsBySession[rec.ID])
		s.storeWorkspaceFolderCache(rec.ID, rec.UpdatedAt, rec.IsTerminated, snap.repoSig, inferred, now)
		if inferred == "" {
			continue
		}
		out[i].Metadata.WorkspaceFolder = inferred
		if rec.IsTerminated && canBackfill {
			if _, err := backfiller.BackfillSessionWorkspaceFolder(ctx, rec.ID, inferred); err != nil && s.logger != nil {
				s.logger.Warn("backfill inferred workspace folder failed", "session", rec.ID, "folder", inferred, "err", err)
			}
		}
	}
	return out
}

func (s *Service) attributeSessionRecordWorkspaceFolder(
	ctx context.Context,
	rec domain.SessionRecord,
	prs []domain.PRFacts,
) domain.SessionRecord {
	res := s.attributeSessionRecordsWorkspaceFolders(ctx, []domain.SessionRecord{rec}, map[domain.SessionID][]domain.PRFacts{rec.ID: prs})
	return res[0]
}

func (s *Service) inferSessionWorkspaceFolder(
	ctx context.Context,
	rec domain.SessionRecord,
	readyRepos []readyWorkspaceRepo,
	turnDiffs []domain.ConversationTurnDiff,
	prs []domain.PRFacts,
) string {
	changedPaths := make(map[string]struct{})
	addPath := func(raw string) {
		rel := normalizeAttributedPath(raw, rec.Metadata.WorkspacePath)
		if rel != "" {
			changedPaths[rel] = struct{}{}
		}
	}

	for _, diff := range turnDiffs {
		for _, f := range diff.Files {
			addPath(f.Path)
			addPath(f.OldPath)
		}
	}

	// Check registered worktrees and workspace root on disk for uncommitted or
	// committed branch changes if the worktree still exists.
	worktreePathsChecked := make(map[string]struct{})
	if worktrees, err := s.store.ListSessionWorktrees(ctx, rec.ID); err == nil {
		for _, wt := range worktrees {
			wtPath := strings.TrimSpace(wt.WorktreePath)
			if wtPath == "" {
				continue
			}
			cleanWT := filepath.Clean(wtPath)
			if _, seen := worktreePathsChecked[cleanWT]; seen {
				continue
			}
			info, statErr := os.Stat(cleanWT)
			if statErr != nil || !info.IsDir() {
				continue
			}
			worktreePathsChecked[cleanWT] = struct{}{}

			prefix := ""
			if wt.RepoName != "" && wt.RepoName != domain.RootWorkspaceRepoName {
				for _, r := range readyRepos {
					if r.name == wt.RepoName {
						prefix = r.relPath
						break
					}
				}
			}
			for _, p := range collectGitChangedPaths(ctx, cleanWT, rec.Metadata.DiffBaseSHA) {
				if prefix != "" {
					addPath(prefix + "/" + p)
				} else {
					addPath(p)
				}
			}
		}
	}

	if rootPath := strings.TrimSpace(rec.Metadata.WorkspacePath); rootPath != "" {
		cleanRoot := filepath.Clean(rootPath)
		if _, seen := worktreePathsChecked[cleanRoot]; !seen {
			if info, statErr := os.Stat(cleanRoot); statErr == nil && info.IsDir() {
				for _, p := range collectGitChangedPaths(ctx, cleanRoot, rec.Metadata.DiffBaseSHA) {
					addPath(p)
				}
			}
		}
	}

	counts := make(map[string]int, len(readyRepos))
	for p := range changedPaths {
		if rel, ok := matchReadyWorkspaceRepo(p, readyRepos); ok {
			counts[rel]++
		}
	}
	if winner := strictMaxFolder(counts); winner != "" {
		return winner
	}

	// Fallback: match pull request repository slug to a unique child repo.
	if len(prs) > 0 {
		prCounts := make(map[string]int)
		for _, pr := range prs {
			prSlug := repoSlugFromPRURL(pr.URL)
			if prSlug == "" {
				continue
			}
			var matched []string
			for _, r := range readyRepos {
				if r.repoSlug != "" && r.repoSlug == prSlug {
					matched = append(matched, r.relPath)
				}
			}
			if len(matched) == 1 {
				prCounts[matched[0]]++
			}
		}
		if winner := strictMaxFolder(prCounts); winner != "" {
			return winner
		}
	}
	return ""
}

func collectGitChangedPaths(ctx context.Context, root, baseSHA string) []string {
	var paths []string
	if out, err := gitWorkspaceOutput(ctx, root, "status", "--porcelain=v1", "-z", "--untracked-files=all"); err == nil && out != "" {
		paths = append(paths, parsePorcelainZPaths(out)...)
	}
	if strings.TrimSpace(baseSHA) != "" {
		if out, err := gitWorkspaceOutput(ctx, root, "diff", "--name-only", "-z", baseSHA+"..HEAD"); err == nil && out != "" {
			for _, entry := range strings.Split(out, "\x00") {
				if entry != "" {
					paths = append(paths, entry)
				}
			}
		}
	}
	return paths
}

func parsePorcelainZPaths(raw string) []string {
	entries := strings.Split(raw, "\x00")
	var out []string
	for i := 0; i < len(entries); i++ {
		entry := entries[i]
		if len(entry) < 4 {
			continue
		}
		status := entry[:2]
		path := entry[3:]
		if path != "" {
			out = append(out, path)
		}
		if strings.ContainsAny(status, "RC") && i+1 < len(entries) {
			i++
			if entries[i] != "" {
				out = append(out, entries[i])
			}
		}
	}
	return out
}

func normalizeAttributedPath(raw, workspaceRoot string) string {
	trimmed := raw
	if strings.TrimSpace(trimmed) == "" {
		return ""
	}
	if workspaceRoot != "" && filepath.IsAbs(trimmed) {
		if rel, err := filepath.Rel(filepath.Clean(workspaceRoot), filepath.Clean(trimmed)); err == nil &&
			rel != "." && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			trimmed = rel
		}
	}
	slashed := filepath.ToSlash(trimmed)
	slashed = strings.TrimPrefix(slashed, "./")
	slashed = strings.TrimLeft(slashed, "/")
	if slashed == "" || slashed == "." || slashed == ".." || strings.HasPrefix(slashed, "../") {
		return ""
	}
	return slashed
}

func matchReadyWorkspaceRepo(relPath string, readyRepos []readyWorkspaceRepo) (string, bool) {
	// readyRepos is already sorted descending by len(relPath).
	for _, repo := range readyRepos {
		if relPath == repo.relPath || strings.HasPrefix(relPath, repo.relPath+"/") {
			return repo.relPath, true
		}
	}
	return "", false
}

func strictMaxFolder(counts map[string]int) string {
	bestFolder := ""
	bestCount := 0
	tied := false
	for folder, count := range counts {
		if count <= 0 {
			continue
		}
		if count > bestCount {
			bestFolder = folder
			bestCount = count
			tied = false
		} else if count == bestCount {
			tied = true
		}
	}
	if tied || bestCount == 0 {
		return ""
	}
	return bestFolder
}
