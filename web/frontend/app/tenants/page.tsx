"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { useQuery } from "@tanstack/react-query";
import { api } from "@/lib/api";
import {
  useTenants,
  useCreateTenant,
  useUpdateTenant,
  useSetTenantStatus,
} from "@/lib/queries";
import type { Tenant, TenantCostResponse } from "@/lib/types";
import { PageHeader } from "@/components/ui/page-header";
import { Panel, PanelHeader, PanelBody } from "@/components/ui/panel";
import { Led } from "@/components/ui/led";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Skeleton } from "@/components/ui/skeleton";
import { EmptyState } from "@/components/ui/empty-state";
import { Switch } from "@/components/ui/switch";
import {
  Sheet,
  SheetContent,
  SheetHeader,
  SheetTitle,
  SheetDescription,
  SheetTrigger,
  SheetFooter,
} from "@/components/ui/sheet";
import { RoleGate } from "@/components/auth/RoleGate";
import { useViewMode } from "@/lib/use-view-mode";
import { ViewToggle } from "@/components/ui/view-toggle";
import { toast } from "sonner";
import { Building2, Plus, UserPlus, Power, Pencil, Wallet, Loader2 } from "lucide-react";

type TenantDraft = {
  name: string;
  slug: string;
  budget_monthly: string;
  rpm_limit: string;
  allowed_models: string;
  allow_self_key: boolean;
};

const emptyDraft: TenantDraft = {
  name: "",
  slug: "",
  budget_monthly: "",
  rpm_limit: "",
  allowed_models: "",
  allow_self_key: false,
};

function draftFromTenant(t: Tenant): TenantDraft {
  return {
    name: t.name,
    slug: t.slug,
    budget_monthly: t.budget_monthly ? String(t.budget_monthly) : "",
    rpm_limit: t.rpm_limit ? String(t.rpm_limit) : "",
    allowed_models: (t.allowed_models ?? []).join(","),
    allow_self_key: t.allow_self_key,
  };
}

function fmtUSD(n: number) {
  return `$${n.toFixed(4)}`;
}

