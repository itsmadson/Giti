"use client";
import { useEffect, useState } from "react";
import { ArrowLeft, Database, Layers, Cloud, Zap } from "lucide-react";
import { useT } from "@/i18n/provider";
import { Drawer } from "@/components/ui/Drawer";
import { Input, Select } from "@/components/ui/Field";
import { Button } from "@/components/ui/Button";
import { useToast } from "@/components/ui/Toast";
import { listStoreTypes, createStore, testStore } from "@/api/dashboard/stores/api";
import type { StoreType } from "@/api/dashboard/stores/types";
import { listWorkspaces } from "@/api/dashboard/workspaces/api";
import type { Workspace } from "@/api/dashboard/workspaces/types";

const catIcon: Record<string, typeof Database> = { Vector: Layers, Raster: Database, Cascade: Cloud };

export function NewStoreWizard({ open, onClose, onCreated }: {
  open: boolean;
  onClose: () => void;
  onCreated: () => void;
}) {
  const { t } = useT();
  const { toast } = useToast();
  const [types, setTypes] = useState<StoreType[]>([]);
  const [workspaces, setWorkspaces] = useState<Workspace[]>([]);
  const [sel, setSel] = useState<StoreType | null>(null);
  const [ws, setWs] = useState("");
  const [name, setName] = useState("");
  const [conn, setConn] = useState<Record<string, string>>({});
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    if (!open) return;
    listStoreTypes().then(setTypes).catch(() => setTypes([]));
    listWorkspaces()
      .then((w) => {
        setWorkspaces(w);
        if (w[0]) setWs(w[0].name);
      })
      .catch(() => setWorkspaces([]));
    setSel(null);
    setName("");
  }, [open]);

  function pick(st: StoreType) {
    setSel(st);
    const c: Record<string, string> = {};
    st.params.forEach((p) => (c[p.key] = p.default ?? ""));
    setConn(c);
  }
  async function test() {
    if (!sel) return;
    const r = await testStore({ type: sel.type, connection: conn });
    toast(r.ok ? { title: t("stores.testOk") } : { title: r.error || t("stores.testFail"), tone: "err" });
  }
  async function create() {
    if (!sel || !ws || !name.trim()) return;
    setBusy(true);
    try {
      await createStore({ workspace: ws, name: name.trim(), type: sel.type, kind: sel.kind, enabled: true, connection: conn });
      toast({ title: t("stores.created") });
      onCreated();
    } catch (e) {
      toast({ title: (e as Error).message, tone: "err" });
    } finally {
      setBusy(false);
    }
  }

  const cats = ["Vector", "Raster", "Cascade"];
  return (
    <Drawer
      open={open}
      onClose={onClose}
      title={sel ? sel.label : t("stores.newSource")}
      footer={
        sel ? (
          <div className="flex justify-between">
            <Button variant="ghost" onClick={() => setSel(null)}>
              <ArrowLeft size={15} /> {t("action.back")}
            </Button>
            <div className="flex gap-2">
              <Button variant="ghost" onClick={test}>
                <Zap size={15} /> {t("stores.test")}
              </Button>
              <Button onClick={create} disabled={busy}>
                {busy ? t("common.loading") : t("action.create")}
              </Button>
            </div>
          </div>
        ) : undefined
      }
    >
      {!sel ? (
        <div className="space-y-5">
          {cats.map((cat) => {
            const list = types.filter((x) => x.category === cat);
            if (!list.length) return null;
            const Icon = catIcon[cat] ?? Database;
            return (
              <div key={cat}>
                <div className="mb-2 text-xs font-medium uppercase tracking-wide text-[var(--color-muted)]">{cat}</div>
                <div className="grid grid-cols-2 gap-2">
                  {list.map((st) => (
                    <button
                      key={st.type}
                      onClick={() => pick(st)}
                      className="flex items-center gap-2 rounded-lg border border-[var(--color-border)] px-3 py-2.5 text-start text-sm hover:border-[var(--color-primary)]"
                    >
                      <Icon size={16} className="text-[var(--color-primary)]" /> {st.label}
                    </button>
                  ))}
                </div>
              </div>
            );
          })}
        </div>
      ) : (
        <div className="space-y-3">
          <div className="grid grid-cols-2 gap-3">
            <Select label={t("workspaces.title")} value={ws} onChange={(e) => setWs(e.target.value)}>
              {workspaces.map((w) => (
                <option key={w.name} value={w.name}>
                  {w.name}
                </option>
              ))}
            </Select>
            <Input label={t("stores.name")} value={name} onChange={(e) => setName(e.target.value)} placeholder="my_store" />
          </div>
          <div className="rounded-lg border border-[var(--color-border)] p-3">
            <div className="mb-2 text-xs font-medium uppercase tracking-wide text-[var(--color-muted)]">{t("stores.connection")}</div>
            <div className="grid grid-cols-2 gap-3">
              {sel.params.map((p) => (
                <Input
                  key={p.key}
                  label={p.label + (p.required ? " *" : "")}
                  type={p.type === "number" ? "number" : p.type === "password" ? "password" : "text"}
                  value={conn[p.key] ?? ""}
                  onChange={(e) => setConn((c) => ({ ...c, [p.key]: e.target.value }))}
                />
              ))}
            </div>
          </div>
        </div>
      )}
    </Drawer>
  );
}
