"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useRouter } from "next/navigation";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import {
  Activity,
  BarChart3,
  Building2,
  FileText,
  KeyRound,
  LayoutDashboard,
  type LucideIcon,
  MessagesSquare,
  Plus,
  RefreshCw,
  Search,
  Settings,
  Users,
  X,
} from "lucide-react";
import { toast } from "sonner";
import { api, currentRole } from "@/lib/api";
import { queryKeys } from "@/lib/queries";
import { cn } from "@/lib/utils";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { EmptyState } from "@/components/ui/empty-state";
import { Input } from "@/components/ui/input";
import { Skeleton } from "@/components/ui/skeleton";

type PaletteSection =
  | "Actions"
  | "Pages"
  | "Providers"
  | "Virtual Keys"
  | "Logs"
  | "Users"
  | "Tenants";

type CommandItem = {
  id: string;
  section: PaletteSection;
  title: string;
  subtitle: string;
  keywords?: string;
  icon: LucideIcon;
  badge?: string;
  priority?: number;
  run: () => void;
};

type PageCommand = {
  href: string;
  label: string;
  code: string;
  keywords: string;
  icon: LucideIcon;
  roles: string[];
};

const pageCommands: PageCommand[] = [
  { href: "/dashboard", label: "Dashboard", code: "dash", keywords: "overview stats home", icon: LayoutDashboard, roles: ["super_admin", "tenant_admin", "tenant_user"] },
  { href: "/providers", label: "Providers", code: "prov", keywords: "upstream models llm routing", icon: MessagesSquare, roles: ["super_admin", "tenant_admin", "tenant_user"] },
  { href: "/virtual-keys", label: "Virtual Keys", code: "keys", keywords: "api keys tokens budget rate limit", icon: KeyRound, roles: ["super_admin", "tenant_admin", "tenant_user"] },
  { href: "/logs", label: "Logs", code: "logs", keywords: "requests telemetry usage records", icon: Activity, roles: ["super_admin", "tenant_admin", "tenant_user"] },
  { href: "/analytics", label: "Analytics", code: "stat", keywords: "charts metrics reporting", icon: BarChart3, roles: ["super_admin", "tenant_admin", "tenant_user"] },
  { href: "/tenants", label: "Tenants", code: "tnnt", keywords: "organizations workspaces", icon: Building2, roles: ["super_admin"] },
  { href: "/users", label: "Users", code: "user", keywords: "accounts members roles", icon: Users, roles: ["super_admin", "tenant_admin"] },
  { href: "/settings", label: "Settings", code: "conf", keywords: "configuration admin", icon: Settings, roles: ["super_admin"] },
];

const sectionOrder: PaletteSection[] = [
  "Actions",
  "Pages",
  "Providers",
  "Virtual Keys",
  "Logs",
  "Users",
  "Tenants",
];

const sectionLimit: Record<PaletteSection, number> = {
  Actions: 6,
  Pages: 8,
  Providers: 6,
  "Virtual Keys": 6,
  Logs: 5,
  Users: 5,
  Tenants: 5,
};

function includesRole(role: string | null, roles: string[]) {
  return role != null && roles.includes(role);
}

function scoreItem(item: CommandItem, query: string) {
  const q = query.trim().toLowerCase();
  if (!q) return item.priority ?? 0;

  const title = item.title.toLowerCase();
  const subtitle = item.subtitle.toLowerCase();
  const keywords = item.keywords?.toLowerCase() ?? "";
  const haystack = `${title} ${subtitle} ${keywords}`;

  if (title === q) return 1000;
  if (title.startsWith(q)) return 800;
  if (title.includes(q)) return 600;
  if (subtitle.includes(q)) return 400;
  if (keywords.includes(q)) return 300;
  return haystack.includes(q) ? 100 : -1;
}

function formatCost(value?: number) {
  if (value == null) return "$0.0000";
  return `$${value.toFixed(4)}`;
}

function formatTime(iso?: string) {
  if (!iso) return "unknown time";
  return new Date(iso).toLocaleString("en-US", {
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
    hour12: false,
  });
}

