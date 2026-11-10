import { describe, expect, it } from "vitest";
import { filterSessionsByFolder, WORKSPACE_WIDE_FOLDER, workspaceFolderForSession } from "./workspace-folder";

const repos = [
	{ name: "api", relativePath: "services/api", repo: "acme/api" },
	{ name: "web", relativePath: "apps/web", repo: "acme/web" },
];

describe("workspace folder attribution", () => {
	it("treats sessions without explicit child-folder attribution as workspace-wide", () => {
		expect(workspaceFolderForSession({}, repos)).toEqual({ id: WORKSPACE_WIDE_FOLDER, label: "Workspace-wide" });
		expect(workspaceFolderForSession({ workspaceFolder: "unknown" }, repos)).toEqual({ id: WORKSPACE_WIDE_FOLDER, label: "Workspace-wide" });
	});

	it("uses the registered visible folder name and filters by its relative path", () => {
		const session = { workspaceFolder: "services/api" };
		expect(workspaceFolderForSession(session, repos)).toEqual({ id: "services/api", label: "api" });
		const unassigned = {};
		expect(filterSessionsByFolder([session, unassigned], "services/api", repos)).toEqual([session]);
		expect(filterSessionsByFolder([session], "all", repos)).toEqual([session]);
	});
});
