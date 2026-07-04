/**
 * LoadingBar — a thin animated sweep bar shown across the top of a panel
 * while data is fetching. Reads as "signal acquiring" on a scope.
 */
export function LoadingBar({ className }: { className?: string }) {
  return (
    <div
      className={`relative h-0.5 w-full overflow-hidden bg-border/50 ${className ?? ""}`}
    >
      <div className="sweep absolute inset-0" />
    </div>
  );
}
