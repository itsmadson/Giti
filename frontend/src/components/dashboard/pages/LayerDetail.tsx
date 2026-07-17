"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { useParams } from "next/navigation";
import { ArrowLeft, Database, Ruler, Layers as LayersIcon } from "lucide-react";
import { useT } from "@/i18n/provider";
import { getLayerDetail } from "@/api/dashboard/layers/api";
import type { LayerDetail as Detail } from "@/api/dashboard/layers/types";
import { Card } from "@/components/ui/Card";
import { Badge } from "@/components/ui/Badge";
import { DataTable } from "@/components/ui/DataTable";
import { LayerPreview } from "@/components/map/LayerPreview";

export function LayerDetail() {
  const { t } = useT();
  const params = useParams();
  const locale = (params?.locale as string) ?? "en";
  const ws = params?.ws as string;
  const name = params?.name as string;
  const [d, setD] = useState<Detail | null>(null);
  const [err, setErr] = useState(false);

  useEffect(() => {
    getLayerDetail(ws, name).then(setD).catch(() => setErr(true));
  }, [ws, name]);

  if (err) return <p className="text-sm text-[var(--color-err)]">{t("common.error")}</p>;
  if (!d) return <p className="text-sm text-[var(--color-muted)]">{t("common.loading")}</p>;

  const id = `${d.workspace}:${d.name}`;
  const facts: [string, React.ReactNode][] = [
    [t("layers.type"), <Badge key="t" tone={d.type === "RASTER" ? "warn" : "primary"}>{d.type}</Badge>],
    [t("stores.name"), <span key="s" className="font-mono">{d.store}</span>],
    [t("layerDetail.table"), <span key="tb" className="font-mono">{d.table}</span>],
    [t("layerDetail.srs"), d.srs || "—"],
    [t("layerDetail.geom"), d.geomColumn ? `${d.geomColumn} (${d.geomType})` : "—"],
    [t("layerDetail.features"), d.featureCount.toLocaleString()],
    [t("layers.style"), d.defaultStyle || "—"],
  ];

  return (
    <div className="mx-auto max-w-5xl space-y-5">
      <div className="flex items-center gap-3">
        <Link
          href={`/${locale}/dashboard/layers`}
          className="rounded-md p-1.5 text-[var(--color-muted)] hover:bg-[var(--color-surface-2)]"
        >
          <ArrowLeft size={18} />
        </Link>
        <div>
          <h1 className="font-display text-xl font-semibold">{d.name}</h1>
          <p className="font-mono text-xs text-[var(--color-muted)]">{id}</p>
        </div>
        <Link
          href={`/${locale}/map?layer=${id}`}
          className="ms-auto text-sm text-[var(--color-primary)] hover:underline"
        >
          {t("action.openMap")}
        </Link>
      </div>

      <div className="grid gap-5 lg:grid-cols-[1fr_1.2fr]">
        <div className="space-y-5">
          <Card className="p-4">
            <div className="mb-3 flex items-center gap-2 text-xs font-medium uppercase tracking-wide text-[var(--color-muted)]">
              <Database size={13} /> {t("layerDetail.info")}
            </div>
            <dl className="space-y-2 text-sm">
              {facts.map(([k, v]) => (
                <div key={k} className="flex items-center justify-between gap-3">
                  <dt className="text-[var(--color-muted)]">{k}</dt>
                  <dd className="text-end">{v}</dd>
                </div>
              ))}
            </dl>
          </Card>

          {d.bbox && (
            <Card className="p-4">
              <div className="mb-2 flex items-center gap-2 text-xs font-medium uppercase tracking-wide text-[var(--color-muted)]">
                <Ruler size={13} /> {t("layerDetail.bbox")}
              </div>
              <p className="font-mono text-xs leading-relaxed text-[var(--color-muted)]">
                {d.bbox.map((n) => n.toFixed(4)).join(", ")}
              </p>
            </Card>
          )}
        </div>

        <Card className="overflow-hidden">
          <div className="flex items-center gap-2 border-b border-[var(--color-border)] px-4 py-2.5 text-xs font-medium uppercase tracking-wide text-[var(--color-muted)]">
            <LayersIcon size={13} /> {t("layerDetail.preview")}
          </div>
          <LayerPreview
            layer={id}
            geomType={d.geomType}
            bbox={d.bbox}
            className="h-[380px] w-full"
          />
        </Card>
      </div>

      <div>
        <div className="mb-2 text-xs font-medium uppercase tracking-wide text-[var(--color-muted)]">
          {t("layerDetail.attributes")}
        </div>
        <DataTable
          columns={[t("layerDetail.attrName"), t("layerDetail.attrType")]}
          rows={d.attributes.map((a) => [
            <span key="n" className="font-mono">{a.name}</span>,
            <span key="t" className="text-[var(--color-muted)]">{a.type}</span>,
          ])}
          empty={t("layerDetail.noAttrs")}
        />
      </div>
    </div>
  );
}
