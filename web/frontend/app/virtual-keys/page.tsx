"use client";

import { useState } from "react";
import { useVirtualKeys, useCreateVirtualKey, useDeleteVirtualKey } from "@/lib/queries";
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
import { KeyRound, Plus, Trash2, Copy, Wallet } from "lucide-react";

function fmtUsd(n?: number) {
  if (n == null) return "0.0000";
  return n.toFixed(4);
}
function pct(used?: number, total?: number) {
  if (!total || total === 0) return 0;
  return Math.min(100, ((used ?? 0) / total) * 100);
}

export default function VirtualKeysPage() {
  const { data: keys, isLoading } = useVirtualKeys();
  const createMut = useCreateVirtualKey();
  const deleteMut = useDeleteVirtualKey();
  const [open, setOpen] = useState(false);
  const [name, setName] = useState("");
  const [budget, setBudget] = useState("");

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
        budget_total: parseFloat(budget) || 0,
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
        }
      />

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
                  {/* Budget readout */}
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
                    {/* Progress bar — phosphor fill */}
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
