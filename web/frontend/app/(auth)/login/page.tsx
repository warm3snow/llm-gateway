"use client";

import { useState, useEffect, FormEvent } from "react";
import { useRouter } from "next/navigation";
import { api, persistAuthSession } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { TerminalSquare, ChevronRight, ShieldCheck } from "lucide-react";

export default function LoginPage() {
  const router = useRouter();
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");
  useEffect(() => {
    sessionStorage.removeItem("tenant_login_token");
    sessionStorage.removeItem("tenant_options");
  }, []);

  async function handleSubmit(e: FormEvent) {
    e.preventDefault();
    setError("");
    sessionStorage.removeItem("tenant_login_token");
    sessionStorage.removeItem("tenant_options");
    const trimmed = email.trim().toLowerCase();
    if (!trimmed) {
      setError("email required");
      return;
    }
    if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(trimmed)) {
      setError("enter a valid email, e.g. user123456@llmgw.org");
      return;
    }
    if (password.length < 3) {
      setError("password too short");
      return;
    }

    setLoading(true);
    try {
      const res = await api.login({ username: trimmed, password });
      if (res.status === "success" && res.token) {
        sessionStorage.removeItem("tenant_login_token");
        sessionStorage.removeItem("tenant_options");
        persistAuthSession(res.token, res.tenant);
        router.push("/dashboard");
      } else if (res.status === "tenant_selection_required") {
        sessionStorage.setItem("tenant_login_token", res.login_token);
        sessionStorage.setItem("tenant_options", JSON.stringify(res.tenants));
        router.push("/select-tenant");
      } else {
        setError("access denied");
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : "login failed");
    } finally {
      setLoading(false);
    }
  }

  return (
    <div className="relative flex min-h-screen items-center justify-center overflow-hidden bg-background p-4">
      {/* Ambient background grid + glow */}
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

      {/* Login card — CRT terminal */}
      <div className="relative w-full max-w-md scan-in">
        {/* Terminal title bar */}
        <div className="flex items-center gap-2 border border-border border-b-0 bg-card/60 px-3 py-2">
          <div className="flex gap-1.5">
            <span className="led bg-destructive text-destructive" />
            <span className="led bg-warning text-warning" />
            <span className="led bg-success text-success" />
          </div>
          <div className="ml-2 flex items-center gap-1.5 font-mono text-[10px] uppercase tracking-wider text-muted-foreground">
            <TerminalSquare className="h-3 w-3 text-primary" />
            llm-gw / login
          </div>
          <span className="ml-auto font-mono text-[9px] uppercase tracking-wider text-muted-foreground/50">
            tty/0
          </span>
        </div>

        {/* Card body */}
        <form
          onSubmit={handleSubmit}
          className="space-y-5 border border-border bg-card/80 p-6 backdrop-blur-sm"
        >
          {/* Brand */}
          <div className="space-y-2">
            <div className="flex items-center gap-2.5">
              <div className="relative flex h-9 w-9 items-center justify-center rounded-sm border border-primary/40 bg-primary/5 box-glow-amber">
                <ShieldCheck className="h-4 w-4 text-primary glow-amber" />
              </div>
              <div>
                <div className="font-mono text-[11px] uppercase tracking-[0.22em] text-primary glow-amber">
                  llm-gateway
                </div>
                <div className="font-mono text-[9px] uppercase tracking-[0.2em] text-muted-foreground">
                  control plane access
                </div>
              </div>
            </div>
          </div>

          <div className="space-y-4">
            {/* Email */}
            <div className="space-y-2">
              <Label htmlFor="email">
                <span className="text-primary">user@</span>llm-gw:~$
              </Label>
              <Input
                id="email"
                type="email"
                value={email}
                onChange={(e) => {
                  setEmail(e.target.value);
                  setError("");
                }}
                placeholder="user123456@llmgw.org"
                autoComplete="username"
                autoFocus
                required
                className="font-mono"
              />
            </div>

            {/* Password */}
            <div className="space-y-2">
              <Label htmlFor="password">
                <span className="text-primary">pass:</span> authenticate
              </Label>
              <Input
                id="password"
                type="password"
                value={password}
                onChange={(e) => {
                  setPassword(e.target.value);
                  setError("");
                }}
                placeholder="••••••••"
                className="font-mono"
              />
            </div>

            {/* Error */}
            {error && (
              <div className="flex items-center gap-2 border border-destructive/30 bg-destructive/5 px-3 py-2 font-mono text-[11px] text-destructive scan-in">
                <span className="led bg-destructive text-destructive pulse-dot" />
                err: {error}
              </div>
            )}

            <Button type="submit" size="lg" className="w-full" disabled={loading}>
              {loading ? (
                <>
                  <span className="blink">█</span>
                  authenticating…
                </>
              ) : (
                <>
                  connect
                  <ChevronRight className="h-3.5 w-3.5" />
                </>
              )}
            </Button>
          </div>
        </form>

        <div className="mt-3 flex items-center justify-between font-mono text-[9px] uppercase tracking-wider text-muted-foreground/40">
          <span>sign in with your account</span>
          <span>v1.0</span>
        </div>
      </div>
    </div>
  );
}
