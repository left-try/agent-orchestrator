package session

import (
	"context"
	"testing"
	"time"

	"github.com/aoagents/agent-orchestrator/backend/internal/domain"
)

type fakeAttributionStore struct {
	*fakeStore
	project       domain.ProjectRecord
	repos         []domain.WorkspaceRepoRecord
	turnDiffs     map[domain.SessionID][]domain.ConversationTurnDiff
	backfilled    map[domain.SessionID]string
	backfillCalls int
	worktrees     map[domain.SessionID][]domain.SessionWorktreeRecord
}

func (f *fakeAttributionStore) GetProject(_ context.Context, _ string) (domain.ProjectRecord, bool, error) {
	return f.project, true, nil
}

func (f *fakeAttributionStore) ListWorkspaceRepos(_ context.Context, _ string) ([]domain.WorkspaceRepoRecord, error) {
	return f.repos, nil
}

func (f *fakeAttributionStore) ListSessionTurnDiffs(_ context.Context, ids []domain.SessionID) (map[domain.SessionID][]domain.ConversationTurnDiff, error) {
	res := make(map[domain.SessionID][]domain.ConversationTurnDiff)
	for _, id := range ids {
		if diffs, ok := f.turnDiffs[id]; ok {
			res[id] = diffs
		}
	}
	return res, nil
}

func (f *fakeAttributionStore) BackfillSessionWorkspaceFolder(_ context.Context, id domain.SessionID, folder string) (bool, error) {
	if f.backfilled == nil {
		f.backfilled = make(map[domain.SessionID]string)
	}
	f.backfilled[id] = folder
	f.backfillCalls++
	return true, nil
}

func (f *fakeAttributionStore) ListSessionWorktrees(_ context.Context, id domain.SessionID) ([]domain.SessionWorktreeRecord, error) {
	return f.worktrees[id], nil
}

func TestWorkspaceFolderAttributionHelpers(t *testing.T) {
	if got := cleanWorkspaceFolderRel("  ./services/api/  "); got != "services/api" {
		t.Errorf("cleanWorkspaceFolderRel = %q, want 'services/api'", got)
	}
	if got := cleanWorkspaceFolderRel("../escape"); got != "" {
		t.Errorf("cleanWorkspaceFolderRel escape = %q, want ''", got)
	}

	originCases := []struct {
		url  string
		slug string
	}{
		{"git@github.com:acme/backend.git", "acme/backend"},
		{"https://gitlab.com/acme/frontend.git", "acme/frontend"},
		{"https://github.com/acme/microservice", "acme/microservice"},
	}
	for _, tc := range originCases {
		if got := repoSlugFromOriginURL(tc.url); got != tc.slug {
			t.Errorf("repoSlugFromOriginURL(%q) = %q, want %q", tc.url, got, tc.slug)
		}
	}

	prCases := []struct {
		url  string
		slug string
	}{
		{"https://github.com/acme/backend/pull/42", "acme/backend"},
		{"https://gitlab.com/acme/frontend/-/merge_requests/12", "acme/frontend"},
	}
	for _, tc := range prCases {
		if got := repoSlugFromPRURL(tc.url); got != tc.slug {
			t.Errorf("repoSlugFromPRURL(%q) = %q, want %q", tc.url, got, tc.slug)
		}
	}

	repos := []readyWorkspaceRepo{
		{name: "api", relPath: "services/api", repoSlug: "acme/api"},
		{name: "web", relPath: "apps/web", repoSlug: "acme/web"},
	}
	if folder, ok := matchReadyWorkspaceRepo("services/api/main.go", repos); !ok || folder != "services/api" {
		t.Errorf("matchReadyWorkspaceRepo = %v, %v", folder, ok)
	}
	if folder, ok := matchReadyWorkspaceRepo("apps/web/src/index.ts", repos); !ok || folder != "apps/web" {
		t.Errorf("matchReadyWorkspaceRepo = %v, %v", folder, ok)
	}
	if _, ok := matchReadyWorkspaceRepo("unknown/file.txt", repos); ok {
		t.Errorf("expected no match for unknown path")
	}

	// Tie returns empty
	tieCounts := map[string]int{"services/api": 2, "apps/web": 2}
	if got := strictMaxFolder(tieCounts); got != "" {
		t.Errorf("strictMaxFolder on tie = %q, want ''", got)
	}
	winnerCounts := map[string]int{"services/api": 3, "apps/web": 1}
	if got := strictMaxFolder(winnerCounts); got != "services/api" {
		t.Errorf("strictMaxFolder = %q, want 'services/api'", got)
	}
}

