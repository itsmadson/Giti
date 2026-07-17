"use client";
import { useEffect, useMemo, useState } from "react";
import { useRouter, useParams } from "next/navigation";
import { Search } from "lucide-react";
import { navGroups } from "@/config/nav";
import { useT } from "@/i18n/provider";
import { icons } from "@/components/icons";

export function CommandPalette() {
  const [open, setOpen] = useState(false);
  const [q, setQ] = useState("");
  const router = useRouter();
  const params = useParams();
  const locale = (params?.locale as string) ?? "en";
  const { t } = useT();

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === "k") {
        e.preventDefault();
        setOpen((o) => !o);
      }
      if (e.key === "Escape") setOpen(false);
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, []);

  useEffect(() => {
    if (!open) setQ("");
  }, [open]);

  const results = useMemo(() => {
    const needle = q.trim().toLowerCase();
    return navGroups
      .map((g) => ({
        labelKey: g.labelKey,
        items: g.items.filter((it) => t(it.key).toLowerCase().includes(needle)),
      }))
      .filter((g) => g.items.length > 0);
  }, [q, t]);

  if (!open) return null;
  const go = (href: string) => {
    setOpen(false);
    router.push(`/${locale}${href}`);
  };

  return (
    <div
      className="fixed inset-0 z-[70] flex items-start justify-center bg-black/40 p-4 pt-[15vh]"
      onClick={() => setOpen(false)}
    >
      <div
        onClick={(e) => e.stopPropagation()}
        className="w-full max-w-lg overflow-hidden rounded-xl border border-[var(--color-border)] bg-[var(--color-surface)] shadow-2xl"
      >
        <div className="flex items-center gap-2 border-b border-[var(--color-border)] px-4 py-3">
          <Search size={16} className="text-[var(--color-muted)]" />
          <input
            autoFocus
            value={q}
            onChange={(e) => setQ(e.target.value)}
            placeholder={t("cmd.placeholder")}
            className="w-full bg-transparent text-sm outline-none"
          />
        </div>
        <div className="max-h-80 overflow-auto p-2">
          {results.length === 0 && (
            <div className="px-3 py-6 text-center text-sm text-[var(--color-muted)]">{t("cmd.empty")}</div>
          )}
          {results.map((g) => (
            <div key={g.labelKey}>
              <div className="px-3 pb-1 pt-2 text-[10px] uppercase tracking-wide text-[var(--color-muted)]">
                {t(g.labelKey)}
              </div>
              {g.items.map((it) => {
                const Icon = icons[it.icon];
                return (
                  <button
                    key={it.href}
                    onClick={() => go(it.href)}
                    className="flex w-full items-center gap-2 rounded-md px-3 py-2 text-start text-sm text-[var(--color-text)] hover:bg-[var(--color-surface-2)]"
                  >
                    <Icon size={15} /> {t(it.key)}
                  </button>
                );
              })}
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}