export default function TenantsPage() {
  const { data: tenantsData, isLoading } = useTenants();
  const tenants = tenantsData ?? [];
  const router = useRouter();
  const createTenant = useCreateTenant();
  const updateTenant = useUpdateTenant();
  const setStatus = useSetTenantStatus();

  const [viewMode, setViewMode] = useViewMode("tenants");
  const [open, setOpen] = useState(false);
  const [editing, setEditing] = useState<Tenant | null>(null);
  const [draft, setDraft] = useState<TenantDraft>(emptyDraft);

  // cost sheet
  const [costTenant, setCostTenant] = useState<Tenant | null>(null);

  function openCreate() {
    setEditing(null);
    setDraft(emptyDraft);
    setOpen(true);
  }

  function openEdit(t: Tenant) {
    setEditing(t);
    setDraft(draftFromTenant(t));
    setOpen(true);
  }

  async function handleSaveTenant(e: React.FormEvent) {
    e.preventDefault();
    const payload = {
      name: draft.name.trim(),
      slug: draft.slug.trim(),
      budget_monthly: Number(draft.budget_monthly) || 0,
      rpm_limit: Number(draft.rpm_limit) || 0,
      allowed_models: draft.allowed_models
        .split(",")
        .map((s) => s.trim())
        .filter(Boolean),
      allow_self_key: draft.allow_self_key,
    };
    try {
      if (editing) {
        await updateTenant.mutateAsync({ id: editing.id, data: payload });
        toast.success("tenant updated", { description: payload.name });
      } else {
        await createTenant.mutateAsync(payload);
        toast.success("tenant created", { description: payload.name });
      }
      setOpen(false);
    } catch (err) {
      toast.error(editing ? "failed to update tenant" : "failed to create tenant", {
        description: err instanceof Error ? err.message : String(err),
      });
    }
  }

  async function handleToggle(id: number, current: string) {
    const next = current === "active" ? "disabled" : "active";
    try {
      await setStatus.mutateAsync({ id, status: next });
      toast.success(`tenant ${next}`);
    } catch (err) {
      toast.error("failed to update status", {
        description: err instanceof Error ? err.message : String(err),
      });
    }
  }

  return (
    <RoleGate allowed={["super_admin"]}>
      <div className="scan-in">
      <PageHeader
        code="06 / tenancy"
        title="Tenants"
        description="Platform-level tenant isolation, monthly budgets and cost tracking"
        actions={
          <div className="flex items-center gap-2">
            <ViewToggle value={viewMode} onChange={setViewMode} />
            <Sheet open={open} onOpenChange={setOpen}>
            <SheetTrigger asChild>
              <Button size="sm" onClick={openCreate}>
                <Plus className="h-3.5 w-3.5" />
                new tenant
              </Button>
            </SheetTrigger>
            <SheetContent side="right" className="w-full sm:max-w-md">
              <SheetHeader className="space-y-2 border-b border-border pb-4">
                <SheetTitle className="font-mono text-sm uppercase tracking-wider text-primary">
                  {editing ? "edit tenant" : "create tenant"}
                </SheetTitle>
                <SheetDescription className="font-mono text-[12px] text-muted-foreground">
                  A tenant isolates keys, usage and analytics. Monthly budget resets each calendar month (0 = unlimited).
                </SheetDescription>
              </SheetHeader>
              <form
                onSubmit={handleSaveTenant}
                className="flex h-full flex-col gap-4 overflow-y-auto p-4"
              >
                <div className="space-y-2">
                  <Label htmlFor="t-name">name</Label>
                  <Input
                    id="t-name"
                    placeholder="Acme Corp"
                    value={draft.name}
                    onChange={(e) => setDraft({ ...draft, name: e.target.value })}
                    autoFocus
                    required
                  />
                </div>
                <div className="space-y-2">
                  <Label htmlFor="t-slug">slug</Label>
                  <Input
                    id="t-slug"
                    placeholder="acme"
                    value={draft.slug}
                    onChange={(e) => setDraft({ ...draft, slug: e.target.value })}
                    disabled={!!editing}
                    required
                  />
                </div>
                <div className="grid grid-cols-2 gap-3">
                  <div className="space-y-2">
                    <Label htmlFor="t-budget">monthly budget ($)</Label>
                    <Input
                      id="t-budget"
                      type="number"
                      min="0"
                      step="0.01"
                      placeholder="0 = unlimited"
                      value={draft.budget_monthly}
                      onChange={(e) => setDraft({ ...draft, budget_monthly: e.target.value })}
                    />
                  </div>
                  <div className="space-y-2">
                    <Label htmlFor="t-rpm">rpm limit</Label>
                    <Input
                      id="t-rpm"
                      type="number"
                      min="0"
                      placeholder="0 = unlimited"
                      value={draft.rpm_limit}
                      onChange={(e) => setDraft({ ...draft, rpm_limit: e.target.value })}
                    />
                  </div>
                </div>
                <div className="space-y-2">
                  <Label htmlFor="t-models">allowed models (comma separated)</Label>
                  <Input
                    id="t-models"
                    placeholder="gpt-4o, claude-sonnet — empty = all"
                    value={draft.allowed_models}
                    onChange={(e) => setDraft({ ...draft, allowed_models: e.target.value })}
                  />
                </div>
                <div className="flex items-center justify-between rounded-sm border border-border px-3 py-2.5">
                  <div>
                    <div className="font-mono text-[12px] text-foreground">allow self-service keys</div>
                    <div className="font-mono text-[10px] text-muted-foreground/60">
                      tenant_user may create their own API keys
                    </div>
                  </div>
                  <Switch
                    checked={draft.allow_self_key}
                    onCheckedChange={(v) => setDraft({ ...draft, allow_self_key: v })}
                  />
                </div>
                <SheetFooter className="mt-auto flex-row gap-2 border-t border-border pt-4">
                  <Button
                    type="button"
                    variant="outline"
                    size="sm"
                    onClick={() => setOpen(false)}
                  >
                    cancel
                  </Button>
                  <Button
                    type="submit"
                    size="sm"
                    disabled={createTenant.isPending || updateTenant.isPending || !draft.name.trim() || !draft.slug.trim()}
                  >
                    {editing ? "save changes" : "create"}
                  </Button>
                </SheetFooter>
              </form>
            </SheetContent>
          </Sheet>
          </div>
        }
      />

      {isLoading ? (
        <div className="grid grid-cols-1 gap-4 md:grid-cols-2 xl:grid-cols-3">
          {Array.from({ length: 3 }).map((_, i) => (
            <Skeleton key={i} className="h-44" />
          ))}
        </div>
      ) : tenants.length === 0 ? (
        <EmptyState
          title="no tenants"
          description="Create a tenant to start isolating resources."
          icon={<Building2 className="h-5 w-5" />}
        />
      ) : viewMode === "list" ? (
        <Panel>
          <PanelBody className="p-0">
            <div className="grid grid-cols-[minmax(0,1.4fr)_96px_180px_72px_88px_144px] items-center gap-3 border-b border-border px-4 py-2 font-mono text-[10px] uppercase tracking-wider text-muted-foreground/60">
              <span className="truncate">tenant</span>
              <span className="truncate">status</span>
              <span className="truncate">month budget</span>
              <span className="truncate">rpm</span>
              <span className="truncate">self-serve</span>
              <span className="truncate text-right">actions</span>
            </div>
            {tenants.map((t) => {
              const active = t.status === "active";
              const budgetPct =
                t.budget_monthly > 0
                  ? Math.min(100, (t.budget_used / t.budget_monthly) * 100)
                  : 0;
              return (
                <div
                  key={t.id}
                  className="grid grid-cols-[minmax(0,1.4fr)_96px_180px_72px_88px_144px] items-center gap-3 border-b border-border/70 px-4 py-3 last:border-0"
                >
                  <div className="min-w-0">
                    <div className="truncate font-mono text-[13px] text-foreground">{t.name}</div>
                    <div className="font-mono text-[10px] text-muted-foreground/60">
                      {t.slug} · #{t.id}
                    </div>
                  </div>
                  <Badge variant={active ? "success" : "outline"} className="w-fit whitespace-nowrap">
                    {active ? "active" : "disabled"}
                  </Badge>
                  <div className="min-w-0 space-y-1">
                    <div className="truncate whitespace-nowrap font-mono text-[11px]">
                      <span className="num text-foreground">{fmtUSD(t.budget_used)}</span>
                      <span className="text-muted-foreground">
                        {" "}
                        / {t.budget_monthly > 0 ? fmtUSD(t.budget_monthly) : "∞"}
                      </span>
                    </div>
                    {/* Always rendered so every row keeps the same height. */}
                    <div className="h-1 w-full overflow-hidden rounded-full bg-secondary">
                      {t.budget_monthly > 0 && (
                        <div
                          className={`h-full rounded-full ${budgetPct >= 100 ? "bg-destructive" : budgetPct >= 80 ? "bg-warning" : "bg-primary"}`}
                          style={{ width: `${budgetPct}%` }}
                        />
                      )}
                    </div>
                  </div>
                  <span className="num whitespace-nowrap font-mono text-[11px]">
                    {t.rpm_limit > 0 ? t.rpm_limit : "∞"}
                  </span>
                  <Badge variant={t.allow_self_key ? "success" : "outline"} className="w-fit whitespace-nowrap">
                    {t.allow_self_key ? "on" : "off"}
                  </Badge>
                  <div className="flex items-center justify-end gap-1">
                    <TenantActions
                      tenant={t}
                      statusPending={setStatus.isPending}
                      onEdit={() => openEdit(t)}
                      onCost={() => setCostTenant(t)}
                      onAddUser={() => router.push(`/users?tenant=${t.id}`)}
                      onToggle={() => handleToggle(t.id, t.status)}
                    />
                  </div>
                </div>
              );
            })}
          </PanelBody>
        </Panel>
      ) : (
        <div className="grid grid-cols-1 gap-4 md:grid-cols-2 xl:grid-cols-3">
          {tenants.map((t) => {
            const active = t.status === "active";
            const budgetPct =
              t.budget_monthly > 0
                ? Math.min(100, (t.budget_used / t.budget_monthly) * 100)
                : 0;
            return (
              <Panel key={t.id} className="flex flex-col">
                <PanelHeader>
                  <div className="flex min-w-0 items-center gap-2.5">
                    <Led tone={active ? "success" : "muted"} pulse={active} />
                    <div className="min-w-0">
                      <div className="truncate font-mono text-[13px] text-foreground">
                        {t.name}
                      </div>
                      <div className="font-mono text-[10px] text-muted-foreground/60">
                        {t.slug} · #{t.id}
                      </div>
                    </div>
                  </div>
                  <Badge variant={active ? "success" : "outline"}>
                    {active ? "active" : "disabled"}
                  </Badge>
                </PanelHeader>
                <PanelBody className="flex-1 space-y-2.5 font-mono text-[11px]">
                  <div className="flex items-center justify-between">
                    <span className="flex items-center gap-1.5 text-muted-foreground">
                      <Wallet className="h-3 w-3" /> month budget
                    </span>
                    <span className="num text-foreground">
                      {t.budget_monthly > 0
                        ? `${fmtUSD(t.budget_used)} / ${fmtUSD(t.budget_monthly)}`
                        : `${fmtUSD(t.budget_used)} / ∞`}
                    </span>
                  </div>
                  {t.budget_monthly > 0 && (
                    <div className="h-1.5 w-full overflow-hidden rounded-full bg-secondary">
                      <div
                        className={`h-full rounded-full ${budgetPct >= 100 ? "bg-destructive" : budgetPct >= 80 ? "bg-warning" : "bg-primary"}`}
                        style={{ width: `${budgetPct}%` }}
                      />
                    </div>
                  )}
                  <div className="flex items-center justify-between text-muted-foreground">
                    <span>rpm</span>
                    <span className="num">{t.rpm_limit > 0 ? t.rpm_limit : "∞"}</span>
                  </div>
                  <div className="flex items-center justify-between text-muted-foreground">
                    <span>self-serve keys</span>
                    <Badge variant={t.allow_self_key ? "success" : "outline"}>
                      {t.allow_self_key ? "on" : "off"}
                    </Badge>
                  </div>
                </PanelBody>
                <div className="flex items-center justify-between border-t border-border px-4 py-2.5">
                  <span className="font-mono text-[9px] uppercase tracking-wider text-muted-foreground/50">
                    {new Date(t.created_at).toLocaleDateString("en-US", {
                      year: "numeric",
                      month: "short",
                      day: "numeric",
                    })}
                  </span>
                  <TenantActions
                    tenant={t}
                    statusPending={setStatus.isPending}
                    onEdit={() => openEdit(t)}
                    onCost={() => setCostTenant(t)}
                    onAddUser={() => router.push(`/users?tenant=${t.id}`)}
                    onToggle={() => handleToggle(t.id, t.status)}
                  />
                </div>
              </Panel>
            );
          })}
        </div>
      )}

      {/* Tenant cost sheet */}
      <TenantCostSheet tenant={costTenant} onClose={() => setCostTenant(null)} />
      </div>
    </RoleGate>
  );
}

