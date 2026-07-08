"use client";

import { useState } from "react";
import { useUsage } from "@/lib/queries";
import { currentRole } from "@/lib/api";
import type { UsageRecord } from "@/lib/types";
import { roleScopeLabel } from "@/components/auth/RoleGate";
import { PageHeader } from "@/components/ui/page-header";
import { Panel } from "@/components/ui/panel";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { EmptyState } from "@/components/ui/empty-state";
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet";
import {
  Activity,
  Filter,
  Download,
  RotateCw,
  ChevronLeft,
  ChevronRight,
  Copy,
  Eye,
  Maximize2,
  Minimize2,
} from "lucide-react";
import { cn } from "@/lib/utils";

const PAGE_SIZE = 50;

type StatusFilter = "all" | "success" | "error";

function statusTone(code: number): "success" | "destructive" {
  return code >= 400 ? "destructive" : "success";
}

function fmtTime(iso: string) {
  const d = new Date(iso);
  return d.toLocaleTimeString("en-US", { hour12: false });
}
function fmtDate(iso: string) {
  return new Date(iso).toLocaleDateString("en-US", {
    month: "2-digit",
    day: "2-digit",
  });
}

function inputSummary(log: UsageRecord) {
  const value = (log.model_input_preview ?? "").replace(/\s+/g, " ").trim();
  if (!value) return "—";
  return value.length > 110 ? `${value.slice(0, 110)}…` : value;
}

function metaValue(value: unknown) {
  if (value == null || value === "") return "—";
  if (typeof value === "boolean") return value ? "yes" : "no";
  return String(value);
}

