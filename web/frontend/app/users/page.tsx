"use client";

import { Suspense, useMemo, useState } from "react";
import { useSearchParams } from "next/navigation";
import { useQuery } from "@tanstack/react-query";
import { api } from "@/lib/api";
import { useSession } from "@/lib/use-session";
import {
  useTenants,
  useUsers,
  useCreateUser,
  useUpdateUserQuota,
  useSetUserStatus,
  useSetUserRole,
} from "@/lib/queries";
import type { TenantUser, UserCostResponse } from "@/lib/types";
import { PageHeader } from "@/components/ui/page-header";
import { Panel, PanelHeader, PanelBody } from "@/components/ui/panel";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Skeleton } from "@/components/ui/skeleton";
import { EmptyState } from "@/components/ui/empty-state";
import {
  Sheet,
  SheetContent,
  SheetHeader,
  SheetTitle,
  SheetDescription,
  SheetTrigger,
  SheetFooter,
} from "@/components/ui/sheet";
import { cn } from "@/lib/utils";
import { useViewMode } from "@/lib/use-view-mode";
import { ViewToggle } from "@/components/ui/view-toggle";
import { toast } from "sonner";
import { Power, UserPlus, Users, Wallet, Loader2, Pencil, ShieldCheck } from "lucide-react";

const EMAIL_RE = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;

function roleLabel(role: string) {
  return role.replace(/_/g, " ");
}

function fmtUSD(n: number) {
  return `$${n.toFixed(4)}`;
}

export default function UsersPage() {
  // useSearchParams needs a Suspense boundary for static export.
  return (
    <Suspense fallback={<Skeleton className="h-64" />}>
      <UsersPageContent />
    </Suspense>
  );
}

