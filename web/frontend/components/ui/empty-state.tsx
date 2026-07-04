import { cn } from "@/lib/utils";

/**
 * EmptyState — for when a query returns no rows.
 * Stays on-theme: a dim "no signal" terminal-style block.
 */
export function EmptyState({
  title = "no signal",
  description = "No data available yet.",
  icon,
  className,
}: {
  title?: string;
  description?: string;
  icon?: React.ReactNode;
  className?: string;
}) {
  return (
    <div
      className={cn(
        "flex flex-col items-center justify-center gap-3 border border-dashed border-border py-16 text-center",
        className
      )}
    >
      {icon && (
        <div className="flex h-10 w-10 items-center justify-center rounded-sm border border-border text-muted-foreground/50">
          {icon}
        </div>
      )}
      <div className="space-y-1">
        <p className="font-mono text-[11px] uppercase tracking-[0.2em] text-muted-foreground/70">
          {title}
        </p>
        <p className="font-mono text-[12px] text-muted-foreground/60">
          {description}
        </p>
      </div>
    </div>
  );
}
