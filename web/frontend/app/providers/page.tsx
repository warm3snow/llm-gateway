"use client";

import { PageHeader } from "@/components/ui/page-header";
import { Tabs, TabsList, TabsTrigger, TabsContent } from "@/components/ui/tabs";
import { ProvidersPanel } from "@/components/admin/providers-panel";
import { PricingPanel } from "@/components/admin/pricing-panel";

export default function ModelManagementPage() {
  return (
    <div className="scan-in">
      <PageHeader
        code="05 / routing"
        title="Models"
        description="Upstream providers and per-model token pricing (database-managed)"
      />
      <Tabs defaultValue="providers">
        <TabsList>
          <TabsTrigger value="providers">providers</TabsTrigger>
          <TabsTrigger value="pricing">model pricing</TabsTrigger>
        </TabsList>
        <TabsContent value="providers">
          <ProvidersPanel />
        </TabsContent>
        <TabsContent value="pricing">
          <PricingPanel />
        </TabsContent>
      </Tabs>
    </div>
  );
}
