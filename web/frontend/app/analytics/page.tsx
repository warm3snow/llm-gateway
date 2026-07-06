"use client";

import { useAnalytics } from "@/lib/queries";
import { PageHeader } from "@/components/ui/page-header";
import { Panel, PanelHeader, PanelTitle, PanelBody } from "@/components/ui/panel";
import { Badge } from "@/components/ui/badge";
import { Led } from "@/components/ui/led";
import { Skeleton } from "@/components/ui/skeleton";
import { EmptyState } from "@/components/ui/empty-state";
import { LoadingBar } from "@/components/ui/loading-bar";
import { BarChart3, Cpu, Server, DollarSign, Zap } from "lucide-react";
import {
  Bar,
  BarChart,
  CartesianGrid,
  Cell,
  Line,
  LineChart,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";
import { cn } from "@/lib/utils";

const palette = [
  "hsl(var(--primary))",
  "hsl(var(--accent))",
  "hsl(var(--success))",
  "hsl(var(--warning))",
  "hsl(var(--info))",
  "hsl(var(--primary-dim))",
];

export default function AnalyticsPage() {
  const { data: stats, isLoading } = useAnalytics();

  const series = stats?.timeSeries ?? [];
  const topModels = stats?.topModels ?? [];
  const topProviders = stats?.topProviders ?? [];
  const maxCount = stats?.maxCount ?? 1;

  // Cost series derived from timeSeries — real spend reported by the backend.
  const costSeries = series.map((p) => ({
    date: p.date,
    cost: Number((p.cost ?? 0).toFixed(4)),
  }));

  return (
    <div className="scan-in">
      <PageHeader
        code="05 / analysis"
        title="Analytics"
        description="Aggregate trends across the gateway"
      />

      {isLoading ? (
        <div className="grid grid-cols-1 gap-4 lg:grid-cols-2">
          <Skeleton className="h-72" />
          <Skeleton className="h-72" />
          <Skeleton className="h-72" />
          <Skeleton className="h-72" />
        </div>
      ) : (
        <div className="space-y-6">
          {/* Request volume — bar chart */}
          <Panel>
            <LoadingBar className={isLoading ? "" : "opacity-0"} />
            <PanelHeader>
              <div className="flex items-center gap-2">
                <BarChart3 className="h-3.5 w-3.5 text-accent" />
                <PanelTitle>request volume · 14d</PanelTitle>
              </div>
              <Badge variant="accent">
                <Led tone="accent" pulse /> streaming
              </Badge>
            </PanelHeader>
            <PanelBody>
              {series.length === 0 ? (
                <EmptyState
                  title="no signal"
                  description="Volume will populate as requests arrive."
                  icon={<BarChart3 className="h-5 w-5" />}
                />
              ) : (
                <div className="h-72 w-full">
                  <ResponsiveContainer width="100%" height="100%">
                    <BarChart
                      data={series}
                      margin={{ top: 8, right: 8, left: -16, bottom: 0 }}
                    >
                      <CartesianGrid
                        strokeDasharray="2 4"
                        stroke="hsl(var(--grid-line) / 0.5)"
                        vertical={false}
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
                        cursor={{ fill: "hsl(var(--accent) / 0.08)" }}
                      />
                      <Bar dataKey="count" radius={[2, 2, 0, 0]}>
                        {series.map((_, i) => (
                          <Cell
                            key={i}
                            fill={palette[i % palette.length]}
                          />
                        ))}
                      </Bar>
                    </BarChart>
                  </ResponsiveContainer>
                </div>
              )}
            </PanelBody>
          </Panel>

          {/* Two-up row: cost trend + token usage */}
          <div className="grid grid-cols-1 gap-6 lg:grid-cols-2">
            {/* Cost trend — line */}
            <Panel>
              <PanelHeader>
                <div className="flex items-center gap-2">
                  <DollarSign className="h-3.5 w-3.5 text-primary" />
                  <PanelTitle>cost trend · 14d</PanelTitle>
                </div>
                <span className="font-mono text-[10px] text-muted-foreground">
                  usd
                </span>
              </PanelHeader>
              <PanelBody>
                {costSeries.length === 0 ? (
                  <EmptyState
                    title="no cost data"
                    description="Spend trends appear here once traffic flows."
                    icon={<DollarSign className="h-5 w-5" />}
                  />
                ) : (
                  <div className="h-56 w-full">
                    <ResponsiveContainer width="100%" height="100%">
                      <LineChart
                        data={costSeries}
                        margin={{ top: 8, right: 8, left: -16, bottom: 0 }}
                      >
                        <CartesianGrid
                          strokeDasharray="2 4"
                          stroke="hsl(var(--grid-line) / 0.5)"
                          vertical={false}
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
                        <Tooltip />
                        <Line
                          type="monotone"
                          dataKey="cost"
                          stroke="hsl(var(--primary))"
                          strokeWidth={1.5}
                          dot={{
                            r: 2,
                            fill: "hsl(var(--primary))",
                          }}
                          activeDot={{
                            r: 4,
                            fill: "hsl(var(--primary))",
                          }}
                        />
                      </LineChart>
                    </ResponsiveContainer>
                  </div>
                )}
              </PanelBody>
            </Panel>

            {/* Top models — horizontal bars */}
            <Panel>
              <PanelHeader>
                <div className="flex items-center gap-2">
                  <Cpu className="h-3.5 w-3.5 text-accent" />
                  <PanelTitle>top models</PanelTitle>
                </div>
                <span className="font-mono text-[10px] text-muted-foreground">
                  by requests
                </span>
              </PanelHeader>
              <PanelBody>
                {topModels.length === 0 ? (
                  <EmptyState
                    title="no model data"
                    description="Model breakdown appears after first request."
                    icon={<Cpu className="h-5 w-5" />}
                  />
                ) : (
                  <ul className="space-y-2">
                    {topModels.slice(0, 8).map((m, i) => {
                      const max = topModels[0]?.count || 1;
                      const pct = (m.count / max) * 100;
                      return (
                        <li key={m.model} className="space-y-1">
                          <div className="flex items-baseline justify-between font-mono text-[11px]">
                            <span className="truncate text-foreground">
                              <span className="mr-2 text-muted-foreground/50">
                                {String(i + 1).padStart(2, "0")}
                              </span>
                              {m.model || "—"}
                            </span>
                            <span className="num text-muted-foreground">
                              {m.count.toLocaleString()}
                            </span>
                          </div>
                          <div className="relative h-1 overflow-hidden rounded-sm bg-secondary">
                            <div
                              className={cn(
                                "absolute inset-y-0 left-0",
                                i === 0
                                  ? "bg-accent box-glow-cyan"
                                  : "bg-primary/70"
                              )}
                              style={{ width: `${pct}%` }}
                            />
                          </div>
                        </li>
                      );
                    })}
                  </ul>
                )}
              </PanelBody>
            </Panel>
          </div>

          {/* Bottom row: top providers + summary stats */}
          <div className="grid grid-cols-1 gap-6 lg:grid-cols-3">
            <Panel className="lg:col-span-2">
              <PanelHeader>
                <div className="flex items-center gap-2">
                  <Server className="h-3.5 w-3.5 text-primary" />
                  <PanelTitle>top providers</PanelTitle>
                </div>
                <span className="font-mono text-[10px] text-muted-foreground">
                  by volume
                </span>
              </PanelHeader>
              <PanelBody>
                {topProviders.length === 0 ? (
                  <EmptyState
                    title="no provider data"
                    description="Provider breakdown appears after first request."
                    icon={<Server className="h-5 w-5" />}
                  />
                ) : (
                  <ul className="space-y-2.5">
                    {topProviders.slice(0, 6).map((p, i) => {
                      const max = topProviders[0]?.count || 1;
                      const pct = (p.count / max) * 100;
                      return (
                        <li
                          key={p.provider}
                          className="flex items-center gap-3"
                        >
                          <span className="w-6 font-mono text-[10px] text-muted-foreground/50">
                            {String(i + 1).padStart(2, "0")}
                          </span>
                          <Led tone={i === 0 ? "primary" : "muted"} />
                          <div className="min-w-0 flex-1">
                            <div className="flex items-baseline justify-between font-mono text-[11px]">
                              <span className="truncate text-foreground">
                                {p.provider || "—"}
                              </span>
                              <span className="num text-muted-foreground">
                                {p.count.toLocaleString()}
                              </span>
                            </div>
                            <div className="relative mt-1 h-1 overflow-hidden rounded-sm bg-secondary">
                              <div
                                className="absolute inset-y-0 left-0 bg-primary box-glow-amber"
                                style={{ width: `${pct}%` }}
                              />
                            </div>
                          </div>
                        </li>
                      );
                    })}
                  </ul>
                )}
              </PanelBody>
            </Panel>

            {/* Summary stat strip */}
            <Panel>
              <PanelHeader>
                <div className="flex items-center gap-2">
                  <Zap className="h-3.5 w-3.5 text-accent" />
                  <PanelTitle>summary</PanelTitle>
                </div>
              </PanelHeader>
              <PanelBody className="space-y-3 font-mono text-[11px]">
                <div className="flex justify-between">
                  <span className="text-muted-foreground">peak / day</span>
                  <span className="num text-foreground">{maxCount}</span>
                </div>
                <div className="flex justify-between">
                  <span className="text-muted-foreground">avg / day</span>
                  <span className="num text-foreground">
                    {series.length > 0
                      ? Math.round(
                          series.reduce((s, p) => s + p.count, 0) /
                            series.length
                        )
                      : 0}
                  </span>
                </div>
                <div className="flex justify-between">
                  <span className="text-muted-foreground">distinct models</span>
                  <span className="num text-accent">
                    {topModels.length}
                  </span>
                </div>
                <div className="flex justify-between">
                  <span className="text-muted-foreground">distinct providers</span>
                  <span className="num text-accent">
                    {topProviders.length}
                  </span>
                </div>
                <div className="flex justify-between border-t border-border pt-3">
                  <span className="text-muted-foreground">window</span>
                  <span className="text-foreground">14d</span>
                </div>
              </PanelBody>
            </Panel>
          </div>
        </div>
      )}
    </div>
  );
}