function TenantActions({
  tenant,
  statusPending,
  onEdit,
  onCost,
  onAddUser,
  onToggle,
}: {
  tenant: Tenant;
  statusPending: boolean;
  onEdit: () => void;
  onCost: () => void;
  onAddUser: () => void;
  onToggle: () => void;
}) {
  return (
    <div className="flex items-center gap-1">
      <Button
        variant="ghost"
        size="icon-sm"
        className="text-muted-foreground hover:text-primary"
        onClick={onEdit}
        aria-label={`Edit tenant ${tenant.name}`}
      >
        <Pencil className="h-3.5 w-3.5" />
      </Button>
      <Button
        variant="ghost"
        size="icon-sm"
        className="text-muted-foreground hover:text-accent"
        onClick={onCost}
        aria-label={`View cost for ${tenant.name}`}
      >
        <Wallet className="h-3.5 w-3.5" />
      </Button>
      <Button
        variant="ghost"
        size="icon-sm"
        className="text-muted-foreground hover:text-primary"
        onClick={onAddUser}
        aria-label={`Add member to ${tenant.name}`}
      >
        <UserPlus className="h-3.5 w-3.5" />
      </Button>
      <Button
        variant="ghost"
        size="icon-sm"
        className="text-muted-foreground hover:text-warning"
        onClick={onToggle}
        disabled={statusPending || tenant.id === 1}
        aria-label={`Toggle status for ${tenant.name}`}
      >
        <Power className="h-3.5 w-3.5" />
      </Button>
    </div>
  );
}

