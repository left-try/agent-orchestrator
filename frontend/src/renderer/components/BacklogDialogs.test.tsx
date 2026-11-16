import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { AddBacklogItemDialog, BacklogItemDetailDialog } from "./BacklogDialogs";
import type { BacklogItem } from "../hooks/useBacklog";

const { postMock, patchMock, deleteMock } = vi.hoisted(() => ({
	postMock: vi.fn(),
	patchMock: vi.fn(),
	deleteMock: vi.fn(),
}));

vi.mock("../lib/api-client", () => ({
	apiClient: {
		POST: (...args: unknown[]) => postMock(...args),
		PATCH: (...args: unknown[]) => patchMock(...args),
		DELETE: (...args: unknown[]) => deleteMock(...args),
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

describe("AddBacklogItemDialog", () => {
	beforeEach(() => {
		postMock.mockReset().mockResolvedValue({ data: { item: { id: "new-b-1" } }, response: { status: 200 } });
	});

	it("renders nothing when closed", () => {
		renderWithClient(
			<AddBacklogItemDialog
				open={false}
				onOpenChange={vi.fn()}
			/>,
		);
		expect(screen.queryByRole("dialog")).toBeNull();
	});

	it("submits new backlog item via API when form is filled", async () => {
		const user = userEvent.setup();
		const onOpenChange = vi.fn();

		renderWithClient(
			<AddBacklogItemDialog
				open={true}
				onOpenChange={onOpenChange}
				defaultProjectId="p1"
			/>,
		);

		const titleInput = screen.getByTestId("backlog-title-input");
		await user.type(titleInput, "Refactor database queries");

		const summaryInput = screen.getByTestId("backlog-summary-input");
		await user.type(summaryInput, "Optimize joins and indexing");

		const promptInput = screen.getByTestId("backlog-prompt-input");
		await user.type(promptInput, "Run explain analyze and add index");

		const addBtn = screen.getByTestId("backlog-submit-button");
		await user.click(addBtn);

		await waitFor(() => {
			expect(postMock).toHaveBeenCalledWith("/api/v1/projects/{projectId}/backlog", {
				params: { path: { projectId: "p1" } },
				body: {
					title: "Refactor database queries",
					summary: "Optimize joins and indexing",
					prompt: "Run explain analyze and add index",
					orchestratorSessionId: undefined,
				},
			});
		});
		expect(onOpenChange).toHaveBeenCalledWith(false);
	});
});

describe("BacklogItemDetailDialog", () => {
	beforeEach(() => {
		patchMock.mockReset().mockResolvedValue({ data: { item: {} }, response: { status: 200 } });
		deleteMock.mockReset().mockResolvedValue({ data: {}, response: { status: 200 } });
		postMock.mockReset().mockResolvedValue({ data: { session: { id: "s-spawned" } }, response: { status: 200 } });
	});

	const mockItem: BacklogItem = {
		id: "b-1",
		projectId: "p1",
		title: "Refactor auth middleware",
		summary: "Use JWT validation middleware instead of session store",
		prompt: "Optimize JWT verification",
		status: "open",
		createdAt: "2026-01-01T00:00:00Z",
		updatedAt: "2026-01-01T00:00:00Z",
	};

	it("renders nothing when open is false or item is null", () => {
		renderWithClient(
			<BacklogItemDetailDialog
				open={false}
				onOpenChange={vi.fn()}
				item={null}
			/>,
		);
		expect(screen.queryByRole("dialog")).toBeNull();
	});

	it("displays item details and allows editing and saving", async () => {
		const user = userEvent.setup();
		const onOpenChange = vi.fn();

		renderWithClient(
			<BacklogItemDetailDialog
				open={true}
				onOpenChange={onOpenChange}
				item={mockItem}
			/>,
		);

		expect(screen.getByText("Backlog item")).toBeInTheDocument();
		const titleInput = screen.getByTestId("backlog-detail-title");
		expect(titleInput).toHaveValue("Refactor auth middleware");

		await user.clear(titleInput);
		await user.type(titleInput, "Refactor auth middleware v2");

		const saveBtn = screen.getByRole("button", { name: "Save changes" });
		await user.click(saveBtn);

		await waitFor(() => {
			expect(patchMock).toHaveBeenCalledWith("/api/v1/backlog/{backlogId}", {
				params: { path: { backlogId: "b-1" } },
				body: {
					title: "Refactor auth middleware v2",
					summary: "Use JWT validation middleware instead of session store",
					prompt: "Optimize JWT verification",
					status: undefined,
				},
			});
		});
		expect(onOpenChange).toHaveBeenCalledWith(false);
	});

	it("triggers implement when Start task button is clicked", async () => {
		const user = userEvent.setup();
		const onOpenChange = vi.fn();

		renderWithClient(
			<BacklogItemDetailDialog
				open={true}
				onOpenChange={onOpenChange}
				item={mockItem}
			/>,
		);

		const startBtn = screen.getByTestId("backlog-start-task-button");
		await user.click(startBtn);

		await waitFor(() => {
			expect(postMock).toHaveBeenCalledWith("/api/v1/backlog/{backlogId}/implement", {
				params: { path: { backlogId: "b-1" } },
				body: {
					harness: undefined,
					model: undefined,
					interfaceMode: undefined,
					async: true,
				},
			});
		});
		expect(onOpenChange).toHaveBeenCalledWith(false);
	});

	it("triggers delete when Delete button is clicked", async () => {
		const user = userEvent.setup();
		const onOpenChange = vi.fn();

		renderWithClient(
			<BacklogItemDetailDialog
				open={true}
				onOpenChange={onOpenChange}
				item={mockItem}
			/>,
		);

		const deleteBtn = screen.getByTestId("backlog-delete-button");
		await user.click(deleteBtn);

		await waitFor(() => {
			expect(deleteMock).toHaveBeenCalledWith("/api/v1/backlog/{backlogId}", {
				params: { path: { backlogId: "b-1" } },
			});
		});
		expect(onOpenChange).toHaveBeenCalledWith(false);
	});
});
