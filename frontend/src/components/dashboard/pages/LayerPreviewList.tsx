"use client";

import { useEffect, useMemo, useState } from "react";
import Link from "next/link";
import { useParams } from "next/navigation";
import { Search, Map as MapIcon, Eye, X } from "lucide-react";
import { useT } from "@/i18n/provider";
import { listLayers, getLayerDetail } from "@/api/dashboard/layers/api";
import type { Layer } from "@/api/dashboard/layers/types";
import { DataTable } from "@/components/ui/DataTable";
import { LayerPreviewMap } from "@/components/map/LayerPreviewMap";

const PAGE = 25;

// WFS GetFeature download URL for a published layer.
function wfsUrl(id: string, format: string): string {
  const base = process.env.NEXT_PUBLIC_API_BASE ?? "";
  const p = new URLSearchParams({
    service: "WFS",
    version: "2.0.0",
    request: "GetFeature",
    typeNames: id,
    outputFormat: format,
    count: "1000",
  });
  return `${base}/giti/wfs?${p.toString()}`;
}

export function LayerPreviewList() {
  const { t } = useT();
  const params = useParams();
  const locale = (params?.locale as string) ?? "en";
  const [items, setItems] = useState<Layer[]>([]);
  const [q, setQ] = useState("");
  const [page, setPage] = useState(0);
  const [preview, setPreview] = useState<{ id: string; geomType: string; bbox?: number[] } | null>(null);

  useEffect(() => {
    listLayers().then(setItems).catch(() => setItems([]));
  }, []);

  async function openPreview(l: Layer) {
    const id = `${l.workspace}:${l.name}`;
    setPreview({ id, geomType: "", bbox: undefined });
    try {
      const d = await getLayerDetail(l.workspace, l.name);
      setPreview({ id, geomType: d.geomType, bbox: d.bbox });
    } catch {
      /* keep basic preview */
    }
  }

  const filtered = useMemo(() => {
    const n = q.trim().toLowerCase();
    return items.filter((l) => `${l.workspace}:${l.name}`.toLowerCase().includes(n));
  }, [items, q]);
  const pages = Math.max(1, Math.ceil(filtered.length / PAGE));
  const shown = filtered.slice(page * PAGE, page * PAGE + PAGE);

  return (
    <div className="mx-auto max-w-4xl space-y-4">
      <div className="flex items-center gap-2 rounded-md border border-[var(--color-border)] bg-[var(--color-surface)] px-3 py-2">
        <Search size={15} className="text-[var(--color-muted)]" />
        <input
          value={q}
          onChange={(e) => {
            setQ(e.target.value);
            setPage(0);
          }}
          placeholder={t("preview.search")}
          className="w-full bg-transparent text-sm outline-none"
        />
      </div>

      <DataTable
        columns={[t("layers.name"), t("preview.map"), t("preview.formats")]}
        rows={shown.map((l) => {
          const id = `${l.workspace}:${l.name}`;
          return [
            <span key="n" className="font-mono">{id}</span>,
            <div key="m" className="flex items-center gap-3">
              <button
                onClick={() => openPreview(l)}
                className="inline-flex items-center gap-1 text-sm text-[var(--color-primary)] hover:underline"
              >
                <Eye size={13} /> {t("preview.open")}
              </button>
              <Link
                href={`/${locale}/map?layer=${id}`}
                className="inline-flex items-center gap-1 text-sm text-[var(--color-muted)] hover:text-[var(--color-text)]"
              >
                <MapIcon size={13} /> {t("preview.fullMap")}
              </Link>
            </div>,
            <span key="f" className="flex flex-wrap gap-2 text-xs">
              <a className="text-[var(--color-primary)] hover:underline" href={wfsUrl(id, "application/json")} target="_blank" rel="noreferrer">GeoJSON</a>
              <a className="text-[var(--color-primary)] hover:underline" href={wfsUrl(id, "GML3")} target="_blank" rel="noreferrer">GML</a>
              <a className="text-[var(--color-primary)] hover:underline" href={wfsUrl(id, "csv")} target="_blank" rel="noreferrer">CSV</a>
              <a className="text-[var(--color-primary)] hover:underline" href={wfsUrl(id, "KML")} target="_blank" rel="noreferrer">KML</a>
            </span>,
          ];
        })}
        empty={t("layers.empty")}
      />

      {pages > 1 && (
        <div className="flex items-center justify-center gap-3 text-sm">
          <button disabled={page === 0} onClick={() => setPage((p) => p - 1)} className="disabled:opacity-40">‹</button>
          <span className="text-[var(--color-muted)]">{page + 1} / {pages}</span>
          <button disabled={page >= pages - 1} onClick={() => setPage((p) => p + 1)} className="disabled:opacity-40">›</button>
        </div>
      )}

      {preview && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4" onClick={() => setPreview(null)}>
          <div
            className="flex h-[80vh] w-full max-w-5xl flex-col overflow-hidden rounded-xl border border-[var(--color-border)] bg-[var(--color-surface)] shadow-2xl"
            onClick={(e) => e.stopPropagation()}
          >
            <div className="flex items-center justify-between border-b border-[var(--color-border)] px-4 py-2.5">
              <span className="font-mono text-sm">{preview.id}</span>
              <button onClick={() => setPreview(null)} className="rounded-md p-1 text-[var(--color-muted)] hover:bg-[var(--color-surface-2)]">
                <X size={18} />
              </button>
            </div>
            <LayerPreviewMap layer={preview.id} geomType={preview.geomType} bbox={preview.bbox} className="flex-1" />
          </div>
        </div>
      )}
    </div>
  );
}
