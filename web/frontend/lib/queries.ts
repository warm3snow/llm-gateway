// React Query hooks for the LLM Gateway API.
// Centralizing these keeps query keys consistent and cacheable across pages.
"use client";

import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "./api";
import type {
  DashboardStats,
  AnalyticsData,
  PeriodSummary,
  Provider,
  ProvidersResponse,
  ProviderHealth,
  AlertEvent,
  ModelPricing,
  RedactConfig,
  VirtualKey,
  VirtualKeysResponse,
  UsageResponse,
  UsageStatus,
  TimeSeriesResponse,
  ServerConfig,
  Tenant,
  TenantUser,
  TenantCostResponse,
  UserCostResponse,
  MeUsageResponse,
} from "./types";

type UpdateProviderData = Parameters<typeof api.updateProvider>[1];
type UpdateVirtualKeyData = Parameters<typeof api.updateVirtualKey>[1];

export const queryKeys = {
  stats: ["stats"] as const,
  analytics: ["analytics"] as const,
  hourlyStats: ["hourlyStats"] as const,
  periodSummary: ["periodSummary"] as const,
  providers: ["providers"] as const,
  providerHealth: ["providerHealth"] as const,
  activeAlerts: ["activeAlerts"] as const,
  virtualKeys: ["virtualKeys"] as const,
  usage: ["usage"] as const,
  config: ["config"] as const,
  pricing: ["pricing"] as const,
  redact: ["redact"] as const,
  tenants: ["tenants"] as const,
  tenantCost: ["tenantCost"] as const,
  users: ["users"] as const,
  userCost: ["userCost"] as const,
  meUsage: ["meUsage"] as const,
  meKeys: ["meKeys"] as const,
};

export function useStats(params?: { tenant_id?: number }) {
  return useQuery<DashboardStats>({
    queryKey: [...queryKeys.stats, params],
    queryFn: () => api.getStats(params),
  });
}

export function useAnalytics(params?: { tenant_id?: number }) {
  return useQuery<AnalyticsData>({
    queryKey: [...queryKeys.analytics, params],
    queryFn: () => api.getAnalytics(params),
  });
}

export function useHourlyStats(params?: { hours?: number; status?: UsageStatus; status_code?: number }) {
  return useQuery<TimeSeriesResponse>({
    queryKey: [queryKeys.hourlyStats[0], params?.hours, params?.status, params?.status_code],
    queryFn: () => api.getHourlyStats(params),
  });
}

export function useProviders() {
  return useQuery<Provider[]>({
    queryKey: queryKeys.providers,
    queryFn: async () => (await api.getProviders()).data,
  });
}

export function useProviderHealth() {
  return useQuery<ProviderHealth[]>({
    queryKey: queryKeys.providerHealth,
    queryFn: async () => (await api.getProviderHealth()).providers,
  });
}

export function useActiveAlerts() {
  return useQuery<AlertEvent[]>({
    queryKey: queryKeys.activeAlerts,
    queryFn: async () => (await api.getActiveAlerts()).events,
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
  status?: UsageStatus;
  status_code?: number;
  tenant_id?: number;
  start_date?: string;
  end_date?: string;
}) {
  return useQuery<UsageResponse>({
    queryKey: [
      queryKeys.usage[0],
      params?.limit,
      params?.offset,
      params?.provider,
      params?.model,
      params?.status,
      params?.status_code,
      params?.tenant_id,
      params?.start_date,
      params?.end_date,
    ],
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

export function usePeriodSummary(period?: string) {
  return useQuery<PeriodSummary>({
    queryKey: [...queryKeys.periodSummary, period],
    queryFn: () => api.getPeriodSummary(period),
  });
}

export function usePricing(provider?: string) {
  return useQuery<ModelPricing[]>({
    queryKey: [...queryKeys.pricing, provider],
    queryFn: async () => (await api.getPricing(provider)).pricings,
  });
}

export function useUpsertPricing() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: api.upsertPricing,
    onSuccess: () => qc.invalidateQueries({ queryKey: queryKeys.pricing }),
  });
}

export function useDeletePricing() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: number) => api.deletePricing(id),
    onSuccess: () => qc.invalidateQueries({ queryKey: queryKeys.pricing }),
  });
}

