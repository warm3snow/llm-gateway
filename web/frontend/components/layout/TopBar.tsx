"use client";

import { usePathname } from "next/navigation";
import Link from "next/link";
import { ChevronRight, Sun, Moon, Command, Search, KeyRound } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
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
import { toast } from "sonner";
import { api, currentTenant, currentUser } from "@/lib/api";

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
  const [currentPassword, setCurrentPassword] = useState("");
  const [newPassword, setNewPassword] = useState("");
  const [changingPassword, setChangingPassword] = useState(false);
  const [session] = useState(() => ({ user: currentUser(), tenant: currentTenant() }));
  const username = session.user?.username ?? "unknown";
  const avatarLabel = username.slice(0, 2).toLowerCase() || "--";
  const tenantLabel = session.tenant?.name ?? (session.user?.tenant_id != null ? `tenant #${session.user.tenant_id}` : "platform");
  const tenantMeta = session.tenant
    ? `${session.tenant.slug} · ${session.tenant.role}`
    : session.user?.role;
  useEffect(() => {
    const frame = requestAnimationFrame(() => setMounted(true));
    return () => cancelAnimationFrame(frame);
  }, []);

  // Build breadcrumb segments
  const segments = pathname.split("/").filter(Boolean);
  const crumbs = segments.map((seg, i) => ({
    label: segmentLabels[seg] ?? seg,
    href: "/" + segments.slice(0, i + 1).join("/"),
  }));

  const handleLogout = () => api.logout();

  async function handleChangePassword(e: React.FormEvent) {
    e.preventDefault();
    if (newPassword.length < 6) {
      toast.error("new password too short", { description: "Use at least 6 characters." });
      return;
    }
    setChangingPassword(true);
    try {
      await api.changeOwnPassword({
        current_password: currentPassword,
        new_password: newPassword,
      });
      setCurrentPassword("");
      setNewPassword("");
      toast.success("password updated");
    } catch (err) {
      toast.error("failed to update password", {
        description: err instanceof Error ? err.message : String(err),
      });
    } finally {
      setChangingPassword(false);
    }
  }

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
              className="flex h-9 items-center gap-2 rounded-sm border border-border bg-card/60 px-1.5 py-1 transition-colors hover:border-border-bright"
              aria-label="User menu"
            >
              <Avatar className="h-6 w-6">
                <AvatarFallback className="bg-primary/10 font-mono text-[10px] text-primary">
                  {avatarLabel}
                </AvatarFallback>
              </Avatar>
              <span className="hidden flex-col items-start leading-none sm:flex">
                <span className="font-mono text-[11px] text-foreground">
                  {username}
                </span>
                <span className="mt-0.5 max-w-28 truncate font-mono text-[9px] text-muted-foreground">
                  {tenantLabel}
                </span>
              </span>
            </button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end" className="w-64">
            <DropdownMenuLabel className="font-mono text-[10px] uppercase tracking-wider text-muted-foreground">
              session
            </DropdownMenuLabel>
            <DropdownMenuLabel className="space-y-1 font-sans text-sm text-foreground">
              <div>{username}</div>
              <div className="font-mono text-[11px] font-normal text-muted-foreground">
                {tenantLabel}
              </div>
              {tenantMeta && (
                <div className="font-mono text-[10px] font-normal text-muted-foreground/70">
                  {tenantMeta}
                </div>
              )}
            </DropdownMenuLabel>
            <DropdownMenuSeparator />
            <form
              className="space-y-2 px-2 py-1.5"
              onSubmit={handleChangePassword}
              onKeyDown={(e) => e.stopPropagation()}
            >
              <div className="flex items-center gap-1.5 font-mono text-[10px] uppercase tracking-wider text-muted-foreground">
                <KeyRound className="h-3 w-3" />
                change password
              </div>
              <div className="space-y-1">
                <Label htmlFor="current-password" className="text-[10px]">
                  current
                </Label>
                <Input
                  id="current-password"
                  type="password"
                  value={currentPassword}
                  onChange={(e) => setCurrentPassword(e.target.value)}
                  className="h-8 font-mono text-[12px]"
                />
              </div>
              <div className="space-y-1">
                <Label htmlFor="new-password" className="text-[10px]">
                  new
                </Label>
                <Input
                  id="new-password"
                  type="password"
                  value={newPassword}
                  onChange={(e) => setNewPassword(e.target.value)}
                  className="h-8 font-mono text-[12px]"
                />
              </div>
              <Button
                type="submit"
                size="sm"
                className="w-full"
                disabled={changingPassword || !currentPassword || !newPassword}
              >
                {changingPassword ? "updating…" : "update"}
              </Button>
            </form>
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
