import { cn } from "@/lib/utils";
import { Panel } from "./panel";
import { TrendingUp, TrendingDown } from "lucide-react";

/**
 * StatTile — a single metric readout, like a panel instrument on a console.
 * Big monospace numeric value, label, optional delta indicator, optional spark.
 */
export function StatTile({
  label,
  code,
  value,
  unit,
  delta,
  deltaDirection,
  tone = "default",
  children,
  className,
}: {
  label: string;
  code?: string;
  value: React.ReactNode;
  unit?: string;
  delta?: string;
  deltaDirection?: "up" | "down" | "neutral";
  tone?: "default" | "accent" | "primary";
  children?: React.ReactNode; // sparkline / chart slot
  className?: string;
}) {
  return (
    <Panel
      className={cn(
        "overflow-hidden",
        tone === "accent" && "box-glow-cyan",
        tone === "primary" && "box-glow-amber",
        className
      )}
    >
      <div className="flex items-start justify-between border-b border-border px-4 py-2.5">
        <div className="flex flex-col gap-0.5">
          <span className="font-mono text-[10px] uppercase tracking-[0.18em] text-muted-foreground">
            {label}
          </span>
          {code && (
            <span className="font-mono text-[9px] uppercase tracking-wider text-muted-foreground/60">
              {code}
            </span>
          )}
        </div>
        {delta && (
          <span
            className={cn(
              "flex items-center gap-1 font-mono text-[10px] uppercase tracking-wider",
              deltaDirection === "up" && "text-success",
              deltaDirection === "down" && "text-destructive",
              deltaDirection === "neutral" && "text-muted-foreground"
            )}
          >
            {deltaDirection === "up" && <TrendingUp className="h-3 w-3" />}
            {deltaDirection === "down" && <TrendingDown className="h-3 w-3" />}
            {delta}
          </span>
        )}
      </div>
      <div className="flex flex-col gap-2 px-4 py-3">
        <div className="flex items-baseline gap-1.5">
          <span
            className={cn(
              "num font-display text-2xl font-medium text-foreground",
              tone === "accent" && "glow-cyan text-accent",
              tone === "primary" && "glow-amber text-primary"
            )}
          >
            {value}
          </span>
          {unit && (
            <span className="font-mono text-[11px] text-muted-foreground">
              {unit}
            </span>
          )}
        </div>
        {children}
      </div>
    </Panel>
  );
}
