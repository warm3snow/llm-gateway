"use client";

import { useState } from "react";
import { usePathname } from "next/navigation";
import Sidebar from "@/components/layout/Sidebar";
import TopBar from "@/components/layout/TopBar";
import { Sheet, SheetContent, SheetTitle } from "@/components/ui/sheet";
import { Menu } from "lucide-react";
import { Button } from "@/components/ui/button";

export default function AppShell({ children }: { children: React.ReactNode }) {
  const pathname = usePathname();
  const [mobileOpen, setMobileOpen] = useState(false);

  // Login renders full-screen, no chrome.
  if (pathname === "/login") {
    return <>{children}</>;
  }

  const menuButton = (
    <Button
      variant="ghost"
      size="icon"
      className="lg:hidden text-muted-foreground hover:text-foreground"
      aria-label="Open navigation"
      onClick={() => setMobileOpen(true)}
    >
      <Menu className="h-5 w-5" />
    </Button>
  );

  return (
    <div className="flex h-screen overflow-hidden bg-background text-foreground">
      {/* Desktop sidebar — fixed rail */}
      <aside className="hidden lg:flex lg:w-60 lg:flex-col lg:shrink-0 border-r border-border bg-card/40">
        <Sidebar />
      </aside>

      {/* Mobile sidebar — Sheet drawer (controlled via state) */}
      <Sheet open={mobileOpen} onOpenChange={setMobileOpen}>
        <SheetContent
          side="left"
          className="w-72 border-r border-border bg-card p-0"
        >
          <SheetTitle className="sr-only">Navigation</SheetTitle>
          <Sidebar onNavigate={() => setMobileOpen(false)} />
        </SheetContent>
      </Sheet>

      {/* Main column */}
      <div className="flex min-w-0 flex-1 flex-col">
        <TopBar menuButton={menuButton} />
        <main className="flex-1 overflow-y-auto">
          <div className="mx-auto w-full max-w-[1400px] px-4 py-6 sm:px-6 lg:px-8 lg:py-8">
            {children}
          </div>
        </main>
      </div>
    </div>
  );
}
