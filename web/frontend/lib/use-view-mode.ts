"use client";

import { useCallback, useSyncExternalStore } from "react";

export type ViewMode = "cards" | "list";

const listeners = new Set<() => void>();

function subscribe(listener: () => void) {
  listeners.add(listener);
  return () => {
    listeners.delete(listener);
  };
}

function storageKey(key: string) {
  return `view-mode:${key}`;
}

function readMode(key: string, fallback: ViewMode): ViewMode {
  try {
    const stored = window.localStorage.getItem(storageKey(key));
    if (stored === "cards" || stored === "list") return stored;
  } catch {
    // localStorage unavailable (private mode) — keep the fallback.
  }
  return fallback;
}

/**
 * Persisted card/list preference for a page. Reads go through
 * useSyncExternalStore so server rendering and the first client render agree,
 * and every consumer of the same key stays in sync.
 */
export function useViewMode(key: string, fallback: ViewMode = "list") {
  const mode = useSyncExternalStore(
    subscribe,
    () => readMode(key, fallback),
    () => fallback
  );

  const update = useCallback(
    (next: ViewMode) => {
      try {
        window.localStorage.setItem(storageKey(key), next);
      } catch {
        // ignore storage failures
      }
      listeners.forEach((listener) => listener());
    },
    [key]
  );

  return [mode, update] as const;
}
