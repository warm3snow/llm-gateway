"use client";

import { usePathname } from "next/navigation";
import Link from "next/link";
import { ChevronRight, Sun, Moon, Command, Search } from "lucide-react";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { Avatar, AvatarFallback } from "@/components/ui/avatar";
import { useTheme } from "next-themes";
import { useEffect, useState } from "react";
import { api } from "@/lib/api";

const segmentLabels: Record<string, string> = {
  dashboard: "dashboard",
  providers: "providers",
  "virtual-keys": "virtual-keys",
  logs: "logs",
  analytics: "analytics",
  settings: "settings",
};

export default function TopBar({
  menuButton,
}: {
  menuButton?: React.ReactNode;
}) {
  const pathname = usePathname();
  const { resolvedTheme, setTheme } = useTheme();
  const [mounted, setMounted] = useState(false);
  useEffect(() => setMounted(true), []);

  // Build breadcrumb segments
  const segments = pathname.split("/").filter(Boolean);
  const crumbs = segments.map((seg, i) => ({
    label: segmentLabels[seg] ?? seg,
    href: "/" + segments.slice(0, i + 1).join("/"),
  }));

  const handleLogout = () => api.logout();

  return (
    <header className="sticky top-0 z-20 flex h-14 shrink-0 items-center gap-3 border-b border-border bg-background/80 px-4 backdrop-blur-md sm:px-6">
      {menuButton}

      {/* Breadcrumbs — terminal path style */}
      <nav className="flex min-w-0 items-center gap-1 font-mono text-[12px]">
        <span className="text-primary/80 glow-amber">~</span>
        {crumbs.length === 0 && (
          <span className="text-muted-foreground">/</span>
        )}
        {crumbs.map((c, i) => {
          const last = i === crumbs.length - 1;
          return (
            <span key={c.href} className="flex items-center gap-1">
              <ChevronRight className="h-3 w-3 text-muted-foreground/50" />
              {last ? (
                <span className="text-foreground">
                  {c.label}
                  <span className="ml-0.5 inline-block h-3.5 w-[7px] translate-y-[1px] bg-primary glow-amber blink" />
                </span>
              ) : (
                <Link
                  href={c.href}
                  className="text-muted-foreground transition-colors hover:text-foreground"
                >
                  {c.label}
                </Link>
              )}
            </span>
          );
        })}
      </nav>

      <div className="ml-auto flex items-center gap-1.5">
        {/* Command palette trigger (visual only — wiring is a future task) */}
        <Button
          variant="ghost"
          size="sm"
          className="hidden h-8 gap-2 font-mono text-[11px] text-muted-foreground hover:text-foreground sm:flex"
          aria-label="Search"
        >
          <Search className="h-3.5 w-3.5" />
          <span>search</span>
          <kbd className="flex items-center gap-0.5 rounded-sm border border-border bg-secondary/60 px-1 py-0.5 font-mono text-[9px] text-muted-foreground">
            <Command className="h-2.5 w-2.5" />K
          </kbd>
        </Button>

        {/* Theme toggle */}
        <Button
          variant="ghost"
          size="icon"
          className="h-8 w-8 text-muted-foreground hover:text-foreground"
          onClick={() => setTheme(resolvedTheme === "dark" ? "light" : "dark")}
          aria-label="Toggle theme"
        >
          {mounted && resolvedTheme === "dark" ? (
            <Sun className="h-4 w-4" />
          ) : (
            <Moon className="h-4 w-4" />
          )}
        </Button>

        {/* User menu */}
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <button
              className="flex h-8 items-center gap-2 rounded-sm border border-border bg-card/60 px-1.5 py-1 transition-colors hover:border-border-bright"
              aria-label="User menu"
            >
              <Avatar className="h-6 w-6">
                <AvatarFallback className="bg-primary/10 font-mono text-[10px] text-primary">
                  ad
                </AvatarFallback>
              </Avatar>
              <span className="hidden font-mono text-[11px] text-muted-foreground sm:inline">
                admin
              </span>
            </button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end" className="w-44">
            <DropdownMenuLabel className="font-mono text-[10px] uppercase tracking-wider text-muted-foreground">
              session
            </DropdownMenuLabel>
            <DropdownMenuLabel className="font-sans text-sm text-foreground">
              admin
            </DropdownMenuLabel>
            <DropdownMenuSeparator />
            <DropdownMenuItem
              className="font-mono text-[12px] text-destructive focus:text-destructive"
              onClick={handleLogout}
            >
              disconnect
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      </div>
    </header>
  );
}