function UsersPageContent() {
  const session = useSession();
  const role = session.role;
  const tenantId = session.tenantId;
  // `null` until the session resolves; keep the user-visible role gating in
  // the "loading" branch instead of denying access during hydration.
  const sessionReady = session !== null && (session.role !== null || session.user !== null);
  const isSuperAdmin = role === "super_admin";
  const [open, setOpen] = useState(false);
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [newRole, setNewRole] = useState("tenant_user");
  const [newBudget, setNewBudget] = useState("");
  const [newRpm, setNewRpm] = useState("");
  const [selectedTenant, setSelectedTenant] = useState("");
  const searchParams = useSearchParams();
  const [filterTenant, setFilterTenant] = useState(
    () => searchParams.get("tenant") ?? ""
  );

  // quota edit sheet
  const [quotaUser, setQuotaUser] = useState<TenantUser | null>(null);
  const [quotaBudget, setQuotaBudget] = useState("");
  const [quotaRpm, setQuotaRpm] = useState("");

  // cost sheet
  const [costUser, setCostUser] = useState<TenantUser | null>(null);

  // role edit sheet
  const [roleUser, setRoleUser] = useState<TenantUser | null>(null);
  const [roleValue, setRoleValue] = useState("tenant_user");
  const currentUsername = session.user?.username;

  const [viewMode, setViewMode] = useViewMode("members");

  const canManageUsers = role === "super_admin" || role === "tenant_admin";


  const { data: tenantsData } = useTenants();
  const tenants = useMemo(() => tenantsData ?? [], [tenantsData]);
  const tenantNameById = useMemo(
    () => new Map(tenants.map((t) => [t.id, t.name])),
    [tenants]
  );
  const tenantBudgetById = useMemo(
    () => new Map(tenants.map((t) => [t.id, t.budget_monthly])),
    [tenants]
  );

  const activeTenantId =
    isSuperAdmin && filterTenant ? Number(filterTenant) : tenantId ?? undefined;
  const { data, isLoading } = useUsers(activeTenantId);
  const users = useMemo(() => data ?? [], [data]);

  // Sum of member quotas for the scope — used to show remaining allocatable budget.
  const allocatedSum = useMemo(
    () => users.reduce((acc, u) => acc + (u.budget_monthly || 0), 0),
    [users]
  );
  const scopeBudget = isSuperAdmin
    ? filterTenant
      ? tenantBudgetById.get(Number(filterTenant)) ?? 0
      : 0
    : tenantId
      ? tenantBudgetById.get(tenantId) ?? 0
      : 0;
  const remainingBudget =
    scopeBudget > 0 ? scopeBudget - allocatedSum : 0;

  const createUser = useCreateUser();
  const setStatus = useSetUserStatus();
  const setUserRole = useSetUserRole();
  const updateQuota = useUpdateUserQuota();

  async function handleCreateUser(e: React.FormEvent) {
    e.preventDefault();
    const trimmedEmail = email.trim().toLowerCase();
    if (!trimmedEmail || !password) return;
    if (!EMAIL_RE.test(trimmedEmail)) {
      toast.error("invalid email", {
        description: "use an address like user123456@llmgw.org",
      });
      return;
    }
    // tenant_admin is always pinned to its own tenant; only super_admin picks.
    const tid = isSuperAdmin ? Number(selectedTenant) : tenantId ?? undefined;
    if (isSuperAdmin && !tid) return;
    try {
      await createUser.mutateAsync({
        email: trimmedEmail,
        password,
        role: isSuperAdmin ? newRole : "tenant_user",
        tenant_id: tid,
        budget_monthly: Number(newBudget) || 0,
        rpm_limit: Number(newRpm) || 0,
      });
      toast.success("member created", { description: trimmedEmail });
      setEmail("");
      setPassword("");
      setNewBudget("");
      setNewRpm("");
      setOpen(false);
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      toast.error(message.toLowerCase().includes("already taken") ? "email already in use" : "failed to create member", {
        description: message,
      });
    }
  }

  function openQuota(user: TenantUser) {
    setQuotaUser(user);
    setQuotaBudget(user.budget_monthly ? String(user.budget_monthly) : "");
    setQuotaRpm(user.rpm_limit ? String(user.rpm_limit) : "");
  }

  async function handleSaveQuota(e: React.FormEvent) {
    e.preventDefault();
    if (!quotaUser) return;
    try {
      await updateQuota.mutateAsync({
        id: quotaUser.id,
        data: {
          budget_monthly: Number(quotaBudget) || 0,
          rpm_limit: Number(quotaRpm) || 0,
        },
      });
      toast.success("quota updated", { description: quotaUser.username });
      setQuotaUser(null);
    } catch (err) {
      toast.error("failed to update quota", {
        description: err instanceof Error ? err.message : String(err),
      });
    }
  }

  function openRole(user: TenantUser) {
    setRoleUser(user);
    setRoleValue(user.role === "tenant_admin" ? "tenant_admin" : "tenant_user");
  }

  async function handleSaveRole(e: React.FormEvent) {
    e.preventDefault();
    if (!roleUser) return;
    try {
      await setUserRole.mutateAsync({
        id: roleUser.id,
        role: roleValue,
        tenantId: roleUser.tenant_id,
      });
      toast.success("role updated", {
        description: `${roleUser.username} → ${roleLabel(roleValue)}`,
      });
      setRoleUser(null);
    } catch (err) {
      toast.error("failed to update role", {
        description: err instanceof Error ? err.message : String(err),
      });
    }
  }

  async function handleToggle(user: TenantUser) {
    const next = user.status === "active" ? "disabled" : "active";
    try {
      await setStatus.mutateAsync({ id: user.id, status: next });
      toast.success(`user ${next}`, { description: user.username });
    } catch (err) {
      toast.error("failed to update user", {
        description: err instanceof Error ? err.message : String(err),
      });
    }
  }

  if (!canManageUsers && role != null) {
    return (
      <EmptyState
        title="access denied"
        description="User management requires tenant_admin or super_admin."
        icon={<Users className="h-5 w-5" />}
      />
    );
  }

  return (
    <div className="scan-in">
      <PageHeader
        code="07 / members"
        title="Members"
        description={
          isSuperAdmin
            ? "Member management across tenants with per-user monthly quotas"
            : "Manage your tenant members and allocate monthly quotas"
        }
        actions={
          <div className="flex items-center gap-2">
            <ViewToggle value={viewMode} onChange={setViewMode} />
            <Sheet open={open} onOpenChange={setOpen}>
            <SheetTrigger asChild>
              <Button size="sm">
                <UserPlus className="h-3.5 w-3.5" />
                new member
              </Button>
            </SheetTrigger>
            <SheetContent side="right" className="w-full sm:max-w-md">
              <SheetHeader className="space-y-2 border-b border-border pb-4">
                <SheetTitle className="font-mono text-sm uppercase tracking-wider text-primary">
                  create member
                </SheetTitle>
                <SheetDescription className="font-mono text-[12px] text-muted-foreground">
                  {isSuperAdmin
                    ? "Create a tenant admin or tenant user scoped to a tenant."
                    : "Create a tenant_user in your tenant."}
                </SheetDescription>
              </SheetHeader>
              <form onSubmit={handleCreateUser} className="flex h-full flex-col gap-4 overflow-y-auto p-4">
                {isSuperAdmin && (
                  <div className="space-y-2">
                    <Label htmlFor="tenant">tenant</Label>
                    <select
                      id="tenant"
                      value={selectedTenant}
                      onChange={(e) => setSelectedTenant(e.target.value)}
                      className="h-9 w-full rounded-sm border border-input bg-background px-3 font-mono text-[13px]"
                      required
                    >
                      <option value="">select tenant</option>
                      {tenants.map((tenant) => (
                        <option key={tenant.id} value={tenant.id}>
                          {tenant.name} · #{tenant.id}
                        </option>
                      ))}
                    </select>
                  </div>
                )}
                <div className="space-y-2">
                  <Label htmlFor="email">email</Label>
                  <Input
                    id="email"
                    type="email"
                    value={email}
                    onChange={(e) => setEmail(e.target.value)}
                    placeholder="user123456@llmgw.org"
                    autoComplete="off"
                    autoFocus
                    required
                  />
                  <p className="font-mono text-[10px] text-muted-foreground/60">
                    Globally unique — used to sign in.
                  </p>
                </div>
                <div className="space-y-2">
                  <Label htmlFor="password">password</Label>
                  <Input
                    id="password"
                    type="password"
                    value={password}
                    onChange={(e) => setPassword(e.target.value)}
                    required
                  />
                </div>
                {isSuperAdmin && (
                  <div className="space-y-2">
                    <Label htmlFor="role">role</Label>
                    <select
                      id="role"
                      value={newRole}
                      onChange={(e) => setNewRole(e.target.value)}
                      className="h-9 w-full rounded-sm border border-input bg-background px-3 font-mono text-[13px]"
                    >
                      <option value="tenant_user">tenant_user</option>
                      <option value="tenant_admin">tenant_admin</option>
                    </select>
                  </div>
                )}
                <div className="grid grid-cols-2 gap-3">
                  <div className="space-y-2">
                    <Label htmlFor="budget">monthly quota ($)</Label>
                    <Input
                      id="budget"
                      type="number"
                      min="0"
                      step="0.01"
                      placeholder="0 = unlimited"
                      value={newBudget}
                      onChange={(e) => setNewBudget(e.target.value)}
                    />
                  </div>
                  <div className="space-y-2">
                    <Label htmlFor="rpm">rpm limit</Label>
                    <Input
                      id="rpm"
                      type="number"
                      min="0"
                      placeholder="0 = unlimited"
                      value={newRpm}
                      onChange={(e) => setNewRpm(e.target.value)}
                    />
                  </div>
                </div>
                {scopeBudget > 0 && (
                  <div className="rounded-sm border border-border bg-secondary/40 px-3 py-2 font-mono text-[11px]">
                    <span className="text-muted-foreground">allocated: </span>
                    <span className="num text-foreground">{fmtUSD(allocatedSum)}</span>
                    <span className="text-muted-foreground"> / {fmtUSD(scopeBudget)} · remaining: </span>
                    <span className={cn("num", remainingBudget < 0 ? "text-destructive" : "text-primary")}>
                      {fmtUSD(remainingBudget)}
                    </span>
                  </div>
                )}
                <SheetFooter className="mt-auto flex-row gap-2 border-t border-border pt-4">
                  <Button type="button" variant="outline" size="sm" onClick={() => setOpen(false)}>
                    cancel
                  </Button>
                  <Button
                    type="submit"
                    size="sm"
                    disabled={
                      createUser.isPending ||
                      !email.trim() ||
                      !password ||
                      (isSuperAdmin && !selectedTenant)
                    }
                  >
                    {createUser.isPending ? "creating…" : "create member"}
                  </Button>
                </SheetFooter>
              </form>
            </SheetContent>
            </Sheet>
          </div>
        }
      />

      {isSuperAdmin && (
        <Panel className="mb-4">
          <PanelBody>
            <div className="flex flex-col gap-2 sm:max-w-xs">
              <Label htmlFor="filter-tenant">filter tenant</Label>
              <select
                id="filter-tenant"
                value={filterTenant}
                onChange={(e) => setFilterTenant(e.target.value)}
                className="h-9 rounded-sm border border-input bg-background px-3 font-mono text-[13px]"
              >
                <option value="">all tenants</option>
                {tenants.map((tenant) => (
                  <option key={tenant.id} value={tenant.id}>
                    {tenant.name} · #{tenant.id}
                  </option>
                ))}
              </select>
            </div>
          </PanelBody>
        </Panel>
      )}

      {isLoading || !sessionReady ? (
        <div className="space-y-2">
          {Array.from({ length: 5 }).map((_, i) => (
            <Skeleton key={i} className="h-14" />
          ))}
        </div>
      ) : users.length === 0 ? (
        <EmptyState
          title="no members"
          description="Create a member to grant tenant console access."
          icon={<Users className="h-5 w-5" />}
        />
      ) : viewMode === "cards" ? (
        <div className="grid grid-cols-1 gap-4 md:grid-cols-2 xl:grid-cols-3">
          {users.map((user) => (
            <MemberCard
              key={user.id}
              user={user}
              tenantLabel={
                user.tenant_id
                  ? tenantNameById.get(user.tenant_id) ?? `tenant #${user.tenant_id}`
                  : "platform"
              }
              isSuperAdmin={isSuperAdmin}
              isSelf={user.username === currentUsername}
              canToggle={isSuperAdmin || user.role === "tenant_user"}
              statusPending={setStatus.isPending}
              onQuota={() => openQuota(user)}
              onRole={() => openRole(user)}
              onCost={() => setCostUser(user)}
              onToggle={() => handleToggle(user)}
            />
          ))}
        </div>
      ) : (
        <Panel>
          <PanelBody className="p-0">
            <div className="grid grid-cols-[minmax(0,1.4fr)_112px_minmax(0,1fr)_152px_98px_204px] items-center gap-3 border-b border-border px-4 py-2 font-mono text-[10px] uppercase tracking-wider text-muted-foreground/60">
              <span className="truncate">member</span>
              <span className="truncate">role</span>
              <span className="truncate">tenant</span>
              <span className="truncate">quota (used / cap)</span>
              <span className="truncate">created</span>
              <span className="truncate text-right">status</span>
            </div>
            {users.map((user) => {
              const active = user.status === "active";
              const canToggle = isSuperAdmin || user.role === "tenant_user";
              const budgetPct =
                user.budget_monthly > 0
                  ? Math.min(100, (user.budget_used / user.budget_monthly) * 100)
                  : 0;
              return (
                <div
                  key={user.id}
                  className="grid grid-cols-[minmax(0,1.4fr)_112px_minmax(0,1fr)_152px_98px_204px] items-center gap-3 border-b border-border/70 px-4 py-3 last:border-0"
                >
                  <div className="min-w-0">
                    <div className="truncate font-mono text-[13px] text-foreground">
                      {user.username}
                    </div>
                    <div className="truncate font-mono text-[10px] text-muted-foreground/60">
                      {user.email || `user #${user.id}`}
                    </div>
                  </div>
                  <Badge
                    variant={user.role === "tenant_admin" ? "default" : "outline"}
                    className="w-fit whitespace-nowrap"
                  >
                    {roleLabel(user.role)}
                  </Badge>
                  <span className="truncate font-mono text-[12px] text-muted-foreground">
                    {user.tenant_id ? tenantNameById.get(user.tenant_id) ?? `tenant #${user.tenant_id}` : "platform"}
                  </span>
                  <MemberQuotaCell user={user} budgetPct={budgetPct} />
                  <span className="whitespace-nowrap font-mono text-[11px] text-muted-foreground/60">
                    {fmtDate(user.created_at)}
                  </span>
                  <div className="flex items-center justify-end gap-1">
                    <Badge variant={active ? "success" : "outline"}>
                      {active ? "active" : "disabled"}
                    </Badge>
                    <MemberActions
                      user={user}
                      isSuperAdmin={isSuperAdmin}
                      isSelf={user.username === currentUsername}
                      canToggle={canToggle}
                      statusPending={setStatus.isPending}
                      onQuota={() => openQuota(user)}
                      onRole={() => openRole(user)}
                      onCost={() => setCostUser(user)}
                      onToggle={() => handleToggle(user)}
                    />
                  </div>
                </div>
              );
            })}
          </PanelBody>
        </Panel>
      )}

      {/* Quota edit sheet */}
      <Sheet open={!!quotaUser} onOpenChange={(v) => !v && setQuotaUser(null)}>
        <SheetContent side="right" className="w-full sm:max-w-md">
          <SheetHeader className="space-y-2 border-b border-border pb-4">
            <SheetTitle className="font-mono text-sm uppercase tracking-wider text-primary">
              edit quota · {quotaUser?.username}
            </SheetTitle>
            <SheetDescription className="font-mono text-[12px] text-muted-foreground">
              Monthly quota resets each calendar month. Sum of member quotas should stay within the tenant budget.
            </SheetDescription>
          </SheetHeader>
          <form onSubmit={handleSaveQuota} className="flex h-full flex-col gap-4 p-4">
            <div className="space-y-2">
              <Label htmlFor="q-budget">monthly quota ($)</Label>
              <Input
                id="q-budget"
                type="number"
                min="0"
                step="0.01"
                placeholder="0 = unlimited"
                value={quotaBudget}
                onChange={(e) => setQuotaBudget(e.target.value)}
                autoFocus
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="q-rpm">rpm limit</Label>
              <Input
                id="q-rpm"
                type="number"
                min="0"
                placeholder="0 = unlimited"
                value={quotaRpm}
                onChange={(e) => setQuotaRpm(e.target.value)}
              />
            </div>
            <SheetFooter className="mt-auto flex-row gap-2 border-t border-border pt-4">
              <Button type="button" variant="outline" size="sm" onClick={() => setQuotaUser(null)}>
                cancel
              </Button>
              <Button type="submit" size="sm" disabled={updateQuota.isPending}>
                {updateQuota.isPending ? "saving…" : "save quota"}
              </Button>
            </SheetFooter>
          </form>
        </SheetContent>
      </Sheet>

      {/* Role edit sheet (super_admin only) */}
      <Sheet open={!!roleUser} onOpenChange={(v) => !v && setRoleUser(null)}>
        <SheetContent side="right" className="w-full sm:max-w-md">
          <SheetHeader className="space-y-2 border-b border-border pb-4">
            <SheetTitle className="font-mono text-sm uppercase tracking-wider text-primary">
              change role · {roleUser?.username}
            </SheetTitle>
            <SheetDescription className="font-mono text-[12px] text-muted-foreground">
              tenant_admin can manage members, keys and quotas in this tenant. The last active tenant_admin cannot be demoted.
            </SheetDescription>
          </SheetHeader>
          <form onSubmit={handleSaveRole} className="flex h-full flex-col gap-4 p-4">
            <div className="space-y-2">
              <Label htmlFor="r-role">role</Label>
              <select
                id="r-role"
                value={roleValue}
                onChange={(e) => setRoleValue(e.target.value)}
                className="h-9 w-full rounded-sm border border-input bg-background px-3 font-mono text-[13px]"
              >
                <option value="tenant_user">tenant_user</option>
                <option value="tenant_admin">tenant_admin</option>
              </select>
            </div>
            <SheetFooter className="mt-auto flex-row gap-2 border-t border-border pt-4">
              <Button type="button" variant="outline" size="sm" onClick={() => setRoleUser(null)}>
                cancel
              </Button>
              <Button
                type="submit"
                size="sm"
                disabled={
                  setUserRole.isPending ||
                  !roleUser ||
                  roleValue === roleUser.role
                }
              >
                {setUserRole.isPending ? "saving…" : "save role"}
              </Button>
            </SheetFooter>
          </form>
        </SheetContent>
      </Sheet>

      {/* Per-user cost sheet */}
      <UserCostSheet user={costUser} onClose={() => setCostUser(null)} />
    </div>
  );
}

