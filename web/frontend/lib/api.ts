import { getCookie, setCookie, deleteCookie } from 'cookies-next';
import { QueryClient } from '@tanstack/react-query';
import type {
  DashboardStats,
  Provider,
  ProvidersResponse,
  VirtualKey,
  VirtualKeysResponse,
  UsageResponse,
  AnalyticsData,
  ServerConfig,
  LoginResponse,
} from './types';

const API_BASE = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080';

async function apiFetch<T>(endpoint: string, options: RequestInit = {}): Promise<T> {
  const token = getCookie('auth_token');
  const authHeaders: Record<string, string> = {
    'Content-Type': 'application/json',
  };
  if (token) {
    authHeaders['Authorization'] = `Bearer ${token}`;
  }
  const res = await fetch(`${API_BASE}${endpoint}`, {
    ...options,
    headers: { ...authHeaders, ...(options.headers as Record<string, string> || {}) },
  });
  if (!res.ok) {
    const err = await res.json().catch(() => ({}));
    throw new Error(err.message || `API Error: ${res.statusText}`);
  }
  const text = await res.text();
  if (!text) return undefined as unknown as T;
  return JSON.parse(text) as T;
}

export const api = {
  login: (data: { username: string; password: string }) =>
    apiFetch<LoginResponse>('/api/v1/auth/login', {
      method: 'POST',
      body: JSON.stringify(data),
    }),
  logout: () => {
    deleteCookie('auth_token', { path: '/' });
    if (typeof window !== 'undefined') {
      window.location.href = '/login';
    }
  },
  getStats: () => apiFetch<DashboardStats>('/api/v1/stats/overview'),
  getAnalytics: () => apiFetch<AnalyticsData>('/api/v1/stats/analytics'),
  getProviders: () => apiFetch<ProvidersResponse>('/api/v1/admin/providers'),
  getVirtualKeys: () => apiFetch<VirtualKeysResponse>('/api/v1/virtual-keys'),
  getUsage: () => apiFetch<UsageResponse>('/api/v1/usage'),
  getConfig: () => apiFetch<{ data: ServerConfig }>('/api/v1/admin/config'),
  updateConfig: (data: ServerConfig) =>
    apiFetch<{ data: ServerConfig }>('/api/v1/admin/config', {
      method: 'POST',
      body: JSON.stringify(data),
    }),
  createVirtualKey: (data: { name: string; budget_total: number }) =>
    apiFetch<VirtualKey>('/api/v1/virtual-keys', {
      method: 'POST',
      body: JSON.stringify(data),
    }),
  deleteVirtualKey: (id: string | number) =>
    apiFetch<void>(`/api/v1/virtual-keys/${id}`, {
      method: 'DELETE',
    }),
};

export const queryClient = new QueryClient({
  defaultOptions: {
    queries: { refetchOnWindowFocus: false, retry: 1 },
  },
});