function TenantCostSheet({ tenant, onClose }: { tenant: Tenant | null; onClose: () => void }) {
  const { data, isLoading } = useQuery<TenantCostResponse>({
    queryKey: ["tenantCost", tenant?.id],
    queryFn: () => api.getTenantCost(tenant!.id, { by: "user" }),
    enabled: !!tenant,
  });

  return (
    <Sheet open={!!tenant} onOpenChange={(v) => !v && onClose()}>
      <SheetContent side="right" className="w-full sm:max-w-lg">
        <SheetHeader className="space-y-2 border-b border-border pb-4">
          <SheetTitle className="font-mono text-sm uppercase tracking-wider text-primary">
            cost · {tenant?.name}
          </SheetTitle>
          <SheetDescription className="font-mono text-[12px] text-muted-foreground">
            Month-to-date spend by user.
          </SheetDescription>
        </SheetHeader>
        <div className="flex-1 overflow-y-auto p-4">
          {isLoading ? (
            <div className="flex justify-center py-8">
              <Loader2 className="h-5 w-5 animate-spin text-muted-foreground" />
            </div>
          ) : !data || data.items.length === 0 ? (
            <EmptyState
              title="no spend yet"
              description="Costs appear as requests flow through this tenant."
              icon={<Wallet className="h-5 w-5" />}
            />
          ) : (
            <div className="space-y-3">
              <div className="flex items-center justify-between rounded-sm border border-border bg-secondary/40 px-3 py-2 font-mono text-[12px]">
                <span className="text-muted-foreground">total · {data.total_requests} req</span>
                <span className="num text-primary">{fmtUSD(data.total_cost)}</span>
              </div>
              <table className="w-full text-[13px]">
                <thead>
                  <tr className="border-b border-border text-left font-mono text-[10px] uppercase tracking-[0.15em] text-muted-foreground">
                    <th className="px-2 py-2">user</th>
                    <th className="px-2 py-2 text-right">requests</th>
                    <th className="px-2 py-2 text-right">tokens in</th>
                    <th className="px-2 py-2 text-right">tokens out</th>
                    <th className="px-2 py-2 text-right">cost</th>
                  </tr>
                </thead>
                <tbody>
                  {data.items.map((item, i) => (
                    <tr key={i} className="border-b border-border/50">
                      <td className="px-2 py-2 font-mono text-[12px] text-foreground">
                        {item.username || `user #${item.user_id ?? "?"}`}
                      </td>
                      <td className="px-2 py-2 text-right font-mono text-[12px]">{item.requests}</td>
                      <td className="px-2 py-2 text-right font-mono text-[12px]">{item.input_tokens}</td>
                      <td className="px-2 py-2 text-right font-mono text-[12px]">{item.output_tokens}</td>
                      <td className="px-2 py-2 text-right font-mono text-[12px] text-primary">{fmtUSD(item.cost)}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </div>
      </SheetContent>
    </Sheet>
  );
}
