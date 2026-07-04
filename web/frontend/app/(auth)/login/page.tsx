"use client";

import { useState, useEffect, FormEvent } from "react";
import { useRouter } from "next/navigation";
import { setCookie } from "cookies-next";
import { api } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { TerminalSquare, ChevronRight, ShieldCheck } from "lucide-react";

const bootLines = [
  "initializing control plane",
  "loading provider registry",
  "mounting routing strategies",
  "ready.",
];

export default function LoginPage() {
  const router = useRouter();
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");
  const [booted, setBooted] = useState(false);
  const [visibleBoot, setVisibleBoot] = useState<string[]>([]);

  // Simulated boot sequence — staggered lines for the CRT warmup feel.
  useEffect(() => {
    let i = 0;
    const timer = setInterval(() => {
      if (i < bootLines.length) {
        setVisibleBoot((prev) => [...prev, bootLines[i]]);
        i++;
      } else {
        clearInterval(timer);
        setBooted(true);
      }
    }, 280);
    return () => clearInterval(timer);
  }, []);

  async function handleSubmit(e: FormEvent) {
    e.preventDefault();
    setError("");
    if (!username.trim()) return setError("username required");
    if (password.length < 3) return setError("password too short");

    setLoading(true);
    try {
      const res = await api.login({ username, password });
      if (res.status === "success" && res.token) {
        setCookie("auth_token", res.token, {
          maxAge: 86400,
          path: "/",
          secure: process.env.NODE_ENV === "production",
          sameSite: "strict",
        });
        router.push("/dashboard");
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

          {/* Boot lines */}
          <div className="space-y-0.5 font-mono text-[11px] text-muted-foreground/70">
            {visibleBoot.map((line, i) => (
              <div key={i} className="scan-in">
                <span className="text-success">›</span> {line}
              </div>
            ))}
            {!booted && (
              <div>
                <span className="text-primary">›</span>{" "}
                <span className="blink">█</span>
              </div>
            )}
          </div>

          {booted && (
            <div className="space-y-4 scan-in">
              {/* Username */}
              <div className="space-y-2">
                <Label htmlFor="username">
                  <span className="text-primary">user@</span>llm-gw:~$
                </Label>
                <Input
                  id="username"
                  value={username}
                  onChange={(e) => {
                    setUsername(e.target.value);
                    setError("");
                  }}
                  placeholder="admin"
                  autoFocus
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

              <Button
                type="submit"
                size="lg"
                className="w-full"
                disabled={loading}
              >
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
          )}
        </form>

        {/* Footer — hint */}
        <div className="mt-3 flex items-center justify-between font-mono text-[9px] uppercase tracking-wider text-muted-foreground/40">
          <span>default: admin / admin123</span>
          <span>v1.0</span>
        </div>
      </div>
    </div>
  );
}
