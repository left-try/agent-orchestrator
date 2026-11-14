import type { WorkspaceRepoSummary, WorkspaceSession } from "../types/workspace";

export const WORKSPACE_WIDE_FOLDER = "__workspace_wide__";

/** Resolve child folders from changed-file attribution. Older sessions without
 * the derived field fall back to their legacy explicit assignment. */
export function workspaceFoldersForSession(
	session: Pick<WorkspaceSession, "workspaceFolder" | "workspaceFolders">,
	repos: readonly WorkspaceRepoSummary[],
): Array<{ id: string; label: string }> {
	const folders = session.workspaceFolders ?? (session.workspaceFolder ? [session.workspaceFolder] : []);
	return folders.flatMap((folder) => {
		const repo = repos.find((candidate) => candidate.relativePath === folder || candidate.name === folder);
		return repo ? [{ id: repo.relativePath, label: repo.name }] : [];
	});
}

export function workspaceFolderForSession(
	session: Pick<WorkspaceSession, "workspaceFolder" | "workspaceFolders">,
	repos: readonly WorkspaceRepoSummary[],
): { id: string; label: string } {
	const folders = workspaceFoldersForSession(session, repos);
	return folders.length === 1 ? folders[0] : { id: WORKSPACE_WIDE_FOLDER, label: "Workspace-wide" };
}

export function filterSessionsByFolder<T extends Pick<WorkspaceSession, "workspaceFolder" | "workspaceFolders">>(
	sessions: readonly T[],
	folderId: string,
	repos: readonly WorkspaceRepoSummary[],
): T[] {
	if (!folderId || folderId === "all") return [...sessions];
	if (folderId === WORKSPACE_WIDE_FOLDER) return sessions.filter((session) => workspaceFoldersForSession(session, repos).length === 0);
	return sessions.filter((session) => workspaceFoldersForSession(session, repos).some((folder) => folder.id === folderId));
}
