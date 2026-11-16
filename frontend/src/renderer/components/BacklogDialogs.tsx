import { useEffect, useState, type FormEvent } from "react";
import { useTranslation } from "react-i18next";
import { Play, Trash2 } from "@/components/app-icons";
import {
	useCreateBacklogItem,
	useDeleteBacklogItem,
	useImplementBacklogItem,
	useUpdateBacklogItem,
	type BacklogItem,
} from "../hooks/useBacklog";
import { Button } from "./ui/button";
import { Input } from "./ui/input";
import { Label } from "./ui/label";
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

const textareaClassName =
	"flex min-h-20 w-full rounded-md border border-border bg-input/90 px-3 py-2 text-xs text-foreground placeholder:text-muted-foreground/60 focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring disabled:cursor-not-allowed disabled:opacity-50";

export function AddBacklogItemDialog({
	open,
	onOpenChange,
	defaultProjectId,
	projects,
	orchestratorSessionId,
	initialTitle = "",
	initialSummary = "",
	initialPrompt = "",
}: {
	open: boolean;
	onOpenChange: (open: boolean) => void;
	defaultProjectId?: string;
	projects?: Array<{ id: string; name: string }>;
	orchestratorSessionId?: string;
	initialTitle?: string;
	initialSummary?: string;
	initialPrompt?: string;
}) {
	const { t } = useTranslation();
	const { create, saving, error, reset } = useCreateBacklogItem();
	const [projectId, setProjectId] = useState(defaultProjectId ?? projects?.[0]?.id ?? "");
	const [title, setTitle] = useState(initialTitle ?? "");
	const [summary, setSummary] = useState(initialSummary ?? "");
	const [prompt, setPrompt] = useState(initialPrompt ?? "");

	useEffect(() => {
		if (!open) return;
		reset();
		setProjectId(defaultProjectId ?? projects?.[0]?.id ?? "");
		setTitle(initialTitle ?? "");
		setSummary(initialSummary ?? "");
		setPrompt(initialPrompt ?? "");
	}, [open, defaultProjectId, projects, initialTitle, initialSummary, initialPrompt, reset]);

	const targetProjectId = defaultProjectId ?? projectId;

	const handleSubmit = async (event: FormEvent) => {
		event.preventDefault();
		if (!targetProjectId || !title.trim()) return;
		await create({
			projectId: targetProjectId,
			orchestratorSessionId,
			title: title.trim(),
			summary: summary.trim() || undefined,
			prompt: prompt.trim() || undefined,
		});
		onOpenChange(false);
	};

	return (
		<Dialog open={open} onOpenChange={onOpenChange}>
			<DialogContent className={settingsDialogContentClass} data-testid="add-backlog-dialog">
				<form className="flex min-h-0 flex-1 flex-col" onSubmit={(event) => void handleSubmit(event)}>
					<DialogHeader className={settingsDialogHeaderClass}>
						<DialogTitle>{t("backlog.addTitle")}</DialogTitle>
						<DialogDescription>{t("backlog.addDescription")}</DialogDescription>
					</DialogHeader>

					<div className={settingsDialogBodyClass}>
						{!defaultProjectId && (projects?.length ?? 0) > 0 ? (
							<div className="flex flex-col gap-1.5">
								<Label htmlFor="backlog-project-select">{t("backlog.project")}</Label>
								<select
									className="h-9 rounded-md border border-border bg-input/90 px-3 text-xs text-foreground focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring"
									data-testid="backlog-project-select"
									id="backlog-project-select"
									onChange={(event) => setProjectId(event.target.value)}
									value={targetProjectId}
								>
									{projects?.map((project) => (
										<option key={project.id} value={project.id}>
											{project.name}
										</option>
									))}
								</select>
							</div>
						) : null}

						<div className="flex flex-col gap-1.5">
							<Label htmlFor="backlog-title-input">{t("backlog.title")}</Label>
							<Input
								autoFocus
								data-testid="backlog-title-input"
								id="backlog-title-input"
								onChange={(event) => setTitle(event.target.value)}
								placeholder={t("backlog.titlePlaceholder")}
								required
								value={title}
							/>
						</div>

						<div className="flex flex-col gap-1.5">
							<Label htmlFor="backlog-summary-input">{t("backlog.summary")}</Label>
							<textarea
								className={textareaClassName}
								data-testid="backlog-summary-input"
								id="backlog-summary-input"
								onChange={(event) => setSummary(event.target.value)}
								placeholder={t("backlog.summaryPlaceholder")}
								rows={3}
								value={summary}
							/>
						</div>

						<div className="flex flex-col gap-1.5">
							<Label htmlFor="backlog-prompt-input">{t("backlog.prompt")}</Label>
							<textarea
								className={textareaClassName}
								data-testid="backlog-prompt-input"
								id="backlog-prompt-input"
								onChange={(event) => setPrompt(event.target.value)}
								placeholder={t("backlog.promptPlaceholder")}
								rows={3}
								value={prompt}
							/>
						</div>

						{error ? (
							<p role="alert" className="text-xs text-destructive">
								{error}
							</p>
						) : null}
					</div>

					<DialogFooter className={settingsDialogFooterClass}>
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
							data-testid="backlog-submit-button"
							disabled={saving || !targetProjectId || !title.trim()}
							size="sm"
							type="submit"
							variant="primary"
						>
							{t("backlog.saveItem")}
						</Button>
					</DialogFooter>
				</form>
			</DialogContent>
		</Dialog>
	);
}

