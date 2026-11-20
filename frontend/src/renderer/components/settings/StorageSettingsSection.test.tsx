import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { StorageSettingsSection } from "./StorageSettingsSection";

const getMock = vi.fn();
const patchMock = vi.fn();
const postMock = vi.fn();

vi.mock("../../lib/api-client", () => ({
	apiClient: {
		GET: (...args: unknown[]) => getMock(...args),
		PATCH: (...args: unknown[]) => patchMock(...args),
		POST: (...args: unknown[]) => postMock(...args),
	},
}));

function renderSection() {
	const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
	return render(
		<QueryClientProvider client={qc}>
			<StorageSettingsSection />
		</QueryClientProvider>,
	);
}

const sampleUsage = {
	dataDir: "/home/ivan/.ao/data",
	totalBytes: 2 * 1024 * 1024 * 1024,
	databaseBytes: 512 * 1024 * 1024,
	worktreesBytes: 1024 * 1024 * 1024,
	worktreeCount: 4,
	artifactsBytes: 256 * 1024 * 1024,
	artifactCount: 12,
	attachmentsBytes: 64 * 1024 * 1024,
	otherBytes: 192 * 1024 * 1024,
	reclaimableBytes: 300 * 1024 * 1024,
	projects: [
		{
			projectId: "proj-a",
			projectName: "Project Alpha",
			worktreesBytes: 1024 * 1024 * 1024,
			worktreeCount: 4,
			activeWorktreeCount: 2,
			reclaimableWorktreeCount: 2,
			reclaimableBytes: 300 * 1024 * 1024,
			artifactsBytes: 256 * 1024 * 1024,
			totalBytes: 1280 * 1024 * 1024,
		},
	],
	policy: {
		autoCleanupEnabled: false,
		retentionDays: 14,
		maxWorktreesPerProject: 0,
	},
};

describe("StorageSettingsSection", () => {
	afterEach(cleanup);
	beforeEach(() => {
		getMock.mockReset();
		patchMock.mockReset();
		postMock.mockReset();
		getMock.mockResolvedValue({ data: sampleUsage });
		patchMock.mockResolvedValue({
			data: { autoCleanupEnabled: true, retentionDays: 14, maxWorktreesPerProject: 0 },
		});
		postMock.mockResolvedValue({
			data: {
				removedWorktrees: 2,
				removedArtifactPreviews: 3,
				freedBytes: 300 * 1024 * 1024,
				skippedDirty: 1,
				skippedActive: 2,
			},
		});
	});

	it("renders storage summary and per-project breakdown", async () => {
		renderSection();
		expect(await screen.findByTestId("storage-total-bytes")).toHaveTextContent("2.00 GB");
		expect(screen.getByTestId("storage-worktrees-bytes")).toHaveTextContent("1.00 GB");
		expect(screen.getByTestId("storage-artifacts-bytes")).toHaveTextContent("256 MB");
		expect(screen.getByTestId("storage-database-bytes")).toHaveTextContent("512 MB");
		expect(screen.getByTestId("storage-project-proj-a")).toHaveTextContent("Project Alpha");
	});

	it("toggles automatic cleanup policy via PATCH /api/v1/storage/policy", async () => {
		const user = userEvent.setup();
		renderSection();
		const toggle = await screen.findByTestId("storage-auto-cleanup-switch");
		await user.click(toggle);

		await waitFor(() => {
			expect(patchMock).toHaveBeenCalledWith("/api/v1/storage/policy", {
				body: {
					autoCleanupEnabled: true,
					retentionDays: 14,
					maxWorktreesPerProject: 0,
				},
			});
		});
	});

	it("runs safe cleanup and displays freed space and preserved dirty count", async () => {
		const user = userEvent.setup();
		renderSection();
		const cleanBtn = await screen.findByTestId("storage-cleanup-all-button");
		await user.click(cleanBtn);

		await waitFor(() => {
			expect(postMock).toHaveBeenCalledWith("/api/v1/storage/cleanup", {
				body: {
					projectId: undefined,
					includeArtifactPreviews: true,
				},
			});
		});
		expect(await screen.findByTestId("storage-cleanup-result")).toHaveTextContent("300 MB");
		expect(screen.getByTestId("storage-cleanup-result")).toHaveTextContent("1 dirty worktrees were preserved");
	});
});
