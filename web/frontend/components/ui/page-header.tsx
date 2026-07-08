import { cn } from "@/lib/utils";

/**
 * PageHeader — the standardized page heading block.
 * Renders a section code (e.g. "01"), a title, optional description,
 * and an actions slot on the right.
 */
export function PageHeader({
  code,
  title,
  description,
  actions,
  className,
}: {
  code?: string;
  title: string;
  description?: string;
  actions?: React.ReactNode;
  className?: string;
}) {
  return (
    <div
      className={cn(
        "mb-6 flex flex-col gap-3 border-b border-border pb-5 sm:flex-row sm:items-end sm:justify-between",
        className
      )}
    >
      <div className="space-y-1">
        {code && (
          <div className="font-mono text-[10px] uppercase tracking-[0.22em] text-primary/80 glow-amber">
            {`// ${code}`}
          </div>
        )}
        <h1 className="font-display text-2xl font-medium tracking-tight text-foreground sm:text-3xl">
          {title}
        </h1>
        {description && (
          <p className="font-mono text-[12px] text-muted-foreground">
            {description}
          </p>
        )}
      </div>
      {actions && <div className="flex items-center gap-2">{actions}</div>}
    </div>
  );
}
