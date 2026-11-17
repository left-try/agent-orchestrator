import type { WorkspaceRepoSummary, WorkspaceSession } from "../types/workspace";

export const WORKSPACE_WIDE_FOLDER = "__workspace_wide__";

export function workspaceFolderRepoForSession(
	session: Pick<WorkspaceSession, "workspaceFolder">,
	repos: readonly WorkspaceRepoSummary[],
): WorkspaceRepoSummary | undefined {
	const folder = session.workspaceFolder;
	if (!folder) return undefined;
	return repos.find((candidate) => candidate.relativePath === folder || candidate.name === folder);
}

export function sessionMatchesWorkspaceFolder(
	session: Pick<WorkspaceSession, "workspaceFolder">,
	folder: string | undefined,
	repos: readonly WorkspaceRepoSummary[],
): boolean {
	if (!folder) return true;
	const matched = workspaceFolderRepoForSession(session, repos);
	if (matched) return matched.relativePath === folder || matched.name === folder;
	return session.workspaceFolder === folder;
}

/** Move one item before another without mutating the current visible order. */
export function reorderWorkspaceItems(ids: readonly string[], activeId: string, overId: string): string[] | null {
	if (activeId === overId) return null;
	const from = ids.indexOf(activeId);
	const to = ids.indexOf(overId);
	if (from < 0 || to < 0) return null;
	const next = [...ids];
	const [moved] = next.splice(from, 1);
	next.splice(to, 0, moved);
	return next;
}

/** Resolve a session's folder only from explicit or inferred attribution. */
export function workspaceFolderForSession(
	session: Pick<WorkspaceSession, "workspaceFolder">,
	repos: readonly WorkspaceRepoSummary[],
): { id: string; label: string } {
	const repo = workspaceFolderRepoForSession(session, repos);
	return repo ? { id: repo.relativePath, label: repo.name } : { id: WORKSPACE_WIDE_FOLDER, label: "Workspace-wide" };
}

export function filterSessionsByFolder<T extends Pick<WorkspaceSession, "workspaceFolder">>(
	sessions: readonly T[],
	folderId: string,
	repos: readonly WorkspaceRepoSummary[],
): T[] {
	if (!folderId || folderId === "all") return [...sessions];
	return sessions.filter((session) => workspaceFolderForSession(session, repos).id === folderId);
}
