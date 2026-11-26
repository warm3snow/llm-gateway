"use client";

import { useEffect } from "react";

/**
 * Applies the persisted colour scheme.
 *
 * The app is dark-only by design, so instead of injecting a theme script we
 * normalise the documentElement class after mount. This keeps the server and
 * client markup identical (no <script> rendered from a React component), which
 * is what React 19 requires.
 */
const THEME_CLASSES = ["light", "dark"];

export function ThemeProvider({ children }: { children: React.ReactNode }) {
  useEffect(() => {
    const root = document.documentElement;
    let stored: string | null = null;
    try {
      stored = window.localStorage.getItem("theme");
    } catch {
      // storage unavailable — fall through to the dark default
    }
    const theme = stored === "light" ? "light" : "dark";
    root.classList.remove(...THEME_CLASSES);
    root.classList.add(theme);
    root.style.colorScheme = theme;
  }, []);

  return <>{children}</>;
}
