"use client";

import { useMemo, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api, currentRole, currentTenantId } from "@/lib/api";
import type { TenantUser } from "@/lib/types";
import { PageHeader } from "@/components/ui/page-header";
import { Panel, PanelBody } from "@/components/ui/panel";
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
import { toast } from "sonner";
import { Power, UserPlus, Users } from "lucide-react";

function roleLabel(role: string) {
  return role.replace(/_/g, " ");
}

export default function UsersPage() {
  const qc = useQueryClient();
  const [role] = useState<string | null>(() => currentRole());
  const [tenantId] = useState<number | null>(() => currentTenantId());
  const [open, setOpen] = useState(false);
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [newRole, setNewRole] = useState("tenant_user");
  const [selectedTenant, setSelectedTenant] = useState(() => {
    const r = currentRole();
    const tid = currentTenantId();
    return r === "tenant_admin" && tid != null ? String(tid) : "";
  });
  const [filterTenant, setFilterTenant] = useState("");

  const isSuperAdmin = role === "super_admin";
  const canManageUsers = role === "super_admin" || role === "tenant_admin";

  const { data: tenantsData } = useQuery({
    queryKey: ["tenants"],
    queryFn: () => api.getTenants(),
    enabled: isSuperAdmin,
  });
  const tenants = tenantsData?.tenants ?? [];
  const tenantNameById = useMemo(
    () => new Map(tenants.map((t) => [t.id, t.name])),
    [tenants]
  );

  const { data, isLoading } = useQuery({
    queryKey: ["users", isSuperAdmin ? filterTenant : tenantId],
    queryFn: () => api.getUsers(isSuperAdmin && filterTenant ? Number(filterTenant) : undefined),
    enabled: canManageUsers,
  });
  const users = data?.users ?? [];

  const createUser = useMutation({
    mutationFn: (v: {
      username: string;
      password: string;
      role: string;
      tenant_id?: number;
    }) => api.createUser(v),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["users"] }),
  });
  const setStatus = useMutation({
    mutationFn: (v: { id: number; status: string }) => api.setUserStatus(v.id, v.status),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["users"] }),
  });

  async function handleCreateUser(e: React.FormEvent) {
    e.preventDefault();
    if (!username.trim() || !password) return;
    const tenant_id = isSuperAdmin ? Number(selectedTenant) : undefined;
    if (isSuperAdmin && !tenant_id) return;
    try {
      await createUser.mutateAsync({
        username: username.trim(),
        password,
        role: isSuperAdmin ? newRole : "tenant_user",
        tenant_id,
      });
      toast.success("user created", { description: username.trim() });
      setUsername("");
      setPassword("");
      setNewRole("tenant_user");
      setOpen(false);
    } catch (err) {
      toast.error("failed to create user", {
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
        code="08 / users"
        title="Users"
        description={
          isSuperAdmin
            ? "Platform user management across all tenants"
            : "Tenant user management for your organization"
        }
        actions={
          <Sheet open={open} onOpenChange={setOpen}>
            <SheetTrigger asChild>
              <Button size="sm">
                <UserPlus className="h-3.5 w-3.5" />
                new user
              </Button>
            </SheetTrigger>
            <SheetContent side="right" className="w-full sm:max-w-md">
              <SheetHeader className="space-y-2 border-b border-border pb-4">
                <SheetTitle className="font-mono text-sm uppercase tracking-wider text-primary">
                  create user
                </SheetTitle>
                <SheetDescription className="font-mono text-[12px] text-muted-foreground">
                  {isSuperAdmin
                    ? "Create a tenant admin or tenant user scoped to a tenant."
                    : "Create a read-only tenant_user in your tenant."}
                </SheetDescription>
              </SheetHeader>
              <form onSubmit={handleCreateUser} className="flex h-full flex-col gap-4 p-4">
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
                  <Label htmlFor="username">username</Label>
                  <Input
                    id="username"
                    value={username}
                    onChange={(e) => setUsername(e.target.value)}
                    autoFocus
                    required
                  />
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
                <SheetFooter className="mt-auto flex-row gap-2 border-t border-border pt-4">
                  <Button type="button" variant="outline" size="sm" onClick={() => setOpen(false)}>
                    cancel
                  </Button>
                  <Button
                    type="submit"
                    size="sm"
                    disabled={
                      createUser.isPending ||
                      !username.trim() ||
                      !password ||
                      (isSuperAdmin && !selectedTenant)
                    }
                  >
                    {createUser.isPending ? "creating…" : "create user"}
                  </Button>
                </SheetFooter>
              </form>
            </SheetContent>
          </Sheet>
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

      {isLoading || role == null ? (
        <div className="space-y-2">
          {Array.from({ length: 5 }).map((_, i) => (
            <Skeleton key={i} className="h-14" />
          ))}
        </div>
      ) : users.length === 0 ? (
        <EmptyState
          title="no users"
          description="Create a user to grant tenant console access."
          icon={<Users className="h-5 w-5" />}
        />
      ) : (
        <Panel>
          <PanelBody className="p-0">
            <div className="grid grid-cols-[1.2fr_1fr_1fr_1fr_auto] gap-3 border-b border-border px-4 py-2 font-mono text-[10px] uppercase tracking-wider text-muted-foreground/60">
              <span>username</span>
              <span>role</span>
              <span>tenant</span>
              <span>created</span>
              <span className="text-right">status</span>
            </div>
            {users.map((user) => {
              const active = user.status === "active";
              const canToggle = isSuperAdmin || user.role === "tenant_user";
              return (
                <div
                  key={user.id}
                  className="grid grid-cols-[1.2fr_1fr_1fr_1fr_auto] items-center gap-3 border-b border-border/70 px-4 py-3 last:border-0"
                >
                  <div className="min-w-0">
                    <div className="truncate font-mono text-[13px] text-foreground">
                      {user.username}
                    </div>
                    <div className="truncate font-mono text-[10px] text-muted-foreground/60">
                      {user.email || `user #${user.id}`}
                    </div>
                  </div>
                  <Badge variant={user.role === "tenant_admin" ? "default" : "outline"}>
                    {roleLabel(user.role)}
                  </Badge>
                  <span className="truncate font-mono text-[12px] text-muted-foreground">
                    {user.tenant_id ? tenantNameById.get(user.tenant_id) ?? `tenant #${user.tenant_id}` : "platform"}
                  </span>
                  <span className="font-mono text-[11px] text-muted-foreground/60">
                    {new Date(user.created_at).toLocaleDateString("en-US", {
                      year: "numeric",
                      month: "short",
                      day: "numeric",
                    })}
                  </span>
                  <div className="flex items-center justify-end gap-2">
                    <Badge variant={active ? "success" : "outline"}>
                      {active ? "active" : "disabled"}
                    </Badge>
                    <Button
                      variant="ghost"
                      size="icon-sm"
                      className={cn(
                        "text-muted-foreground",
                        active ? "hover:text-warning" : "hover:text-success"
                      )}
                      onClick={() => handleToggle(user)}
                      disabled={setStatus.isPending || !canToggle}
                      aria-label="Toggle user status"
                    >
                      <Power className="h-3.5 w-3.5" />
                    </Button>
                  </div>
                </div>
              );
            })}
          </PanelBody>
        </Panel>
      )}
    </div>
  );
}
