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
  SelectTenantResponse,
  TenantsResponse,
  TenantUsersResponse,
  Tenant,
  TenantUser,
  TenantMembership,
} from './types';

const API_BASE = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080';
const AUTH_COOKIE = 'auth_token';
const TENANT_COOKIE = 'current_tenant';

const sessionCookieOptions = {
  maxAge: 86400,
  path: '/',
  secure: process.env.NODE_ENV === 'production',
  sameSite: 'strict' as const,
};

export function persistAuthSession(token: string, tenant?: TenantMembership) {
  setCookie(AUTH_COOKIE, token, sessionCookieOptions);
  if (tenant) {
    setCookie(TENANT_COOKIE, JSON.stringify(tenant), sessionCookieOptions);
  } else {
    deleteCookie(TENANT_COOKIE, { path: '/' });
  }
}

async function apiFetch<T>(endpoint: string, options: RequestInit = {}): Promise<T> {
  const token = getCookie(AUTH_COOKIE);
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
    throw new Error(err.error || err.message || `API Error: ${res.statusText}`);
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
  selectTenant: (data: { login_token: string; tenant_id: number }) =>
    apiFetch<SelectTenantResponse>('/api/v1/auth/select-tenant', {
      method: 'POST',
      body: JSON.stringify(data),
    }),
  changeOwnPassword: (data: { current_password: string; new_password: string }) =>
    apiFetch<void>('/api/v1/users/me/password', {
      method: 'PUT',
      body: JSON.stringify(data),
    }),
  logout: () => {
    deleteCookie(AUTH_COOKIE, { path: '/' });
    deleteCookie(TENANT_COOKIE, { path: '/' });
    if (typeof window !== 'undefined') {
      window.location.href = '/login';
    }
  },
  getStats: () => apiFetch<DashboardStats>('/api/v1/stats/overview'),
  getAnalytics: () => apiFetch<AnalyticsData>('/api/v1/stats/analytics'),
  getProviders: () => apiFetch<ProvidersResponse>('/api/v1/admin/providers'),
  getVirtualKeys: () => apiFetch<VirtualKeysResponse>('/api/v1/virtual-keys'),
  getUsage: (params?: {
    limit?: number;
    offset?: number;
    provider?: string;
    model?: string;
    status_code?: number;
  }) => {
    const qs = params
      ? new URLSearchParams(
          Object.entries(params)
            .filter(([, v]) => v != null && v !== '')
            .map(([k, v]) => [k, String(v)])
        ).toString()
      : '';
    return apiFetch<UsageResponse>(`/api/v1/usage${qs ? `?${qs}` : ''}`);
  },
  getConfig: () => apiFetch<{ data: ServerConfig }>('/api/v1/admin/config'),
  updateConfig: (data: ServerConfig) =>
    apiFetch<{ data: ServerConfig }>('/api/v1/admin/config', {
      method: 'POST',
      body: JSON.stringify(data),
    }),
  createProvider: (data: {
    name: string;
    provider: string;
    apiKey: string;
    customHost?: string;
    weight?: number;
    requestTimeout?: number;
  }) =>
    apiFetch<void>('/api/v1/admin/providers', {
      method: 'POST',
      body: JSON.stringify(data),
    }),
  updateProvider: (name: string, data: {
    provider: string;
    apiKey?: string;
    customHost?: string;
    weight?: number;
    requestTimeout?: number;
  }) =>
    apiFetch<void>(`/api/v1/admin/providers/${encodeURIComponent(name)}`, {
      method: 'PUT',
      body: JSON.stringify(data),
    }),
  deleteProvider: (name: string) =>
    apiFetch<void>(`/api/v1/admin/providers/${encodeURIComponent(name)}`, {
      method: 'DELETE',
    }),
  createVirtualKey: (data: { name: string; budget_total?: number }) =>
    apiFetch<VirtualKey>('/api/v1/virtual-keys', {
      method: 'POST',
      body: JSON.stringify(data),
    }),
  updateVirtualKey: (id: string | number, data: {
    name?: string;
    budget_total?: number;
    rate_limit?: number;
    rate_limit_window?: number;
    providers?: string[];
  }) =>
    apiFetch<VirtualKey>(`/api/v1/virtual-keys/${id}`, {
      method: 'PUT',
      body: JSON.stringify(data),
    }),
  deleteVirtualKey: (id: string | number) =>
    apiFetch<void>(`/api/v1/virtual-keys/${id}`, {
      method: 'DELETE',
    }),
  // Tenant management (super_admin only).
  getTenants: () => apiFetch<TenantsResponse>('/api/v1/tenants'),
  createTenant: (data: { name: string; slug: string }) =>
    apiFetch<{ tenant: Tenant }>('/api/v1/tenants', {
      method: 'POST',
      body: JSON.stringify(data),
    }),
  setTenantStatus: (id: number, status: string) =>
    apiFetch<void>(`/api/v1/tenants/${id}/status`, {
      method: 'PUT',
      body: JSON.stringify({ status }),
    }),
  getTenantUsers: (tenantId?: number) =>
    apiFetch<TenantUsersResponse>(
      `/api/v1/tenants/users${tenantId ? `?tenant_id=${tenantId}` : ''}`
    ),
  createTenantUser: (data: {
    username: string;
    email?: string;
    password: string;
    tenant_id: number;
    role?: string;
  }) =>
    apiFetch<{ user: TenantUser }>('/api/v1/tenants/users', {
      method: 'POST',
      body: JSON.stringify(data),
    }),
  getUsers: (tenantId?: number) =>
    apiFetch<TenantUsersResponse>(
      `/api/v1/users${tenantId ? `?tenant_id=${tenantId}` : ''}`
    ),
  createUser: (data: {
    username: string;
    email?: string;
    password: string;
    tenant_id?: number;
    role: string;
  }) =>
    apiFetch<{ user: TenantUser }>('/api/v1/users', {
      method: 'POST',
      body: JSON.stringify(data),
    }),
  setUserStatus: (id: number, status: string) =>
    apiFetch<void>(`/api/v1/users/${id}/status`, {
      method: 'PUT',
      body: JSON.stringify({ status }),
    }),
};

