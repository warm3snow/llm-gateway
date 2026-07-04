"use client";

import { useState } from "react";
import { useConfig, useUpdateConfig } from "@/lib/queries";
import type { ServerConfig } from "@/lib/types";
import { PageHeader } from "@/components/ui/page-header";
import { Panel, PanelHeader, PanelTitle, PanelBody } from "@/components/ui/panel";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import { Skeleton } from "@/components/ui/skeleton";
import { Badge } from "@/components/ui/badge";
import { toast } from "sonner";
import {
  Save,
  Server,
  ShieldCheck,
  Database,
  Zap,
  RotateCcw,
} from "lucide-react";

export default function SettingsPage() {
  const { data: initialConfig, isLoading } = useConfig();
  const updateMut = useUpdateConfig();
  const [draft, setDraft] = useState<ServerConfig | null>(null);

  const config = draft ?? initialConfig ?? {};

  function setServer(field: keyof NonNullable<ServerConfig["server"]>, value: unknown) {
    setDraft((prev) => ({
      ...(prev ?? initialConfig ?? {}),
      server: { ...(prev?.server ?? initialConfig?.server ?? {}), [field]: value },
    }));
  }
  function setSecurity(field: keyof NonNullable<ServerConfig["security"]>, value: unknown) {
    setDraft((prev) => ({
      ...(prev ?? initialConfig ?? {}),
      security: { ...(prev?.security ?? initialConfig?.security ?? {}), [field]: value },
    }));
  }
  function setCache(field: keyof NonNullable<ServerConfig["cache"]>, value: unknown) {
    setDraft((prev) => ({
      ...(prev ?? initialConfig ?? {}),
      cache: { ...(prev?.cache ?? initialConfig?.cache ?? {}), [field]: value },
    }));
  }
  function setDb(field: keyof NonNullable<ServerConfig["database"]>, value: unknown) {
    setDraft((prev) => ({
      ...(prev ?? initialConfig ?? {}),
      database: { ...(prev?.database ?? initialConfig?.database ?? {}), [field]: value },
    }));
  }

  function reset() {
    setDraft(null);
    toast.info("draft discarded");
  }

  async function save() {
    try {
      await updateMut.mutateAsync(config);
      toast.success("config saved", {
        description: "Changes applied to the gateway",
      });
      setDraft(null);
    } catch (err) {
      toast.error("save failed", {
        description: err instanceof Error ? err.message : String(err),
      });
    }
  }

  const isDirty = draft != null;

  const corsStr = (config.security?.allowedOrigins ?? []).join(", ");
  const cacheTtlSec = Math.round((config.cache?.defaultTTL ?? 0) / 1e9) || 0;

  return (
    <div className="scan-in">
      <PageHeader
        code="06 / system"
        title="Settings"
        description="Gateway configuration — applied on save"
        actions={
          <div className="flex items-center gap-2">
            {isDirty && (
              <Button
                variant="ghost"
                size="sm"
                onClick={reset}
                disabled={updateMut.isPending}
              >
                <RotateCcw className="h-3.5 w-3.5" />
                discard
              </Button>
            )}
            <Button
              size="sm"
              onClick={save}
              disabled={!isDirty || updateMut.isPending}
            >
              <Save className="h-3.5 w-3.5" />
              {updateMut.isPending ? "applying…" : "apply config"}
            </Button>
          </div>
        }
      />

      {/* Dirty indicator */}
      {isDirty && (
        <div className="mb-4 flex items-center gap-2 border border-warning/30 bg-warning/5 px-3 py-2">
          <span className="led bg-warning text-warning pulse-dot" />
          <span className="font-mono text-[11px] text-warning">
            unsaved changes — pending apply
          </span>
        </div>
      )}

      {isLoading ? (
        <div className="space-y-6">
          {Array.from({ length: 3 }).map((_, i) => (
            <Skeleton key={i} className="h-64" />
          ))}
        </div>
      ) : (
        <div className="space-y-6">
          {/* General / Server */}
          <Panel>
            <PanelHeader>
              <div className="flex items-center gap-2">
                <Server className="h-3.5 w-3.5 text-primary" />
                <PanelTitle>server</PanelTitle>
              </div>
              <Badge variant="outline">general</Badge>
            </PanelHeader>
            <PanelBody className="grid grid-cols-1 gap-4 md:grid-cols-2">
              <div className="space-y-2">
                <Label htmlFor="port">listen port</Label>
                <Input
                  id="port"
                  type="number"
                  value={config.server?.port ?? 8080}
                  onChange={(e) => setServer("port", parseInt(e.target.value) || 0)}
                />
              </div>
              <div className="space-y-2">
                <Label htmlFor="host">host</Label>
                <Input
                  id="host"
                  value={config.server?.host ?? "0.0.0.0"}
                  onChange={(e) => setServer("host", e.target.value)}
                />
              </div>
            </PanelBody>
          </Panel>

          {/* Security */}
          <Panel>
            <PanelHeader>
              <div className="flex items-center gap-2">
                <ShieldCheck className="h-3.5 w-3.5 text-accent" />
                <PanelTitle>security</PanelTitle>
              </div>
              <Badge variant="outline">auth</Badge>
            </PanelHeader>
            <PanelBody className="space-y-4">
              <div className="space-y-2">
                <Label htmlFor="cors">allowed origins (comma-separated)</Label>
                <Input
                  id="cors"
                  value={corsStr}
                  placeholder="http://localhost:3000"
                  onChange={(e) =>
                    setSecurity(
                      "allowedOrigins",
                      e.target.value
                        .split(",")
                        .map((s) => s.trim())
                        .filter(Boolean)
                    )
                  }
                />
                <p className="font-mono text-[10px] text-muted-foreground/60">
                  Leave empty to allow all origins
                </p>
              </div>
              <div className="space-y-2">
                <Label htmlFor="jwt">jwt secret</Label>
                <Input
                  id="jwt"
                  type="password"
                  value={config.security?.jwtSecret ?? ""}
                  placeholder="••••••••••••"
                  onChange={(e) => setSecurity("jwtSecret", e.target.value)}
                />
              </div>
            </PanelBody>
          </Panel>

          {/* Cache */}
          <Panel>
            <PanelHeader>
              <div className="flex items-center gap-2">
                <Zap className="h-3.5 w-3.5 text-primary" />
                <PanelTitle>cache</PanelTitle>
              </div>
              <Badge variant="outline">response</Badge>
            </PanelHeader>
            <PanelBody className="space-y-4">
              <div className="flex items-center justify-between gap-4 rounded-sm border border-border bg-background/30 px-3 py-2.5">
                <div className="space-y-0.5">
                  <Label htmlFor="cache-enabled">enabled</Label>
                  <p className="font-mono text-[10px] text-muted-foreground/60">
                    Cache identical responses
                  </p>
                </div>
                <Switch
                  id="cache-enabled"
                  checked={config.cache?.enabled ?? false}
                  onCheckedChange={(v) => setCache("enabled", v)}
                />
              </div>
              <div className="space-y-2">
                <Label htmlFor="ttl">default ttl (seconds)</Label>
                <Input
                  id="ttl"
                  type="number"
                  value={cacheTtlSec}
                  onChange={(e) =>
                    setCache("defaultTTL", (parseInt(e.target.value) || 0) * 1e9)
                  }
                />
              </div>
            </PanelBody>
          </Panel>

          {/* Database */}
          <Panel>
            <PanelHeader>
              <div className="flex items-center gap-2">
                <Database className="h-3.5 w-3.5 text-accent" />
                <PanelTitle>database</PanelTitle>
              </div>
              <Badge variant="outline">storage</Badge>
            </PanelHeader>
            <PanelBody className="grid grid-cols-1 gap-4 md:grid-cols-2">
              <div className="space-y-2">
                <Label htmlFor="driver">driver</Label>
                <Input
                  id="driver"
                  value={config.database?.driver ?? "sqlite"}
                  onChange={(e) => setDb("driver", e.target.value)}
                />
              </div>
              <div className="space-y-2">
                <Label htmlFor="dsn">dsn</Label>
                <Input
                  id="dsn"
                  type="password"
                  value={config.database?.dsn ?? ""}
                  placeholder="••••••••"
                  onChange={(e) => setDb("dsn", e.target.value)}
                />
              </div>
            </PanelBody>
          </Panel>

          {/* JSON preview — raw config dump for debugging */}
          <Panel>
            <PanelHeader>
              <div className="flex items-center gap-2">
                <span className="font-mono text-[10px] uppercase tracking-[0.18em] text-muted-foreground">
                  raw config
                </span>
              </div>
              <Badge variant="outline">json</Badge>
            </PanelHeader>
            <PanelBody>
              <pre className="max-h-72 overflow-auto rounded-sm bg-background/60 p-3 font-mono text-[11px] text-muted-foreground">
                {JSON.stringify(config, null, 2)}
              </pre>
            </PanelBody>
          </Panel>
        </div>
      )}
    </div>
  );
}