function fmtDate(iso: string) {
  return new Date(iso).toLocaleDateString("en-US", {
    year: "numeric",
    month: "short",
    day: "numeric",
  });
}

function MemberQuotaCell({ user, budgetPct }: { user: TenantUser; budgetPct: number }) {
  return (
    <div className="min-w-0 space-y-1">
      <div className="truncate whitespace-nowrap font-mono text-[11px]">
        <span className="num text-foreground">{fmtUSD(user.budget_used)}</span>
        <span className="text-muted-foreground">
          {" "}
          / {user.budget_monthly > 0 ? fmtUSD(user.budget_monthly) : "∞"}
        </span>
      </div>
      {/* The track is always rendered so every row keeps the same two-line
          height, which keeps the list grid aligned. */}
      <div className="h-1 w-full overflow-hidden rounded-full bg-secondary">
        {user.budget_monthly > 0 && (
          <div
            className={`h-full rounded-full ${budgetPct >= 100 ? "bg-destructive" : budgetPct >= 80 ? "bg-warning" : "bg-primary"}`}
            style={{ width: `${budgetPct}%` }}
          />
        )}
      </div>
    </div>
  );
}

function MemberActions({
  user,
  isSuperAdmin,
  isSelf,
  canToggle,
  statusPending,
  onQuota,
  onRole,
  onCost,
  onToggle,
}: {
  user: TenantUser;
  isSuperAdmin: boolean;
  isSelf: boolean;
  canToggle: boolean;
  statusPending: boolean;
  onQuota: () => void;
  onRole: () => void;
  onCost: () => void;
  onToggle: () => void;
}) {
  return (
    <>
      <Button
        variant="ghost"
        size="icon-sm"
        className="text-muted-foreground hover:text-primary"
        onClick={onQuota}
        aria-label={`Edit quota for ${user.username}`}
      >
        <Pencil className="h-3.5 w-3.5" />
      </Button>
      {isSuperAdmin && (
        <Button
          variant="ghost"
          size="icon-sm"
          className="text-muted-foreground hover:text-primary"
          onClick={onRole}
          disabled={isSelf}
          aria-label={`Change role for ${user.username}`}
        >
          <ShieldCheck className="h-3.5 w-3.5" />
        </Button>
      )}
      <Button
        variant="ghost"
        size="icon-sm"
        className="text-muted-foreground hover:text-accent"
        onClick={onCost}
        aria-label={`View cost for ${user.username}`}
      >
        <Wallet className="h-3.5 w-3.5" />
      </Button>
      <Button
        variant="ghost"
        size="icon-sm"
        className={cn(
          "text-muted-foreground",
          user.status === "active" ? "hover:text-warning" : "hover:text-success"
        )}
        onClick={onToggle}
        disabled={statusPending || !canToggle}
        aria-label={`Toggle status for ${user.username}`}
      >
        <Power className="h-3.5 w-3.5" />
      </Button>
    </>
  );
}

