import { describe, expect, it } from "vitest";
import {
	filterSessionsByFolder,
	sessionMatchesWorkspaceFolder,
	WORKSPACE_WIDE_FOLDER,
	workspaceFolderForSession,
	workspaceFolderRepoForSession,
} from "./workspace-folder";

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

	it("finds matching repo by relativePath or name with workspaceFolderRepoForSession", () => {
		expect(workspaceFolderRepoForSession({ workspaceFolder: "services/api" }, repos)).toEqual(repos[0]);
		expect(workspaceFolderRepoForSession({ workspaceFolder: "api" }, repos)).toEqual(repos[0]);
		expect(workspaceFolderRepoForSession({ workspaceFolder: "apps/web" }, repos)).toEqual(repos[1]);
		expect(workspaceFolderRepoForSession({ workspaceFolder: "other" }, repos)).toBeUndefined();
		expect(workspaceFolderRepoForSession({}, repos)).toBeUndefined();
	});

	it("matches workspace folder correctly with sessionMatchesWorkspaceFolder", () => {
		const apiSession = { workspaceFolder: "services/api" };
		const webSession = { workspaceFolder: "apps/web" };
		const rootSession = {};

		expect(sessionMatchesWorkspaceFolder(apiSession, "services/api", repos)).toBe(true);
		expect(sessionMatchesWorkspaceFolder(apiSession, "api", repos)).toBe(true);
		expect(sessionMatchesWorkspaceFolder(apiSession, "apps/web", repos)).toBe(false);
		expect(sessionMatchesWorkspaceFolder(rootSession, "services/api", repos)).toBe(false);
		expect(sessionMatchesWorkspaceFolder(webSession, "apps/web", repos)).toBe(true);
	});
});

