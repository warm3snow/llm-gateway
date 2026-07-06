"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import {
  LayoutDashboard,
  MessagesSquare,
  KeyRound,
  Activity,
  BarChart3,
  Settings,
  Power,
  TerminalSquare,
} from "lucide-react";
import { cn } from "@/lib/utils";
import { api } from "@/lib/api";
import { useRouter } from "next/navigation";
import { useStats } from "@/lib/queries";

const routes = [
  { href: "/dashboard", label: "Dashboard", code: "dash", icon: LayoutDashboard },
  { href: "/providers", label: "Providers", code: "prov", icon: MessagesSquare },
  { href: "/virtual-keys", label: "Virtual Keys", code: "keys", icon: KeyRound },
  { href: "/logs", label: "Logs", code: "logs", icon: Activity },
  { href: "/analytics", label: "Analytics", code: "stat", icon: BarChart3 },
  { href: "/settings", label: "Settings", code: "conf", icon: Settings },
];

export default function Sidebar({ onNavigate }: { onNavigate?: () => void }) {
  const pathname = usePathname();
  const router = useRouter();

  // Live gateway status derived from the stats query — green when the API
  // responds, amber when it errors or is unreachable.
  const { data: stats, isError, isLoading } = useStats();
  const online = !isError && !isLoading && !!stats;

  const isActive = (href: string) =>
    pathname === href || pathname.startsWith(href + "/");

  const handleLogout = () => {
    api.logout();
  };

  return (
    <div className="flex h-full flex-col">
      {/* Brand / system header */}
      <div className="flex h-14 items-center gap-2.5 border-b border-border px-4">
        <div className="relative flex h-8 w-8 items-center justify-center rounded-sm border border-primary/40 bg-primary/5 box-glow-amber">
          <TerminalSquare className="h-4 w-4 text-primary glow-amber" />
        </div>
        <div className="flex min-w-0 flex-col leading-none">
          <span className="font-mono text-[11px] uppercase tracking-[0.18em] text-primary glow-amber">
            llm-gw
          </span>
          <span className="font-mono text-[9px] uppercase tracking-[0.2em] text-muted-foreground">
            control plane
          </span>
        </div>
        <div className="ml-auto flex items-center gap-1.5">
          <span
            className={cn(
              "led",
              online
                ? "bg-success text-success pulse-dot"
                : "bg-warning text-warning"
            )}
          />
          <span className="font-mono text-[9px] uppercase tracking-wider text-muted-foreground">
            {online ? "online" : "offline"}
          </span>
        </div>
      </div>

      {/* Section label */}
      <div className="px-4 pt-4 pb-2">
        <span className="font-mono text-[9px] uppercase tracking-[0.22em] text-muted-foreground/70">
          // modules
        </span>
      </div>

      {/* Nav */}
      <nav className="flex-1 space-y-0.5 px-2">
        {routes.map((route) => {
          const active = isActive(route.href);
          return (
            <Link
              key={route.href}
              href={route.href}
              onClick={onNavigate}
              className={cn(
                "group relative flex items-center gap-3 rounded-sm px-3 py-2 font-mono text-[13px] transition-all",
                active
                  ? "bg-primary/8 text-primary"
                  : "text-muted-foreground hover:bg-secondary/60 hover:text-foreground"
              )}
            >
              {/* Active rail — a glowing amber bar on the left edge */}
              {active && (
                <span
                  className="absolute left-0 top-1/2 h-5 w-[2px] -translate-y-1/2 bg-primary glow-amber"
                  aria-hidden
                />
              )}
              <route.icon
                className={cn(
                  "h-4 w-4 shrink-0 transition-colors",
                  active
                    ? "text-primary"
                    : "text-muted-foreground/70 group-hover:text-foreground"
                )}
              />
              <span className="flex-1 truncate">{route.label}</span>
              <span
                className={cn(
                  "font-mono text-[9px] uppercase tracking-wider",
                  active ? "text-primary/70" : "text-muted-foreground/40"
                )}
              >
                {route.code}
              </span>
            </Link>
          );
        })}
      </nav>

      {/* System status footer */}
      <div className="border-t border-border px-4 py-3">
        <div className="mb-3 flex items-center justify-between font-mono text-[9px] uppercase tracking-wider text-muted-foreground/60">
          <span>sys.status</span>
          <span
            className={cn(
              "num",
              online ? "text-success" : "text-warning"
            )}
          >
            {online ? "operational" : "degraded"}
          </span>
        </div>
        <button
          onClick={handleLogout}
          className={cn(
            "flex w-full items-center gap-2.5 rounded-sm border border-border px-3 py-2 font-mono text-[12px] text-muted-foreground",
            "transition-all hover:border-destructive/40 hover:bg-destructive/5 hover:text-destructive"
          )}
        >
          <Power className="h-3.5 w-3.5" />
          <span>disconnect</span>
          <span className="ml-auto font-mono text-[9px] uppercase tracking-wider opacity-50">
            ^D
          </span>
        </button>
      </div>
    </div>
  );
}
