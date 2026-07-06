// Shared TypeScript types — match backend snake_case JSON field names.
// Source of truth for all API responses consumed by the frontend.

export interface DashboardStats {
  totalRequests: number;
  totalTokens: number;
  totalCost: number;
  activeProviders: number;
  activeVirtualKeys: number;
  successRate: number;
}

export interface Provider {
  name: string;
  provider: string;
  apiKey: string;
  customHost: string;
  weight: number;
  enabled: boolean;
  requestTimeout: number;
}

export interface VirtualKey {
  id: number;
  name: string;
  key_hash_prefix: string;
  budget_total: number;
  budget_used: number;
  status: string;
  created_at: string;
}

export interface UsageRecord {
  id: number;
  request_id: string;
  virtual_key_name?: string;
  provider: string;
  model: string;
  endpoint: string;
  status_code: number;
  error_message?: string;
  input_tokens: number;
  output_tokens: number;
  cost: number;
  created_at: string;
}

export interface TimeSeriesPoint {
  date: string;
  count: number;
  cost: number;
}

export interface TopItem {
  model?: string;
  provider?: string;
  count: number;
  cost?: number;
  tokens?: number;
}

export interface AnalyticsData {
  timeSeries?: TimeSeriesPoint[];
  topModels?: TopItem[];
  topProviders?: TopItem[];
  maxCount?: number;
}

export interface ServerConfig {
  server?: {
    port?: number;
    host?: string;
  };
  security?: {
    allowedOrigins?: string[];
    jwtSecret?: string;
    adminPass?: string;
  };
  cache?: {
    enabled?: boolean;
    defaultTTL?: number; // nanoseconds from backend
  };
  database?: {
    driver?: string;
    dsn?: string;
  };
}

// API response wrappers
export interface ProvidersResponse {
  data: Provider[];
}

export interface VirtualKeysResponse {
  virtual_keys: VirtualKey[];
}

export interface UsageResponse {
  records: UsageRecord[];
  total: number;
}

export interface LoginResponse {
  token: string;
  status: string;
}
