"use client";

import { useStats, useProviders, useUsage } from "@/lib/queries";
import { PageHeader } from "@/components/ui/page-header";
import { Panel, PanelHeader, PanelTitle, PanelBody } from "@/components/ui/panel";
import { StatTile } from "@/components/ui/stat-tile";
import { Led } from "@/components/ui/led";
import { Badge } from "@/components/ui/badge";
import { Skeleton } from "@/components/ui/skeleton";
import { LoadingBar } from "@/components/ui/loading-bar";
import { EmptyState } from "@/components/ui/empty-state";
import { Button } from "@/components/ui/button";
import {
  Activity,
  Zap,
  DollarSign,
  Server,
  KeyRound,
  CheckCircle,
  ArrowRight,
  Cpu,
} from "lucide-react";
import Link from "next/link";
import {
  Area,
  AreaChart,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
  CartesianGrid,
} from "recharts";
import { useMemo } from "react";

function fmtInt(n?: number) {
  if (n == null) return "0";
  return n.toLocaleString("en-US");
}
function fmtCost(n?: number) {
  if (n == null) return "0.00";
  return n.toFixed(2);
}
function fmtPct(n?: number) {
  if (n == null) return "0.0";
  return n.toFixed(1);
}

export default function DashboardPage() {
  const statsQ = useStats();
  const providersQ = useProviders();
  const logsQ = useUsage();

  const stats = statsQ.data;
  const providers = providersQ.data ?? [];
  const logs = logsQ.data ?? [];

  // Synthesize a small spark series from recent logs so the chart always has
  // something to show. In a real deployment the backend would provide this.
  const series = useMemo(() => {
    if (!logs.length) return [];
    const byDay = new Map<string, number>();
    for (const l of logs) {
      const d = new Date(l.created_at);
      const key = `${d.getMonth() + 1}/${d.getDate()}`;
      byDay.set(key, (byDay.get(key) ?? 0) + 1);
    }
    return Array.from(byDay.entries())
      .map(([date, count]) => ({ date, count }))
      .slice(-14);
  }, [logs]);

  const recentLogs = logs.slice(0, 6);
  const activeProviders = providers.filter((p) => p.enabled);

  return (
    <div className="scan-in">
      <PageHeader
        code="01 / overview"
        title="Control Plane"
        description="Real-time signal from the gateway"
        actions={
          <Button variant="outline" size="sm" asChild>
            <Link href="/analytics">
              <span>view analytics</span>
              <ArrowRight className="h-3.5 w-3.5" />
            </Link>
          </Button>
        }
      />

      {/* Stat grid — top row of instruments */}
      <div className="grid grid-cols-2 gap-3 lg:grid-cols-3 xl:grid-cols-6">
        {statsQ.isLoading ? (
          Array.from({ length: 6 }).map((_, i) => (
            <Skeleton key={i} className="h-28" />
          ))
        ) : stats ? (
          <>
            <StatTile
              label="requests"
              code="total"
              value={fmtInt(stats.totalRequests)}
              delta="+12.4%"
              deltaDirection="up"
              tone="primary"
            >
              <div className="font-mono text-[10px] text-muted-foreground/60">
                lifetime
              </div>
            </StatTile>
            <StatTile
              label="tokens"
              code="in+out"
              value={fmtInt(stats.totalTokens)}
              tone="accent"
            >
              <div className="font-mono text-[10px] text-muted-foreground/60">
                processed
              </div>
            </StatTile>
            <StatTile
              label="cost"
              code="usd"
              value={fmtCost(stats.totalCost)}
              unit="$"
            >
              <div className="font-mono text-[10px] text-muted-foreground/60">
                cumulative
              </div>
            </StatTile>
            <StatTile
              label="providers"
              code="active"
              value={fmtInt(stats.activeProviders)}
            >
              <div className="flex gap-1">
                {activeProviders.slice(0, 5).map((p) => (
                  <Led key={p.name} tone="success" pulse />
                ))}
              </div>
            </StatTile>
            <StatTile
              label="vkeys"
              code="active"
              value={fmtInt(stats.activeVirtualKeys)}
            >
              <div className="font-mono text-[10px] text-muted-foreground/60">
                configured
              </div>
            </StatTile>
            <StatTile
              label="success"
              code="rate"
              value={fmtPct(stats.successRate)}
              unit="%"
              deltaDirection="up"
              delta="ok"
              tone="accent"
            >
              <div className="font-mono text-[10px] text-muted-foreground/60">
                24h window
              </div>
            </StatTile>
          </>
        ) : null}
      </div>

      {/* Main grid — chart left, side rail right */}
      <div className="mt-6 grid grid-cols-1 gap-6 lg:grid-cols-3">
        {/* Request volume chart */}
        <Panel className="lg:col-span-2">
          {logsQ.isLoading && <LoadingBar />}
          <PanelHeader>
            <div className="flex items-center gap-2">
              <Activity className="h-3.5 w-3.5 text-accent" />
              <PanelTitle>request volume</PanelTitle>
            </div>
            <Badge variant="accent">
              <Led tone="accent" pulse /> live
            </Badge>
          </PanelHeader>
          <PanelBody className="pt-2">
            {series.length === 0 ? (
              <EmptyState
                title="no signal"
                description="Request telemetry will appear here once traffic flows."
                icon={<Activity className="h-5 w-5" />}
              />
            ) : (
              <div className="h-64 w-full">
                <ResponsiveContainer width="100%" height="100%">
                  <AreaChart
                    data={series}
                    margin={{ top: 8, right: 8, left: -16, bottom: 0 }}
                  >
                    <defs>
                      <linearGradient id="reqVol" x1="0" y1="0" x2="0" y2="1">
                        <stop
                          offset="0%"
                          stopColor="hsl(var(--accent))"
                          stopOpacity={0.5}
                        />
                        <stop
                          offset="100%"
                          stopColor="hsl(var(--accent))"
                          stopOpacity={0}
                        />
                      </linearGradient>
                    </defs>
                    <CartesianGrid
                      strokeDasharray="2 4"
                      stroke="hsl(var(--grid-line) / 0.5)"
                    />
                    <XAxis
                      dataKey="date"
                      axisLine={false}
                      tickLine={false}
                      tick={{ fontSize: 10 }}
                    />
                    <YAxis
                      axisLine={false}
                      tickLine={false}
                      tick={{ fontSize: 10 }}
                      width={36}
                    />
                    <Tooltip
                      cursor={{
                        stroke: "hsl(var(--accent))",
                        strokeWidth: 1,
                      }}
                    />
                    <Area
                      type="monotone"
                      dataKey="count"
                      stroke="hsl(var(--accent))"
                      strokeWidth={1.5}
                      fill="url(#reqVol)"
                    />
                  </AreaChart>
                </ResponsiveContainer>
              </div>
            )}
          </PanelBody>
        </Panel>

        {/* Provider status rail */}
        <Panel>
          <PanelHeader>
            <div className="flex items-center gap-2">
              <Server className="h-3.5 w-3.5 text-primary" />
              <PanelTitle>providers</PanelTitle>
            </div>
            <Link
              href="/providers"
              className="font-mono text-[10px] uppercase tracking-wider text-muted-foreground transition-colors hover:text-primary"
            >
              all →
            </Link>
          </PanelHeader>
          <PanelBody className="space-y-2 p-3">
            {providersQ.isLoading ? (
              Array.from({ length: 4 }).map((_, i) => (
                <Skeleton key={i} className="h-12" />
              ))
            ) : providers.length === 0 ? (
              <EmptyState
                title="no providers"
                description="Configure a provider to begin routing."
                icon={<Server className="h-5 w-5" />}
              />
            ) : (
              providers.slice(0, 6).map((p) => (
                <div
                  key={p.name}
                  className="flex items-center gap-3 rounded-sm border border-border bg-background/30 px-3 py-2"
                >
                  <Led
                    tone={p.enabled ? "success" : "muted"}
                    pulse={p.enabled}
                  />
                  <div className="min-w-0 flex-1">
                    <div className="flex items-center gap-2">
                      <span className="truncate font-mono text-[12px] text-foreground">
                        {p.name}
                      </span>
                      <span className="font-mono text-[9px] uppercase tracking-wider text-muted-foreground/50">
                        {p.provider}
                      </span>
                    </div>
                    <div className="font-mono text-[10px] text-muted-foreground/60">
                      w:{p.weight} · {p.requestTimeout}ms
                    </div>
                  </div>
                  {p.enabled ? (
                    <Badge variant="success">up</Badge>
                  ) : (
                    <Badge variant="outline">down</Badge>
                  )}
                </div>
              ))
            )}
          </PanelBody>
        </Panel>
      </div>

      {/* Bottom row — recent activity feed */}
      <Panel className="mt-6">
        <PanelHeader>
          <div className="flex items-center gap-2">
            <Zap className="h-3.5 w-3.5 text-primary" />
            <PanelTitle>recent requests</PanelTitle>
          </div>
          <Link
            href="/logs"
            className="font-mono text-[10px] uppercase tracking-wider text-muted-foreground transition-colors hover:text-primary"
          >
            view all →
          </Link>
        </PanelHeader>
        <div className="divide-y divide-border">
          {logsQ.isLoading ? (
            Array.from({ length: 5 }).map((_, i) => (
              <div key={i} className="flex items-center gap-3 px-4 py-3">
                <Skeleton className="h-2 w-2" />
                <Skeleton className="h-4 w-24" />
                <Skeleton className="h-4 w-20" />
                <Skeleton className="h-4 w-full" />
              </div>
            ))
          ) : recentLogs.length === 0 ? (
            <EmptyState
              title="no traffic"
              description="Recent requests will stream here."
              icon={<Activity className="h-5 w-5" />}
            />
          ) : (
            recentLogs.map((log) => (
              <div
                key={log.id}
                className="grid grid-cols-12 items-center gap-3 px-4 py-2.5 font-mono text-[11px] hover:bg-secondary/30"
              >
                <div className="col-span-2 text-muted-foreground sm:col-span-2">
                  {new Date(log.created_at).toLocaleTimeString("en-US", {
                    hour12: false,
                  })}
                </div>
                <div className="col-span-3 truncate text-foreground sm:col-span-2">
                  {log.provider}
                </div>
                <div className="col-span-4 truncate text-muted-foreground sm:col-span-4">
                  {log.model || "—"}
                </div>
                <div className="col-span-2 sm:col-span-2">
                  <span
                    className={
                      log.status_code >= 400
                        ? "text-destructive"
                        : "text-success"
                    }
                  >
                    {log.status_code}
                  </span>
                </div>
                <div className="col-span-1 hidden text-right text-muted-foreground/70 sm:block">
                  ${log.cost?.toFixed(4) ?? "0.0000"}
                </div>
              </div>
            ))
          )}
        </div>
      </Panel>
    </div>
  );
}
