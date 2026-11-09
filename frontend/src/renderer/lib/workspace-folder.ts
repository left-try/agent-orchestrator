import type { WorkspaceRepoSummary, WorkspaceSession } from "../types/workspace";

export const WORKSPACE_WIDE_FOLDER = "__workspace_wide__";

/** Resolve a session's folder only from explicit attribution. Older sessions
 * and tasks without a child-repo assignment belong to the workspace itself. */
export function workspaceFolderForSession(
	session: Pick<WorkspaceSession, "workspaceFolder">,
	repos: readonly WorkspaceRepoSummary[],
): { id: string; label: string } {
	const folder = session.workspaceFolder;
	const repo = folder ? repos.find((candidate) => candidate.relativePath === folder || candidate.name === folder) : undefined;
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
