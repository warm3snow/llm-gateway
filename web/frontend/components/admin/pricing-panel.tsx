"use client";

import { useState } from "react";
import {
  usePricing,
  useUpsertPricing,
  useDeletePricing,
  useSyncPricing,
} from "@/lib/queries";
import type { ModelPricing } from "@/lib/types";
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
import { toast } from "sonner";
import { Plus, RefreshCw, Trash2, Pencil, Coins } from "lucide-react";
import { cn } from "@/lib/utils";
import { useViewMode } from "@/lib/use-view-mode";
import { ViewToggle } from "@/components/ui/view-toggle";

// Shared styling for the native <select>, matching the <Input> component.
const selectClass = cn(
  "flex h-9 w-full rounded-sm border border-border bg-background/50 px-3 py-1 font-mono text-[13px] text-foreground ring-offset-background",
  "focus-visible:outline-none focus-visible:border-primary/60 focus-visible:ring-1 focus-visible:ring-primary/40",
  "disabled:cursor-not-allowed disabled:opacity-50 transition-colors"
);

const PROVIDER_NAMES = ["openai", "anthropic", "gemini", "deepseek", "glm", "kimi", "groq", "mistral", "cohere", "ollama"];

function fmtPrice(n?: number) {
  if (n == null || n === 0) return "—";
  return `$${n.toFixed(8).replace(/0+$/, "").replace(/\.$/, "")}`;
}

type PricingDraft = {
  id?: number;
  provider: string;
  model: string;
  input_price: string;
  output_price: string;
  cache_read_price: string;
  cache_write_price: string;
};

const emptyDraft: PricingDraft = {
  provider: "openai",
  model: "",
  input_price: "",
  output_price: "",
  cache_read_price: "",
  cache_write_price: "",
};

