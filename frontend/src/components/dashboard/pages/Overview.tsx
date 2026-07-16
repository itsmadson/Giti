"use client";

import { useEffect, useState } from "react";
import { useT } from "@/i18n/provider";
import { getOverview } from "@/api/dashboard/overview/api";
import type { OverviewStats } from "@/api/dashboard/overview/types";
import { StatCard } from "@/components/ui/StatCard";
import { Card } from "@/components/ui/Card";
import { Badge } from "@/components/ui/Badge";

export function Overview() {
  const { t } = useT();
  const [stats, setStats] = useState<OverviewStats | null>(null);

  useEffect(() => {
    getOverview().then(setStats).catch(() => setStats(null));
  }, []);

  const online = stats?.services.filter((s) => s.online).length ?? 0;
  const total = stats?.services.length ?? 0;

  return (
    <div className="mx-auto max-w-6xl space-y-6">
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
        <StatCard label={t("overview.workspaces")} value={stats?.workspaces ?? "—"} />
        <StatCard label={t("overview.layers")} value={stats?.layers ?? "—"} />
        <StatCard
          label={t("overview.health")}
          value={total ? `${online}/${total}` : "—"}
          hint={`${online} ${t("overview.online")}`}
        />
        <StatCard label={t("overview.cacheHit")} value="—" />
      </div>

      <Card className="p-5">
        <div className="mb-4 text-xs font-medium uppercase tracking-wide text-[var(--color-muted)]">
          {t("overview.health")}
        </div>
        <div className="flex flex-wrap gap-2">
          {(stats?.services ?? []).map((s) => (
            <Badge key={s.name} tone={s.online ? "ok" : "err"}>
              <span
                className="inline-block h-1.5 w-1.5 rounded-full"
                style={{ background: s.online ? "var(--color-ok)" : "var(--color-err)" }}
              />
              <span className="font-mono">{s.name}</span>
            </Badge>
          ))}
          {!stats && <span className="text-sm text-[var(--color-muted)]">{t("common.loading")}</span>}
        </div>
      </Card>
    </div>
  );
}
