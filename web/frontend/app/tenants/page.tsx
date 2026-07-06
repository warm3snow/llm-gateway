"use client";

import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "@/lib/api";
import { PageHeader } from "@/components/ui/page-header";
import { Panel, PanelHeader, PanelBody } from "@/components/ui/panel";
import { Led } from "@/components/ui/led";
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
import { toast } from "sonner";
import { Building2, Plus, UserPlus, Power } from "lucide-react";

export default function TenantsPage() {
  const qc = useQueryClient();
  const { data, isLoading } = useQuery({
    queryKey: ["tenants"],
    queryFn: () => api.getTenants(),
  });
  const tenants = data?.tenants ?? [];

  const createTenant = useMutation({
    mutationFn: (v: { name: string; slug: string }) => api.createTenant(v),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["tenants"] }),
  });
  const setStatus = useMutation({
    mutationFn: (v: { id: number; status: string }) =>
      api.setTenantStatus(v.id, v.status),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["tenants"] }),
  });
  const createUser = useMutation({
    mutationFn: (v: {
      username: string;
      password: string;
      tenant_id: number;
    }) => api.createTenantUser(v),
  });

  const [open, setOpen] = useState(false);
  const [name, setName] = useState("");
  const [slug, setSlug] = useState("");

  // user creation sheet
  const [userOpen, setUserOpen] = useState(false);
  const [userTenantId, setUserTenantId] = useState<number | null>(null);
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");

  async function handleCreateTenant(e: React.FormEvent) {
    e.preventDefault();
    if (!name.trim() || !slug.trim()) return;
    try {
      await createTenant.mutateAsync({ name: name.trim(), slug: slug.trim() });
      toast.success("tenant created", { description: name.trim() });
      setName("");
      setSlug("");
      setOpen(false);
    } catch (err) {
      toast.error("failed to create tenant", {
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

  async function handleCreateUser(e: React.FormEvent) {
    e.preventDefault();
    if (userTenantId == null || !username.trim() || !password) return;
    try {
      await createUser.mutateAsync({
        username: username.trim(),
        password,
        tenant_id: userTenantId,
      });
      toast.success("tenant user created", { description: username.trim() });
      setUsername("");
      setPassword("");
      setUserOpen(false);
    } catch (err) {
      toast.error("failed to create user", {
        description: err instanceof Error ? err.message : String(err),
      });
    }
  }

  return (
    <div className="scan-in">
      <PageHeader
        code="07 / tenancy"
        title="Tenants"
        description="Platform-level tenant isolation and user provisioning"
        actions={
          <Sheet open={open} onOpenChange={setOpen}>
            <SheetTrigger asChild>
              <Button size="sm">
                <Plus className="h-3.5 w-3.5" />
                new tenant
              </Button>
            </SheetTrigger>
            <SheetContent side="right" className="w-full sm:max-w-md">
              <SheetHeader className="space-y-2 border-b border-border pb-4">
                <SheetTitle className="font-mono text-sm uppercase tracking-wider text-primary">
                  create tenant
                </SheetTitle>
                <SheetDescription className="font-mono text-[12px] text-muted-foreground">
                  A tenant isolates virtual keys, usage and analytics.
                </SheetDescription>
              </SheetHeader>
              <form
                onSubmit={handleCreateTenant}
                className="flex h-full flex-col gap-4 p-4"
              >
                <div className="space-y-2">
                  <Label htmlFor="t-name">name</Label>
                  <Input
                    id="t-name"
                    placeholder="Acme Corp"
                    value={name}
                    onChange={(e) => setName(e.target.value)}
                    autoFocus
                    required
                  />
                </div>
                <div className="space-y-2">
                  <Label htmlFor="t-slug">slug</Label>
                  <Input
                    id="t-slug"
                    placeholder="acme"
                    value={slug}
                    onChange={(e) => setSlug(e.target.value)}
                    required
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
                    disabled={createTenant.isPending || !name.trim() || !slug.trim()}
                  >
                    {createTenant.isPending ? "creating…" : "create"}
                  </Button>
                </SheetFooter>
              </form>
            </SheetContent>
          </Sheet>
        }
      />

      {isLoading ? (
        <div className="grid grid-cols-1 gap-4 md:grid-cols-2 xl:grid-cols-3">
          {Array.from({ length: 3 }).map((_, i) => (
            <Skeleton key={i} className="h-36" />
          ))}
        </div>
      ) : tenants.length === 0 ? (
        <EmptyState
          title="no tenants"
          description="Create a tenant to start isolating resources."
          icon={<Building2 className="h-5 w-5" />}
        />
      ) : (
        <div className="grid grid-cols-1 gap-4 md:grid-cols-2 xl:grid-cols-3">
          {tenants.map((t) => {
            const active = t.status === "active";
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
                <PanelBody className="flex-1" />
                <div className="flex items-center justify-between border-t border-border px-4 py-2.5">
                  <span className="font-mono text-[9px] uppercase tracking-wider text-muted-foreground/50">
                    {new Date(t.created_at).toLocaleDateString("en-US", {
                      year: "numeric",
                      month: "short",
                      day: "numeric",
                    })}
                  </span>
                  <div className="flex items-center gap-1">
                    <Button
                      variant="ghost"
                      size="icon-sm"
                      className="text-muted-foreground hover:text-primary"
                      onClick={() => {
                        setUserTenantId(t.id);
                        setUserOpen(true);
                      }}
                      aria-label="Add user"
                    >
                      <UserPlus className="h-3.5 w-3.5" />
                    </Button>
                    <Button
                      variant="ghost"
                      size="icon-sm"
                      className="text-muted-foreground hover:text-warning"
                      onClick={() => handleToggle(t.id, t.status)}
                      disabled={setStatus.isPending || t.id === 1}
                      aria-label="Toggle status"
                    >
                      <Power className="h-3.5 w-3.5" />
                    </Button>
                  </div>
                </div>
              </Panel>
            );
          })}
        </div>
      )}

      {/* Create-user sheet */}
      <Sheet open={userOpen} onOpenChange={setUserOpen}>
        <SheetContent side="right" className="w-full sm:max-w-md">
          <SheetHeader className="space-y-2 border-b border-border pb-4">
            <SheetTitle className="font-mono text-sm uppercase tracking-wider text-primary">
              add tenant admin
            </SheetTitle>
            <SheetDescription className="font-mono text-[12px] text-muted-foreground">
              Creates a tenant_admin login scoped to tenant #{userTenantId}.
            </SheetDescription>
          </SheetHeader>
          <form
            onSubmit={handleCreateUser}
            className="flex h-full flex-col gap-4 p-4"
          >
            <div className="space-y-2">
              <Label htmlFor="u-name">username</Label>
              <Input
                id="u-name"
                value={username}
                onChange={(e) => setUsername(e.target.value)}
                autoFocus
                required
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="u-pass">password</Label>
              <Input
                id="u-pass"
                type="password"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                required
              />
            </div>
            <SheetFooter className="mt-auto flex-row gap-2 border-t border-border pt-4">
              <Button
                type="button"
                variant="outline"
                size="sm"
                onClick={() => setUserOpen(false)}
              >
                cancel
              </Button>
              <Button
                type="submit"
                size="sm"
                disabled={
                  createUser.isPending || !username.trim() || !password
                }
              >
                {createUser.isPending ? "creating…" : "create user"}
              </Button>
            </SheetFooter>
          </form>
        </SheetContent>
      </Sheet>
    </div>
  );
}
