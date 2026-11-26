"use client";

import { useConfig } from "@/lib/queries";
import { PageHeader } from "@/components/ui/page-header";
import { Panel, PanelHeader, PanelBody } from "@/components/ui/panel";
import { Badge } from "@/components/ui/badge";
import { Skeleton } from "@/components/ui/skeleton";
import { RoleGate } from "@/components/auth/RoleGate";
import { PrivacyPanel } from "./privacy-panel";

export default function SettingsPage() {
  // Raw config is read-only here: server/database/cache/security are managed
  // through config.yaml (or environment variables) and a restart, not the
  // console. Only privacy masking is tunable at runtime.
  const { data: config, isLoading } = useConfig();

  return (
    <RoleGate allowed={["super_admin"]}>
      <div className="scan-in">
        <PageHeader
          code="08 / system"
          title="Settings"
          description="Runtime privacy masking and the effective gateway configuration"
        />

        <div className="space-y-6">
          <PrivacyPanel />

          <Panel>
            <PanelHeader>
              <span className="font-mono text-[10px] uppercase tracking-[0.18em] text-muted-foreground">
                raw config
              </span>
              <Badge variant="outline">read-only</Badge>
            </PanelHeader>
            <PanelBody>
              {isLoading ? (
                <Skeleton className="h-56" />
              ) : (
                <pre className="max-h-96 overflow-auto rounded-sm bg-background/60 p-3 font-mono text-[11px] text-muted-foreground">
                  {JSON.stringify(config ?? {}, null, 2)}
                </pre>
              )}
              <p className="mt-3 font-mono text-[10px] text-muted-foreground/60">
                Infrastructure settings (server, database, cache, security) live in config.yaml and apply on restart.
              </p>
            </PanelBody>
          </Panel>
        </div>
      </div>
    </RoleGate>
  );
}
