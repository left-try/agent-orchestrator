import { FolderClosed } from "lucide-react";
import type { WorkspaceRepoSummary, WorkspaceSession } from "../types/workspace";
import { workspaceFolderForSession } from "../lib/workspace-folder";

export function WorkspaceFolderMarker({ session, repos }: { session: WorkspaceSession; repos: readonly WorkspaceRepoSummary[] }) {
	if (repos.length === 0) return null;
	const folder = workspaceFolderForSession(session, repos);
	return (
		<span className="inline-flex max-w-full items-center gap-1 rounded border border-border/70 px-1.5 py-0.5 text-2xs text-muted-foreground" data-workspace-folder={folder.id} title={folder.label}>
			<FolderClosed aria-hidden="true" className="size-3 shrink-0" />
			<span className="truncate">{folder.label}</span>
		</span>
	);
}
