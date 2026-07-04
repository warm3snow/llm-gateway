import * as React from "react";
import { cn } from "@/lib/utils";

/**
 * Panel — a recessed "rack unit" surface. Used as the base for cards,
 * tables, and stat tiles throughout the control-room interface.
 */
const Panel = React.forwardRef<
  HTMLDivElement,
  React.HTMLAttributes<HTMLDivElement>
>(({ className, ...props }, ref) => (
  <div
    ref={ref}
    className={cn(
      "relative rounded-sm border border-border bg-card text-card-foreground",
      "shadow-[inset_0_1px_0_hsl(var(--border-bright)/0.25)]",
      className
    )}
    {...props}
  />
));
Panel.displayName = "Panel";

const PanelHeader = React.forwardRef<
  HTMLDivElement,
  React.HTMLAttributes<HTMLDivElement>
>(({ className, ...props }, ref) => (
  <div
    ref={ref}
    className={cn(
      "flex items-center justify-between gap-2 border-b border-border px-4 py-2.5",
      className
    )}
    {...props}
  />
));
PanelHeader.displayName = "PanelHeader";

const PanelTitle = React.forwardRef<
  HTMLParagraphElement,
  React.HTMLAttributes<HTMLHeadingElement>
>(({ className, ...props }, ref) => (
  <h3
    ref={ref}
    className={cn(
      "font-mono text-[11px] uppercase tracking-[0.18em] text-muted-foreground",
      className
    )}
    {...props}
  />
));
PanelTitle.displayName = "PanelTitle";

const PanelBody = React.forwardRef<
  HTMLDivElement,
  React.HTMLAttributes<HTMLDivElement>
>(({ className, ...props }, ref) => (
  <div ref={ref} className={cn("p-4", className)} {...props} />
));
PanelBody.displayName = "PanelBody";

export { Panel, PanelHeader, PanelTitle, PanelBody };
