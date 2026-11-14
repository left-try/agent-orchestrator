import { FolderClosed } from "lucide-react";
import type { WorkspaceRepoSummary, WorkspaceSession } from "../types/workspace";

/** Compact folder context for the open chat; list and board views stay uncluttered. */
export function WorkspaceFolderMarker({ session, repos = [] }: { session: WorkspaceSession; repos?: readonly WorkspaceRepoSummary[] }) {
	const folderPaths = session.workspaceFolders ?? (session.workspaceFolder ? [session.workspaceFolder] : []);
	if (folderPaths.length === 0) return null;
	const folders = folderPaths.map((folderPath) => {
		const repo = repos.find((candidate) => candidate.relativePath === folderPath || candidate.name === folderPath);
		return { path: folderPath, name: repo?.name ?? folderPath.split(/[\\/]/).filter(Boolean).at(-1) ?? folderPath };
	});
	const label = folders.map((folder) => folder.name).join(", ");
	const path = folders.map((folder) => folder.path).join(",");
	return (
		<span aria-label={`${folders.length === 1 ? "Folder" : "Folders"}: ${label}`} className="inline-flex max-w-40 min-w-0 items-center gap-1 rounded border border-border/70 px-1.5 text-2xs text-muted-foreground" data-testid="workspace-folder-marker" data-workspace-folder={path} title={folders.map((folder) => folder.path).join("\n")}>
			<FolderClosed aria-hidden="true" className="size-3 shrink-0" />
			<span className="truncate">{label}</span>
		</span>
	);
}
