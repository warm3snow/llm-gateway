"use client";

import { useState, type ReactNode } from "react";
import { currentRole } from "@/lib/api";
import { EmptyState } from "@/components/ui/empty-state";
import { ShieldCheck } from "lucide-react";

export function RoleGate({
  allowed,
  children,
}: {
  allowed: string[];
  children: ReactNode;
}) {
  const [role] = useState<string | null>(() => currentRole());

  if (!role || !allowed.includes(role)) {
    return (
      <EmptyState
        title="access denied"
        description="This view is not available for your role."
        icon={<ShieldCheck className="h-5 w-5" />}
      />
    );
  }

  return <>{children}</>;
}

export function roleScopeLabel(role: string | null) {
  if (role === "super_admin") return "All tenants";
  if (role === "tenant_admin") return "Your tenant";
  if (role === "tenant_user") return "Your keys and requests";
  return "Current scope";
}
