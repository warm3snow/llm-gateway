"use client";

import { LayoutGrid, List } from "lucide-react";
import { cn } from "@/lib/utils";
import type { ViewMode } from "@/lib/use-view-mode";

/** Compact card/list switch used by every list page. */
export function ViewToggle({
  value,
  onChange,
  className,
}: {
  value: ViewMode;
  onChange: (next: ViewMode) => void;
  className?: string;
}) {
  return (
    <div
      role="group"
      aria-label="display mode"
      className={cn(
        "flex items-center gap-0.5 rounded-sm border border-border bg-card/40 p-0.5",
        className
      )}
    >
      {(
        [
          { mode: "cards", label: "cards", icon: LayoutGrid },
          { mode: "list", label: "list", icon: List },
        ] as const
      ).map(({ mode, label, icon: Icon }) => (
        <button
          key={mode}
          type="button"
          aria-pressed={value === mode}
          onClick={() => onChange(mode)}
          className={cn(
            "flex items-center gap-1.5 rounded-sm px-2 py-1 font-mono text-[10px] uppercase tracking-wider transition-colors",
            value === mode
              ? "bg-primary/10 text-primary"
              : "text-muted-foreground hover:text-foreground"
          )}
        >
          <Icon className="h-3 w-3" />
          {label}
        </button>
      ))}
    </div>
  );
}
