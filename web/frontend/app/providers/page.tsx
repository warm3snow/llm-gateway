"use client";

import { useState } from "react";
import {
  useProviders,
  useProviderHealth,
  useCreateProvider,
  useUpdateProvider,
  useDeleteProvider,
} from "@/lib/queries";
import { currentRole } from "@/lib/api";
import type { Provider } from "@/lib/types";
import { PageHeader } from "@/components/ui/page-header";
import { Panel, PanelHeader, PanelBody } from "@/components/ui/panel";
import { Led } from "@/components/ui/led";
import { Badge } from "@/components/ui/badge";
import { Skeleton } from "@/components/ui/skeleton";
import { EmptyState } from "@/components/ui/empty-state";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
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
import { Server, Plus, Cpu, Globe, Gauge, Key, Trash2, Pencil } from "lucide-react";
import { cn } from "@/lib/utils";

const PROVIDER_TYPES = [
  "openai",
  "anthropic",
  "gemini",
  "azure",
  "cohere",
  "groq",
  "deepseek",
  "mistral",
  "ollama",
  "glm",
  "kimi",
] as const;

// Shared styling for the native <select>, matching the <Input> component.
const selectClass = cn(
  "flex h-9 w-full rounded-sm border border-border bg-background/50 px-3 py-1 font-mono text-[13px] text-foreground ring-offset-background",
  "focus-visible:outline-none focus-visible:border-primary/60 focus-visible:ring-1 focus-visible:ring-primary/40",
  "disabled:cursor-not-allowed disabled:opacity-50 transition-colors"
);

function maskKey(k?: string) {
  if (!k) return "—";
  if (k.length <= 8) return k;
  return `${k.slice(0, 4)}••••${k.slice(-4)}`;
}

