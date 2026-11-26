"use client";

import { useSyncExternalStore } from "react";
import { currentRole, currentTenant, currentTenantId, currentUser } from "./api";

export interface SessionSnapshot {
  user: ReturnType<typeof currentUser>;
  tenant: ReturnType<typeof currentTenant>;
  role: string | null;
  tenantId: number | null;
}

// One shared snapshot: useSyncExternalStore compares by reference, so the
// object must only change when the underlying session actually changes.
const EMPTY: SessionSnapshot = { user: null, tenant: null, role: null, tenantId: null };
let snapshot: SessionSnapshot = EMPTY;
let snapshotKey = "";

function readSessionKey(): string {
  if (typeof document === "undefined") return "";
  // Cheap change detector: cookie contents of the auth/tenant cookies.
  const match = document.cookie.match(/(?:^|;\s*)(auth_token|current_tenant)=/g);
  return match ? match.join("|") + document.cookie.length : "";
}

function getSnapshot(): SessionSnapshot {
  const key = readSessionKey();
  if (key === snapshotKey) return snapshot;
  snapshotKey = key;
  if (!key) {
    snapshot = EMPTY;
    return snapshot;
  }
  snapshot = {
    user: currentUser(),
    tenant: currentTenant(),
    role: currentRole(),
    tenantId: currentTenantId(),
  };
  return snapshot;
}

// Cookies are not available during server rendering, so the server snapshot is
// always empty. React uses it while hydrating and then re-renders with the real
// session, which keeps server and client markup identical.
function getServerSnapshot(): SessionSnapshot {
  return EMPTY;
}

function subscribe(onChange: () => void) {
  window.addEventListener("focus", onChange);
  return () => window.removeEventListener("focus", onChange);
}

/** The signed-in session, or an empty snapshot before hydration / when signed out. */
export function useSession(): SessionSnapshot {
  return useSyncExternalStore(subscribe, getSnapshot, getServerSnapshot);
}

/** The current role, or `null` during hydration and when signed out. */
export function useRole(): string | null {
  return useSession().role;
}
