import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { KanbanCustomizeDialog } from "./KanbanCustomizeDialog";
import { defaultKanbanSettings } from "../hooks/useSettings";

const { patchMock } = vi.hoisted(() => ({
	patchMock: vi.fn(),
}));

vi.mock("../lib/api-client", () => ({
	apiClient: {
		PATCH: (...args: unknown[]) => patchMock(...args),
	},
	apiErrorCode: () => undefined,
	apiErrorMessage: (_err: unknown, fallback: string) => fallback,
	getApiBaseUrl: () => "http://127.0.0.1:3001",
	subscribeApiBaseUrl: () => () => undefined,
}));

function renderWithClient(ui: React.ReactElement) {
	const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
	return render(<QueryClientProvider client={client}>{ui}</QueryClientProvider>);
}

describe("KanbanCustomizeDialog", () => {
	beforeEach(() => {
		patchMock.mockReset().mockResolvedValue({ data: { kanban: defaultKanbanSettings() }, response: { status: 200 } });
	});

	it("renders nothing when closed", () => {
		renderWithClient(
			<KanbanCustomizeDialog
				open={false}
				onOpenChange={vi.fn()}
				settings={defaultKanbanSettings()}
			/>,
		);
		expect(screen.queryByRole("dialog")).toBeNull();
	});

	it("renders customize options and columns when open", () => {
		renderWithClient(
			<KanbanCustomizeDialog
				open={true}
				onOpenChange={vi.fn()}
				settings={defaultKanbanSettings()}
			/>,
		);

		expect(screen.getByRole("dialog")).toBeInTheDocument();
		expect(screen.getByText("Customize board")).toBeInTheDocument();
		expect(screen.getByText("Move merged PRs to Done")).toBeInTheDocument();

		const switches = screen.getAllByRole("switch");
		expect(switches.length).toBeGreaterThanOrEqual(6);
	});

	it("keeps footer actions in one right-aligned row", () => {
		renderWithClient(
			<KanbanCustomizeDialog
				open={true}
				onOpenChange={vi.fn()}
				settings={defaultKanbanSettings()}
			/>,
		);

		const reset = screen.getByTestId("kanban-reset-defaults");
		const footer = reset.closest('[data-slot="dialog-footer"]');
		expect(footer).not.toBeNull();
		expect(footer).toHaveClass("flex-nowrap", "justify-end");
		expect(Array.from(footer!.children)).toEqual([
			reset,
			screen.getByTestId("kanban-cancel-button"),
			screen.getByTestId("kanban-save-button"),
		]);
		expect(reset).toHaveClass("min-w-0", "shrink");
		expect(reset.querySelector("span")).toHaveClass("truncate");
		expect(screen.getByTestId("kanban-cancel-button")).toHaveClass("shrink-0");
		expect(screen.getByTestId("kanban-save-button")).toHaveClass("shrink-0");
	});

	it("toggles moveMergedToDone and enables Backlog column, then saves", async () => {
		const user = userEvent.setup();
		const onOpenChange = vi.fn();

		renderWithClient(
			<KanbanCustomizeDialog
				open={true}
				onOpenChange={onOpenChange}
				settings={defaultKanbanSettings()}
			/>,
		);

		// Toggle moveMergedToDone
		const mergedSwitch = screen.getByTestId("kanban-move-merged-switch");
		await user.click(mergedSwitch);

		// Enable Backlog column
		const backlogToggle = screen.getByTestId("kanban-column-toggle-backlog");
		await user.click(backlogToggle);

		// Click Save
		const saveBtn = screen.getByRole("button", { name: "Save" });
		await user.click(saveBtn);

		await waitFor(() => {
			expect(patchMock).toHaveBeenCalledWith("/api/v1/settings/kanban", {
				body: expect.objectContaining({
					moveMergedToDone: false,
					columns: expect.arrayContaining([
						expect.objectContaining({ id: "backlog", enabled: true }),
					]),
				}),
			});
		});
		expect(onOpenChange).toHaveBeenCalledWith(false);
	});

	it("edits a custom column label and resets to defaults", async () => {
		const user = userEvent.setup();
		const onOpenChange = vi.fn();

		renderWithClient(
			<KanbanCustomizeDialog
				open={true}
				onOpenChange={onOpenChange}
				settings={defaultKanbanSettings()}
			/>,
		);

		const buildingInput = screen.getByTestId("kanban-column-label-building");
		await user.clear(buildingInput);
		await user.type(buildingInput, "In Progress");

		// Click Reset
		const resetBtn = screen.getByRole("button", { name: "Reset to defaults" });
		await user.click(resetBtn);

		// Save and verify defaults are restored
		const saveBtn = screen.getByRole("button", { name: "Save" });
		await user.click(saveBtn);

		await waitFor(() => {
			expect(patchMock).toHaveBeenCalledWith("/api/v1/settings/kanban", {
				body: expect.objectContaining({
					columns: expect.arrayContaining([
						expect.objectContaining({ id: "building", enabled: true }),
					]),
				}),
			});
		});
	});

	it("reorders columns using move up / move down buttons", async () => {
		const user = userEvent.setup();
		const onOpenChange = vi.fn();

		renderWithClient(
			<KanbanCustomizeDialog
				open={true}
				onOpenChange={onOpenChange}
				settings={defaultKanbanSettings()}
			/>,
		);

		// Move first column (backlog) down
		const moveDownBtn = screen.getByTestId("kanban-column-down-backlog");
		await user.click(moveDownBtn);

		// Save
		const saveBtn = screen.getByRole("button", { name: "Save" });
		await user.click(saveBtn);

		await waitFor(() => {
			expect(patchMock).toHaveBeenCalledWith("/api/v1/settings/kanban", {
				body: expect.objectContaining({
					columns: expect.arrayContaining([
						expect.objectContaining({ id: "building" }),
						expect.objectContaining({ id: "backlog" }),
					]),
				}),
			});
		});
	});

	it("renders footer actions in a horizontal row without flex-col", () => {
		renderWithClient(
			<KanbanCustomizeDialog
				open={true}
				onOpenChange={vi.fn()}
				settings={defaultKanbanSettings()}
			/>,
		);

		const footer = screen.getByTestId("kanban-save-button").closest('[data-slot="dialog-footer"]');
		expect(footer).not.toBeNull();
		expect(footer?.className).toContain("flex-row");
		expect(footer?.className).not.toContain("flex-col");
	});
});
