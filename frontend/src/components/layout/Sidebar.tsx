"use client";

import Link from "next/link";
import { usePathname, useParams } from "next/navigation";
import { motion } from "framer-motion";
import { useT } from "@/i18n/provider";
import { icons } from "@/components/icons";
import { cn } from "@/lib/utils";

type NavItem = { key: string; href: string; icon: string };

const items: NavItem[] = [
  { key: "nav.overview", href: "/dashboard", icon: "overview" },
  { key: "nav.workspaces", href: "/dashboard/workspaces", icon: "workspaces" },
  { key: "nav.stores", href: "/dashboard/stores", icon: "stores" },
  { key: "nav.layers", href: "/dashboard/layers", icon: "layers" },
  { key: "nav.styles", href: "/dashboard/styles", icon: "styles" },
  { key: "nav.tileCache", href: "/dashboard/tile-cache", icon: "tileCache" },
  { key: "nav.security", href: "/dashboard/security", icon: "security" },
  { key: "nav.wps", href: "/dashboard/wps", icon: "wps" },
  { key: "nav.conversions", href: "/dashboard/conversions", icon: "conversions" },
  { key: "nav.settings", href: "/dashboard/settings", icon: "settings" },
];

export function Sidebar() {
  const { t } = useT();
  const pathname = usePathname();
  const params = useParams();
  const locale = (params?.locale as string) ?? "en";
  const Brand = icons.brand;

  const isActive = (href: string) => {
    const full = `/${locale}${href}`;
    if (href === "/dashboard") return pathname === full;
    return pathname.startsWith(full);
  };

  return (
    <aside className="flex w-60 shrink-0 flex-col border-e border-[var(--color-border)] bg-[var(--color-surface)]">
      <div className="flex items-center gap-2.5 px-5 py-5">
        <div className="flex h-8 w-8 items-center justify-center rounded-lg bg-[var(--color-accent)] text-[var(--color-primary-fg)]">
          <Brand size={17} />
        </div>
        <span className="font-display text-base font-semibold tracking-tight">{t("app.name")}</span>
      </div>

      <nav className="flex-1 px-2.5 py-2">
        <Link
          href={`/${locale}/map`}
          className={cn(
            "relative mb-2 flex items-center gap-3 rounded-md px-3 py-2 text-sm font-medium",
            isActive("/map")
              ? "text-[var(--color-text)]"
              : "text-[var(--color-muted)] hover:text-[var(--color-text)]",
          )}
        >
          <Meridian show={isActive("/map")} />
          <MapPin /> {t("nav.map")}
        </Link>

        <div className="my-2 h-px bg-[var(--color-border)]" />

        {items.map((it) => {
          const Icon = icons[it.icon];
          const active = isActive(it.href);
          return (
            <Link
              key={it.href}
              href={`/${locale}${it.href}`}
              className={cn(
                "relative flex items-center gap-3 rounded-md px-3 py-2 text-sm font-medium",
                active
                  ? "text-[var(--color-text)]"
                  : "text-[var(--color-muted)] hover:text-[var(--color-text)]",
              )}
            >
              <Meridian show={active} />
              <Icon size={17} /> {t(it.key)}
            </Link>
          );
        })}
      </nav>

      <div className="px-5 py-4 text-xs text-[var(--color-muted)]">{t("app.tagline")}</div>
    </aside>
  );
}

// the "prime meridian": a teal marker that slides to the active item
function Meridian({ show }: { show: boolean }) {
  if (!show) return null;
  return (
    <motion.span
      layoutId="meridian"
      className="absolute inset-y-1 start-0 w-[3px] rounded-full bg-[var(--color-accent)]"
      transition={{ type: "spring", stiffness: 500, damping: 40 }}
    />
  );
}

function MapPin() {
  const Icon = icons.map;
  return <Icon size={17} />;
}
