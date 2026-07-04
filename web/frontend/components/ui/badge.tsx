import * as React from "react";
import { cva, type VariantProps } from "class-variance-authority";
import { cn } from "@/lib/utils";

const badgeVariants = cva(
  "inline-flex items-center gap-1.5 rounded-sm border px-2 py-0.5 font-mono text-[10px] uppercase tracking-wider transition-colors",
  {
    variants: {
      variant: {
        default:
          "border-primary/30 bg-primary/8 text-primary",
        secondary:
          "border-border bg-secondary text-secondary-foreground",
        accent:
          "border-accent/30 bg-accent/8 text-accent",
        destructive:
          "border-destructive/30 bg-destructive/8 text-destructive",
        success:
          "border-success/30 bg-success/8 text-success",
        warning:
          "border-warning/30 bg-warning/8 text-warning",
        outline:
          "border-border-bright text-muted-foreground",
      },
    },
    defaultVariants: {
      variant: "default",
    },
  }
);

export interface BadgeProps
  extends React.HTMLAttributes<HTMLDivElement>,
    VariantProps<typeof badgeVariants> {}

function Badge({ className, variant, ...props }: BadgeProps) {
  return (
    <div className={cn(badgeVariants({ variant }), className)} {...props} />
  );
}

export { Badge, badgeVariants };