export default function LogsPage() {
  const [page, setPage] = useState(0);
  const [filter, setFilter] = useState<StatusFilter>("all");
  const [selected, setSelected] = useState<UsageRecord | null>(null);
  const [expanded, setExpanded] = useState(false);
  const [role] = useState<string | null>(() => currentRole());

  const { data, isLoading, refetch, isFetching } = useUsage({
    limit: PAGE_SIZE,
    offset: page * PAGE_SIZE,
  });

  const logs = data?.records ?? [];
  const total = data?.total ?? 0;
  const pageCount = Math.max(1, Math.ceil(total / PAGE_SIZE));

  // Status filter is applied client-side within the current page.
  const filtered = logs.filter((l) => {
    if (filter === "all") return true;
    if (filter === "success") return l.status_code < 400;
    if (filter === "error") return l.status_code >= 400;
    return true;
  });

  async function copyPreview() {
    if (!selected?.model_input_preview) return;
    await navigator.clipboard?.writeText(selected.model_input_preview);
  }

  const filterOptions: { key: StatusFilter; label: string; count: number }[] = [
    { key: "all", label: "all", count: logs.length },
    {
      key: "success",
      label: "ok",
      count: logs.filter((l) => l.status_code < 400).length,
    },
    {
      key: "error",
      label: "err",
      count: logs.filter((l) => l.status_code >= 400).length,
    },
  ];

  return (
    <div className="scan-in">
      <PageHeader
        code="04 / telemetry"
        title="Request Logs"
        description={`Every proxied request, in order · ${roleScopeLabel(role)}`}
        actions={
          <div className="flex items-center gap-2">
            <Button
              variant="outline"
              size="sm"
              onClick={() => refetch()}
              disabled={isFetching}
            >
              <RotateCw className={cn("h-3.5 w-3.5", isFetching && "animate-spin")} />
              refresh
            </Button>
            <Button variant="ghost" size="sm" disabled>
              <Download className="h-3.5 w-3.5" />
              export
            </Button>
          </div>
        }
      />

      {/* Filter rail */}
      <div className="mb-4 flex items-center gap-2">
        <div className="flex items-center gap-1.5 font-mono text-[10px] uppercase tracking-wider text-muted-foreground/60">
          <Filter className="h-3 w-3" />
          filter
        </div>
        <div className="flex items-center gap-1 rounded-sm border border-border bg-card/40 p-1">
          {filterOptions.map((opt) => (
            <button
              key={opt.key}
              onClick={() => {
                setPage(0);
                setFilter(opt.key);
              }}
              className={cn(
                "flex items-center gap-1.5 rounded-sm px-2.5 py-1 font-mono text-[11px] uppercase tracking-wider transition-all",
                filter === opt.key
                  ? "bg-primary/10 text-primary box-glow-amber"
                  : "text-muted-foreground hover:text-foreground"
              )}
            >
              <span>{opt.label}</span>
              <span className="num text-[10px] opacity-70">{opt.count}</span>
            </button>
          ))}
        </div>
      </div>

      <Panel>
        {isLoading ? (
          <div className="space-y-px">
            {Array.from({ length: 8 }).map((_, i) => (
              <Skeleton key={i} className="h-12 rounded-none" />
            ))}
          </div>
        ) : filtered.length === 0 ? (
          <EmptyState
            title="no records"
            description={
              filter === "all"
                ? "Logs will appear here once requests are routed."
                : `No ${filter} records in the current window.`
            }
            icon={<Activity className="h-5 w-5" />}
            className="border-0"
          />
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full min-w-[900px] border-collapse">
              <thead>
                <tr className="border-b border-border bg-background/40">
                  {[
                    "time",
                    "vkey",
                    "provider",
                    "model",
                    "input",
                    "endpoint",
                    "status",
                    "tokens",
                    "cost",
                    "details",
                  ].map((h) => (
                    <th
                      key={h}
                      className="px-4 py-2 text-left font-mono text-[10px] uppercase tracking-wider text-muted-foreground/70"
                    >
                      {h}
                    </th>
                  ))}
                </tr>
              </thead>
              <tbody className="divide-y divide-border/60">
                {filtered.map((log) => {
                  const tone = statusTone(log.status_code);
                  return (
                    <tr
                      key={log.id}
                      className="group font-mono text-[11px] transition-colors hover:bg-secondary/30"
                    >
                      <td className="px-4 py-2.5 whitespace-nowrap">
                        <div className="text-foreground">{fmtTime(log.created_at)}</div>
                        <div className="text-[9px] text-muted-foreground/50">
                          {fmtDate(log.created_at)}
                        </div>
                      </td>
                      <td className="px-4 py-2.5">
                        <span className="text-muted-foreground">
                          {log.virtual_key_name || "—"}
                        </span>
                      </td>
                      <td className="px-4 py-2.5 text-foreground">
                        {log.provider}
                      </td>
                      <td className="px-4 py-2.5">
                        <span className="text-muted-foreground">
                          {log.model || "—"}
                        </span>
                      </td>
                      <td className="max-w-[260px] px-4 py-2.5">
                        <div className="flex items-center gap-2">
                          <span className="truncate text-muted-foreground/70">
                            {inputSummary(log)}
                          </span>
                          {log.model_input_truncated && (
                            <Badge variant="warning">truncated</Badge>
                          )}
                        </div>
                      </td>
                      <td className="px-4 py-2.5">
                        <span className="text-muted-foreground/70">
                          {log.endpoint}
                        </span>
                      </td>
                      <td className="px-4 py-2.5">
                        <Badge variant={tone}>
                          <span className="led bg-current" />
                          {log.status_code}
                        </Badge>
                      </td>
                      <td className="px-4 py-2.5">
                        <span className="num">
                          <span className="text-foreground">
                            {log.input_tokens}
                          </span>
                          <span className="text-muted-foreground/40"> / </span>
                          <span className="text-muted-foreground">
                            {log.output_tokens}
                          </span>
                        </span>
                      </td>
                      <td className="px-4 py-2.5">
                        <span className="num text-foreground">
                          ${log.cost?.toFixed(6) ?? "0.000000"}
                        </span>
                      </td>
                      <td className="px-4 py-2.5">
                        <Button
                          variant="ghost"
                          size="sm"
                          onClick={() => {
                            setSelected(log);
                            setExpanded(false);
                          }}
                        >
                          <Eye className="h-3.5 w-3.5" />
                          details
                        </Button>
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        )}
      </Panel>

      <Sheet open={!!selected} onOpenChange={(open) => !open && setSelected(null)}>
        <SheetContent side="right" className="w-full overflow-y-auto sm:max-w-2xl">
          {selected && (
            <div className="flex min-h-full flex-col gap-4 p-4">
              <SheetHeader className="border-b border-border pb-4 pr-8">
                <SheetTitle className="font-mono text-sm uppercase tracking-wider text-primary">
                  request detail
                </SheetTitle>
                <SheetDescription className="font-mono text-[12px] text-muted-foreground">
                  {selected.provider} · {selected.model || "unknown model"} · {fmtTime(selected.created_at)}
                </SheetDescription>
              </SheetHeader>

              <div className="grid grid-cols-2 gap-2 font-mono text-[11px] md:grid-cols-3">
                {[
                  ["status", selected.status_code],
                  ["request", selected.request_id],
                  ["tenant", role === "super_admin" ? selected.tenant_id : undefined],
                  ["vkey", selected.virtual_key_name],
                  ["creator", selected.virtual_key_created_by_username],
                  ["endpoint", selected.endpoint],
                  ["input tokens", selected.input_tokens],
                  ["output tokens", selected.output_tokens],
                  ["cost", `$${selected.cost?.toFixed(6) ?? "0.000000"}`],
                  ["input kind", selected.model_input_kind],
                  ["input bytes", selected.model_input_bytes],
                  ["preview bytes", selected.model_input_preview_bytes],
                  ["truncated", selected.model_input_truncated],
                ].map(([label, value]) => (
                  <div key={String(label)} className="rounded-sm border border-border bg-card/40 p-2">
                    <div className="uppercase tracking-wider text-muted-foreground/60">{String(label)}</div>
                    <div className="mt-1 break-words text-foreground">{metaValue(value)}</div>
                  </div>
                ))}
              </div>

              <div className="flex items-center justify-between gap-2 border-b border-border pb-2">
                <div className="font-mono text-[10px] uppercase tracking-wider text-muted-foreground/70">
                  sanitized input preview
                </div>
                <div className="flex items-center gap-2">
                  <Button
                    variant="outline"
                    size="sm"
                    onClick={copyPreview}
                    disabled={!selected.model_input_preview}
                  >
                    <Copy className="h-3.5 w-3.5" />
                    copy
                  </Button>
                  <Button variant="ghost" size="sm" onClick={() => setExpanded((v) => !v)}>
                    {expanded ? <Minimize2 className="h-3.5 w-3.5" /> : <Maximize2 className="h-3.5 w-3.5" />}
                    {expanded ? "collapse" : "expand"}
                  </Button>
                </div>
              </div>

              <pre
                className={cn(
                  "overflow-auto whitespace-pre-wrap break-words rounded-sm border border-border bg-background/70 p-3 font-mono text-[12px] text-foreground",
                  expanded ? "max-h-none" : "max-h-[60vh]"
                )}
              >
                {selected.model_input_preview || "No input preview captured for this request."}
              </pre>

              <div className="rounded-sm border border-warning/30 bg-warning/5 p-3 font-mono text-[11px] text-warning">
                {selected.model_input_truncated
                  ? "Preview truncated. Only the first bytes of sanitized input are stored; the raw request is not persisted."
                  : "Sanitized preview only. The raw request is not persisted."}
              </div>

              {selected.error_message && (
                <div className="rounded-sm border border-destructive/30 bg-destructive/5 p-3 font-mono text-[11px] text-destructive">
                  {selected.error_message}
                </div>
              )}
            </div>
          )}
        </SheetContent>
      </Sheet>

      {/* Pagination footer */}
      <div className="mt-4 flex items-center justify-between font-mono text-[11px] text-muted-foreground">
        <span className="uppercase tracking-wider text-muted-foreground/60">
          {total.toLocaleString()} records
        </span>
        <div className="flex items-center gap-2">
          <Button
            variant="outline"
            size="sm"
            onClick={() => setPage((p) => Math.max(0, p - 1))}
            disabled={page === 0 || isFetching}
          >
            <ChevronLeft className="h-3.5 w-3.5" />
            prev
          </Button>
          <span className="num text-foreground">
            {page + 1} <span className="text-muted-foreground/50">/ {pageCount}</span>
          </span>
          <Button
            variant="outline"
            size="sm"
            onClick={() => setPage((p) => p + 1)}
            disabled={(page + 1) * PAGE_SIZE >= total || isFetching}
          >
            next
            <ChevronRight className="h-3.5 w-3.5" />
          </Button>
        </div>
      </div>
    </div>
  );
}
