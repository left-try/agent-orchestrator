import { useState } from "react";
import { useTranslation } from "react-i18next";
import { RefreshCw, Trash2 } from "@/components/app-icons";
import {
	formatBytes,
	useRunStorageCleanup,
	useStorageUsage,
	useUpdateStoragePolicy,
	type StorageCleanupResponse,
	type StoragePolicy,
} from "../../hooks/useStorage";
import { Button } from "../ui/button";
import { Input } from "../ui/input";
import { Switch } from "../ui/switch";
import { SettingsSection } from "./SettingsSection";

const RETENTION_OPTIONS = [7, 14, 30, 60, 90] as const;

export function StorageSettingsSection({ titleHidden = false }: { titleHidden?: boolean }) {
	const { t } = useTranslation();
	const usageQuery = useStorageUsage(true);
	const updatePolicy = useUpdateStoragePolicy();
	const runCleanup = useRunStorageCleanup();
	const [lastCleanup, setLastCleanup] = useState<StorageCleanupResponse | null>(null);
	const [actionError, setActionError] = useState<string | null>(null);

	const usage = usageQuery.data;

	const handlePolicyPatch = async (next: StoragePolicy) => {
		setActionError(null);
		try {
			await updatePolicy.mutateAsync(next);
		} catch (err) {
			setActionError(err instanceof Error ? err.message : String(err));
		}
	};

	const handleCleanup = async (projectId?: string) => {
		setActionError(null);
		try {
			const res = await runCleanup.mutateAsync({
				projectId,
				includeArtifactPreviews: !projectId,
			});
			setLastCleanup(res);
		} catch (err) {
			setActionError(err instanceof Error ? err.message : String(err));
		}
	};

	return (
		<SettingsSection
			title={t("settings.storage.title", "Storage")}
			titleHidden={titleHidden}
		>
			<div className="flex flex-col gap-4">
				<div className="flex items-center justify-between gap-3">
					<p className="text-xs text-muted-foreground">
						{t(
							"settings.storage.description",
							"Inspect disk usage for session worktrees, artifacts, and local database files, and configure automatic cleanup of unused worktrees.",
						)}
					</p>
					<Button
						data-testid="storage-refresh-button"
						disabled={usageQuery.isFetching}
						onClick={() => void usageQuery.refetch()}
						size="sm"
						type="button"
						variant="ghost"
					>
						<RefreshCw className={`size-3.5 ${usageQuery.isFetching ? "animate-spin" : ""}`} aria-hidden="true" />
						{t("common.refresh", "Refresh")}
					</Button>
				</div>

				{usageQuery.isLoading && !usage ? (
					<div className="rounded-lg border border-border bg-card/40 p-4 text-xs text-muted-foreground" role="status">
						{t("settings.storage.scanning", "Scanning local storage…")}
					</div>
				) : usageQuery.error ? (
					<div className="rounded-lg border border-destructive/40 bg-destructive/5 p-3 text-xs text-destructive" role="alert">
						{usageQuery.error instanceof Error ? usageQuery.error.message : String(usageQuery.error)}
					</div>
				) : usage ? (
					<>
						{/* Summary metrics */}
						<div className="grid grid-cols-2 gap-2.5 sm:grid-cols-4" data-testid="storage-summary-grid">
							<div className="rounded-lg border border-border bg-card/40 p-3">
								<div className="text-[11px] text-muted-foreground">
									{t("settings.storage.total", "Total usage")}
								</div>
								<div className="mt-1 text-base font-semibold tabular-nums text-foreground" data-testid="storage-total-bytes">
									{formatBytes(usage.totalBytes)}
								</div>
							</div>
							<div className="rounded-lg border border-border bg-card/40 p-3">
								<div className="text-[11px] text-muted-foreground">
									{t("settings.storage.worktrees", "Worktrees")} ({usage.worktreeCount})
								</div>
								<div className="mt-1 text-base font-semibold tabular-nums text-foreground" data-testid="storage-worktrees-bytes">
									{formatBytes(usage.worktreesBytes)}
								</div>
							</div>
							<div className="rounded-lg border border-border bg-card/40 p-3">
								<div className="text-[11px] text-muted-foreground">
									{t("settings.storage.artifacts", "Artifacts")} ({usage.artifactCount})
								</div>
								<div className="mt-1 text-base font-semibold tabular-nums text-foreground" data-testid="storage-artifacts-bytes">
									{formatBytes(usage.artifactsBytes)}
								</div>
							</div>
							<div className="rounded-lg border border-border bg-card/40 p-3">
								<div className="text-[11px] text-muted-foreground">
									{t("settings.storage.database", "Database")}
								</div>
								<div className="mt-1 text-base font-semibold tabular-nums text-foreground" data-testid="storage-database-bytes">
									{formatBytes(usage.databaseBytes)}
								</div>
							</div>
						</div>

						{/* Reclaimable banner & manual cleanup */}
						<div className="flex flex-wrap items-center justify-between gap-3 rounded-lg border border-border bg-card/40 p-3.5">
							<div className="flex flex-col gap-0.5">
								<span className="text-xs font-medium text-foreground">
									{t("settings.storage.reclaimable", "Reclaimable space")}:{" "}
									<span className="tabular-nums" data-testid="storage-reclaimable-bytes">
										{formatBytes(usage.reclaimableBytes)}
									</span>
								</span>
								<span className="text-[11px] text-muted-foreground">
									{t(
										"settings.storage.reclaimableHint",
										"Removes clean, terminated worktrees and preview caches. Active sessions and worktrees with uncommitted changes are never deleted.",
									)}
								</span>
							</div>
							<Button
								data-testid="storage-cleanup-all-button"
								disabled={runCleanup.isPending || usage.reclaimableBytes <= 0}
								onClick={() => void handleCleanup(undefined)}
								size="sm"
								type="button"
								variant="secondary"
							>
								<Trash2 className="size-3.5" aria-hidden="true" />
								{t("settings.storage.cleanNow", "Clean up unused")}
							</Button>
						</div>

						{lastCleanup ? (
							<div
								className="rounded-lg border border-border bg-muted/40 px-3 py-2 text-xs text-foreground"
								data-testid="storage-cleanup-result"
								role="status"
							>
								{t("settings.storage.cleanupSummary", "Freed {{size}} ({{worktrees}} worktrees, {{previews}} preview caches removed).", {
									size: formatBytes(lastCleanup.freedBytes),
									worktrees: lastCleanup.removedWorktrees,
									previews: lastCleanup.removedArtifactPreviews,
								})}
								{lastCleanup.skippedDirty > 0
									? ` ${t("settings.storage.skippedDirty", "{{count}} dirty worktrees were preserved.", { count: lastCleanup.skippedDirty })}`
									: ""}
							</div>
						) : null}

						{actionError ? (
							<div className="rounded-lg border border-destructive/40 bg-destructive/5 px-3 py-2 text-xs text-destructive" role="alert">
								{actionError}
							</div>
						) : null}

						{/* Auto-cleanup policy */}
						<div className="flex flex-col gap-3 rounded-lg border border-border bg-card/40 p-3.5">
							<div className="flex items-center justify-between gap-3">
								<div className="flex flex-col gap-0.5">
									<span className="text-xs font-medium text-foreground">
										{t("settings.storage.autoCleanupTitle", "Automatic worktree cleanup")}
									</span>
									<span className="text-[11px] text-muted-foreground">
										{t(
											"settings.storage.autoCleanupHint",
											"Automatically remove clean, terminated worktrees older than the retention period.",
										)}
									</span>
								</div>
								<Switch
									aria-label={t("settings.storage.autoCleanupTitle", "Automatic worktree cleanup")}
									checked={usage.policy.autoCleanupEnabled}
									data-testid="storage-auto-cleanup-switch"
									disabled={updatePolicy.isPending}
									onCheckedChange={(checked) =>
										void handlePolicyPatch({
											...usage.policy,
											autoCleanupEnabled: checked,
										})
									}
								/>
							</div>

							<div className="grid grid-cols-1 gap-3 border-t border-border/60 pt-3 sm:grid-cols-2">
								<label className="flex flex-col gap-1 text-xs">
									<span className="text-muted-foreground">
										{t("settings.storage.retentionDays", "Retention period (days)")}
									</span>
									<select
										aria-label={t("settings.storage.retentionDays", "Retention period (days)")}
										className="h-8 rounded-md border border-border bg-input px-2 text-xs text-foreground"
										data-testid="storage-retention-select"
										disabled={updatePolicy.isPending}
										onChange={(e) =>
											void handlePolicyPatch({
												...usage.policy,
												retentionDays: Number(e.target.value),
											})
										}
										value={usage.policy.retentionDays}
									>
										{RETENTION_OPTIONS.map((days) => (
											<option key={days} value={days}>
												{days} {t("settings.storage.days", "days")}
											</option>
										))}
									</select>
								</label>
								<label className="flex flex-col gap-1 text-xs">
									<span className="text-muted-foreground">
										{t("settings.storage.maxWorktrees", "Max worktrees per project (0 = unlimited)")}
									</span>
									<Input
										aria-label={t("settings.storage.maxWorktrees", "Max worktrees per project (0 = unlimited)")}
										className="h-8 text-xs tabular-nums"
										data-testid="storage-max-worktrees-input"
										disabled={updatePolicy.isPending}
										min={0}
										max={500}
										onChange={(e) => {
											const parsed = Math.max(0, Math.min(500, Number(e.target.value) || 0));
											void handlePolicyPatch({
												...usage.policy,
												maxWorktreesPerProject: parsed,
											});
										}}
										type="number"
										value={usage.policy.maxWorktreesPerProject}
									/>
								</label>
							</div>
						</div>

						{/* Per-project breakdown */}
						<div className="flex flex-col gap-2">
							<h3 className="text-xs font-medium text-foreground">
								{t("settings.storage.byProject", "Usage by project")}
							</h3>
							{usage.projects.length === 0 ? (
								<p className="text-xs text-muted-foreground">
									{t("settings.storage.noProjects", "No project storage recorded yet.")}
								</p>
							) : (
								<div className="divide-y divide-border/60 rounded-lg border border-border bg-card/40">
									{usage.projects.map((project) => (
										<div
											key={project.projectId}
											className="flex flex-wrap items-center justify-between gap-2 px-3.5 py-2.5 text-xs"
											data-testid={`storage-project-${project.projectId}`}
										>
											<div className="flex min-w-0 flex-col">
												<span className="truncate font-medium text-foreground">{project.projectName}</span>
												<span className="text-[11px] text-muted-foreground">
													{project.worktreeCount} {t("settings.storage.worktreesLower", "worktrees")} (
													{project.activeWorktreeCount} {t("settings.storage.active", "active")}
													{project.reclaimableWorktreeCount > 0
														? `, ${project.reclaimableWorktreeCount} ${t("settings.storage.reclaimableLower", "reclaimable")}`
														: ""}
													) · {t("settings.storage.artifacts", "Artifacts")}: {formatBytes(project.artifactsBytes)}
												</span>
											</div>
											<div className="flex items-center gap-2">
												<span className="font-medium tabular-nums text-foreground">
													{formatBytes(project.totalBytes)}
												</span>
												{project.reclaimableWorktreeCount > 0 ? (
													<Button
														data-testid={`storage-clean-project-${project.projectId}`}
														disabled={runCleanup.isPending}
														onClick={() => void handleCleanup(project.projectId)}
														size="sm"
														type="button"
														variant="ghost"
													>
														<Trash2 className="size-3.5" aria-hidden="true" />
														{t("settings.storage.clean", "Clean")}
													</Button>
												) : null}
											</div>
										</div>
									))}
								</div>
							)}
						</div>
					</>
				) : null}
			</div>
		</SettingsSection>
	);
}
