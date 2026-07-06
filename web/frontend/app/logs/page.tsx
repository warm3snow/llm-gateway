"use client";

import { useEffect, useState } from "react";
import { useUsage } from "@/lib/queries";
import { PageHeader } from "@/components/ui/page-header";
import { Panel, PanelHeader, PanelTitle } from "@/components/ui/panel";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { EmptyState } from "@/components/ui/empty-state";
import {
  Activity,
  Filter,
  Download,
  RotateCw,
  ChevronLeft,
  ChevronRight,
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

export default function LogsPage() {
  const [page, setPage] = useState(0);
  const [filter, setFilter] = useState<StatusFilter>("all");
  const { data, isLoading, refetch, isFetching } = useUsage({
    limit: PAGE_SIZE,
    offset: page * PAGE_SIZE,
  });

  const logs = data?.records ?? [];
  const total = data?.total ?? 0;
  const pageCount = Math.max(1, Math.ceil(total / PAGE_SIZE));

  // Reset to the first page whenever the status filter changes so the
  // client-side filter always operates on a fresh page.
  useEffect(() => {
    setPage(0);
  }, [filter]);

  // Status filter is applied client-side within the current page.
  const filtered = logs.filter((l) => {
    if (filter === "all") return true;
    if (filter === "success") return l.status_code < 400;
    if (filter === "error") return l.status_code >= 400;
    return true;
  });

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
        description="Every proxied request, in order"
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
              onClick={() => setFilter(opt.key)}
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
                    "endpoint",
                    "status",
                    "tokens",
                    "cost",
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
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        )}
      </Panel>

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
