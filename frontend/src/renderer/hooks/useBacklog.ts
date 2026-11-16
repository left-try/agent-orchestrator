import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import type { components } from "../../api/schema";
import { apiClient, apiErrorMessage } from "../lib/api-client";
import { workspaceQueryKey } from "./useWorkspaceQuery";

export type BacklogItem = components["schemas"]["BacklogItemResponse"];
export type BacklogItemStatus = BacklogItem["status"];

export const backlogRootQueryKey = ["backlog"] as const;

export function backlogQueryKey(projectId?: string) {
	return projectId ? (["backlog", projectId] as const) : (["backlog", "all"] as const);
}

export function useBacklogItems(projectId?: string, enabled = true) {
	const query = useQuery({
		queryKey: backlogQueryKey(projectId),
		enabled,
		refetchInterval: 15_000,
		queryFn: async (): Promise<BacklogItem[]> => {
			if (typeof apiClient?.GET !== "function") return [];
			if (projectId) {
				const response = await apiClient.GET("/api/v1/projects/{projectId}/backlog", {
					params: { path: { projectId } },
				});
				const { data, error } = response ?? {};
				if (error) throw error;
				return data?.items ?? [];
			}
			const response = await apiClient.GET("/api/v1/backlog", {});
			const { data, error } = response ?? {};
			if (error) throw error;
			return data?.items ?? [];
		},
	});

	return {
		items: query.data ?? [],
		isLoading: query.isLoading,
		error: query.error ? apiErrorMessage(query.error) : undefined,
	};
}

export function useCreateBacklogItem() {
	const queryClient = useQueryClient();
	const mutation = useMutation({
		mutationFn: async (input: {
			projectId: string;
			orchestratorSessionId?: string;
			title: string;
			summary?: string;
			prompt?: string;
		}) => {
			const { data, error } = await apiClient.POST("/api/v1/projects/{projectId}/backlog", {
				params: { path: { projectId: input.projectId } },
				body: {
					orchestratorSessionId: input.orchestratorSessionId,
					title: input.title,
					summary: input.summary,
					prompt: input.prompt,
				},
			});
			if (error) throw error;
			return data?.item;
		},
		onSuccess: () => queryClient.invalidateQueries({ queryKey: backlogRootQueryKey }),
	});

	return {
		create: mutation.mutateAsync,
		saving: mutation.isPending,
		error: mutation.error ? apiErrorMessage(mutation.error) : undefined,
		reset: mutation.reset,
	};
}

export function useUpdateBacklogItem() {
	const queryClient = useQueryClient();
	const mutation = useMutation({
		mutationFn: async (input: {
			id: string;
			title?: string;
			summary?: string;
			prompt?: string;
			status?: BacklogItemStatus;
		}) => {
			const { data, error } = await apiClient.PATCH("/api/v1/backlog/{backlogId}", {
				params: { path: { backlogId: input.id } },
				body: {
					title: input.title,
					summary: input.summary,
					prompt: input.prompt,
					status: input.status,
				},
			});
			if (error) throw error;
			return data?.item;
		},
		onSuccess: () => queryClient.invalidateQueries({ queryKey: backlogRootQueryKey }),
	});

	return {
		update: mutation.mutateAsync,
		saving: mutation.isPending,
		error: mutation.error ? apiErrorMessage(mutation.error) : undefined,
		reset: mutation.reset,
	};
}

export function useDeleteBacklogItem() {
	const queryClient = useQueryClient();
	const mutation = useMutation({
		mutationFn: async (id: string) => {
			const { error } = await apiClient.DELETE("/api/v1/backlog/{backlogId}", {
				params: { path: { backlogId: id } },
			});
			if (error) throw error;
		},
		onSuccess: () => queryClient.invalidateQueries({ queryKey: backlogRootQueryKey }),
	});

	return {
		remove: mutation.mutateAsync,
		deleting: mutation.isPending,
		error: mutation.error ? apiErrorMessage(mutation.error) : undefined,
		reset: mutation.reset,
	};
}

export function useImplementBacklogItem() {
	const queryClient = useQueryClient();
	const mutation = useMutation({
		mutationFn: async (input: {
			id: string;
			harness?: string;
			model?: string;
			interfaceMode?: "chat" | "tui";
		}) => {
			const { data, error } = await apiClient.POST("/api/v1/backlog/{backlogId}/implement", {
				params: { path: { backlogId: input.id } },
				body: {
					harness: input.harness,
					model: input.model,
					interfaceMode: input.interfaceMode,
					async: true,
				},
			});
			if (error) throw error;
			return data;
		},
		onSuccess: async () => {
			await Promise.all([
				queryClient.invalidateQueries({ queryKey: backlogRootQueryKey }),
				queryClient.invalidateQueries({ queryKey: workspaceQueryKey }),
			]);
		},
	});

	return {
		implement: mutation.mutateAsync,
		starting: mutation.isPending,
		error: mutation.error ? apiErrorMessage(mutation.error) : undefined,
		reset: mutation.reset,
	};
}
