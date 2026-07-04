import { cn } from "@/lib/utils";

/**
 * LED — a small status indicator dot, like the indicator LEDs on rack hardware.
 * `tone` controls the color (mapped to theme tokens) and the glow shadow.
 */
const toneMap = {
  success: "bg-success text-success",
  warning: "bg-warning text-warning",
  destructive: "bg-destructive text-destructive",
  accent: "bg-accent text-accent",
  primary: "bg-primary text-primary",
  muted: "bg-muted-foreground text-muted-foreground",
} as const;

export function Led({
  tone = "success",
  pulse = false,
  className,
}: {
  tone?: keyof typeof toneMap;
  pulse?: boolean;
  className?: string;
}) {
  return (
    <span
      className={cn(
        "led inline-block",
        toneMap[tone],
        pulse && "pulse-dot",
        className
      )}
      aria-hidden
    />
  );
}
