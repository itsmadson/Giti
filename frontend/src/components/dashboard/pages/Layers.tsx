"use client";

import { useEffect, useMemo, useState } from "react";
import Link from "next/link";
import { useParams } from "next/navigation";
import { Search, Pencil, Eye, X } from "lucide-react";
import { useT } from "@/i18n/provider";
import { listLayers, getLayerDetail } from "@/api/dashboard/layers/api";
import type { Layer } from "@/api/dashboard/layers/types";
import { DataTable } from "@/components/ui/DataTable";
import { Badge } from "@/components/ui/Badge";
import { LayerEditDrawer } from "@/components/dashboard/layers/LayerEditDrawer";
import { LayerPreviewMap } from "@/components/map/LayerPreviewMap";
import { serviceSamples } from "@/lib/basemaps";

const PAGE = 25;

export function Layers() {
  const { t } = useT();
  const params = useParams();
  const locale = (params?.locale as string) ?? "en";
  const [items, setItems] = useState<Layer[]>([]);
  const [q, setQ] = useState("");
  const [page, setPage] = useState(0);
  const [edit, setEdit] = useState<Layer | null>(null);
  const [preview, setPreview] = useState<{ id: string; geomType: string; bbox?: number[] } | null>(null);

  const load = () => listLayers().then(setItems).catch(() => setItems([]));
  useEffect(() => {
    load();
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
    <div className="mx-auto max-w-5xl space-y-4">
      <div className="flex items-center gap-2 rounded-md border border-[var(--color-border)] bg-[var(--color-surface)] px-3 py-2">
        <Search size={15} className="text-[var(--color-muted)]" />
        <input
          value={q}
          onChange={(e) => {
            setQ(e.target.value);
            setPage(0);
          }}
          placeholder={t("layers.search")}
          className="w-full bg-transparent text-sm outline-none"
        />
      </div>

      <DataTable
        columns={[t("layers.name"), t("layers.type"), t("layers.style"), t("layers.services"), ""]}
        rows={shown.map((l) => {
          const id = `${l.workspace}:${l.name}`;
          const s = serviceSamples(id);
          return [
            <Link
              key="n"
              href={`/${locale}/dashboard/layers/${l.workspace}/${l.name}`}
              className="font-mono text-[var(--color-primary)] hover:underline"
            >
              {id}
            </Link>,
            <Badge key="t" tone={l.type === "RASTER" ? "warn" : "primary"}>{l.type}</Badge>,
            <span key="s" className="text-[var(--color-muted)]">{l.defaultStyle}</span>,
            <span key="svc" className="flex flex-wrap gap-2 text-xs">
              <a className="text-[var(--color-primary)] hover:underline" href={s.wms} target="_blank" rel="noreferrer">WMS</a>
              <a className="text-[var(--color-primary)] hover:underline" href={s.wfs} target="_blank" rel="noreferrer">WFS</a>
              <a className="text-[var(--color-muted)] hover:underline" href={s.wmsCaps} target="_blank" rel="noreferrer">Caps</a>
            </span>,
            <div key="a" className="flex items-center justify-end gap-3">
              <button
                onClick={() => openPreview(l)}
                className="inline-flex items-center gap-1 text-sm text-[var(--color-primary)] hover:underline"
              >
                <Eye size={13} /> {t("preview.open")}
              </button>
              <button
                onClick={() => setEdit(l)}
                className="inline-flex items-center gap-1 text-sm text-[var(--color-muted)] hover:text-[var(--color-text)]"
              >
                <Pencil size={13} /> {t("action.edit")}
              </button>
            </div>,
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

      {edit && (
        <LayerEditDrawer
          ws={edit.workspace}
          name={edit.name}
          open
          onClose={() => setEdit(null)}
          onSaved={() => {
            setEdit(null);
            load();
          }}
        />
      )}

      {preview && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4" onClick={() => setPreview(null)}>
          <div
            className="flex h-[80vh] w-full max-w-5xl flex-col overflow-hidden rounded-xl border border-[var(--color-border)] bg-[var(--color-surface)] shadow-2xl"
            onClick={(e) => e.stopPropagation()}
          >
            <div className="flex items-center justify-between border-b border-[var(--color-border)] px-4 py-2.5">
              <span className="font-mono text-sm">{preview.id}</span>
              <div className="flex items-center gap-3 text-xs">
                <a className="text-[var(--color-primary)] hover:underline" href={serviceSamples(preview.id, preview.bbox).wms} target="_blank" rel="noreferrer">WMS PNG</a>
                <a className="text-[var(--color-primary)] hover:underline" href={serviceSamples(preview.id, preview.bbox).wfs} target="_blank" rel="noreferrer">WFS JSON</a>
                <Link className="text-[var(--color-muted)] hover:text-[var(--color-text)]" href={`/${locale}/map?layer=${preview.id}`}>{t("preview.fullMap")}</Link>
                <button onClick={() => setPreview(null)} className="rounded-md p-1 text-[var(--color-muted)] hover:bg-[var(--color-surface-2)]">
                  <X size={18} />
                </button>
              </div>
            </div>
            <LayerPreviewMap layer={preview.id} geomType={preview.geomType} bbox={preview.bbox} className="flex-1" />
          </div>
        </div>
      )}
    </div>
  );
}
