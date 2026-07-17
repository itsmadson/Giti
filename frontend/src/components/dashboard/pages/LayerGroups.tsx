"use client";

import { useEffect, useState } from "react";
import { Plus, Trash2, Layers as LayersIcon, ArrowUp, ArrowDown, X } from "lucide-react";
import { useT } from "@/i18n/provider";
import {
  listGroups,
  saveGroup,
  deleteGroup,
  computeGroupBounds,
  type LayerGroup,
  type GroupMember,
} from "@/api/dashboard/groups/api";
import { listLayers } from "@/api/dashboard/layers/api";
import type { Layer } from "@/api/dashboard/layers/types";
import { listWorkspaces } from "@/api/dashboard/workspaces/api";
import type { Workspace } from "@/api/dashboard/workspaces/types";
import { DataTable } from "@/components/ui/DataTable";
import { Badge } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { Drawer } from "@/components/ui/Drawer";
import { Input, Select } from "@/components/ui/Field";
import { useToast } from "@/components/ui/Toast";

const blank = (ws: string): LayerGroup => ({
  workspace: ws,
  name: "",
  title: "",
  abstract: "",
  mode: "SINGLE",
  srs: "EPSG:4326",
  members: [],
});

export function LayerGroups() {
  const { t } = useT();
  const { toast } = useToast();
  const [items, setItems] = useState<LayerGroup[]>([]);
  const [layers, setLayers] = useState<Layer[]>([]);
  const [workspaces, setWorkspaces] = useState<Workspace[]>([]);
  const [edit, setEdit] = useState<LayerGroup | null>(null);

  const load = () => listGroups().then(setItems).catch(() => setItems([]));
  useEffect(() => {
    load();
    listLayers().then(setLayers).catch(() => setLayers([]));
    listWorkspaces().then(setWorkspaces).catch(() => setWorkspaces([]));
  }, []);

  async function remove(g: LayerGroup) {
    if (!confirm(t("groups.deleteConfirm", { name: g.name }))) return;
    await deleteGroup(g.workspace, g.name).catch(() => {});
    toast({ title: t("groups.deleted") });
    load();
  }

  return (
    <div className="mx-auto max-w-4xl space-y-4">
      <div className="flex items-center justify-between">
        <p className="text-sm text-[var(--color-muted)]">{t("groups.subtitle")}</p>
        <Button onClick={() => setEdit(blank(workspaces[0]?.name ?? ""))}>
          <Plus size={15} /> {t("groups.add")}
        </Button>
      </div>

      <DataTable
        columns={[t("groups.name"), t("groups.mode"), t("groups.members"), ""]}
        rows={items.map((g) => [
          <button
            key="n"
            onClick={() => setEdit(g)}
            className="flex items-center gap-2 font-mono text-[var(--color-primary)] hover:underline"
          >
            <LayersIcon size={13} /> {g.workspace}:{g.name}
          </button>,
          <Badge key="m" tone="neutral">{g.mode}</Badge>,
          <span key="c" className="text-[var(--color-muted)]">{g.members.length}</span>,
          <button key="d" onClick={() => remove(g)} className="flex justify-end text-[var(--color-muted)] hover:text-[var(--color-err)]">
            <Trash2 size={15} />
          </button>,
        ])}
        empty={t("groups.empty")}
      />

      {edit && (
        <GroupEditor
          initial={edit}
          layers={layers}
          workspaces={workspaces}
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

function GroupEditor({ initial, layers, workspaces, onClose, onSaved }: {
  initial: LayerGroup;
  layers: Layer[];
  workspaces: Workspace[];
  onClose: () => void;
  onSaved: () => void;
}) {
  const { t } = useT();
  const { toast } = useToast();
  const [g, setG] = useState<LayerGroup>(initial);
  const isNew = !initial.name;
  const [pick, setPick] = useState("");
  const [busy, setBusy] = useState(false);

  const set = (p: Partial<LayerGroup>) => setG((x) => ({ ...x, ...p }));
  const setMembers = (members: GroupMember[]) => set({ members });

  function addMember() {
    if (!pick) return;
    setMembers([...g.members, { layer: pick, style: "" }]);
    setPick("");
  }
  function move(i: number, dir: -1 | 1) {
    const m = [...g.members];
    const j = i + dir;
    if (j < 0 || j >= m.length) return;
    [m[i], m[j]] = [m[j], m[i]];
    setMembers(m);
  }

  async function bounds() {
    if (isNew) return;
    const r = await computeGroupBounds(g.workspace, g.name);
    if (r.bounds) {
      set({ bounds: r.bounds });
      toast({ title: t("groups.boundsDone") });
    }
  }
  async function save() {
    if (!g.workspace || !g.name.trim()) {
      toast({ title: t("groups.nameRequired"), tone: "err" });
      return;
    }
    setBusy(true);
    try {
      await saveGroup({ ...g, name: g.name.trim() });
      toast({ title: t("groups.saved") });
      onSaved();
    } catch (e) {
      toast({ title: (e as Error).message, tone: "err" });
    } finally {
      setBusy(false);
    }
  }

  return (
    <Drawer
      open
      onClose={onClose}
      title={isNew ? t("groups.add") : `${g.workspace}:${g.name}`}
      footer={
        <div className="flex justify-between">
          <Button variant="ghost" onClick={bounds} disabled={isNew}>{t("groups.computeBounds")}</Button>
          <div className="flex gap-2">
            <Button variant="ghost" onClick={onClose}>{t("action.cancel")}</Button>
            <Button onClick={save} disabled={busy}>{busy ? t("common.loading") : t("action.save")}</Button>
          </div>
        </div>
      }
    >
      <div className="space-y-3">
        <div className="grid grid-cols-2 gap-3">
          <Select label={t("workspaces.title")} value={g.workspace} onChange={(e) => set({ workspace: e.target.value })} disabled={!isNew}>
            {workspaces.map((w) => (
              <option key={w.name} value={w.name}>{w.name}</option>
            ))}
          </Select>
          <Input label={t("groups.name")} value={g.name} onChange={(e) => set({ name: e.target.value })} disabled={!isNew} />
        </div>
        <div className="grid grid-cols-2 gap-3">
          <Input label={t("layerEdit.title")} value={g.title} onChange={(e) => set({ title: e.target.value })} />
          <Select label={t("groups.mode")} value={g.mode} onChange={(e) => set({ mode: e.target.value })}>
            <option value="SINGLE">Single</option>
            <option value="OPAQUE">Opaque container</option>
            <option value="NAMED">Named tree</option>
            <option value="EO">Earth Observation</option>
          </Select>
        </div>

        <div className="rounded-lg border border-[var(--color-border)] p-3">
          <div className="mb-2 text-xs font-medium uppercase tracking-wide text-[var(--color-muted)]">{t("groups.members")}</div>
          <div className="space-y-1">
            {g.members.map((m, i) => (
              <div key={i} className="flex items-center gap-2 rounded-md border border-[var(--color-border)] px-2 py-1.5 text-sm">
                <span className="flex-1 font-mono text-xs">{m.layer}</span>
                <Input
                  className="!w-28"
                  placeholder={t("layers.style")}
                  value={m.style}
                  onChange={(e) => {
                    const mm = [...g.members];
                    mm[i] = { ...mm[i], style: e.target.value };
                    setMembers(mm);
                  }}
                />
                <button onClick={() => move(i, -1)} className="text-[var(--color-muted)] hover:text-[var(--color-text)]"><ArrowUp size={14} /></button>
                <button onClick={() => move(i, 1)} className="text-[var(--color-muted)] hover:text-[var(--color-text)]"><ArrowDown size={14} /></button>
                <button onClick={() => setMembers(g.members.filter((_, j) => j !== i))} className="text-[var(--color-muted)] hover:text-[var(--color-err)]"><X size={14} /></button>
              </div>
            ))}
          </div>
          <div className="mt-2 flex gap-2">
            <Select value={pick} onChange={(e) => setPick(e.target.value)}>
              <option value="">—</option>
              {layers.map((l) => {
                const id = `${l.workspace}:${l.name}`;
                return <option key={id} value={id}>{id}</option>;
              })}
            </Select>
            <Button variant="ghost" onClick={addMember}>+</Button>
          </div>
        </div>

        {g.bounds && (
          <p className="font-mono text-xs text-[var(--color-muted)]">
            {t("layerDetail.bbox")}: {g.bounds.map((n) => n.toFixed(3)).join(", ")}
          </p>
        )}
      </div>
    </Drawer>
  );
}
