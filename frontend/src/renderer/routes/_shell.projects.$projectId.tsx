import { createFileRoute } from "@tanstack/react-router";
import { SessionsBoard } from "../components/SessionsBoard";

type ProjectBoardSearch = { folder?: string };

export const Route = createFileRoute("/_shell/projects/$projectId")({
	validateSearch: (search: Record<string, unknown>): ProjectBoardSearch => ({
		folder: typeof search.folder === "string" && search.folder ? search.folder : undefined,
	}),
	component: ProjectBoardRoute,
});

function ProjectBoardRoute() {
	const { projectId } = Route.useParams();
	const { folder } = Route.useSearch();
	return <SessionsBoard projectId={projectId} folder={folder} />;
}
