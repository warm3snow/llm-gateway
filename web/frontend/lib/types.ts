// Shared TypeScript types — match backend snake_case JSON field names.
// Source of truth for all API responses consumed by the frontend.

export interface DashboardStats {
  totalRequests: number;
  totalTokens: number;
  totalCost: number;
  activeProviders: number;
  activeVirtualKeys: number;
  successRate: number;
  cacheReadTokens?: number;
  cacheWriteTokens?: number;
  reasoningTokens?: number;
  totalRedactions?: number;
}

export interface ModelPricing {
  id: number;
  provider: string;
  model: string;
  input_price: number;
  output_price: number;
  cache_read_price?: number;
  cache_write_price?: number;
  currency: string;
  source: string;
  updated_at: string;
}

export interface PricingResponse {
  pricings: ModelPricing[];
}

export interface PricingSyncResponse {
  status: string;
  count: number;
  message: string;
}

export interface RedactRule {
  kind: string;
  prefix?: string;
  regex?: string;
}

export interface RedactConfig {
  enabled: boolean;
  secrets: boolean;
  personal: boolean;
  words?: string[];
  wordsAnyCase?: boolean;
  wordsWhole?: boolean;
  rules?: RedactRule[];
  kinds?: Record<string, boolean>;
  actions?: Record<string, string>;
  allow?: string[];
}

export interface RedactPreviewResponse {
  masked: string;
  enabled: boolean;
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

export interface ProviderHealth {
  id: number;
  provider_name: string;
  provider_type: string;
  healthy: boolean;
  status: string;
  latency_ms: number;
  error_message?: string;
  last_checked_at: string;
  last_success_at?: string;
  consecutive_errors: number;
}

export interface AlertEvent {
  id: number;
  tenant_id: number;
  rule_id: number;
  virtual_key_id: number;
  status: string;
  message: string;
  value: number;
  threshold: number;
  fired_at: string;
  resolved_at?: string;
}

export interface VirtualKey {
  id: number;
  tenant_id?: number;
  user_id?: number; // owning user — cost is attributed to this user
  user_username?: string;
  created_by_user_id?: number;
  created_by_username?: string;
  name: string;
  key_hash_prefix: string;
  budget_total: number;
  budget_used: number;
  budget_remaining?: number;
  rate_limit?: number;
  rate_limit_window?: number;
  providers?: string;
  status: string;
  created_at: string;
  updated_at?: string;
}

export interface Tenant {
  id: number;
  name: string;
  slug: string;
  status: string;
  // Quota & policy (platform-managed)
  budget_monthly: number; // USD per calendar month, 0 = unlimited
  budget_used: number; // month-to-date spend
  tpm_limit: number; // 0 = unlimited
  rpm_limit: number; // 0 = unlimited
  allowed_models: string[]; // empty = all models
  allow_self_key: boolean; // tenant_user may create own keys
  budget_reset_at?: string;
  created_at: string;
}

export interface TenantUser {
  id: number;
  tenant_id?: number;
  username: string;
  email?: string;
  role: string;
  status: string;
  // Per-user monthly quota (tenant_admin assigns; sum must fit tenant budget)
  budget_monthly: number;
  budget_used: number; // month-to-date spend
  rpm_limit: number;
  budget_reset_at?: string;
  created_at: string;
}

export interface TenantsResponse {
  tenants: Tenant[];
}

export interface TenantUsersResponse {
  users: TenantUser[];
}

export interface UsageRecord {
  id: number;
  tenant_id?: number;
  user_id?: number;
  username?: string;
  request_id: string;
  virtual_key_id?: number;
  virtual_key_name?: string;
  virtual_key_created_by_user_id?: number;
  virtual_key_created_by_username?: string;
  provider: string;
  model: string;
  endpoint: string;
  status_code: number;
  error_message?: string;
  model_input_kind?: string;
  model_input_preview?: string;
  model_input_truncated?: boolean;
  model_input_bytes?: number;
  model_input_preview_bytes?: number;
  input_tokens: number;
  output_tokens: number;
  cache_read_tokens?: number;
  cache_write_tokens?: number;
  reasoning_tokens?: number;
  cost: number;
  latency_ms?: number;
  tries?: number;
  redactions?: number;
  created_at: string;
}

export type UsageStatus = "success" | "error";

export interface TimeSeriesPoint {
  date: string;
  count: number;
  cost: number;
}

export interface TimeSeriesResponse {
  timeSeries: TimeSeriesPoint[];
}

export interface TopItem {
  model?: string;
  provider?: string;
  count: number;
  cost?: number;
  tokens?: number;
}

export interface PeriodSummary {
  period: string;
  since?: string;
  totalRequests: number;
  errors: number;
  inputTokens: number;
  outputTokens: number;
  cacheReadTokens: number;
  cacheWriteTokens: number;
  reasoningTokens: number;
  cost: number;
  totalRedactions: number;
  series?: TimeSeriesPoint[];
  topModels?: TopItem[];
  topProviders?: TopItem[];
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

export interface ProviderHealthResponse {
  providers: ProviderHealth[];
}

export interface AlertEventsResponse {
  events: AlertEvent[];
}

export interface VirtualKeysResponse {
  virtual_keys: VirtualKey[];
}

export interface UsageResponse {
  records: UsageRecord[];
  total: number;
}

export interface TenantMembership {
  tenant_id: number;
  name: string;
  slug: string;
  role: string;
  status: string;
}

// Cost breakdown row: cost aggregated by user / key / model.
export interface CostBreakdownItem {
  user_id?: number;
  username?: string;
  virtual_key_id?: number;
  virtual_key_name?: string;
  model?: string;
  provider?: string;
  requests: number;
  input_tokens: number;
  output_tokens: number;
  cost: number;
}

export interface TenantCostResponse {
  tenant_id: number;
  since?: string;
  total_requests: number;
  total_cost: number;
  items: CostBreakdownItem[];
}

export interface UserCostResponse {
  user_id: number;
  since?: string;
  total_requests: number;
  total_cost: number;
  items: CostBreakdownItem[];
}

export interface MeUsageResponse {
  username: string;
  role: string;
  budget_monthly: number;
  budget_used: number;
  rpm_limit: number;
  total_requests: number;
  total_cost: number;
  input_tokens: number;
  output_tokens: number;
}

export type LoginResponse =
  | {
      status: 'success';
      token: string;
      tenant?: TenantMembership;
    }
  | {
      status: 'tenant_selection_required';
      login_token: string;
      tenants: TenantMembership[];
    };

export interface SelectTenantResponse {
  status: 'success';
  token: string;
  tenant: TenantMembership;
}
