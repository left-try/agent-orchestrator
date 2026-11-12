import { FolderClosed } from "lucide-react";
import type { WorkspaceRepoSummary, WorkspaceSession } from "../types/workspace";

/** Compact folder context for the open chat; list and board views stay uncluttered. */
export function WorkspaceFolderMarker({ session, repos = [] }: { session: WorkspaceSession; repos?: readonly WorkspaceRepoSummary[] }) {
	const folderPath = session.workspaceFolder;
	if (!folderPath) return null;
	const repo = repos.find((candidate) => candidate.relativePath === folderPath || candidate.name === folderPath);
	const folder = repo?.name ?? folderPath.split(/[\\/]/).filter(Boolean).at(-1) ?? folderPath;
	return (
		<span aria-label={`Folder: ${folder}`} className="inline-flex max-w-28 min-w-0 items-center gap-1 rounded border border-border/70 px-1.5 text-2xs text-muted-foreground" data-testid="workspace-folder-marker" data-workspace-folder={folderPath} title={folderPath}>
			<FolderClosed aria-hidden="true" className="size-3 shrink-0" />
			<span className="truncate">{folder}</span>
		</span>
	);
}
