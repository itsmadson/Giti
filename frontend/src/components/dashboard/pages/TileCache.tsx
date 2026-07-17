"use client";

import { useEffect, useState } from "react";
import { Plus, Trash2, Save } from "lucide-react";
import { useT } from "@/i18n/provider";
import {
  listGridsets,
  saveGridset,
  deleteGridset,
  listBlobStores,
  saveBlobStore,
  deleteBlobStore,
  getQuota,
  setQuota,
  type Gridset,
  type BlobStore,
  type DiskQuota,
} from "@/api/dashboard/tilecache/api";
import { DataTable } from "@/components/ui/DataTable";
import { Button } from "@/components/ui/Button";
import { Badge } from "@/components/ui/Badge";
import { Input, Select } from "@/components/ui/Field";
import { useToast } from "@/components/ui/Toast";

type Tab = "gridsets" | "blobstores" | "quota";

export function TileCache() {
  const { t } = useT();
  const [tab, setTab] = useState<Tab>("gridsets");
  const tabs: [Tab, string][] = [
    ["gridsets", t("gwc.gridsets")],
    ["blobstores", t("gwc.blobstores")],
    ["quota", t("gwc.quota")],
  ];
  return (
    <div className="mx-auto max-w-4xl space-y-4">
      <div className="flex gap-1 border-b border-[var(--color-border)]">
        {tabs.map(([id, label]) => (
          <button
            key={id}
            onClick={() => setTab(id)}
            className={
              "px-3 py-2 text-sm " +
              (tab === id ? "border-b-2 border-[var(--color-primary)] text-[var(--color-text)]" : "text-[var(--color-muted)]")
            }
          >
            {label}
          </button>
        ))}
      </div>
      {tab === "gridsets" && <Gridsets />}
      {tab === "blobstores" && <BlobStores />}
      {tab === "quota" && <Quota />}
    </div>
  );
}

function Gridsets() {
  const { t } = useT();
  const { toast } = useToast();
  const [items, setItems] = useState<Gridset[]>([]);
  const [form, setForm] = useState<Gridset>({ name: "", srs: "EPSG:3857", tileSize: 256, levels: 22 });
  const load = () => listGridsets().then(setItems).catch(() => setItems([]));
  useEffect(() => {
    load();
  }, []);
  async function add() {
    if (!form.name.trim()) return;
    await saveGridset(form).catch((e) => toast({ title: (e as Error).message, tone: "err" }));
    toast({ title: t("gwc.saved") });
    setForm({ name: "", srs: "EPSG:3857", tileSize: 256, levels: 22 });
    load();
  }
  return (
    <div className="space-y-3">
      <div className="grid grid-cols-[1fr_1fr_90px_90px_auto] items-end gap-2">
        <Input label={t("gwc.name")} value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} />
        <Input label="SRS" value={form.srs} onChange={(e) => setForm({ ...form, srs: e.target.value })} />
        <Input label={t("gwc.tileSize")} type="number" value={String(form.tileSize)} onChange={(e) => setForm({ ...form, tileSize: +e.target.value })} />
        <Input label={t("gwc.levels")} type="number" value={String(form.levels)} onChange={(e) => setForm({ ...form, levels: +e.target.value })} />
        <Button onClick={add}><Plus size={15} /></Button>
      </div>
      <DataTable
        columns={[t("gwc.name"), "SRS", t("gwc.tileSize"), t("gwc.levels"), ""]}
        rows={items.map((g) => [
          <span key="n" className="font-mono">{g.name}</span>,
          g.srs,
          String(g.tileSize),
          String(g.levels),
          <button key="d" onClick={() => deleteGridset(g.name).then(load)} className="flex justify-end text-[var(--color-muted)] hover:text-[var(--color-err)]"><Trash2 size={15} /></button>,
        ])}
        empty="—"
      />
    </div>
  );
}

function BlobStores() {
  const { t } = useT();
  const { toast } = useToast();
  const [items, setItems] = useState<BlobStore[]>([]);
  const [form, setForm] = useState<BlobStore>({ name: "", type: "file", isDefault: false });
  const load = () => listBlobStores().then(setItems).catch(() => setItems([]));
  useEffect(() => {
    load();
  }, []);
  async function add() {
    if (!form.name.trim()) return;
    await saveBlobStore(form).catch((e) => toast({ title: (e as Error).message, tone: "err" }));
    toast({ title: t("gwc.saved") });
    setForm({ name: "", type: "file", isDefault: false });
    load();
  }
  return (
    <div className="space-y-3">
      <div className="grid grid-cols-[1fr_1fr_auto] items-end gap-2">
        <Input label={t("gwc.name")} value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} />
        <Select label={t("gwc.type")} value={form.type} onChange={(e) => setForm({ ...form, type: e.target.value })}>
          <option value="file">File (volume)</option>
          <option value="s3">S3 / MinIO</option>
        </Select>
        <Button onClick={add}><Plus size={15} /></Button>
      </div>
      <DataTable
        columns={[t("gwc.name"), t("gwc.type"), t("gwc.default"), ""]}
        rows={items.map((b) => [
          <span key="n" className="font-mono">{b.name}</span>,
          <Badge key="t" tone="neutral">{b.type}</Badge>,
          b.isDefault ? <Badge key="d" tone="ok">default</Badge> : "",
          <button key="x" onClick={() => deleteBlobStore(b.name).then(load)} className="flex justify-end text-[var(--color-muted)] hover:text-[var(--color-err)]"><Trash2 size={15} /></button>,
        ])}
        empty="—"
      />
    </div>
  );
}

function Quota() {
  const { t } = useT();
  const { toast } = useToast();
  const [q, setQ] = useState<DiskQuota>({ policy: "LRU", maxBytes: 0 });
  useEffect(() => {
    getQuota().then(setQ).catch(() => {});
  }, []);
  const mb = Math.round(q.maxBytes / (1024 * 1024));
  return (
    <div className="max-w-md space-y-3">
      <Select label={t("gwc.policy")} value={q.policy} onChange={(e) => setQ({ ...q, policy: e.target.value })}>
        <option value="LRU">LRU (evict least-recently-used)</option>
        <option value="LFU">LFU (evict least-frequently-used)</option>
      </Select>
      <Input
        label={t("gwc.maxMb")}
        type="number"
        value={String(mb)}
        onChange={(e) => setQ({ ...q, maxBytes: +e.target.value * 1024 * 1024 })}
      />
      <p className="text-xs text-[var(--color-muted)]">{t("gwc.quotaHint")}</p>
      <Button onClick={() => setQuota(q).then(() => toast({ title: t("gwc.saved") }))}>
        <Save size={15} /> {t("action.save")}
      </Button>
    </div>
  );
}
