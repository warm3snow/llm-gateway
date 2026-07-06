// React Query hooks for the LLM Gateway API.
// Centralizing these keeps query keys consistent and cacheable across pages.
"use client";

import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "./api";
import type {
  DashboardStats,
  AnalyticsData,
  Provider,
  ProvidersResponse,
  VirtualKey,
  VirtualKeysResponse,
  UsageResponse,
  ServerConfig,
} from "./types";

type UpdateProviderData = Parameters<typeof api.updateProvider>[1];
type UpdateVirtualKeyData = Parameters<typeof api.updateVirtualKey>[1];

export const queryKeys = {
  stats: ["stats"] as const,
  analytics: ["analytics"] as const,
  providers: ["providers"] as const,
  virtualKeys: ["virtualKeys"] as const,
  usage: ["usage"] as const,
  config: ["config"] as const,
};

export function useStats() {
  return useQuery<DashboardStats>({
    queryKey: queryKeys.stats,
    queryFn: api.getStats,
  });
}

export function useAnalytics() {
  return useQuery<AnalyticsData>({
    queryKey: queryKeys.analytics,
    queryFn: api.getAnalytics,
  });
}

export function useProviders() {
  return useQuery<Provider[]>({
    queryKey: queryKeys.providers,
    queryFn: async () => (await api.getProviders()).data,
  });
}

export function useVirtualKeys() {
  return useQuery<VirtualKey[]>({
    queryKey: queryKeys.virtualKeys,
    queryFn: async () => (await api.getVirtualKeys()).virtual_keys,
  });
}

export function useCreateVirtualKey() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: api.createVirtualKey,
    onSuccess: () => qc.invalidateQueries({ queryKey: queryKeys.virtualKeys }),
  });
}

export function useUpdateVirtualKey() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ id, data }: { id: number; data: UpdateVirtualKeyData }) =>
      api.updateVirtualKey(id, data),
    onSuccess: () => qc.invalidateQueries({ queryKey: queryKeys.virtualKeys }),
  });
}

export function useDeleteVirtualKey() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: number) => api.deleteVirtualKey(id),
    onSuccess: () => qc.invalidateQueries({ queryKey: queryKeys.virtualKeys }),
  });
}

export function useUsage(params?: {
  limit?: number;
  offset?: number;
  provider?: string;
  model?: string;
  status_code?: number;
}) {
  return useQuery<UsageResponse>({
    queryKey: [...queryKeys.usage, params],
    queryFn: () => api.getUsage(params),
  });
}

export function useCreateProvider() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: api.createProvider,
    onSuccess: () => qc.invalidateQueries({ queryKey: queryKeys.providers }),
  });
}

export function useUpdateProvider() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ name, data }: { name: string; data: UpdateProviderData }) =>
      api.updateProvider(name, data),
    onSuccess: () => qc.invalidateQueries({ queryKey: queryKeys.providers }),
  });
}

export function useDeleteProvider() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (name: string) => api.deleteProvider(name),
    onSuccess: () => qc.invalidateQueries({ queryKey: queryKeys.providers }),
  });
}

export function useConfig() {
  return useQuery<ServerConfig>({
    queryKey: queryKeys.config,
    queryFn: async () => (await api.getConfig()).data,
  });
}

export function useUpdateConfig() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (data: ServerConfig) => api.updateConfig(data),
    onSuccess: () => qc.invalidateQueries({ queryKey: queryKeys.config }),
  });
}

// Re-export response types if needed elsewhere
export type {
  ProvidersResponse,
  VirtualKeysResponse,
  UsageResponse,
};
