import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import type { components } from "../../api/schema";
import { apiClient } from "../lib/api-client";
import { apiErrorException } from "../lib/api-error-exception";

export type StorageUsageResponse = components["schemas"]["StorageUsageResponse"];
export type ProjectStorageUsage = components["schemas"]["ProjectStorageUsageDTO"];
export type StoragePolicy = components["schemas"]["StoragePolicyDTO"];
export type StorageCleanupRequest = components["schemas"]["StorageCleanupRequest"];
export type StorageCleanupResponse = components["schemas"]["StorageCleanupResponse"];

export const storageUsageQueryKey = ["storage-usage"] as const;

export function useStorageUsage(enabled = true) {
	return useQuery({
		queryKey: storageUsageQueryKey,
		enabled,
		queryFn: async (): Promise<StorageUsageResponse> => {
			const { data, error } = await apiClient.GET("/api/v1/storage/usage");
			if (error) throw apiErrorException(error);
			return data as StorageUsageResponse;
		},
	});
}

export function useUpdateStoragePolicy() {
	const queryClient = useQueryClient();
	return useMutation({
		mutationFn: async (policy: StoragePolicy): Promise<StoragePolicy> => {
			const { data, error } = await apiClient.PATCH("/api/v1/storage/policy", {
				body: policy,
			});
			if (error) throw apiErrorException(error);
			return data as StoragePolicy;
		},
		onSuccess: () => {
			void queryClient.invalidateQueries({ queryKey: storageUsageQueryKey });
		},
	});
}

export function useRunStorageCleanup() {
	const queryClient = useQueryClient();
	return useMutation({
		mutationFn: async (request: StorageCleanupRequest): Promise<StorageCleanupResponse> => {
			const { data, error } = await apiClient.POST("/api/v1/storage/cleanup", {
				body: request,
			});
			if (error) throw apiErrorException(error);
			return data as StorageCleanupResponse;
		},
		onSuccess: () => {
			void queryClient.invalidateQueries({ queryKey: storageUsageQueryKey });
		},
	});
}

export function formatBytes(bytes: number): string {
	if (!Number.isFinite(bytes) || bytes <= 0) return "0 B";
	const units = ["B", "KB", "MB", "GB", "TB"];
	let value = bytes;
	let unitIndex = 0;
	while (value >= 1024 && unitIndex < units.length - 1) {
		value /= 1024;
		unitIndex++;
	}
	const digits = unitIndex === 0 ? 0 : value >= 100 ? 0 : value >= 10 ? 1 : 2;
	return `${value.toFixed(digits)} ${units[unitIndex]}`;
}
