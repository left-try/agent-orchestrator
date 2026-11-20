import { LoaderCircle } from "lucide-react";
import { useTranslation } from "react-i18next";
import { Gauge } from "@/components/app-icons";
import {
	agentAccountProviderFor,
	agentAccountProviderNames,
	useAgentAccounts,
	type AgentAccountLimitWindow,
} from "../hooks/useAgentAccounts";
import { useAgentReadinessQuery, type AgentReadinessSnapshot } from "../hooks/useAgentReadinessQuery";
import { formatCountdown, groupAccounts, usedPercent } from "../lib/usage-limits";
import { formatRelativeTime } from "../lib/usage-format";
import { cn } from "../lib/utils";
import { useUiStore } from "../stores/ui-store";
import {
	DropdownMenu,
	DropdownMenuContent,
	DropdownMenuItem,
	DropdownMenuSeparator,
	DropdownMenuTrigger,
} from "./ui/dropdown-menu";
import { Tooltip, TooltipContent, TooltipTrigger } from "./ui/tooltip";

interface SubscriptionSummaryRow {
	id: string;
	name: string;
	detail: string;
	reset?: string;
	remainingPercent?: number;
	checkedAt?: string;
	stale?: boolean;
}

function describeNonAccountAgent(agent: AgentReadinessSnapshot, t: ReturnType<typeof useTranslation>["t"]): string {
	if (!agent.installed) {
		return t("sidebar.agentStatusNotInstalled", "Not installed");
	}
	if (agent.authentication?.status === "authenticated") {
		return t("sidebar.agentLimitsNotReported", "Signed in · limits not reported");
	}
	if (agent.authentication?.status === "unauthenticated") {
		return t("sidebar.agentStatusSignIn", "Sign in required");
	}
	return t("sidebar.agentLimitsNotReportedShort", "Limits not reported by provider");
}

/**
 * Compact SVG icon trigger for the sidebar's bottom utility row.
 * Opens a menu showing live subscription allowances for Codex and Claude Code
 * alongside status rows for every other available agent.
 */