export function useSyncPricing() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (data?: { providers?: string[]; dry_run?: boolean }) =>
      api.syncPricing(data),
    onSuccess: () => qc.invalidateQueries({ queryKey: queryKeys.pricing }),
  });
}

export function useRedactConfig() {
  return useQuery<RedactConfig>({
    queryKey: queryKeys.redact,
    queryFn: async () => (await api.getRedactConfig()).data,
  });
}

// ---- Tenant / user / self-service hooks ----

export function useTenants() {
  return useQuery<Tenant[]>({
    queryKey: queryKeys.tenants,
    queryFn: async () => (await api.getTenants()).tenants,
  });
}

export function useCreateTenant() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: api.createTenant,
    onSuccess: () => qc.invalidateQueries({ queryKey: queryKeys.tenants }),
  });
}

export function useUpdateTenant() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ id, data }: { id: number; data: Parameters<typeof api.updateTenant>[1] }) =>
      api.updateTenant(id, data),
    onSuccess: () => qc.invalidateQueries({ queryKey: queryKeys.tenants }),
  });
}

export function useSetTenantStatus() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ id, status }: { id: number; status: string }) =>
      api.setTenantStatus(id, status),
    onSuccess: () => qc.invalidateQueries({ queryKey: queryKeys.tenants }),
  });
}

export function useTenantCost(tenantId?: number, by?: string) {
  return useQuery<TenantCostResponse>({
    queryKey: [...queryKeys.tenantCost, tenantId, by],
    queryFn: () => api.getTenantCost(tenantId!, { by }),
    enabled: !!tenantId,
  });
}

export function useUsers(tenantId?: number) {
  return useQuery<TenantUser[]>({
    queryKey: [...queryKeys.users, tenantId],
    queryFn: async () => (await api.getUsers(tenantId || undefined)).users,
  });
}

export function useCreateUser() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: api.createUser,
    onSuccess: () => qc.invalidateQueries({ queryKey: queryKeys.users }),
  });
}

export function useUpdateUserQuota() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ id, data }: { id: number; data: Parameters<typeof api.updateUserQuota>[1] }) =>
      api.updateUserQuota(id, data),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.users });
    },
  });
}

export function useUserCost(userId?: number, by?: string) {
  return useQuery<UserCostResponse>({
    queryKey: [...queryKeys.userCost, userId, by],
    queryFn: () => api.getUserCost(userId!, { by }),
    enabled: !!userId,
  });
}

export function useSetUserStatus() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ id, status }: { id: number; status: string }) =>
      api.setUserStatus(id, status),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.users });
    },
  });
}

export function useSetUserRole() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({
      id,
      role,
      tenantId,
    }: {
      id: number;
      role: string;
      tenantId?: number;
    }) => api.setUserRole(id, role, tenantId),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.users });
    },
  });
}

export function useMeUsage() {
  return useQuery<MeUsageResponse>({
    queryKey: queryKeys.meUsage,
    queryFn: () => api.getMeUsage(),
  });
}

export function useMeKeys() {
  return useQuery<VirtualKey[]>({
    queryKey: queryKeys.meKeys,
    queryFn: async () => (await api.getMeKeys()).virtual_keys,
  });
}

export function useCreateMeKey() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: api.createMeKey,
    onSuccess: () => qc.invalidateQueries({ queryKey: queryKeys.meKeys }),
  });
}

export function useDeleteMeKey() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: number) => api.deleteMeKey(id),
    onSuccess: () => qc.invalidateQueries({ queryKey: queryKeys.meKeys }),
  });
}

export function useUpdateRedactConfig() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (data: RedactConfig) => api.updateRedactConfig(data),
    onSuccess: () => qc.invalidateQueries({ queryKey: queryKeys.redact }),
  });
}

// Re-export response types if needed elsewhere
export type {
  ProvidersResponse,
  VirtualKeysResponse,
  UsageResponse,
};
