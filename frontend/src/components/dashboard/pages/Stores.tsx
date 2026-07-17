"use client";

import { useEffect, useState } from "react";
import { Plus, Database, TableProperties, Check } from "lucide-react";
import { useT } from "@/i18n/provider";
import { listStores, createPgStore, listStoreTables, publishTable } from "@/api/dashboard/stores/api";
import type { Store, StoreTable, PgConnection } from "@/api/dashboard/stores/types";
import { listWorkspaces } from "@/api/dashboard/workspaces/api";
import type { Workspace } from "@/api/dashboard/workspaces/types";
import { DataTable } from "@/components/ui/DataTable";
import { Button } from "@/components/ui/Button";
import { Badge } from "@/components/ui/Badge";
import { Modal } from "@/components/ui/Modal";
import { Input, Select } from "@/components/ui/Field";

const emptyConn: PgConnection = {
  host: "self",
  port: "5432",
  database: "giti",
  user: "giti",
  passwd: "",
  schema: "public",
};

export function Stores() {
  const { t } = useT();
  const [items, setItems] = useState<Store[]>([]);
  const [workspaces, setWorkspaces] = useState<Workspace[]>([]);
  const [adding, setAdding] = useState(false);
  const [manage, setManage] = useState<Store | null>(null);

  const load = () => listStores().then(setItems).catch(() => setItems([]));
  useEffect(() => {
    load();
    listWorkspaces().then(setWorkspaces).catch(() => setWorkspaces([]));
  }, []);

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
          s.kind === "datastore" ? (
            <button
              key="m"
              onClick={() => setManage(s)}
              className="inline-flex items-center gap-1 text-sm text-[var(--color-primary)] hover:underline"
            >
              <TableProperties size={14} /> {t("stores.publish")}
            </button>
          ) : null,
        ])}
        empty={t("stores.empty")}
      />

      <AddStoreModal
        open={adding}
        onClose={() => setAdding(false)}
        workspaces={workspaces}
        onCreated={() => {
          setAdding(false);
          load();
        }}
      />

      {manage && (
        <PublishModal store={manage} onClose={() => setManage(null)} />
      )}
    </div>
  );
}

function AddStoreModal({
  open,
  onClose,
  workspaces,
  onCreated,
}: {
  open: boolean;
  onClose: () => void;
  workspaces: Workspace[];
  onCreated: () => void;
}) {
  const { t } = useT();
  const [ws, setWs] = useState("");
  const [name, setName] = useState("");
  const [conn, setConn] = useState<PgConnection>(emptyConn);
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState("");
  const self = conn.host === "self";

  useEffect(() => {
    if (open && workspaces.length && !ws) setWs(workspaces[0].name);
  }, [open, workspaces, ws]);

  function set<K extends keyof PgConnection>(k: K, v: string) {
    setConn((c) => ({ ...c, [k]: v }));
  }

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    if (!ws || !name.trim()) return;
    setBusy(true);
    setErr("");
    try {
      await createPgStore(ws, name.trim(), conn);
      setName("");
      setConn(emptyConn);
      onCreated();
    } catch (e) {
      setErr((e as Error).message || t("common.error"));
    } finally {
      setBusy(false);
    }
  }

  return (
    <Modal open={open} onClose={onClose} title={t("stores.addTitle")}>
      <form onSubmit={submit} className="space-y-3">
        <div className="grid grid-cols-2 gap-3">
          <Select label={t("workspaces.title")} value={ws} onChange={(e) => setWs(e.target.value)}>
            {workspaces.length === 0 && <option value="">—</option>}
            {workspaces.map((w) => (
              <option key={w.name} value={w.name}>{w.name}</option>
            ))}
          </Select>
          <Input
            label={t("stores.name")}
            value={name}
            onChange={(e) => setName(e.target.value)}
            placeholder="my_postgis"
          />
        </div>

        <div className="rounded-lg border border-[var(--color-border)] p-3">
          <div className="mb-2 text-xs font-medium uppercase tracking-wide text-[var(--color-muted)]">
            PostGIS
          </div>
          <div className="grid grid-cols-2 gap-3">
            <Input
              label={t("stores.host")}
              value={conn.host}
              onChange={(e) => set("host", e.target.value)}
              placeholder="self"
            />
            {!self && (
              <>
                <Input label={t("stores.port")} value={conn.port} onChange={(e) => set("port", e.target.value)} />
                <Input label={t("stores.database")} value={conn.database} onChange={(e) => set("database", e.target.value)} />
                <Input label={t("stores.user")} value={conn.user} onChange={(e) => set("user", e.target.value)} />
                <Input label={t("stores.password")} type="password" value={conn.passwd} onChange={(e) => set("passwd", e.target.value)} />
              </>
            )}
            <Input label={t("stores.schema")} value={conn.schema} onChange={(e) => set("schema", e.target.value)} />
          </div>
          {self && (
            <p className="mt-2 text-xs text-[var(--color-muted)]">{t("stores.selfHint")}</p>
          )}
        </div>

        {err && <p className="text-sm text-[var(--color-err)]">{err}</p>}

        <div className="flex justify-end gap-2 pt-1">
          <Button type="button" variant="ghost" onClick={onClose}>
            {t("action.cancel")}
          </Button>
          <Button type="submit" disabled={busy}>
            {busy ? t("common.loading") : t("action.create")}
          </Button>
        </div>
      </form>
    </Modal>
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
