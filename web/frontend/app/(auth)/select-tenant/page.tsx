"use client";

import { useEffect, useState, useSyncExternalStore } from "react";
import { useRouter } from "next/navigation";
import { setCookie } from "cookies-next";
import { Building2, ChevronRight, ShieldCheck } from "lucide-react";
import { api } from "@/lib/api";
import type { TenantMembership } from "@/lib/types";
import { Button } from "@/components/ui/button";

const emptySelection = {
  loginToken: "",
  tenants: [] as TenantMembership[],
  invalid: false,
};

function readTenantSelection() {
  const token = sessionStorage.getItem("tenant_login_token");
  const rawTenants = sessionStorage.getItem("tenant_options");
  if (!token || !rawTenants) {
    return { ...emptySelection, invalid: true };
  }
  try {
    const tenants = JSON.parse(rawTenants) as TenantMembership[];
    if (!Array.isArray(tenants) || tenants.length === 0) {
      return { ...emptySelection, invalid: true };
    }
    return { loginToken: token, tenants, invalid: false };
  } catch {
    return { ...emptySelection, invalid: true };
  }
}

function subscribeTenantSelection() {
  return () => {};
}

export default function SelectTenantPage() {
  const router = useRouter();
  const selection = useSyncExternalStore(
    subscribeTenantSelection,
    readTenantSelection,
    () => emptySelection
  );
  const [selected, setSelected] = useState<number | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");
  const { loginToken, tenants } = selection;
  const selectedTenantID = selected ?? tenants[0]?.tenant_id ?? null;

  useEffect(() => {
    if (selection.invalid) {
      router.replace("/login");
    }
  }, [router, selection.invalid]);

  async function handleContinue() {
    if (!loginToken || selectedTenantID == null) return;
    setLoading(true);
    setError("");
    try {
      const res = await api.selectTenant({
        login_token: loginToken,
        tenant_id: selectedTenantID,
      });
      sessionStorage.removeItem("tenant_login_token");
      sessionStorage.removeItem("tenant_options");
      setCookie("auth_token", res.token, {
        maxAge: 86400,
        path: "/",
        secure: process.env.NODE_ENV === "production",
        sameSite: "strict",
      });
      router.push("/dashboard");
    } catch (err) {
      setError(err instanceof Error ? err.message : "failed to select tenant");
    } finally {
      setLoading(false);
    }
  }

  return (
    <div className="relative flex min-h-screen items-center justify-center overflow-hidden bg-background p-4">
      <div
        className="pointer-events-none absolute inset-0 opacity-40"
        style={{
          backgroundImage:
            "linear-gradient(to right, hsl(var(--grid-line) / 0.35) 1px, transparent 1px), linear-gradient(to bottom, hsl(var(--grid-line) / 0.35) 1px, transparent 1px)",
          backgroundSize: "48px 48px",
        }}
      />
      <div
        className="pointer-events-none absolute left-1/2 top-1/2 h-[600px] w-[600px] -translate-x-1/2 -translate-y-1/2 rounded-full"
        style={{
          background:
            "radial-gradient(circle, hsl(var(--primary) / 0.12) 0%, transparent 60%)",
        }}
      />

      <div className="relative w-full max-w-lg scan-in">
        <div className="flex items-center gap-2 border border-border border-b-0 bg-card/60 px-3 py-2">
          <div className="flex gap-1.5">
            <span className="led bg-destructive text-destructive" />
            <span className="led bg-warning text-warning" />
            <span className="led bg-success text-success" />
          </div>
          <div className="ml-2 flex items-center gap-1.5 font-mono text-[10px] uppercase tracking-wider text-muted-foreground">
            <Building2 className="h-3 w-3 text-primary" />
            llm-gw / select-tenant
          </div>
          <span className="ml-auto font-mono text-[9px] uppercase tracking-wider text-muted-foreground/50">
            tty/1
          </span>
        </div>

        <div className="space-y-5 border border-border bg-card/80 p-6 backdrop-blur-sm">
          <div className="flex items-center gap-2.5">
            <div className="relative flex h-9 w-9 items-center justify-center rounded-sm border border-primary/40 bg-primary/5 box-glow-amber">
              <ShieldCheck className="h-4 w-4 text-primary glow-amber" />
            </div>
            <div>
              <div className="font-mono text-[11px] uppercase tracking-[0.22em] text-primary glow-amber">
                choose organization
              </div>
              <div className="font-mono text-[9px] uppercase tracking-[0.2em] text-muted-foreground">
                multiple tenant memberships detected
              </div>
            </div>
          </div>

          <div className="space-y-2">
            {tenants.map((tenant) => {
              const active = selectedTenantID === tenant.tenant_id;
              return (
                <button
                  key={tenant.tenant_id}
                  type="button"
                  onClick={() => setSelected(tenant.tenant_id)}
                  className={`flex w-full items-center justify-between border px-3 py-3 text-left transition-colors ${
                    active
                      ? "border-primary bg-primary/10"
                      : "border-border bg-background/60 hover:border-border-bright"
                  }`}
                >
                  <div>
                    <div className="font-mono text-sm text-foreground">
                      {tenant.name}
                    </div>
                    <div className="font-mono text-[11px] text-muted-foreground">
                      {tenant.slug} · {tenant.role}
                    </div>
                  </div>
                  {active && <ChevronRight className="h-4 w-4 text-primary" />}
                </button>
              );
            })}
          </div>

          {error && (
            <div className="flex items-center gap-2 border border-destructive/30 bg-destructive/5 px-3 py-2 font-mono text-[11px] text-destructive scan-in">
              <span className="led bg-destructive text-destructive pulse-dot" />
              err: {error}
            </div>
          )}

          <Button
            type="button"
            size="lg"
            className="w-full"
            disabled={loading || selectedTenantID == null}
            onClick={handleContinue}
          >
            {loading ? (
              <>
                <span className="blink">█</span>
                entering…
              </>
            ) : (
              <>
                enter organization
                <ChevronRight className="h-3.5 w-3.5" />
              </>
            )}
          </Button>
        </div>
      </div>
    </div>
  );
}
