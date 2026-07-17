"use client";

import { useEffect, useMemo, useState } from "react";
import Link from "next/link";
import { useParams } from "next/navigation";
import { Search, Plus, Trash2, Palette } from "lucide-react";
import { useT } from "@/i18n/provider";
import { listStyles, deleteStyle } from "@/api/dashboard/styles/api";
import type { Style } from "@/api/dashboard/styles/types";
import { DataTable } from "@/components/ui/DataTable";
import { Badge } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { useToast } from "@/components/ui/Toast";

const PAGE = 25;

export function Styles() {
  const { t } = useT();
  const { toast } = useToast();
  const params = useParams();
  const locale = (params?.locale as string) ?? "en";
  const [items, setItems] = useState<Style[]>([]);
  const [q, setQ] = useState("");
  const [page, setPage] = useState(0);

  const load = () => listStyles().then(setItems).catch(() => setItems([]));
  useEffect(() => {
    load();
  }, []);

  const filtered = useMemo(() => {
    const n = q.trim().toLowerCase();
    return items.filter((s) => s.name.toLowerCase().includes(n));
  }, [items, q]);
  const pages = Math.max(1, Math.ceil(filtered.length / PAGE));
  const shown = filtered.slice(page * PAGE, page * PAGE + PAGE);

  async function remove(name: string) {
    if (!confirm(t("styles.deleteConfirm", { name }))) return;
    try {
      await deleteStyle(name);
      toast({ title: t("styles.deleted") });
      load();
    } catch (e) {
      toast({ title: (e as Error).message, tone: "err" });
    }
  }

  return (
    <div className="mx-auto max-w-4xl space-y-4">
      <div className="flex items-center gap-3">
        <div className="flex flex-1 items-center gap-2 rounded-md border border-[var(--color-border)] bg-[var(--color-surface)] px-3 py-2">
          <Search size={15} className="text-[var(--color-muted)]" />
          <input
            value={q}
            onChange={(e) => {
              setQ(e.target.value);
              setPage(0);
            }}
            placeholder={t("styles.search")}
            className="w-full bg-transparent text-sm outline-none"
          />
        </div>
        <Link href={`/${locale}/dashboard/styles/new`}>
          <Button>
            <Plus size={15} /> {t("styles.add")}
          </Button>
        </Link>
      </div>

      <DataTable
        columns={[t("styles.name"), t("styles.format"), ""]}
        rows={shown.map((s) => [
          <Link
            key="n"
            href={`/${locale}/dashboard/styles/${encodeURIComponent(s.name)}`}
            className="flex items-center gap-2 font-mono text-[var(--color-primary)] hover:underline"
          >
            <Palette size={13} /> {s.name}
          </Link>,
          <Badge key="f" tone="neutral">{s.format}</Badge>,
          <button
            key="d"
            onClick={() => remove(s.name)}
            className="flex justify-end text-[var(--color-muted)] hover:text-[var(--color-err)]"
          >
            <Trash2 size={15} />
          </button>,
        ])}
        empty={t("styles.empty")}
      />

      {pages > 1 && (
        <div className="flex items-center justify-center gap-3 text-sm">
          <button disabled={page === 0} onClick={() => setPage((p) => p - 1)} className="disabled:opacity-40">‹</button>
          <span className="text-[var(--color-muted)]">{page + 1} / {pages}</span>
          <button disabled={page >= pages - 1} onClick={() => setPage((p) => p + 1)} className="disabled:opacity-40">›</button>
        </div>
      )}
    </div>
  );
}
