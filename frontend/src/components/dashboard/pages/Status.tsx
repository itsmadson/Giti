"use client";

import { useEffect, useState } from "react";
import { useT } from "@/i18n/provider";
import { Card } from "@/components/ui/Card";
import { Badge } from "@/components/ui/Badge";

const BASE = process.env.NEXT_PUBLIC_API_BASE ?? "";

const checks: { name: string; path: string }[] = [
  { name: "gateway", path: "/healthz" },
  { name: "catalog", path: "/api/v1/store-types" },
  { name: "auth", path: "/giti/rest/security/roles.json" },
  { name: "wfs", path: "/giti/wfs?service=WFS&request=GetCapabilities" },
  { name: "wms", path: "/giti/wms?service=WMS&request=GetCapabilities" },
  { name: "tiles", path: "/tiles/iran:cities/0/0/0.pbf" },
];

export function Status() {
  const { t } = useT();
  const [state, setState] = useState<Record<string, boolean | null>>({});

  useEffect(() => {
    checks.forEach(async (c) => {
      try {
        const res = await fetch(BASE + c.path, { method: "GET" });
        setState((s) => ({ ...s, [c.name]: res.ok || res.status === 400 }));
      } catch {
        setState((s) => ({ ...s, [c.name]: false }));
      }
    });
  }, []);

  return (
    <div className="mx-auto max-w-2xl space-y-4">
      <Card className="p-5">
        <div className="mb-3 text-xs font-medium uppercase tracking-wide text-[var(--color-muted)]">{t("status.services")}</div>
        <div className="space-y-2">
          {checks.map((c) => {
            const up = state[c.name];
            return (
              <div key={c.name} className="flex items-center justify-between border-b border-[var(--color-border)] py-2 last:border-0">
                <span className="font-mono text-sm">{c.name}</span>
                {up === undefined ? (
                  <Badge tone="neutral">{t("common.loading")}</Badge>
                ) : up ? (
                  <Badge tone="ok">{t("status.up")}</Badge>
                ) : (
                  <Badge tone="err">{t("status.down")}</Badge>
                )}
              </div>
            );
          })}
        </div>
      </Card>
    </div>
  );
}
