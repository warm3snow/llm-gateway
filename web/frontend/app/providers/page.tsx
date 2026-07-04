"use client";

import { useProviders } from "@/lib/queries";
import { PageHeader } from "@/components/ui/page-header";
import { Panel, PanelHeader, PanelTitle, PanelBody } from "@/components/ui/panel";
import { Led } from "@/components/ui/led";
import { Badge } from "@/components/ui/badge";
import { Skeleton } from "@/components/ui/skeleton";
import { EmptyState } from "@/components/ui/empty-state";
import { Button } from "@/components/ui/button";
import { Server, Plus, Cpu, Globe, Gauge, Key } from "lucide-react";

function maskKey(k?: string) {
  if (!k) return "—";
  if (k.length <= 8) return k;
  return `${k.slice(0, 4)}••••${k.slice(-4)}`;
}

export default function ProvidersPage() {
  const { data: providers, isLoading } = useProviders();

  return (
    <div className="scan-in">
      <PageHeader
        code="02 / routing"
        title="Providers"
        description="Upstream LLM backends and their routing configuration"
        actions={
          <Button variant="outline" size="sm" disabled>
            <Plus className="h-3.5 w-3.5" />
            add provider
          </Button>
        }
      />

      {isLoading ? (
        <div className="grid grid-cols-1 gap-4 md:grid-cols-2 xl:grid-cols-3">
          {Array.from({ length: 6 }).map((_, i) => (
            <Skeleton key={i} className="h-48" />
          ))}
        </div>
      ) : !providers || providers.length === 0 ? (
        <EmptyState
          title="no providers"
          description="Add an upstream provider to start routing requests."
          icon={<Server className="h-5 w-5" />}
        />
      ) : (
        <div className="grid grid-cols-1 gap-4 md:grid-cols-2 xl:grid-cols-3">
          {providers.map((p) => (
            <Panel key={p.name} className="flex flex-col">
              {/* Header row — name + status */}
              <PanelHeader>
                <div className="flex min-w-0 items-center gap-2.5">
                  <Led
                    tone={p.enabled ? "success" : "muted"}
                    pulse={p.enabled}
                  />
                  <div className="min-w-0">
                    <div className="flex items-baseline gap-2">
                      <span className="truncate font-mono text-[13px] text-foreground">
                        {p.name}
                      </span>
                      <span className="font-mono text-[9px] uppercase tracking-wider text-muted-foreground/60">
                        {p.provider}
                      </span>
                    </div>
                  </div>
                </div>
                {p.enabled ? (
                  <Badge variant="success">up</Badge>
                ) : (
                  <Badge variant="outline">down</Badge>
                )}
              </PanelHeader>

              <PanelBody className="flex-1 space-y-2.5 font-mono text-[11px]">
                <div className="flex items-center justify-between gap-2">
                  <span className="flex items-center gap-1.5 text-muted-foreground">
                    <Gauge className="h-3 w-3" /> weight
                  </span>
                  <span className="num text-foreground">{p.weight}</span>
                </div>
                <div className="flex items-center justify-between gap-2">
                  <span className="flex items-center gap-1.5 text-muted-foreground">
                    <Cpu className="h-3 w-3" /> timeout
                  </span>
                  <span className="num text-foreground">
                    {p.requestTimeout}ms
                  </span>
                </div>
                <div className="flex items-center justify-between gap-2">
                  <span className="flex items-center gap-1.5 text-muted-foreground">
                    <Key className="h-3 w-3" /> api key
                  </span>
                  <span className="num text-muted-foreground">
                    {maskKey(p.apiKey)}
                  </span>
                </div>
                {p.customHost && (
                  <div className="flex items-center justify-between gap-2">
                    <span className="flex items-center gap-1.5 text-muted-foreground">
                      <Globe className="h-3 w-3" /> host
                    </span>
                    <span className="truncate text-muted-foreground">
                      {p.customHost}
                    </span>
                  </div>
                )}
              </PanelBody>

              {/* Footer — terminal-style id strip */}
              <div className="flex items-center justify-between border-t border-border px-4 py-2 font-mono text-[9px] uppercase tracking-wider text-muted-foreground/50">
                <span>id://{p.provider}</span>
                <span>configured</span>
              </div>
            </Panel>
          ))}
        </div>
      )}
    </div>
  );
}
