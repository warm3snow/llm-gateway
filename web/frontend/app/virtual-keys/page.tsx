"use client";

import { useEffect, useState } from "react";
import {
  useVirtualKeys,
  useProviders,
  useCreateVirtualKey,
  useUpdateVirtualKey,
  useDeleteVirtualKey,
} from "@/lib/queries";
import { currentRole } from "@/lib/api";
import type { VirtualKey } from "@/lib/types";
import { PageHeader } from "@/components/ui/page-header";
import { Panel, PanelHeader, PanelTitle, PanelBody } from "@/components/ui/panel";
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
import { KeyRound, Plus, Trash2, Copy, Wallet, Pencil, Gauge, Server } from "lucide-react";

function fmtUsd(n?: number) {
  if (n == null) return "0.0000";
  return n.toFixed(4);
}
function pct(used?: number, total?: number) {
  if (!total || total === 0) return 0;
  return Math.min(100, ((used ?? 0) / total) * 100);
}

function parseProviders(value?: string) {
  if (!value) return [];
  return value.split(",").map((p) => p.trim()).filter(Boolean);
}

export default function VirtualKeysPage() {
  const { data: keys, isLoading } = useVirtualKeys();
  const { data: providers } = useProviders();
  const createMut = useCreateVirtualKey();
  const updateMut = useUpdateVirtualKey();
  const deleteMut = useDeleteVirtualKey();
  const [open, setOpen] = useState(false);
  const [name, setName] = useState("");
  const [budget, setBudget] = useState("");
  const [editOpen, setEditOpen] = useState(false);
  const [editing, setEditing] = useState<VirtualKey | null>(null);
  const [editName, setEditName] = useState("");
  const [editBudget, setEditBudget] = useState("");
  const [editRateLimit, setEditRateLimit] = useState("0");
  const [editProviderNames, setEditProviderNames] = useState<string[]>([]);
  const [role, setRole] = useState<string | null>(null);

  useEffect(() => {
    setRole(currentRole());
  }, []);

  const canCreateKey = role !== null;
  const canManageKey = role !== null && role !== "tenant_user";
  const canSetBudget = role !== null && role !== "tenant_user";

  function resetForm() {
    setName("");
    setBudget("");
  }

  async function handleCreate(e: React.FormEvent) {
    e.preventDefault();
    if (!name.trim()) return;
    try {
      await createMut.mutateAsync({
        name: name.trim(),
        ...(canSetBudget ? { budget_total: parseFloat(budget) || 0 } : {}),
      });
      toast.success("virtual key created", {
        description: name.trim(),
      });
      resetForm();
      setOpen(false);
    } catch (err) {
      toast.error("failed to create key", {
        description: err instanceof Error ? err.message : String(err),
      });
    }
  }

  function openEdit(key: VirtualKey) {
    setEditing(key);
    setEditName(key.name);
    setEditBudget(String(key.budget_total ?? 0));
    setEditRateLimit(String(key.rate_limit ?? 0));
    setEditProviderNames(parseProviders(key.providers));
    setEditOpen(true);
  }

  function toggleProvider(name: string) {
    setEditProviderNames((current) =>
      current.includes(name)
        ? current.filter((p) => p !== name)
        : [...current, name]
    );
  }

  async function handleUpdate(e: React.FormEvent) {
    e.preventDefault();
    if (!editing) return;
    try {
      await updateMut.mutateAsync({
        id: editing.id,
        data: {
          name: editName.trim(),
          budget_total: parseFloat(editBudget) || 0,
          rate_limit: parseInt(editRateLimit, 10) || 0,
          rate_limit_window: 60,
          providers: editProviderNames,
        },
      });
      toast.success("virtual key updated", { description: editName.trim() });
      setEditOpen(false);
      setEditing(null);
    } catch (err) {
      toast.error("failed to update key", {
        description: err instanceof Error ? err.message : String(err),
      });
    }
  }

  async function handleDelete(id: number, name: string) {
    try {
      await deleteMut.mutateAsync(id);
      toast.success("key revoked", { description: name });
    } catch (err) {
      toast.error("failed to revoke", {
        description: err instanceof Error ? err.message : String(err),
      });
    }
  }

  return (
    <div className="scan-in">
      <PageHeader
        code="03 / auth"
        title="Virtual Keys"
        description="Issued credentials with budget and rate-limit tracking"
        actions={
          canCreateKey ? (
          <Sheet open={open} onOpenChange={setOpen}>
            <SheetTrigger asChild>
              <Button size="sm">
                <Plus className="h-3.5 w-3.5" />
                issue key
              </Button>
            </SheetTrigger>
            <SheetContent side="right" className="w-full sm:max-w-md">
              <SheetHeader className="space-y-2 border-b border-border pb-4">
                <SheetTitle className="font-mono text-sm uppercase tracking-wider text-primary">
                  issue new key
                </SheetTitle>
                <SheetDescription className="font-mono text-[12px] text-muted-foreground">
                  Creates a virtual API key with a budget cap.
                </SheetDescription>
              </SheetHeader>
              <form
                onSubmit={handleCreate}
                className="flex h-full flex-col gap-4 p-4"
              >
                <div className="space-y-2">
                  <Label htmlFor="k-name">name</Label>
                  <Input
                    id="k-name"
                    placeholder="production-app"
                    value={name}
                    onChange={(e) => setName(e.target.value)}
                    autoFocus
                    required
                  />
                </div>
                {canSetBudget && (
                  <div className="space-y-2">
                    <Label htmlFor="k-budget">budget (usd)</Label>
                    <Input
                      id="k-budget"
                      type="number"
                      placeholder="100.00"
                      value={budget}
                      onChange={(e) => setBudget(e.target.value)}
                      step="0.01"
                      min="0"
                    />
                  </div>
                )}
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
                    disabled={createMut.isPending || !name.trim()}
                  >
                    {createMut.isPending ? "issuing…" : "issue key"}
                  </Button>
                </SheetFooter>
              </form>
            </SheetContent>
          </Sheet>
          ) : undefined
        }
      />

      <Sheet open={editOpen} onOpenChange={setEditOpen}>
        <SheetContent side="right" className="w-full sm:max-w-md">
          <SheetHeader className="space-y-2 border-b border-border pb-4">
            <SheetTitle className="font-mono text-sm uppercase tracking-wider text-primary">
              edit virtual key
            </SheetTitle>
            <SheetDescription className="font-mono text-[12px] text-muted-foreground">
              Empty provider binding means all providers are allowed.
            </SheetDescription>
          </SheetHeader>
          <form onSubmit={handleUpdate} className="flex h-full flex-col gap-4 overflow-y-auto p-4">
            <div className="space-y-2">
              <Label htmlFor="ek-name">name</Label>
              <Input id="ek-name" value={editName} onChange={(e) => setEditName(e.target.value)} required />
            </div>
            <div className="grid grid-cols-2 gap-3">
              <div className="space-y-2">
                <Label htmlFor="ek-budget">budget (usd)</Label>
                <Input id="ek-budget" type="number" value={editBudget} onChange={(e) => setEditBudget(e.target.value)} step="0.01" min="0" />
              </div>
              <div className="space-y-2">
                <Label htmlFor="ek-rate">rate limit / min</Label>
                <Input id="ek-rate" type="number" value={editRateLimit} onChange={(e) => setEditRateLimit(e.target.value)} min="0" />
              </div>
            </div>
            <div className="space-y-2">
              <Label>bound providers</Label>
              <div className="rounded-sm border border-border bg-background/40 p-3 font-mono text-[12px]">
                <button
                  type="button"
                  className="mb-2 text-muted-foreground hover:text-primary"
                  onClick={() => setEditProviderNames([])}
                >
                  bind all providers
                </button>
                <div className="space-y-2">
                  {(providers || []).map((p) => (
                    <label key={p.name} className="flex items-center gap-2 text-muted-foreground">
                      <input
                        type="checkbox"
                        checked={editProviderNames.includes(p.name)}
                        onChange={() => toggleProvider(p.name)}
                      />
                      <span>{p.name}</span>
                      <span className="text-[10px] text-muted-foreground/60">{p.provider}</span>
                    </label>
                  ))}
                  {(!providers || providers.length === 0) && (
                    <span className="text-muted-foreground/60">no providers configured</span>
                  )}
                </div>
              </div>
              <p className="font-mono text-[10px] text-muted-foreground">
                current: {editProviderNames.length === 0 ? "all providers" : editProviderNames.join(", ")}
              </p>
            </div>
            <SheetFooter className="mt-auto flex-row gap-2 border-t border-border pt-4">
              <Button type="button" variant="outline" size="sm" onClick={() => setEditOpen(false)}>
                cancel
              </Button>
              <Button type="submit" size="sm" disabled={updateMut.isPending || !editName.trim()}>
                {updateMut.isPending ? "saving…" : "save changes"}
              </Button>
            </SheetFooter>
          </form>
        </SheetContent>
      </Sheet>

      {isLoading ? (
        <div className="grid grid-cols-1 gap-4 md:grid-cols-2 xl:grid-cols-3">
          {Array.from({ length: 6 }).map((_, i) => (
            <Skeleton key={i} className="h-44" />
          ))}
        </div>
      ) : !keys || keys.length === 0 ? (
        <EmptyState
          title="no keys issued"
          description="Issue a virtual key to start routing authenticated traffic."
          icon={<KeyRound className="h-5 w-5" />}
        />
      ) : (
        <div className="grid grid-cols-1 gap-4 md:grid-cols-2 xl:grid-cols-3">
          {keys.map((k) => {
            const usage = pct(k.budget_used, k.budget_total);
            const isActive = k.status === "active";
            const overBudget = usage >= 100;
            const boundProviders = parseProviders(k.providers);
            return (
              <Panel key={k.id} className="flex flex-col">
                <PanelHeader>
                  <div className="flex min-w-0 items-center gap-2.5">
                    <Led
                      tone={isActive ? (overBudget ? "warning" : "success") : "muted"}
                      pulse={isActive && !overBudget}
                    />
                    <div className="min-w-0">
                      <div className="truncate font-mono text-[13px] text-foreground">
                        {k.name}
                      </div>
                      <div className="font-mono text-[10px] text-muted-foreground/60">
                        {k.key_hash_prefix || `key #${k.id}`}
                      </div>
                    </div>
                  </div>
                  {isActive ? (
                    overBudget ? (
                      <Badge variant="warning">over budget</Badge>
                    ) : (
                      <Badge variant="success">active</Badge>
                    )
                  ) : (
                    <Badge variant="outline">revoked</Badge>
                  )}
                </PanelHeader>

                <PanelBody className="flex-1 space-y-3">
                  <div className="space-y-1.5">
                    <div className="flex items-baseline justify-between font-mono text-[11px]">
                      <span className="flex items-center gap-1.5 text-muted-foreground">
                        <Wallet className="h-3 w-3" /> budget
                      </span>
                      <span className="num">
                        <span className="text-foreground">
                          ${fmtUsd(k.budget_used)}
                        </span>
                        <span className="text-muted-foreground">
                          {" "}
                          / ${fmtUsd(k.budget_total)}
                        </span>
                      </span>
                    </div>
                    <div className="relative h-1.5 overflow-hidden rounded-sm bg-secondary">
                      <div
                        className={`absolute inset-y-0 left-0 ${
                          overBudget
                            ? "bg-warning glow-amber"
                            : "bg-primary box-glow-amber"
                        }`}
                        style={{ width: `${usage}%` }}
                      />
                    </div>
                    <div className="flex justify-end font-mono text-[10px] text-muted-foreground/60">
                      {usage.toFixed(1)}% used
                    </div>
                  </div>
                  <div className="space-y-2 font-mono text-[11px]">
                    <div className="flex items-center justify-between gap-2">
                      <span className="flex items-center gap-1.5 text-muted-foreground">
                        <Gauge className="h-3 w-3" /> rate limit
                      </span>
                      <span className="num text-foreground">
                        {k.rate_limit && k.rate_limit > 0 ? `${k.rate_limit}/min` : "unlimited"}
                      </span>
                    </div>
                    <div className="flex items-center justify-between gap-2">
                      <span className="flex items-center gap-1.5 text-muted-foreground">
                        <Server className="h-3 w-3" /> providers
                      </span>
                      <span className="truncate text-muted-foreground">
                        {boundProviders.length === 0 ? "all" : boundProviders.join(", ")}
                      </span>
                    </div>
                  </div>
                </PanelBody>

                {/* Footer — created date + actions */}
                <div className="flex items-center justify-between border-t border-border px-4 py-2.5">
                  <span className="font-mono text-[9px] uppercase tracking-wider text-muted-foreground/50">
                    {new Date(k.created_at).toLocaleDateString("en-US", {
                      year: "numeric",
                      month: "short",
                      day: "numeric",
                    })}
                  </span>
                  <div className="flex items-center gap-1">
                    {canManageKey && (
                      <Button
                        variant="ghost"
                        size="icon-sm"
                        className="text-muted-foreground hover:text-primary"
                        onClick={() => openEdit(k)}
                        aria-label="Edit key"
                      >
                        <Pencil className="h-3.5 w-3.5" />
                      </Button>
                    )}
                    <Button
                      variant="ghost"
                      size="icon-sm"
                      className="text-muted-foreground hover:text-primary"
                      onClick={() => {
                        navigator.clipboard
                          ?.writeText(k.key_hash_prefix || "")
                          .then(() => toast.success("key prefix copied"))
                          .catch(() => toast.error("clipboard unavailable"));
                      }}
                      aria-label="Copy key prefix"
                    >
                      <Copy className="h-3.5 w-3.5" />
                    </Button>
                    {canManageKey && (
                      <Button
                        variant="ghost"
                        size="icon-sm"
                        className="text-muted-foreground hover:text-destructive hover:border-destructive/40"
                        onClick={() => handleDelete(k.id, k.name)}
                        disabled={deleteMut.isPending}
                        aria-label="Revoke key"
                      >
                        <Trash2 className="h-3.5 w-3.5" />
                      </Button>
                    )}
                  </div>
                </div>
              </Panel>
            );
          })}
        </div>
      )}
    </div>
  );
}
