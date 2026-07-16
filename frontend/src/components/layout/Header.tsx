"use client";

import { useRouter, usePathname, useParams } from "next/navigation";
import { useTheme } from "next-themes";
import { useEffect, useState } from "react";
import { Sun, Moon, LogOut, Languages } from "lucide-react";
import { useT } from "@/i18n/provider";
import { useSession, clearSession } from "@/api/auth/store";

// path suffix → nav dictionary key for the header title
const titleKey: Record<string, string> = {
  "/dashboard": "nav.overview",
  "/dashboard/workspaces": "nav.workspaces",
  "/dashboard/stores": "nav.stores",
  "/dashboard/layers": "nav.layers",
  "/dashboard/styles": "nav.styles",
  "/dashboard/tile-cache": "nav.tileCache",
  "/dashboard/security": "nav.security",
  "/dashboard/wps": "nav.wps",
  "/dashboard/conversions": "nav.conversions",
  "/dashboard/settings": "nav.settings",
  "/map": "nav.map",
};

export function Header() {
  const { t, locale } = useT();
  const router = useRouter();
  const pathname = usePathname();
  const params = useParams();
  const session = useSession();
  const { theme, setTheme } = useTheme();
  const [mounted, setMounted] = useState(false);
  useEffect(() => setMounted(true), []);

  const suffix = pathname.replace(`/${params.locale}`, "") || "/dashboard";
  const title = t(titleKey[suffix] ?? "nav.overview");

  function toggleLocale() {
    const next = locale === "en" ? "fa" : "en";
    router.push(`/${next}${suffix}`);
  }

  function signOut() {
    clearSession();
    router.replace(`/${locale}/login`);
  }

  return (
    <header className="flex h-14 items-center justify-between border-b border-[var(--color-border)] bg-[var(--color-surface)] px-6">
      <h1 className="font-display text-sm font-semibold tracking-tight">{title}</h1>
      <div className="flex items-center gap-1">
        <button
          onClick={toggleLocale}
          className="flex items-center gap-1.5 rounded-md px-2.5 py-1.5 text-xs font-medium text-[var(--color-muted)] hover:bg-[var(--color-surface-2)] hover:text-[var(--color-text)]"
        >
          <Languages size={15} /> {locale === "en" ? "فارسی" : "EN"}
        </button>
        {mounted && (
          <button
            onClick={() => setTheme(theme === "dark" ? "light" : "dark")}
            className="rounded-md p-2 text-[var(--color-muted)] hover:bg-[var(--color-surface-2)] hover:text-[var(--color-text)]"
            aria-label="Toggle theme"
          >
            {theme === "dark" ? <Sun size={16} /> : <Moon size={16} />}
          </button>
        )}
        <div className="mx-2 h-5 w-px bg-[var(--color-border)]" />
        <span className="font-mono text-xs text-[var(--color-muted)]">{session?.user}</span>
        <button
          onClick={signOut}
          className="ms-1 rounded-md p-2 text-[var(--color-muted)] hover:bg-[var(--color-surface-2)] hover:text-[var(--color-err)]"
          aria-label={t("action.signOut")}
        >
          <LogOut size={16} />
        </button>
      </div>
    </header>
  );
}