export default function ProvidersPage() {
  const { data: providers, isLoading } = useProviders();
  const { data: providerHealth } = useProviderHealth();
  const createMut = useCreateProvider();
  const updateMut = useUpdateProvider();
  const deleteMut = useDeleteProvider();

  const [open, setOpen] = useState(false);
  const [name, setName] = useState("");
  const [providerType, setProviderType] = useState<string>("openai");
  const [apiKey, setApiKey] = useState("");
  const [customHost, setCustomHost] = useState("");
  const [weight, setWeight] = useState("1");
  const [requestTimeout, setRequestTimeout] = useState("30000");
  const [editOpen, setEditOpen] = useState(false);
  const [editing, setEditing] = useState<Provider | null>(null);
  const [editProviderType, setEditProviderType] = useState("openai");
  const [editApiKey, setEditApiKey] = useState("");
  const [editCustomHost, setEditCustomHost] = useState("");
  const [editWeight, setEditWeight] = useState("1");
  const [editRequestTimeout, setEditRequestTimeout] = useState("30000");
  const [role] = useState<string | null>(() => currentRole());

  const canManageProviders = role === "super_admin";

  function resetForm() {
    setName("");
    setProviderType("openai");
    setApiKey("");
    setCustomHost("");
    setWeight("1");
    setRequestTimeout("30000");
  }

  async function handleCreate(e: React.FormEvent) {
    e.preventDefault();
    if (!name.trim()) return;
    try {
      await createMut.mutateAsync({
        name: name.trim(),
        provider: providerType,
        apiKey: apiKey.trim(),
        customHost: customHost.trim() || undefined,
        weight: parseInt(weight, 10) || 1,
        requestTimeout: parseInt(requestTimeout, 10) || 30000,
      });
      toast.success("provider added", { description: name.trim() });
      resetForm();
      setOpen(false);
    } catch (err) {
      toast.error("failed to add provider", {
        description: err instanceof Error ? err.message : String(err),
      });
    }
  }

  function openEdit(provider: Provider) {
    setEditing(provider);
    setEditProviderType(provider.provider || "openai");
    setEditApiKey("");
    setEditCustomHost(provider.customHost || "");
    setEditWeight(String(provider.weight || 1));
    setEditRequestTimeout(String(provider.requestTimeout || 30000));
    setEditOpen(true);
  }

  async function handleUpdate(e: React.FormEvent) {
    e.preventDefault();
    if (!editing) return;
    try {
      await updateMut.mutateAsync({
        name: editing.name,
        data: {
          provider: editProviderType,
          apiKey: editApiKey.trim() || undefined,
          customHost: editCustomHost.trim() || undefined,
          weight: parseInt(editWeight, 10) || 1,
          requestTimeout: parseInt(editRequestTimeout, 10) || 30000,
        },
      });
      toast.success("provider updated", { description: editing.name });
      setEditOpen(false);
      setEditing(null);
    } catch (err) {
      toast.error("failed to update provider", {
        description: err instanceof Error ? err.message : String(err),
      });
    }
  }

  async function handleDelete(providerName: string) {
    try {
      await deleteMut.mutateAsync(providerName);
      toast.success("provider removed", { description: providerName });
    } catch (err) {
      toast.error("failed to remove", {
        description: err instanceof Error ? err.message : String(err),
      });
    }
  }

  return (
    <div className="scan-in">
      <PageHeader
        code="02 / routing"
        title="Providers"
        description="Upstream LLM backends and their routing configuration"
        actions={
          canManageProviders ? (
          <Sheet open={open} onOpenChange={setOpen}>
            <SheetTrigger asChild>
              <Button variant="outline" size="sm">
                <Plus className="h-3.5 w-3.5" />
                add provider
              </Button>
            </SheetTrigger>
            <SheetContent side="right" className="w-full sm:max-w-md">
              <SheetHeader className="space-y-2 border-b border-border pb-4">
                <SheetTitle className="font-mono text-sm uppercase tracking-wider text-primary">
                  add provider
                </SheetTitle>
                <SheetDescription className="font-mono text-[12px] text-muted-foreground">
                  Configure an upstream LLM backend.
                </SheetDescription>
              </SheetHeader>
              <form
                onSubmit={handleCreate}
                className="flex h-full flex-col gap-4 overflow-y-auto p-4"
              >
                <div className="space-y-2">
                  <Label htmlFor="p-name">name</Label>
                  <Input
                    id="p-name"
                    placeholder="openai-primary"
                    value={name}
                    onChange={(e) => setName(e.target.value)}
                    autoFocus
                    required
                  />
                </div>
                <div className="space-y-2">
                  <Label htmlFor="p-type">provider type</Label>
                  <select
                    id="p-type"
                    className={selectClass}
                    value={providerType}
                    onChange={(e) => setProviderType(e.target.value)}
                  >
                    {PROVIDER_TYPES.map((t) => (
                      <option key={t} value={t}>
                        {t}
                      </option>
                    ))}
                  </select>
                </div>
                <div className="space-y-2">
                  <Label htmlFor="p-key">api key</Label>
                  <Input
                    id="p-key"
                    type="password"
                    placeholder="sk-..."
                    value={apiKey}
                    onChange={(e) => setApiKey(e.target.value)}
                  />
                </div>
                <div className="space-y-2">
                  <Label htmlFor="p-host">custom host (optional)</Label>
                  <Input
                    id="p-host"
                    placeholder="https://api.example.com/v1"
                    value={customHost}
                    onChange={(e) => setCustomHost(e.target.value)}
                  />
                </div>
                <div className="grid grid-cols-2 gap-3">
                  <div className="space-y-2">
                    <Label htmlFor="p-weight">weight</Label>
                    <Input
                      id="p-weight"
                      type="number"
                      value={weight}
                      onChange={(e) => setWeight(e.target.value)}
                      min="0"
                    />
                  </div>
                  <div className="space-y-2">
                    <Label htmlFor="p-timeout">timeout (ms)</Label>
                    <Input
                      id="p-timeout"
                      type="number"
                      value={requestTimeout}
                      onChange={(e) => setRequestTimeout(e.target.value)}
                      min="0"
                    />
                  </div>
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
                    {createMut.isPending ? "adding…" : "add provider"}
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
              edit provider
            </SheetTitle>
            <SheetDescription className="font-mono text-[12px] text-muted-foreground">
              API key is unchanged when left blank.
            </SheetDescription>
          </SheetHeader>
          <form onSubmit={handleUpdate} className="flex h-full flex-col gap-4 overflow-y-auto p-4">
            <div className="space-y-2">
              <Label htmlFor="ep-name">name</Label>
              <Input id="ep-name" value={editing?.name || ""} disabled />
            </div>
            <div className="space-y-2">
              <Label htmlFor="ep-type">provider type</Label>
              <select
                id="ep-type"
                className={selectClass}
                value={editProviderType}
                onChange={(e) => setEditProviderType(e.target.value)}
              >
                {PROVIDER_TYPES.map((t) => (
                  <option key={t} value={t}>{t}</option>
                ))}
              </select>
            </div>
            <div className="space-y-2">
              <Label htmlFor="ep-key">new api key (optional)</Label>
              <Input
                id="ep-key"
                type="password"
                placeholder="leave blank to keep existing"
                value={editApiKey}
                onChange={(e) => setEditApiKey(e.target.value)}
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="ep-host">custom host (optional)</Label>
              <Input
                id="ep-host"
                placeholder="https://api.example.com/v1"
                value={editCustomHost}
                onChange={(e) => setEditCustomHost(e.target.value)}
              />
            </div>
            <div className="grid grid-cols-2 gap-3">
              <div className="space-y-2">
                <Label htmlFor="ep-weight">weight</Label>
                <Input id="ep-weight" type="number" value={editWeight} onChange={(e) => setEditWeight(e.target.value)} min="0" />
              </div>
              <div className="space-y-2">
                <Label htmlFor="ep-timeout">timeout (ms)</Label>
                <Input id="ep-timeout" type="number" value={editRequestTimeout} onChange={(e) => setEditRequestTimeout(e.target.value)} min="0" />
              </div>
            </div>
            <SheetFooter className="mt-auto flex-row gap-2 border-t border-border pt-4">
              <Button type="button" variant="outline" size="sm" onClick={() => setEditOpen(false)}>
                cancel
              </Button>
              <Button type="submit" size="sm" disabled={updateMut.isPending || !editing}>
                {updateMut.isPending ? "saving…" : "save changes"}
              </Button>
            </SheetFooter>
          </form>
        </SheetContent>
      </Sheet>

      {isLoading ? (
        <div className="grid grid-cols-1 gap-4 md:grid-cols-2 xl:grid-cols-3">
          {Array.from({ length: 6 }).map((_, i) => (
            <Skeleton key={i} className="h-48" />
          ))}
        </div>
      ) : !providers || providers.length === 0 ? (
        <EmptyState
          title="no providers"
          description="Add an upstream provider to start routing requests."
          icon={<Server className="h-5 w-5" />}
        />
      ) : (
        <div className="grid grid-cols-1 gap-4 md:grid-cols-2 xl:grid-cols-3">
          {providers.map((p) => {
            const health = providerHealth?.find((h) => h.provider_name === p.name);
            const healthLabel = !p.enabled ? "disabled" : health ? health.status : "unchecked";
            const healthVariant = !p.enabled ? "outline" : health?.healthy ? "success" : health ? "destructive" : "outline";
            return (
            <Panel key={p.name} className="flex flex-col">
              {/* Header row — name + status */}
              <PanelHeader>
                <div className="flex min-w-0 items-center gap-2.5">
                  <Led
                    tone={p.enabled ? "success" : "muted"}
                    pulse={p.enabled}
                  />
                  <div className="min-w-0">
                    <div className="flex items-baseline gap-2">
                      <span className="truncate font-mono text-[13px] text-foreground">
                        {p.name}
                      </span>
                      <span className="font-mono text-[9px] uppercase tracking-wider text-muted-foreground/60">
                        {p.provider}
                      </span>
                    </div>
                  </div>
                </div>
                <Badge variant={healthVariant}>{healthLabel}</Badge>
              </PanelHeader>

              <PanelBody className="flex-1 space-y-2.5 font-mono text-[11px]">
                <div className="flex items-center justify-between gap-2">
                  <span className="flex items-center gap-1.5 text-muted-foreground">
                    <Gauge className="h-3 w-3" /> weight
                  </span>
                  <span className="num text-foreground">{p.weight}</span>
                </div>
                <div className="flex items-center justify-between gap-2">
                  <span className="flex items-center gap-1.5 text-muted-foreground">
                    <Cpu className="h-3 w-3" /> timeout
                  </span>
                  <span className="num text-foreground">
                    {p.requestTimeout}ms
                  </span>
                </div>
                {health && (
                  <div className="flex items-center justify-between gap-2">
                    <span className="text-muted-foreground">health</span>
                    <span className="truncate text-right text-muted-foreground">
                      {health.latency_ms}ms · {health.consecutive_errors} err
                    </span>
                  </div>
                )}
                {health?.error_message && (
                  <div className="truncate rounded-sm border border-destructive/30 bg-destructive/5 px-2 py-1 text-destructive">
                    {health.error_message}
                  </div>
                )}
                <div className="flex items-center justify-between gap-2">
                  <span className="flex items-center gap-1.5 text-muted-foreground">
                    <Key className="h-3 w-3" /> api key
                  </span>
                  <span className="num text-muted-foreground">
                    {maskKey(p.apiKey)}
                  </span>
                </div>
                {p.customHost && (
                  <div className="flex items-center justify-between gap-2">
                    <span className="flex items-center gap-1.5 text-muted-foreground">
                      <Globe className="h-3 w-3" /> host
                    </span>
                    <span className="truncate text-muted-foreground">
                      {p.customHost}
                    </span>
                  </div>
                )}
              </PanelBody>

              {/* Footer — terminal-style id strip + delete */}
              <div className="flex items-center justify-between border-t border-border px-4 py-2 font-mono text-[9px] uppercase tracking-wider text-muted-foreground/50">
                <span>id://{p.provider}</span>
                <div className="flex items-center gap-2">
                  <span>{p.enabled ? "configured" : "unconfigured"}</span>
                  {canManageProviders && (
                    <>
                      <Button
                        variant="ghost"
                        size="icon-sm"
                        className="text-muted-foreground hover:text-primary"
                        onClick={() => openEdit(p)}
                        aria-label={`Edit provider ${p.name}`}
                      >
                        <Pencil className="h-3.5 w-3.5" />
                      </Button>
                      <Button
                        variant="ghost"
                        size="icon-sm"
                        className="text-muted-foreground hover:text-destructive hover:border-destructive/40"
                        onClick={() => handleDelete(p.name)}
                        disabled={deleteMut.isPending || !p.enabled}
                        aria-label={`Remove provider ${p.name}`}
                      >
                        <Trash2 className="h-3.5 w-3.5" />
                      </Button>
                    </>
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
