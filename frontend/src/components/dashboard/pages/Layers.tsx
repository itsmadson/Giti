"use client";

import { useEffect, useMemo, useState } from "react";
import Link from "next/link";
import { useParams } from "next/navigation";
import { Search, Pencil } from "lucide-react";
import { useT } from "@/i18n/provider";
import { listLayers } from "@/api/dashboard/layers/api";
import type { Layer } from "@/api/dashboard/layers/types";
import { DataTable } from "@/components/ui/DataTable";
import { Badge } from "@/components/ui/Badge";
import { LayerEditDrawer } from "@/components/dashboard/layers/LayerEditDrawer";

const PAGE = 25;

export function Layers() {
  const { t } = useT();
  const params = useParams();
  const locale = (params?.locale as string) ?? "en";
  const [items, setItems] = useState<Layer[]>([]);
  const [q, setQ] = useState("");
  const [page, setPage] = useState(0);
  const [edit, setEdit] = useState<Layer | null>(null);

  const load = () => listLayers().then(setItems).catch(() => setItems([]));
  useEffect(() => {
    load();
  }, []);

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
          placeholder={t("layers.search")}
          className="w-full bg-transparent text-sm outline-none"
        />
      </div>

      <DataTable
        columns={[t("layers.name"), t("layers.type"), t("layers.style"), ""]}
        rows={shown.map((l) => [
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
          <div key="m" className="flex items-center justify-end gap-3">
            <button
              onClick={() => setEdit(l)}
              className="inline-flex items-center gap-1 text-sm text-[var(--color-primary)] hover:underline"
            >
              <Pencil size={13} /> {t("action.edit")}
            </button>
            <Link
              href={`/${locale}/map?layer=${l.workspace}:${l.name}`}
              className="text-sm text-[var(--color-muted)] hover:text-[var(--color-text)]"
            >
              {t("action.openMap")}
            </Link>
          </div>,
        ])}
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
    </div>
  );
}
