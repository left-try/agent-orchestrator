import { cleanup, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { AgentAccount } from "../hooks/useAgentAccounts";
import type { AgentReadiness } from "../hooks/useAgentReadinessQuery";
import { useUiStore } from "../stores/ui-store";
import { TooltipProvider } from "./ui/tooltip";
import { SidebarSubscriptionAllowance } from "./SidebarSubscriptionAllowance";

const accountsState = vi.hoisted(() => ({
	current: { data: undefined as AgentAccount[] | undefined, isLoading: true, error: null as unknown },
}));

const readinessState = vi.hoisted(() => ({
	current: { data: { agents: [] } as AgentReadiness | undefined, isLoading: false, error: null as unknown },
}));

vi.mock("../hooks/useAgentAccounts", async (importOriginal) => ({
	...(await importOriginal<typeof import("../hooks/useAgentAccounts")>()),
	agentAccountProviderNames: { claude: "Claude Code", codex: "Codex" },
	useAgentAccounts: () => accountsState.current,
}));

vi.mock("../hooks/useAgentReadinessQuery", async (importOriginal) => ({
	...(await importOriginal<typeof import("../hooks/useAgentReadinessQuery")>()),
	useAgentReadinessQuery: () => readinessState.current,
}));

function renderAllowance() {
	return render(
		<TooltipProvider>
			<SidebarSubscriptionAllowance />
		</TooltipProvider>,
	);
}

describe("SidebarSubscriptionAllowance", () => {
	afterEach(cleanup);
	beforeEach(() => {
		accountsState.current = { data: undefined, isLoading: true, error: null };
		readinessState.current = { data: { agents: [] }, isLoading: false, error: null };
		useUiStore.setState({ settingsModal: null });
	});

	it("renders a compact SVG icon trigger and shows loading inside the menu while reading limits", async () => {
		const user = userEvent.setup();
		renderAllowance();
		const trigger = screen.getByTestId("sidebar-subscription-trigger");
		expect(trigger.querySelector("svg")).not.toBeNull();

		await user.click(trigger);
		expect(screen.getByRole("status")).toHaveTextContent("Loading");
	});

	it("shows remaining allowance for Codex/Claude and status rows for all other agents", async () => {
		const user = userEvent.setup();
		accountsState.current = {
			isLoading: false,
			error: null,
			data: [
				{
					id: "codex-device",
					provider: "codex",
					default: true,
					device: true,
					signedIn: true,
					loggedIn: true,
					limits: {
						fetchedAt: new Date().toISOString(),
						windows: [
							{
								id: "session",
								label: "Session (5h)",
								usedPercent: 25,
								resetsAt: new Date(Date.now() + 2 * 60 * 60_000).toISOString(),
							},
							{ id: "weekly", label: "Weekly", usedPercent: 10 },
						],
					},
				},
				{
					id: "claude-device",
					provider: "claude",
					default: true,
					device: true,
					signedIn: true,
					loggedIn: true,
					limits: {
						fetchedAt: new Date().toISOString(),
						windows: [{ id: "weekly", label: "Weekly", usedPercent: 80 }],
					},
				},
			],
		};
		readinessState.current = {
			isLoading: false,
			error: null,
			data: {
				agents: [
					{
						id: "codex",
						name: "Codex",
						installed: true,
						authentication: { status: "authenticated" },
					},
					{
						id: "agy",
						name: "Google Antigravity",
						installed: true,
						authentication: { status: "authenticated" },
					},
					{
						id: "opencode",
						name: "OpenCode",
						installed: false,
						authentication: { status: "unauthenticated" },
					},
				] as unknown as AgentReadiness["agents"],
			},
		};

		renderAllowance();
		const trigger = screen.getByTestId("sidebar-subscription-trigger");
		expect(trigger).toHaveAttribute("aria-label", expect.stringContaining("20%"));

		await user.click(trigger);
		const rows = screen.getAllByTestId("sidebar-subscription-allowance");
		expect(rows).toHaveLength(4);
		expect(rows[0]).toHaveTextContent("Codex");
		expect(rows[0]).toHaveTextContent("75%");
		expect(rows[1]).toHaveTextContent("Claude Code");
		expect(rows[1]).toHaveTextContent("20%");
		expect(rows[2]).toHaveTextContent("Google Antigravity");
		expect(rows[2]).toHaveTextContent("Signed in · limits not reported");
		expect(rows[3]).toHaveTextContent("OpenCode");
		expect(rows[3]).toHaveTextContent("Not installed");

		await user.click(screen.getByRole("menuitem"));
		expect(useUiStore.getState().settingsModal).toMatchObject({ scope: "global", section: "harness" });
	});

	it("does not invent allowance percentages for unsupported providers", async () => {
		const user = userEvent.setup();
		accountsState.current = { data: [], isLoading: false, error: null };
		readinessState.current = {
			isLoading: false,
			error: null,
			data: {
				agents: [
					{
						id: "gemini",
						name: "Gemini CLI",
						installed: true,
						authentication: { status: "unauthenticated" },
					},
				] as unknown as AgentReadiness["agents"],
			},
		};
		renderAllowance();
		await user.click(screen.getByTestId("sidebar-subscription-trigger"));
		expect(screen.getByTestId("sidebar-subscription-allowance")).toHaveTextContent("Gemini CLI");
		expect(screen.getByTestId("sidebar-subscription-allowance")).toHaveTextContent("Sign in required");
		expect(screen.queryByText(/% left/)).not.toBeInTheDocument();
	});

	it("shows fallback message when offline with no agents", async () => {
		const user = userEvent.setup();
		accountsState.current = { data: [], isLoading: false, error: new Error("offline") };
		renderAllowance();
		await user.click(screen.getByTestId("sidebar-subscription-trigger"));
		expect(screen.getByTestId("sidebar-subscription-allowance")).toHaveTextContent("Subscription limits unavailable");
	});
});
