import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { ArrowDown, ArrowUp, RotateCcw } from "@/components/app-icons";
import {
	defaultKanbanSettings,
	normalizeKanbanSettings,
	useUpdateKanbanSettings,
	type KanbanColumnId,
	type KanbanSettings,
} from "../hooks/useSettings";
import { getKanbanColumnView } from "../lib/session-presentation";
import { Button } from "./ui/button";
import { Input } from "./ui/input";
import { Switch } from "./ui/switch";
import {
	Dialog,
	DialogContent,
	DialogDescription,
	DialogFooter,
	DialogHeader,
	DialogTitle,
	settingsDialogBodyClass,
	settingsDialogContentClass,
	settingsDialogFooterClass,
	settingsDialogHeaderClass,
} from "./ui/dialog";

export function KanbanCustomizeDialog({
	open,
	onOpenChange,
	settings,
	hostId,
}: {
	open: boolean;
	onOpenChange: (open: boolean) => void;
	settings: KanbanSettings;
	hostId?: string;
}) {
	const { t } = useTranslation();
	const { update, saving, error, reset } = useUpdateKanbanSettings(hostId);
	const [draft, setDraft] = useState<KanbanSettings>(() => normalizeKanbanSettings(settings));

	useEffect(() => {
		if (!open) return;
		reset();
		setDraft(normalizeKanbanSettings(settings));
	}, [open, settings, reset]);

	const enabledCount = draft.columns.filter((col) => col.enabled).length;

	const toggleColumn = (id: KanbanColumnId, enabled: boolean) => {
		setDraft((current) => ({
			...current,
			columns: current.columns.map((col) => (col.id === id ? { ...col, enabled } : col)),
		}));
	};

	const setColumnLabel = (id: KanbanColumnId, label: string) => {
		setDraft((current) => ({
			...current,
			columns: current.columns.map((col) => (col.id === id ? { ...col, label } : col)),
		}));
	};

	const moveColumn = (index: number, delta: -1 | 1) => {
		const target = index + delta;
		if (target < 0 || target >= draft.columns.length) return;
		setDraft((current) => {
			const next = [...current.columns];
			const [moved] = next.splice(index, 1);
			next.splice(target, 0, moved);
			return { ...current, columns: next };
		});
	};

	const handleSave = async () => {
		if (enabledCount === 0) return;
		await update(draft);
		onOpenChange(false);
	};

	return (
		<Dialog open={open} onOpenChange={onOpenChange}>
			<DialogContent
				className={settingsDialogContentClass}
				data-testid="kanban-customize-dialog"
			>
				<DialogHeader className={settingsDialogHeaderClass}>
					<DialogTitle>{t("kanban.customizeTitle")}</DialogTitle>
					<DialogDescription>{t("kanban.customizeDescription")}</DialogDescription>
				</DialogHeader>

				<div className={settingsDialogBodyClass}>
					<div className="flex items-start justify-between gap-4 rounded-md border border-border bg-surface px-3.5 py-3">
						<div className="min-w-0 flex-1">
							<div className="text-sm font-medium text-foreground">
								{t("kanban.moveMergedToDone")}
							</div>
							<p className="mt-0.5 text-xs text-muted-foreground">
								{t("kanban.moveMergedToDoneHint")}
							</p>
						</div>
						<Switch
							aria-label={t("kanban.moveMergedToDone")}
							checked={draft.moveMergedToDone}
							className="mt-0.5"
							data-testid="kanban-move-merged-switch"
							onCheckedChange={(checked) =>
								setDraft((current) => ({ ...current, moveMergedToDone: checked }))
							}
						/>
					</div>

					<div className="flex flex-col gap-2">
						<div>
							<div className="text-sm font-medium text-foreground">
								{t("kanban.columnsHeading")}
							</div>
							<p className="mt-0.5 text-xs text-muted-foreground">
								{t("kanban.columnsHint")}
							</p>
						</div>

						<div className="flex flex-col divide-y divide-border rounded-md border border-border bg-surface">
							{draft.columns.map((col, index) => {
								const columnView = getKanbanColumnView(col.id, t);
								return (
									<div
										key={col.id}
										className="flex items-center gap-3 px-3 py-2.5"
										data-testid={`kanban-column-row-${col.id}`}
									>
										<Switch
											aria-label={t("kanban.columnToggleAria", { label: columnView.label })}
											checked={col.enabled}
											data-testid={`kanban-column-toggle-${col.id}`}
											onCheckedChange={(checked) => toggleColumn(col.id, checked)}
										/>
										<span
											aria-hidden="true"
											className="size-2.5 shrink-0 rounded-full"
											style={{ backgroundColor: columnView.dot }}
										/>
										<Input
											aria-label={t("kanban.columnLabelAria", { label: columnView.label })}
											className="h-8 flex-1 text-xs"
											data-testid={`kanban-column-label-${col.id}`}
											onChange={(event) => setColumnLabel(col.id, event.target.value)}
											placeholder={columnView.label}
											value={col.label ?? ""}
										/>
										<div className="flex shrink-0 items-center gap-1">
											<Button
												aria-label={t("kanban.moveUpAria", { label: columnView.label })}
												data-testid={`kanban-column-up-${col.id}`}
												disabled={index === 0}
												onClick={() => moveColumn(index, -1)}
												size="icon-sm"
												type="button"
												variant="quiet"
											>
												<ArrowUp className="size-3.5" aria-hidden="true" />
											</Button>
											<Button
												aria-label={t("kanban.moveDownAria", { label: columnView.label })}
												data-testid={`kanban-column-down-${col.id}`}
												disabled={index === draft.columns.length - 1}
												onClick={() => moveColumn(index, 1)}
												size="icon-sm"
												type="button"
												variant="quiet"
											>
												<ArrowDown className="size-3.5" aria-hidden="true" />
											</Button>
										</div>
									</div>
								);
							})}
						</div>
					</div>

					{enabledCount === 0 ? (
						<p role="alert" className="text-xs text-destructive">
							{t("kanban.atLeastOneColumn")}
						</p>
					) : null}
					{error ? (
						<p role="alert" className="text-xs text-destructive">
							{error}
						</p>
					) : null}
				</div>

				<DialogFooter className={settingsDialogFooterClass}>
					<Button
						className="mr-auto"
						data-testid="kanban-reset-defaults"
						disabled={saving}
						onClick={() => setDraft(defaultKanbanSettings())}
						size="sm"
						type="button"
						variant="ghost"
					>
						<RotateCcw className="size-3.5" aria-hidden="true" />
						{t("kanban.resetDefaults")}
					</Button>
					<Button
						disabled={saving}
						onClick={() => onOpenChange(false)}
						size="sm"
						type="button"
						variant="ghost"
					>
						{t("kanban.cancel")}
					</Button>
					<Button
						data-testid="kanban-save-button"
						disabled={saving || enabledCount === 0}
						onClick={() => void handleSave()}
						size="sm"
						type="button"
						variant="primary"
					>
						{t("kanban.save")}
					</Button>
				</DialogFooter>
			</DialogContent>
		</Dialog>
	);
}