function MemberCard({
  user,
  tenantLabel,
  isSuperAdmin,
  isSelf,
  canToggle,
  statusPending,
  onQuota,
  onRole,
  onCost,
  onToggle,
}: {
  user: TenantUser;
  tenantLabel: string;
  isSuperAdmin: boolean;
  isSelf: boolean;
  canToggle: boolean;
  statusPending: boolean;
  onQuota: () => void;
  onRole: () => void;
  onCost: () => void;
  onToggle: () => void;
}) {
  const active = user.status === "active";
  const budgetPct =
    user.budget_monthly > 0
      ? Math.min(100, (user.budget_used / user.budget_monthly) * 100)
      : 0;
  return (
    <Panel className="flex flex-col">
      <PanelHeader>
        <div className="min-w-0">
          <div className="truncate font-mono text-[13px] text-foreground">
            {user.username}
          </div>
          <div className="truncate font-mono text-[10px] text-muted-foreground/60">
            {user.email || `user #${user.id}`}
          </div>
        </div>
        <Badge variant={active ? "success" : "outline"}>
          {active ? "active" : "disabled"}
        </Badge>
      </PanelHeader>
      <PanelBody className="flex-1 space-y-2.5 font-mono text-[11px]">
        <div className="flex items-center justify-between gap-2">
          <span className="text-muted-foreground">role</span>
          <Badge variant={user.role === "tenant_admin" ? "default" : "outline"}>
            {roleLabel(user.role)}
          </Badge>
        </div>
        <div className="flex items-center justify-between gap-2">
          <span className="text-muted-foreground">tenant</span>
          <span className="truncate text-foreground">{tenantLabel}</span>
        </div>
        <div className="space-y-1">
          <span className="text-muted-foreground">quota</span>
          <MemberQuotaCell user={user} budgetPct={budgetPct} />
        </div>
      </PanelBody>
      <div className="flex items-center justify-between border-t border-border px-4 py-2">
        <span className="font-mono text-[9px] uppercase tracking-wider text-muted-foreground/50">
          {fmtDate(user.created_at)}
        </span>
        <div className="flex items-center gap-1">
          <MemberActions
            user={user}
            isSuperAdmin={isSuperAdmin}
            isSelf={isSelf}
            canToggle={canToggle}
            statusPending={statusPending}
            onQuota={onQuota}
            onRole={onRole}
            onCost={onCost}
            onToggle={onToggle}
          />
        </div>
      </div>
    </Panel>
  );
}

function UserCostSheet({ user, onClose }: { user: TenantUser | null; onClose: () => void }) {
  const { data, isLoading } = useQuery<UserCostResponse>({
    queryKey: ["userCost", user?.id],
    queryFn: () => api.getUserCost(user!.id, { by: "model" }),
    enabled: !!user,
  });

  return (
    <Sheet open={!!user} onOpenChange={(v) => !v && onClose()}>
      <SheetContent side="right" className="w-full sm:max-w-lg">
        <SheetHeader className="space-y-2 border-b border-border pb-4">
          <SheetTitle className="font-mono text-sm uppercase tracking-wider text-primary">
            cost · {user?.username}
          </SheetTitle>
          <SheetDescription className="font-mono text-[12px] text-muted-foreground">
            Month-to-date spend by model.
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
              description="Costs appear as this member uses the gateway."
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
                    <th className="px-2 py-2">model</th>
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
                        {item.model || "—"}
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