// currentRole decodes the (unverified) JWT payload to read the user's role.
// This is UI-only gating; the backend independently enforces authorization.
function currentPayload(): Record<string, unknown> | null {
  const token = getCookie(AUTH_COOKIE);
  if (!token || typeof token !== 'string') return null;
  const parts = token.split('.');
  if (parts.length < 2) return null;
  try {
    const payload = parts[1].replace(/-/g, '+').replace(/_/g, '/');
    const padded = payload.padEnd(payload.length + ((4 - (payload.length % 4)) % 4), '=');
    return JSON.parse(atob(padded)) as Record<string, unknown>;
  } catch {
    return null;
  }
}

export function currentUser(): {
  username: string;
  email?: string;
  role?: string;
  tenant_id?: number;
} | null {
  const payload = currentPayload();
  const username = payload?.username;
  if (typeof username !== 'string' || !username) return null;
  const email = payload.email;
  const role = payload.role;
  const tenantId = payload.tenant_id;
  return {
    username,
    email: typeof email === 'string' ? email : undefined,
    role: typeof role === 'string' ? role : undefined,
    tenant_id: typeof tenantId === 'number' ? tenantId : undefined,
  };
}

export function currentRole(): string | null {
  return currentUser()?.role ?? null;
}

export function currentTenantId(): number | null {
  return currentUser()?.tenant_id ?? null;
}

export function currentTenant(): TenantMembership | null {
  const rawTenant = getCookie(TENANT_COOKIE);
  if (!rawTenant || typeof rawTenant !== 'string') return null;
  try {
    const tenant = JSON.parse(rawTenant) as TenantMembership;
    const tenantId = currentTenantId();
    if (
      typeof tenant.tenant_id !== 'number' ||
      typeof tenant.name !== 'string' ||
      typeof tenant.slug !== 'string' ||
      typeof tenant.role !== 'string' ||
      typeof tenant.status !== 'string' ||
      (tenantId != null && tenant.tenant_id !== tenantId)
    ) {
      return null;
    }
    return tenant;
  } catch {
    return null;
  }
}

export const queryClient = new QueryClient({
  defaultOptions: {
    queries: { refetchOnWindowFocus: false, retry: 1 },
  },
});