export function BacklogItemDetailDialog({
	item,
	open,
	onOpenChange,
	projectName,
	canImplement = true,
	onOpenSession,
}: {
	item: BacklogItem | null;
	open: boolean;
	onOpenChange: (open: boolean) => void;
	projectName?: string;
	canImplement?: boolean;
	onOpenSession?: (projectId: string, sessionId: string) => void;
}) {
	const { t } = useTranslation();
	const { update, saving, error: updateError, reset: resetUpdate } = useUpdateBacklogItem();
	const { remove, deleting, error: deleteError, reset: resetDelete } = useDeleteBacklogItem();
	const { implement, starting, error: implementError, reset: resetImplement } = useImplementBacklogItem();
	const [title, setTitle] = useState(item?.title ?? "");
	const [summary, setSummary] = useState(item?.summary ?? "");
	const [prompt, setPrompt] = useState(item?.prompt ?? "");

	useEffect(() => {
		if (!open || !item) return;
		resetUpdate();
		resetDelete();
		resetImplement();
		setTitle(item.title ?? "");
		setSummary(item.summary ?? "");
		setPrompt(item.prompt ?? "");
	}, [open, item, resetUpdate, resetDelete, resetImplement]);

	if (!item) return null;

	const busy = saving || deleting || starting;
	const combinedError = updateError ?? deleteError ?? implementError;

	const handleSave = async () => {
		if (!title.trim()) return;
		await update({
			id: item.id,
			title: title.trim(),
			summary: summary.trim(),
			prompt: prompt.trim(),
		});
		onOpenChange(false);
	};

	const handleStartTask = async () => {
		if (title.trim() !== item.title || summary.trim() !== (item.summary ?? "") || prompt.trim() !== (item.prompt ?? "")) {
			await update({
				id: item.id,
				title: title.trim() || item.title,
				summary: summary.trim(),
				prompt: prompt.trim(),
			});
		}
		const result = await implement({ id: item.id });
		onOpenChange(false);
		if (result?.sessionId && onOpenSession) {
			onOpenSession(result.item.projectId, result.sessionId);
		}
	};

	const handleDelete = async () => {
		await remove(item.id);
		onOpenChange(false);
	};

	return (
		<Dialog open={open} onOpenChange={onOpenChange}>
			<DialogContent className={settingsDialogContentClass} data-testid="backlog-detail-dialog">
				<DialogHeader className={settingsDialogHeaderClass}>
					<div className="flex items-center gap-2 pr-8">
						<DialogTitle className="truncate">{t("backlog.editTitle")}</DialogTitle>
						<span className="rounded bg-muted px-1.5 py-0.5 text-2xs font-medium text-muted-foreground">
							{item.status === "implemented" ? t("backlog.statusImplemented") : t("backlog.statusOpen")}
						</span>
						{projectName ? (
							<span className="truncate text-xs text-muted-foreground">· {projectName}</span>
						) : null}
					</div>
					<DialogDescription>{t("backlog.editDescription")}</DialogDescription>
				</DialogHeader>

				<div className={settingsDialogBodyClass}>
					<div className="flex flex-col gap-1.5">
						<Label htmlFor="backlog-detail-title">{t("backlog.title")}</Label>
						<Input
							data-testid="backlog-detail-title"
							id="backlog-detail-title"
							onChange={(event) => setTitle(event.target.value)}
							placeholder={t("backlog.titlePlaceholder")}
							value={title}
						/>
					</div>

					<div className="flex flex-col gap-1.5">
						<Label htmlFor="backlog-detail-summary">{t("backlog.summary")}</Label>
						<textarea
							className={textareaClassName}
							data-testid="backlog-detail-summary"
							id="backlog-detail-summary"
							onChange={(event) => setSummary(event.target.value)}
							placeholder={t("backlog.summaryPlaceholder")}
							rows={4}
							value={summary}
						/>
					</div>

					<div className="flex flex-col gap-1.5">
						<Label htmlFor="backlog-detail-prompt">{t("backlog.prompt")}</Label>
						<textarea
							className={textareaClassName}
							data-testid="backlog-detail-prompt"
							id="backlog-detail-prompt"
							onChange={(event) => setPrompt(event.target.value)}
							placeholder={t("backlog.promptPlaceholder")}
							rows={4}
							value={prompt}
						/>
					</div>

					{combinedError ? (
						<p role="alert" className="text-xs text-destructive">
							{combinedError}
						</p>
					) : null}
				</div>

				<DialogFooter className={settingsDialogFooterClass}>
					<Button
						className="mr-auto text-destructive hover:text-destructive"
						data-testid="backlog-delete-button"
						disabled={busy}
						onClick={() => void handleDelete()}
						size="sm"
						type="button"
						variant="ghost"
					>
						<Trash2 className="size-3.5" aria-hidden="true" />
						{t("backlog.delete")}
					</Button>
					<Button
						disabled={busy || !title.trim()}
						onClick={() => void handleSave()}
						size="sm"
						type="button"
						variant="secondary"
					>
						{t("backlog.saveChanges")}
					</Button>
					{item.status === "implemented" && item.spawnedSessionId && onOpenSession ? (
						<Button
							data-testid="backlog-open-session-button"
							disabled={busy}
							onClick={() => {
								onOpenChange(false);
								onOpenSession(item.projectId, item.spawnedSessionId!);
							}}
							size="sm"
							type="button"
							variant="primary"
						>
							{t("backlog.openSession")}
						</Button>
					) : canImplement ? (
						<Button
							data-testid="backlog-start-task-button"
							disabled={busy || !title.trim()}
							onClick={() => void handleStartTask()}
							size="sm"
							type="button"
							variant="primary"
						>
							<Play className="size-3.5" aria-hidden="true" />
							{t("backlog.startTask")}
						</Button>
					) : null}
				</DialogFooter>
			</DialogContent>
		</Dialog>
	);
}