export function SidebarSubscriptionAllowance({
	className,
	tabIndex = 0,
}: {
	className?: string;
	tabIndex?: number;
}) {
	const { t, i18n } = useTranslation();
	const accounts = useAgentAccounts();
	const readiness = useAgentReadinessQuery();
	const openGlobalSettings = useUiStore((state) => state.openGlobalSettings);

	const grouped = groupAccounts(accounts.data ?? []);
	const accountRows: SubscriptionSummaryRow[] = grouped.flatMap(({ provider, accounts: providerAccounts }) => {
		const defaultAccount = providerAccounts.find((candidate) => candidate.default);
		if (!defaultAccount) return [];
		const providerName = agentAccountProviderNames[provider];
		if (!defaultAccount.signedIn) {
			return [{
				id: `provider-${provider}`,
				name: providerName,
				detail: t("sidebar.subscriptionAllowanceSignIn", { provider: providerName }),
			}];
		}
		const tightestWindow = defaultAccount.limits?.windows.reduce<AgentAccountLimitWindow | undefined>(
			(lowest, candidate) => {
				if (!lowest || usedPercent(candidate.usedPercent) > usedPercent(lowest.usedPercent)) return candidate;
				return lowest;
			},
			undefined,
		);
		if (!tightestWindow) {
			return [{
				id: `provider-${provider}`,
				name: providerName,
				detail: t("sidebar.providerSubscriptionAllowanceUnavailable", { provider: providerName }),
				checkedAt: defaultAccount.limits?.fetchedAt,
				stale: Boolean(defaultAccount.problem || defaultAccount.problemKind),
			}];
		}
		const remaining = 100 - usedPercent(tightestWindow.usedPercent);
		const percent = new Intl.NumberFormat(i18n.language, { style: "percent", maximumFractionDigits: 0 }).format(
			remaining / 100,
		);
		const reset = formatCountdown(tightestWindow.resetsAt, new Date(), i18n.language);
		return [{
			id: `provider-${provider}`,
			name: providerName,
			detail: t("sidebar.subscriptionAllowanceRemaining", {
				provider: providerName,
				window: tightestWindow.label,
				percent,
			}),
			reset: reset ? t("sidebar.subscriptionAllowanceReset", { value: reset }) : undefined,
			remainingPercent: remaining,
			checkedAt: defaultAccount.limits?.fetchedAt,
			stale: Boolean(defaultAccount.problem || defaultAccount.problemKind || accounts.error),
		}];
	});

	const coveredProviders = new Set(grouped.map((g) => g.provider));
	const otherAgentRows: SubscriptionSummaryRow[] = (readiness.data?.agents ?? [])
		.filter((agent) => {
			const mapped = agentAccountProviderFor(agent.id);
			return !mapped || !coveredProviders.has(mapped);
		})
		.map((agent) => ({
			id: `agent-${agent.id}`,
			name: agent.name || agent.id,
			detail: describeNonAccountAgent(agent, t),
		}));

	const allRows: SubscriptionSummaryRow[] =
		accountRows.length > 0 || otherAgentRows.length > 0
			? [...accountRows, ...otherAgentRows]
			: [{
				id: "unavailable",
				name: t("sidebar.subscriptionsHeading", "Agent subscriptions"),
				detail: accounts.error
					? t("sidebar.subscriptionAllowanceUnavailable")
					: t("sidebar.subscriptionAllowanceNoAccounts"),
			}];

	const lowestRemaining = accountRows.reduce<number | undefined>((min, row) => {
		if (row.remainingPercent === undefined) return min;
		return min === undefined ? row.remainingPercent : Math.min(min, row.remainingPercent);
	}, undefined);

	const triggerLabel =
		lowestRemaining !== undefined
			? `${t("sidebar.subscriptionsHeading", "Agent subscriptions")} (${lowestRemaining}%)`
			: t("sidebar.subscriptionsHeading", "Agent subscriptions");

	return (
		<DropdownMenu>
			<Tooltip>
				<TooltipTrigger asChild>
					<DropdownMenuTrigger asChild>
						<button
							aria-label={triggerLabel}
							className={cn(
								"inline-flex size-9 shrink-0 items-center justify-center rounded-lg text-muted-foreground hover:bg-interactive-hover hover:text-foreground focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring [&_svg]:size-icon-md",
								className,
							)}
							data-testid="sidebar-subscription-trigger"
							tabIndex={tabIndex}
							type="button"
						>
							<span className="relative z-[1] grid place-items-center">
								{accounts.isLoading && !accounts.data ? (
									<LoaderCircle
										aria-hidden="true"
										className="size-icon-sm animate-spin motion-reduce:animate-none"
									/>
								) : (
									<Gauge aria-hidden="true" />
								)}
							</span>
						</button>
					</DropdownMenuTrigger>
				</TooltipTrigger>
				<TooltipContent>{triggerLabel}</TooltipContent>
			</Tooltip>
			<DropdownMenuContent
				side="top"
				align="end"
				sideOffset={8}
				collisionPadding={8}
				className="w-72 p-1.5"
				data-testid="sidebar-subscription-menu"
			>
				<div className="px-2 py-1 text-[11px] font-medium text-muted-foreground">
					{t("sidebar.subscriptionsHeading", "Agent subscriptions")}
				</div>
				{accounts.isLoading && !accounts.data ? (
					<div className="flex items-center gap-2 px-2 py-2 text-xs text-muted-foreground" role="status">
						<LoaderCircle aria-hidden="true" className="size-3.5 animate-spin motion-reduce:animate-none" />
						<span>{t("common.loading")}</span>
					</div>
				) : (
					<div className="flex flex-col gap-1 py-0.5">
						{allRows.map((row) => {
							const checkedAt = row.checkedAt
								? row.stale
									? t("usage.limits.staleFrom", {
										value: formatRelativeTime(row.checkedAt, new Date(), i18n.language),
									})
									: t("usage.limits.checked", {
										value: formatRelativeTime(row.checkedAt, new Date(), i18n.language),
									})
								: undefined;
							return (
								<div
									key={row.id}
									className="flex flex-col gap-1 rounded-md px-2 py-1.5 text-xs"
									data-testid="sidebar-subscription-allowance"
								>
									<div className="flex items-center justify-between gap-2">
										<span className="font-medium text-foreground">{row.name}</span>
										{row.remainingPercent !== undefined ? (
											<span className="tabular-nums text-muted-foreground">
												{row.remainingPercent}%
											</span>
										) : null}
									</div>
									{row.remainingPercent !== undefined ? (
										<div className="h-1 w-full overflow-hidden rounded-full bg-muted">
											<div
												className={cn(
													"h-full rounded-full transition-all",
													row.remainingPercent <= 15
														? "bg-destructive"
														: row.remainingPercent <= 35
															? "bg-warning"
															: "bg-primary",
												)}
												style={{ width: `${Math.max(0, Math.min(100, row.remainingPercent))}%` }}
											/>
										</div>
									) : null}
									<span className="text-[11px] text-muted-foreground">
										{[row.detail, row.reset].filter(Boolean).join(" · ")}
									</span>
									{checkedAt ? (
										<span className="text-[10px] text-muted-foreground/75">{checkedAt}</span>
									) : null}
								</div>
							);
						})}
					</div>
				)}
				<DropdownMenuSeparator />
				<DropdownMenuItem onSelect={() => openGlobalSettings("harness")}>
					{t("usage.limits.openSettings")}
				</DropdownMenuItem>
			</DropdownMenuContent>
		</DropdownMenu>
	);
}