export function CommandPalette({
  open,
  onOpenChange,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
}) {
  const router = useRouter();
  const queryClient = useQueryClient();
  const inputRef = useRef<HTMLInputElement>(null);
  const [query, setQuery] = useState("");
  const [activeIndex, setActiveIndex] = useState(0);
  const [role] = useState<string | null>(() => currentRole());

  const canManageProviders = role === "super_admin";
  const canManageTenants = role === "super_admin";
  const canManageUsers = role === "super_admin" || role === "tenant_admin";
  const canCreateKey = role != null;

  const providersQuery = useQuery({
    queryKey: queryKeys.providers,
    queryFn: async () => (await api.getProviders()).data,
    enabled: open,
  });
  const virtualKeysQuery = useQuery({
    queryKey: queryKeys.virtualKeys,
    queryFn: async () => (await api.getVirtualKeys()).virtual_keys,
    enabled: open,
  });
  const usageQuery = useQuery({
    queryKey: [queryKeys.usage[0], "palette", 20],
    queryFn: () => api.getUsage({ limit: 20, offset: 0 }),
    enabled: open,
  });
  const usersQuery = useQuery({
    queryKey: ["users", "palette"],
    queryFn: () => api.getUsers(),
    enabled: open && canManageUsers,
  });
  const tenantsQuery = useQuery({
    queryKey: ["tenants", "palette"],
    queryFn: () => api.getTenants(),
    enabled: open && canManageTenants,
  });

  const navigate = useCallback((href: string, message?: string) => {
    router.push(href);
    if (message) toast.info(message);
  }, [router]);

  const refreshData = useCallback((label: string, queryKey: readonly unknown[]) => {
    queryClient.invalidateQueries({ queryKey });
    toast.success("refresh queued", { description: label });
  }, [queryClient]);

  const allItems = useMemo<CommandItem[]>(() => {
    const items: CommandItem[] = [];

    if (canCreateKey) {
      items.push({
        id: "action:create-key",
        section: "Actions",
        title: "Issue virtual key",
        subtitle: "Open Virtual Keys and create a new API key",
        keywords: "create new token credential budget rate limit",
        icon: Plus,
        badge: "action",
        priority: 100,
        run: () => navigate("/virtual-keys", "Use issue key to create a virtual key."),
      });
    }

    if (canManageProviders) {
      items.push({
        id: "action:add-provider",
        section: "Actions",
        title: "Add provider",
        subtitle: "Open Providers and register an upstream LLM backend",
        keywords: "create provider upstream model routing",
        icon: Plus,
        badge: "action",
        priority: 95,
        run: () => navigate("/providers", "Use add provider to register an upstream backend."),
      });
    }

    if (canManageTenants) {
      items.push({
        id: "action:create-tenant",
        section: "Actions",
        title: "Create tenant",
        subtitle: "Open Tenants and create an isolated workspace",
        keywords: "new organization workspace",
        icon: Plus,
        badge: "action",
        priority: 90,
        run: () => navigate("/tenants", "Use new tenant to create a workspace."),
      });
    }

    if (canManageUsers) {
      items.push({
        id: "action:create-user",
        section: "Actions",
        title: "Create user",
        subtitle: "Open Users and add a tenant member",
        keywords: "new member account role invite",
        icon: Plus,
        badge: "action",
        priority: 88,
        run: () => navigate("/users", "Use add user to create a tenant member."),
      });
    }

    items.push(
      {
        id: "action:refresh-dashboard",
        section: "Actions",
        title: "Refresh dashboard stats",
        subtitle: "Invalidate stats and analytics data",
        keywords: "reload metrics charts dashboard analytics",
        icon: RefreshCw,
        badge: "refresh",
        priority: 75,
        run: () => {
          queryClient.invalidateQueries({ queryKey: queryKeys.stats });
          queryClient.invalidateQueries({ queryKey: queryKeys.analytics });
          queryClient.invalidateQueries({ queryKey: queryKeys.hourlyStats });
          toast.success("refresh queued", { description: "Dashboard, analytics and hourly stats" });
        },
      },
      {
        id: "action:refresh-logs",
        section: "Actions",
        title: "Refresh request logs",
        subtitle: "Reload recent usage records",
        keywords: "reload usage telemetry records requests",
        icon: RefreshCw,
        badge: "refresh",
        priority: 70,
        run: () => refreshData("Usage logs", queryKeys.usage),
      }
    );

    pageCommands
      .filter((page) => includesRole(role, page.roles))
      .forEach((page, index) => {
        items.push({
          id: `page:${page.href}`,
          section: "Pages",
          title: page.label,
          subtitle: `Open ${page.href}`,
          keywords: `${page.code} ${page.keywords}`,
          icon: page.icon,
          badge: page.code,
          priority: 50 - index,
          run: () => navigate(page.href),
        });
      });

    providersQuery.data?.forEach((provider) => {
      items.push({
        id: `provider:${provider.name}`,
        section: "Providers",
        title: provider.name,
        subtitle: `${provider.provider} · ${provider.enabled ? "enabled" : "disabled"} · weight ${provider.weight}`,
        keywords: `${provider.provider} ${provider.customHost ?? ""} api key upstream model`,
        icon: MessagesSquare,
        badge: provider.enabled ? "enabled" : "off",
        run: () => navigate("/providers", `Provider: ${provider.name}`),
      });
    });

    virtualKeysQuery.data?.forEach((key) => {
      items.push({
        id: `virtual-key:${key.id}`,
        section: "Virtual Keys",
        title: key.name,
        subtitle: `${key.status} · ${formatCost(key.budget_used)} / ${formatCost(key.budget_total)} · ${key.key_hash_prefix}`,
        keywords: `${key.status} ${key.created_by_username ?? ""} ${key.providers ?? ""} budget rate limit token credential`,
        icon: KeyRound,
        badge: key.status,
        run: () => navigate("/virtual-keys", `Virtual key: ${key.name}`),
      });
    });

    usageQuery.data?.records?.forEach((log) => {
      items.push({
        id: `log:${log.id}`,
        section: "Logs",
        title: log.request_id || `request #${log.id}`,
        subtitle: `${log.provider} · ${log.model || "unknown model"} · ${log.status_code} · ${formatTime(log.created_at)}`,
        keywords: `${log.virtual_key_name ?? ""} ${log.endpoint} ${log.error_message ?? ""} ${log.model_input_preview ?? ""}`,
        icon: FileText,
        badge: String(log.status_code),
        run: () => navigate("/logs", `Request: ${log.request_id || log.id}`),
      });
    });

    usersQuery.data?.users?.forEach((user) => {
      items.push({
        id: `user:${user.id}`,
        section: "Users",
        title: user.username,
        subtitle: `${user.role.replace(/_/g, " ")} · ${user.status}${user.email ? ` · ${user.email}` : ""}`,
        keywords: `${user.email ?? ""} ${user.role} ${user.status} account member`,
        icon: Users,
        badge: user.status,
        run: () => navigate("/users", `User: ${user.username}`),
      });
    });

    tenantsQuery.data?.tenants?.forEach((tenant) => {
      items.push({
        id: `tenant:${tenant.id}`,
        section: "Tenants",
        title: tenant.name,
        subtitle: `${tenant.slug} · ${tenant.status}`,
        keywords: `${tenant.slug} ${tenant.status} workspace organization`,
        icon: Building2,
        badge: tenant.status,
        run: () => navigate("/tenants", `Tenant: ${tenant.name}`),
      });
    });

    return items;
  }, [
    canCreateKey,
    canManageProviders,
    canManageTenants,
    canManageUsers,
    navigate,
    providersQuery.data,
    queryClient,
    refreshData,
    role,
    tenantsQuery.data,
    usageQuery.data,
    usersQuery.data,
    virtualKeysQuery.data,
  ]);

  const groupedItems = useMemo(() => {
    return sectionOrder
      .map((section) => {
        const items = allItems
          .map((item) => ({ item, score: scoreItem(item, query) }))
          .filter(({ item, score }) => item.section === section && score >= 0)
          .sort((a, b) => b.score - a.score || a.item.title.localeCompare(b.item.title))
          .slice(0, sectionLimit[section])
          .map(({ item }) => item);
        return { section, items };
      })
      .filter((group) => group.items.length > 0);
  }, [allItems, query]);

  const flatItems = useMemo(
    () => groupedItems.flatMap((group) => group.items),
    [groupedItems]
  );

  const isLoading =
    open &&
    [providersQuery, virtualKeysQuery, usageQuery, usersQuery, tenantsQuery].some(
      (q) => q.isFetching && !q.data
    );

  useEffect(() => {
    const onKeyDown = (event: KeyboardEvent) => {
      if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === "k") {
        event.preventDefault();
        onOpenChange(!open);
      }
      if (event.key === "Escape" && open) {
        onOpenChange(false);
      }
    };
    window.addEventListener("keydown", onKeyDown);
    return () => window.removeEventListener("keydown", onKeyDown);
  }, [onOpenChange, open]);

  useEffect(() => {
    if (!open) return;
    const frame = requestAnimationFrame(() => {
      setQuery("");
      setActiveIndex(0);
      inputRef.current?.focus();
    });
    return () => cancelAnimationFrame(frame);
  }, [open]);

  const activeItemIndex = Math.min(activeIndex, Math.max(0, flatItems.length - 1));

  function execute(item: CommandItem | undefined) {
    if (!item) return;
    onOpenChange(false);
    setQuery("");
    item.run();
  }

  function handleKeyDown(event: React.KeyboardEvent<HTMLInputElement>) {
    if (event.key === "ArrowDown") {
      event.preventDefault();
      setActiveIndex((current) => (flatItems.length ? (current + 1) % flatItems.length : 0));
    } else if (event.key === "ArrowUp") {
      event.preventDefault();
      setActiveIndex((current) =>
        flatItems.length ? (current - 1 + flatItems.length) % flatItems.length : 0
      );
    } else if (event.key === "Enter") {
      event.preventDefault();
      execute(flatItems[activeItemIndex]);
    }
  }

  if (!open) return null;

  const activeItem = flatItems[activeItemIndex];

  return (
    <div
      className="fixed inset-0 z-50 flex items-start justify-center bg-background/70 px-4 pt-20 backdrop-blur-sm sm:pt-24"
      role="dialog"
      aria-modal="true"
      aria-label="Command palette"
      onClick={() => onOpenChange(false)}
    >
      <div
        className="w-full max-w-2xl overflow-hidden rounded-sm border border-border-bright bg-card shadow-xl"
        onClick={(event) => event.stopPropagation()}
      >
        <div className="flex items-center gap-2 border-b border-border px-3 py-2">
          <Search className="h-4 w-4 text-muted-foreground" />
          <Input
            ref={inputRef}
            value={query}
            onChange={(event) => {
              setQuery(event.target.value);
              setActiveIndex(0);
            }}
            onKeyDown={handleKeyDown}
            placeholder="Search pages, providers, keys, logs, users, actions..."
            aria-label="Search commands"
            aria-controls="command-palette-results"
            aria-activedescendant={activeItem ? `palette-option-${activeItem.id}` : undefined}
            className="h-9 border-0 bg-transparent px-0 font-mono text-sm shadow-none focus-visible:ring-0 focus-visible:ring-offset-0"
          />
          <kbd className="hidden items-center gap-0.5 rounded-sm border border-border bg-secondary/60 px-1.5 py-0.5 font-mono text-[9px] text-muted-foreground sm:flex">
            esc
          </kbd>
          <Button
            type="button"
            variant="ghost"
            size="icon-sm"
            aria-label="Close command palette"
            onClick={() => onOpenChange(false)}
          >
            <X className="h-4 w-4" />
          </Button>
        </div>

        <div
          id="command-palette-results"
          role="listbox"
          aria-label="Command results"
          className="max-h-[70vh] overflow-y-auto p-1.5"
        >
          {isLoading && flatItems.length === 0 ? (
            <div className="space-y-1.5 p-1.5" aria-busy="true" aria-label="Loading commands">
              {Array.from({ length: 6 }).map((_, index) => (
                <Skeleton key={index} className="h-12 rounded-sm" />
              ))}
            </div>
          ) : flatItems.length === 0 ? (
            <EmptyState
              title="no commands found"
              description="Try a page name, provider, virtual key, request id, user, tenant, or action like refresh."
              icon={<Search className="h-5 w-5" />}
              className="border-0 py-10"
            />
          ) : (
            groupedItems.map((group) => (
              <div key={group.section} className="py-1">
                <div className="px-2 py-1 font-mono text-[9px] uppercase tracking-[0.2em] text-muted-foreground/60">
                  {group.section}
                </div>
                <div className="space-y-0.5">
                  {group.items.map((item) => {
                    const absoluteIndex = flatItems.findIndex((candidate) => candidate.id === item.id);
                    const active = activeItemIndex === absoluteIndex;
                    const Icon = item.icon;
                    return (
                      <button
                        key={item.id}
                        id={`palette-option-${item.id}`}
                        type="button"
                        role="option"
                        aria-selected={active}
                        className={cn(
                          "flex w-full items-center gap-3 rounded-sm px-3 py-2.5 text-left transition-colors",
                          active
                            ? "bg-secondary text-foreground"
                            : "text-muted-foreground hover:bg-secondary/60 hover:text-foreground"
                        )}
                        onMouseEnter={() => setActiveIndex(absoluteIndex)}
                        onClick={() => execute(item)}
                      >
                        <span className="flex h-7 w-7 shrink-0 items-center justify-center rounded-sm border border-border bg-background/60">
                          <Icon className="h-3.5 w-3.5" />
                        </span>
                        <span className="min-w-0 flex-1">
                          <span className="block truncate font-mono text-[13px] text-foreground">
                            {item.title}
                          </span>
                          <span className="block truncate font-mono text-[11px] text-muted-foreground/70">
                            {item.subtitle}
                          </span>
                        </span>
                        {item.badge && (
                          <Badge variant="outline" className="shrink-0">
                            {item.badge}
                          </Badge>
                        )}
                      </button>
                    );
                  })}
                </div>
              </div>
            ))
          )}
        </div>

        <div className="flex items-center justify-between border-t border-border px-3 py-2 font-mono text-[10px] text-muted-foreground/70">
          <span>↑↓ select · enter run · esc close</span>
          <span>{flatItems.length} results</span>
        </div>
      </div>
    </div>
  );
}
