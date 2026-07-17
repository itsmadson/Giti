"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { useParams } from "next/navigation";
import { useT } from "@/i18n/provider";
import { listLayers } from "@/api/dashboard/layers/api";
import type { Layer } from "@/api/dashboard/layers/types";
import { DataTable } from "@/components/ui/DataTable";
import { Badge } from "@/components/ui/Badge";

export function Layers() {
  const { t } = useT();
  const params = useParams();
  const locale = (params?.locale as string) ?? "en";
  const [items, setItems] = useState<Layer[]>([]);

  useEffect(() => {
    listLayers().then(setItems).catch(() => setItems([]));
  }, []);

  return (
    <div className="mx-auto max-w-4xl space-y-4">
      <DataTable
        columns={[t("layers.name"), t("layers.type"), t("layers.style"), ""]}
        rows={items.map((l) => [
          <Link
            key="n"
            href={`/${locale}/dashboard/layers/${l.workspace}/${l.name}`}
            className="font-mono text-[var(--color-primary)] hover:underline"
          >
            {l.workspace}:{l.name}
          </Link>,
          <Badge key="t" tone={l.type === "RASTER" ? "warn" : "primary"}>
            {l.type}
          </Badge>,
          <span key="s" className="text-[var(--color-muted)]">{l.defaultStyle}</span>,
          <Link
            key="m"
            href={`/${locale}/map?layer=${l.workspace}:${l.name}`}
            className="text-sm text-[var(--color-primary)] hover:underline"
          >
            {t("action.openMap")}
          </Link>,
        ])}
        empty={t("layers.empty")}
      />
    </div>
  );
}