export function PricingPanel() {
  const { data: pricings, isLoading } = usePricing();
  const upsertMut = useUpsertPricing();
  const deleteMut = useDeletePricing();
  const syncMut = useSyncPricing();
  const [open, setOpen] = useState(false);
  const [draft, setDraft] = useState<PricingDraft>(emptyDraft);
  const [filter, setFilter] = useState("");
  const [viewMode, setViewMode] = useViewMode("pricing");

  const rows = (pricings ?? []).filter(
    (p) =>
      !filter ||
      p.model.toLowerCase().includes(filter.toLowerCase()) ||
      p.provider.toLowerCase().includes(filter.toLowerCase())
  );

  function openCreate() {
    setDraft(emptyDraft);
    setOpen(true);
  }

  function openEdit(p: ModelPricing) {
    setDraft({
      id: p.id,
      provider: p.provider,
      model: p.model,
      input_price: p.input_price ? String(p.input_price) : "",
      output_price: p.output_price ? String(p.output_price) : "",
      cache_read_price: p.cache_read_price ? String(p.cache_read_price) : "",
      cache_write_price: p.cache_write_price ? String(p.cache_write_price) : "",
    });
    setOpen(true);
  }

  function save() {
    if (!draft.model.trim()) {
      toast.error("model is required");
      return;
    }
    const payload = {
      provider: draft.provider,
      model: draft.model.trim(),
      input_price: Number(draft.input_price) || 0,
      output_price: Number(draft.output_price) || 0,
      cache_read_price: Number(draft.cache_read_price) || 0,
      cache_write_price: Number(draft.cache_write_price) || 0,
      currency: "USD",
    };
    upsertMut.mutate(payload, {
      onSuccess: () => {
        toast.success(draft.id ? "pricing updated" : "pricing saved");
        setOpen(false);
      },
      onError: (e) => toast.error(e.message),
    });
  }

  function sync() {
    syncMut.mutate(undefined, {
      onSuccess: (res) => toast.success(res.message || `synced ${res.count} rows`),
      onError: (e) => toast.error(e.message),
    });
  }

  return (
    <div>
      <div className="mb-4 flex justify-end gap-2">
        <ViewToggle value={viewMode} onChange={setViewMode} />
        <Button variant="outline" size="sm" onClick={sync} disabled={syncMut.isPending}>
          <RefreshCw className={cn("h-3.5 w-3.5", syncMut.isPending && "animate-spin")} />
          sync portkey
        </Button>
        <Button size="sm" onClick={openCreate}>
          <Plus className="h-3.5 w-3.5" />
          add price
        </Button>
      </div>

      <Panel>
        <PanelHeader>
          <div className="flex items-center gap-2">
            <Coins className="h-3.5 w-3.5 text-accent" />
            <span className="font-mono text-[11px] uppercase tracking-[0.15em] text-muted-foreground">
              price book · {rows.length} models
            </span>
          </div>
          <Input
            value={filter}
            onChange={(e) => setFilter(e.target.value)}
            placeholder="filter model / provider"
            className="h-8 w-56 font-mono text-[12px]"
          />
        </PanelHeader>
        <PanelBody>
          {isLoading ? (
            <div className="space-y-2">
              {Array.from({ length: 6 }).map((_, i) => (
                <Skeleton key={i} className="h-12" />
              ))}
            </div>
          ) : rows.length === 0 ? (
            <EmptyState
              title="no prices yet"
              description="Sync from Portkey or add prices manually — cost stays 0 without a matching price."
              icon={<Coins className="h-5 w-5" />}
            />
          ) : viewMode === "cards" ? (
            <div className="grid grid-cols-1 gap-3 md:grid-cols-2 xl:grid-cols-3">
              {rows.map((p) => (
                <Panel key={p.id} className="flex flex-col">
                  <PanelHeader>
                    <div className="min-w-0">
                      <div className="truncate font-mono text-[13px] text-foreground">{p.model}</div>
                      <div className="font-mono text-[10px] text-muted-foreground/60">{p.provider}</div>
                    </div>
                    <Badge variant={p.source === "manual" ? "default" : "accent"}>{p.source}</Badge>
                  </PanelHeader>
                  <PanelBody className="flex-1 space-y-2 font-mono text-[11px]">
                    {[
                      ["input", p.input_price],
                      ["output", p.output_price],
                      ["cache read", p.cache_read_price],
                      ["cache write", p.cache_write_price],
                    ].map(([label, value]) => (
                      <div key={String(label)} className="flex items-center justify-between gap-2">
                        <span className="text-muted-foreground">{String(label)}</span>
                        <span className="num text-foreground">{fmtPrice(value as number)}</span>
                      </div>
                    ))}
                  </PanelBody>
                  <div className="flex items-center justify-end gap-1 border-t border-border px-3 py-2">
                    <Button variant="ghost" size="icon-sm" onClick={() => openEdit(p)} aria-label={`Edit ${p.model}`}>
                      <Pencil className="h-3.5 w-3.5" />
                    </Button>
                    <Button
                      variant="ghost"
                      size="icon-sm"
                      onClick={() =>
                        deleteMut.mutate(p.id, {
                          onSuccess: () => toast.success("pricing deleted"),
                          onError: (e) => toast.error(e.message),
                        })
                      }
                      aria-label={`Delete ${p.model}`}
                    >
                      <Trash2 className="h-3.5 w-3.5 text-destructive" />
                    </Button>
                  </div>
                </Panel>
              ))}
            </div>
          ) : (
            <div className="overflow-x-auto">
              <table className="w-full text-[13px]">
                <thead>
                  <tr className="border-b border-border text-left font-mono text-[10px] uppercase tracking-[0.15em] text-muted-foreground">
                    <th className="px-3 py-2">provider</th>
                    <th className="px-3 py-2">model</th>
                    <th className="px-3 py-2 text-right">input</th>
                    <th className="px-3 py-2 text-right">output</th>
                    <th className="px-3 py-2 text-right">cache read</th>
                    <th className="px-3 py-2 text-right">cache write</th>
                    <th className="px-3 py-2">source</th>
                    <th className="px-3 py-2" />
                  </tr>
                </thead>
                <tbody>
                  {rows.map((p) => (
                    <tr key={p.id} className="border-b border-border/50 hover:bg-muted/30">
                      <td className="px-3 py-2 font-mono text-[12px]">{p.provider}</td>
                      <td className="px-3 py-2 font-mono text-[12px] text-primary">{p.model}</td>
                      <td className="px-3 py-2 text-right font-mono text-[12px]">{fmtPrice(p.input_price)}</td>
                      <td className="px-3 py-2 text-right font-mono text-[12px]">{fmtPrice(p.output_price)}</td>
                      <td className="px-3 py-2 text-right font-mono text-[12px]">{fmtPrice(p.cache_read_price)}</td>
                      <td className="px-3 py-2 text-right font-mono text-[12px]">{fmtPrice(p.cache_write_price)}</td>
                      <td className="px-3 py-2">
                        <Badge variant={p.source === "manual" ? "default" : "accent"}>{p.source}</Badge>
                      </td>
                      <td className="px-3 py-2">
                        <div className="flex justify-end gap-1">
                          <Button variant="ghost" size="sm" onClick={() => openEdit(p)}>
                            <Pencil className="h-3.5 w-3.5" />
                          </Button>
                          <Button
                            variant="ghost"
                            size="sm"
                            onClick={() =>
                              deleteMut.mutate(p.id, {
                                onSuccess: () => toast.success("pricing deleted"),
                                onError: (e) => toast.error(e.message),
                              })
                            }
                          >
                            <Trash2 className="h-3.5 w-3.5 text-destructive" />
                          </Button>
                        </div>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </PanelBody>
      </Panel>

      <Sheet open={open} onOpenChange={setOpen}>
        <SheetTrigger asChild>
          <span />
        </SheetTrigger>
        <SheetContent>
          <SheetHeader>
            <SheetTitle>{draft.id ? "edit pricing" : "add pricing"}</SheetTitle>
            <SheetDescription>
              Prices are per token (Portkey convention). E.g. $3/1M input tokens = 0.000003.
            </SheetDescription>
          </SheetHeader>
          <div className="grid gap-3 px-4">
            <div className="grid gap-1.5">
              <Label>provider</Label>
              <select
                className={selectClass}
                value={draft.provider}
                onChange={(e) => setDraft({ ...draft, provider: e.target.value })}
                disabled={!!draft.id}
              >
                {PROVIDER_NAMES.map((p) => (
                  <option key={p} value={p}>
                    {p}
                  </option>
                ))}
              </select>
            </div>
            <div className="grid gap-1.5">
              <Label>model</Label>
              <Input
                value={draft.model}
                onChange={(e) => setDraft({ ...draft, model: e.target.value })}
                placeholder="gpt-4o"
                disabled={!!draft.id}
              />
            </div>
            <div className="grid grid-cols-2 gap-3">
              <div className="grid gap-1.5">
                <Label>input price</Label>
                <Input
                  value={draft.input_price}
                  onChange={(e) => setDraft({ ...draft, input_price: e.target.value })}
                  placeholder="0.000003"
                />
              </div>
              <div className="grid gap-1.5">
                <Label>output price</Label>
                <Input
                  value={draft.output_price}
                  onChange={(e) => setDraft({ ...draft, output_price: e.target.value })}
                  placeholder="0.000012"
                />
              </div>
              <div className="grid gap-1.5">
                <Label>cache read price</Label>
                <Input
                  value={draft.cache_read_price}
                  onChange={(e) => setDraft({ ...draft, cache_read_price: e.target.value })}
                  placeholder="optional"
                />
              </div>
              <div className="grid gap-1.5">
                <Label>cache write price</Label>
                <Input
                  value={draft.cache_write_price}
                  onChange={(e) => setDraft({ ...draft, cache_write_price: e.target.value })}
                  placeholder="optional"
                />
              </div>
            </div>
          </div>
          <SheetFooter>
            <Button variant="outline" onClick={() => setOpen(false)}>
              cancel
            </Button>
            <Button onClick={save} disabled={upsertMut.isPending}>
              {draft.id ? "save changes" : "add pricing"}
            </Button>
          </SheetFooter>
        </SheetContent>
      </Sheet>
    </div>
  );
}
