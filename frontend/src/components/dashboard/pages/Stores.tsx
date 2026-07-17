"use client";

import { useEffect, useState } from "react";
import { Plus, Database, TableProperties, Check, Trash2 } from "lucide-react";
import { useT } from "@/i18n/provider";
import { listStores, listStoreTables, publishTable, deleteStore } from "@/api/dashboard/stores/api";
import type { Store, StoreTable } from "@/api/dashboard/stores/types";
import { DataTable } from "@/components/ui/DataTable";
import { Button } from "@/components/ui/Button";
import { Badge } from "@/components/ui/Badge";
import { Modal } from "@/components/ui/Modal";
import { useToast } from "@/components/ui/Toast";
import { ApiError } from "@/api/client";
import { NewStoreWizard } from "@/components/dashboard/stores/NewStoreWizard";

export function Stores() {
  const { t } = useT();
  const { toast } = useToast();
  const [items, setItems] = useState<Store[]>([]);
  const [adding, setAdding] = useState(false);
  const [manage, setManage] = useState<Store | null>(null);

  const load = () => listStores().then(setItems).catch(() => setItems([]));
  useEffect(() => {
    load();
  }, []);

  async function remove(s: Store) {
    if (!confirm(t("stores.deleteConfirm", { name: s.name }))) return;
    try {
      await deleteStore(s.workspace, s.name, false);
      toast({ title: t("stores.delete") });
      load();
    } catch (e) {
      const msg = e instanceof ApiError && e.status === 409 ? t("stores.hasLayers") : (e as Error).message;
      toast({ title: msg, tone: "err" });
    }
  }

  return (
    <div className="mx-auto max-w-4xl space-y-4">
      <div className="flex items-center justify-between">
        <p className="text-sm text-[var(--color-muted)]">{t("stores.subtitle")}</p>
        <Button onClick={() => setAdding(true)}>
          <Plus size={15} /> {t("stores.add")}
        </Button>
      </div>

      <DataTable
        columns={[t("stores.name"), t("stores.type"), t("workspaces.title"), t("stores.status"), ""]}
        rows={items.map((s) => [
          <span key="n" className="flex items-center gap-2 font-mono">
            <Database size={14} className="text-[var(--color-primary)]" />
            {s.name}
          </span>,
          <Badge key="t" tone="primary">{s.type}</Badge>,
          <span key="w" className="font-mono text-xs">{s.workspace}</span>,
          <Badge key="s" tone={s.enabled ? "ok" : "neutral"}>
            {s.enabled ? t("stores.enabled") : t("stores.disabled")}
          </Badge>,
          <div key="m" className="flex items-center justify-end gap-3">
            {s.kind === "datastore" && (
              <button
                onClick={() => setManage(s)}
                className="inline-flex items-center gap-1 text-sm text-[var(--color-primary)] hover:underline"
              >
                <TableProperties size={14} /> {t("stores.publish")}
              </button>
            )}
            <button
              onClick={() => remove(s)}
              title={t("stores.delete")}
              className="text-[var(--color-muted)] hover:text-[var(--color-err)]"
            >
              <Trash2 size={15} />
            </button>
          </div>,
        ])}
        empty={t("stores.empty")}
      />

      <NewStoreWizard
        open={adding}
        onClose={() => setAdding(false)}
        onCreated={() => {
          setAdding(false);
          load();
        }}
      />

      {manage && <PublishModal store={manage} onClose={() => setManage(null)} />}
    </div>
  );
}

function PublishModal({ store, onClose }: { store: Store; onClose: () => void }) {
  const { t } = useT();
  const [tables, setTables] = useState<StoreTable[] | null>(null);
  const [err, setErr] = useState("");
  const [busy, setBusy] = useState("");

  const load = () =>
    listStoreTables(store.workspace, store.name)
      .then(setTables)
      .catch((e) => setErr((e as Error).message));
  useEffect(() => {
    load();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  async function publish(table: string) {
    setBusy(table);
    try {
      await publishTable(store.workspace, store.name, table);
      await load();
    } finally {
      setBusy("");
    }
  }

  return (
    <Modal open onClose={onClose} title={`${t("stores.publishFrom")} ${store.name}`}>
      {err && <p className="text-sm text-[var(--color-err)]">{err}</p>}
      {tables === null && !err && <p className="text-sm text-[var(--color-muted)]">{t("common.loading")}</p>}
      {tables && tables.length === 0 && (
        <p className="text-sm text-[var(--color-muted)]">{t("stores.noTables")}</p>
      )}
      <div className="max-h-[50vh] space-y-1 overflow-auto">
        {tables?.map((tbl) => (
          <div
            key={tbl.name}
            className="flex items-center justify-between rounded-md border border-[var(--color-border)] px-3 py-2"
          >
            <div className="min-w-0">
              <div className="font-mono text-sm">{tbl.name}</div>
              <div className="text-xs text-[var(--color-muted)]">
                {tbl.geomType} · {tbl.srs}
              </div>
            </div>
            {tbl.published ? (
              <Badge tone="ok"><Check size={12} /> {t("stores.published")}</Badge>
            ) : (
              <Button
                variant="ghost"
                onClick={() => publish(tbl.name)}
                disabled={busy === tbl.name}
              >
                {busy === tbl.name ? t("common.loading") : t("stores.publishOne")}
              </Button>
            )}
          </div>
        ))}
      </div>
    </Modal>
  );
}