func TestAttributeSessionRecordsWorkspaceFoldersFromTurnDiffs(t *testing.T) {
	now := time.Now()
	store := &fakeAttributionStore{
		fakeStore: newFakeStore(),
		project: domain.ProjectRecord{
			ID:   "p-workspace",
			Kind: domain.ProjectKindWorkspace,
		},
		repos: []domain.WorkspaceRepoRecord{
			{Name: "api", RelativePath: "services/api", RepoOriginURL: "git@github.com:acme/api.git", GitStatus: domain.GitStatusReady},
			{Name: "web", RelativePath: "apps/web", RepoOriginURL: "git@github.com:acme/web.git", GitStatus: domain.GitStatusReady},
		},
		turnDiffs: map[domain.SessionID][]domain.ConversationTurnDiff{
			"sess-1": {
				{
					Files: []domain.ConversationDiffFile{
						{Path: "services/api/server.go"},
						{Path: "services/api/handler.go"},
					},
				},
			},
			"sess-2": {
				{
					Files: []domain.ConversationDiffFile{
						{Path: "apps/web/App.tsx"},
					},
				},
			},
			"sess-tie": {
				{
					Files: []domain.ConversationDiffFile{
						{Path: "services/api/model.go"},
						{Path: "apps/web/model.ts"},
					},
				},
			},
		},
	}

	svc := &Service{
		store:       store,
		clock:       func() time.Time { return now },
		folderCache: make(map[domain.SessionID]workspaceFolderCacheEntry),
	}

	recs := []domain.SessionRecord{
		{ID: "sess-1", ProjectID: "p-workspace", Kind: domain.KindWorker, IsTerminated: true, UpdatedAt: now},
		{ID: "sess-2", ProjectID: "p-workspace", Kind: domain.KindWorker, IsTerminated: false, UpdatedAt: now},
		{ID: "sess-tie", ProjectID: "p-workspace", Kind: domain.KindWorker, IsTerminated: true, UpdatedAt: now},
	}

	attributed := svc.attributeSessionRecordsWorkspaceFolders(context.Background(), recs, nil)

	if attributed[0].Metadata.WorkspaceFolder != "services/api" {
		t.Errorf("sess-1 folder = %q, want 'services/api'", attributed[0].Metadata.WorkspaceFolder)
	}
	if attributed[1].Metadata.WorkspaceFolder != "apps/web" {
		t.Errorf("sess-2 folder = %q, want 'apps/web'", attributed[1].Metadata.WorkspaceFolder)
	}
	if attributed[2].Metadata.WorkspaceFolder != "" {
		t.Errorf("sess-tie folder = %q, want ''", attributed[2].Metadata.WorkspaceFolder)
	}

	// Verify terminated sess-1 was backfilled into DB
	if store.backfilled["sess-1"] != "services/api" {
		t.Errorf("sess-1 backfilled = %q, want 'services/api'", store.backfilled["sess-1"])
	}
	// Verify live sess-2 was not backfilled into DB
	if _, ok := store.backfilled["sess-2"]; ok {
		t.Errorf("sess-2 should not be backfilled while live")
	}

	// Verify caching on second call (no extra backfills)
	backfillCountBefore := store.backfillCalls
	attributed2 := svc.attributeSessionRecordsWorkspaceFolders(context.Background(), recs, nil)
	if attributed2[0].Metadata.WorkspaceFolder != "services/api" {
		t.Errorf("sess-1 cached folder = %q, want 'services/api'", attributed2[0].Metadata.WorkspaceFolder)
	}
	if store.backfillCalls != backfillCountBefore {
		t.Errorf("expected cache hit to not invoke backfill again")
	}
}

func TestAttributeSessionRecordsWorkspaceFoldersFromPRFallback(t *testing.T) {
	now := time.Now()
	store := &fakeAttributionStore{
		fakeStore: newFakeStore(),
		project: domain.ProjectRecord{
			ID:   "p-workspace",
			Kind: domain.ProjectKindWorkspace,
		},
		repos: []domain.WorkspaceRepoRecord{
			{Name: "api", RelativePath: "services/api", RepoOriginURL: "git@github.com:acme/api.git", GitStatus: domain.GitStatusReady},
			{Name: "web", RelativePath: "apps/web", RepoOriginURL: "git@github.com:acme/web.git", GitStatus: domain.GitStatusReady},
		},
	}

	svc := &Service{
		store:       store,
		clock:       func() time.Time { return now },
		folderCache: make(map[domain.SessionID]workspaceFolderCacheEntry),
	}

	recs := []domain.SessionRecord{
		{ID: "sess-pr", ProjectID: "p-workspace", Kind: domain.KindWorker, IsTerminated: true, UpdatedAt: now},
	}
	prs := map[domain.SessionID][]domain.PRFacts{
		"sess-pr": {
			{URL: "https://github.com/acme/web/pull/101"},
		},
	}

	attributed := svc.attributeSessionRecordsWorkspaceFolders(context.Background(), recs, prs)
	if attributed[0].Metadata.WorkspaceFolder != "apps/web" {
		t.Errorf("sess-pr folder = %q, want 'apps/web'", attributed[0].Metadata.WorkspaceFolder)
	}
	if store.backfilled["sess-pr"] != "apps/web" {
		t.Errorf("sess-pr backfilled = %q, want 'apps/web'", store.backfilled["sess-pr"])
	}
}
