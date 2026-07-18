"use client";

import { useState } from "react";
import { X, Plus, Trash2 } from "lucide-react";
import { useT } from "@/i18n/provider";
import { Button } from "@/components/ui/Button";
import { Input, Select } from "@/components/ui/Field";
import { useToast } from "@/components/ui/Toast";
import { createStyle, updateStyle, validateStyle } from "@/api/dashboard/styles/api";
import {
  generateSld,
  newRule,
  type GeomKind,
  type StyleModel,
  type StyleRule,
  type FilterOp,
} from "@/lib/sld";

function inferGeom(geomType?: string): GeomKind {
  const g = (geomType ?? "").toUpperCase();
  if (g.includes("POINT")) return "point";
  if (g.includes("LINE")) return "line";
  return "polygon";
}

const OPS: FilterOp[] = [">", ">=", "<", "<=", "=", "!=", "like"];

export function StyleBuilder({ layer, geomType, columns, edit, onClose, onSaved }: {
  layer: string; // ws:name
  geomType?: string;
  columns: string[];
  edit?: { name: string; model?: StyleModel }; // present = editing an existing style
  onClose: () => void;
  onSaved: (styleName: string) => void;
}) {
  const { t } = useT();
  const { toast } = useToast();
  const isEdit = !!edit;
  const [name, setName] = useState(edit?.name ?? `${layer.split(":").pop()}_style`);
  const [geom, setGeom] = useState<GeomKind>(edit?.model?.geom ?? inferGeom(geomType));
  const [rules, setRules] = useState<StyleRule[]>(
    edit?.model?.rules?.length ? edit.model.rules : [{ ...newRule(inferGeom(geomType)), name: "default" }],
  );
  const [busy, setBusy] = useState(false);

  const model: StyleModel = { geom, rules };
  const set = (i: number, patch: Partial<StyleRule>) =>
    setRules((rs) => rs.map((r, j) => (j === i ? { ...r, ...patch } : r)));

  async function save() {
    if (!name.trim()) {
      toast({ title: t("styleEdit.nameRequired"), tone: "err" });
      return;
    }
    setBusy(true);
    try {
      const sld = generateSld(name.trim(), model);
      const v = await validateStyle("sld", sld);
      if (!v.ok) {
        toast({ title: t("styleEdit.invalid"), tone: "err" });
        return;
      }
      if (isEdit) await updateStyle(name.trim(), "sld", sld, model);
      else await createStyle(name.trim(), "sld", sld, model);
      toast({ title: t("styleEdit.saved") });
      onSaved(name.trim());
    } catch (e) {
      toast({ title: (e as Error).message, tone: "err" });
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="fixed inset-0 z-[60] flex items-center justify-center bg-black/50 p-4" onClick={onClose}>
      <div
        className="flex h-[85vh] w-full max-w-3xl flex-col overflow-hidden rounded-xl border border-[var(--color-border)] bg-[var(--color-surface)] shadow-2xl"
        onClick={(e) => e.stopPropagation()}
      >
        <div className="flex items-center justify-between border-b border-[var(--color-border)] px-5 py-3">
          <h2 className="font-display text-base font-semibold">{t("builder.title")}</h2>
          <button onClick={onClose} className="rounded-md p-1 text-[var(--color-muted)] hover:bg-[var(--color-surface-2)]"><X size={18} /></button>
        </div>

        <div className="flex-1 space-y-4 overflow-auto px-5 py-4">
          <div className="grid grid-cols-2 gap-3">
            <Input label={t("styles.name")} value={name} onChange={(e) => setName(e.target.value)} disabled={isEdit} />
            <Select label={t("builder.geom")} value={geom} onChange={(e) => setGeom(e.target.value as GeomKind)}>
              <option value="polygon">{t("builder.polygon")}</option>
              <option value="line">{t("builder.line")}</option>
              <option value="point">{t("builder.point")}</option>
            </Select>
          </div>
          <p className="text-xs text-[var(--color-muted)]">{t("builder.hint")}</p>

          {rules.map((r, i) => (
            <div key={i} className="space-y-3 rounded-lg border border-[var(--color-border)] p-3">
              <div className="flex items-center justify-between">
                <Input className="!w-40" value={r.name} onChange={(e) => set(i, { name: e.target.value })} />
                {rules.length > 1 && (
                  <button onClick={() => setRules((rs) => rs.filter((_, j) => j !== i))} className="text-[var(--color-muted)] hover:text-[var(--color-err)]"><Trash2 size={15} /></button>
                )}
              </div>

              {/* condition: when column op value */}
              <div className="grid grid-cols-[auto_1fr_80px_1fr] items-center gap-2 text-sm">
                <span className="text-xs text-[var(--color-muted)]">{t("builder.when")}</span>
                <Select value={r.filter?.column ?? ""} onChange={(e) => set(i, { filter: e.target.value ? { column: e.target.value, op: r.filter?.op ?? ">", value: r.filter?.value ?? "" } : undefined })}>
                  <option value="">{t("builder.always")}</option>
                  {columns.map((c) => <option key={c} value={c}>{c}</option>)}
                </Select>
                <Select value={r.filter?.op ?? ">"} disabled={!r.filter} onChange={(e) => r.filter && set(i, { filter: { ...r.filter, op: e.target.value as FilterOp } })}>
                  {OPS.map((o) => <option key={o} value={o}>{o}</option>)}
                </Select>
                <Input value={r.filter?.value ?? ""} disabled={!r.filter} placeholder={t("builder.value")} onChange={(e) => r.filter && set(i, { filter: { ...r.filter, value: e.target.value } })} />
              </div>

              {/* symbolizer */}
              <div className="flex flex-wrap items-end gap-3">
                {geom !== "line" && (
                  <ColorField label={t("builder.fill")} value={r.fill} onChange={(v) => set(i, { fill: v })} />
                )}
                <label className="space-y-1 text-xs">
                  <span className="block text-[var(--color-muted)]">{t("builder.opacity")}</span>
                  <input type="range" min={0} max={1} step={0.1} value={r.fillOpacity} onChange={(e) => set(i, { fillOpacity: +e.target.value })} />
                </label>
                <ColorField label={t("builder.stroke")} value={r.stroke} onChange={(v) => set(i, { stroke: v })} />
                <NumField label={t("builder.strokeW")} value={r.strokeWidth} onChange={(v) => set(i, { strokeWidth: v })} />
                {geom === "point" && (
                  <>
                    <label className="space-y-1 text-xs">
                      <span className="block text-[var(--color-muted)]">{t("builder.mark")}</span>
                      <select value={r.mark} onChange={(e) => set(i, { mark: e.target.value as StyleRule["mark"] })} className="rounded-md border border-[var(--color-border)] bg-[var(--color-surface)] px-2 py-1 text-sm">
                        <option value="circle">●</option><option value="square">■</option><option value="triangle">▲</option><option value="star">★</option>
                      </select>
                    </label>
                    <NumField label={t("builder.size")} value={r.size} onChange={(v) => set(i, { size: v })} />
                  </>
                )}
              </div>

              {/* label + zoom */}
              <div className="flex flex-wrap items-end gap-3">
                <label className="space-y-1 text-xs">
                  <span className="block text-[var(--color-muted)]">{t("builder.label")}</span>
                  <select value={r.labelColumn ?? ""} onChange={(e) => set(i, { labelColumn: e.target.value || undefined })} className="rounded-md border border-[var(--color-border)] bg-[var(--color-surface)] px-2 py-1 text-sm">
                    <option value="">{t("builder.noLabel")}</option>
                    {columns.map((c) => <option key={c} value={c}>{c}</option>)}
                  </select>
                </label>
                {r.labelColumn && <NumField label={t("builder.labelSize")} value={r.labelSize} onChange={(v) => set(i, { labelSize: v })} />}
                {r.labelColumn && <ColorField label={t("builder.labelColor")} value={r.labelColor} onChange={(v) => set(i, { labelColor: v })} />}
                <NumField label={t("builder.minZoom")} value={r.minZoom ?? 0} onChange={(v) => set(i, { minZoom: v || undefined })} />
                <NumField label={t("builder.maxZoom")} value={r.maxZoom ?? 0} onChange={(v) => set(i, { maxZoom: v || undefined })} />
              </div>
            </div>
          ))}

          <Button variant="ghost" onClick={() => setRules((rs) => [...rs, { ...newRule(geom), name: `rule ${rs.length + 1}` }])}>
            <Plus size={15} /> {t("builder.addRule")}
          </Button>
        </div>

        <div className="flex justify-end gap-2 border-t border-[var(--color-border)] px-5 py-3">
          <Button variant="ghost" onClick={onClose}>{t("action.cancel")}</Button>
          <Button onClick={save} disabled={busy}>{busy ? t("common.loading") : t("builder.saveApply")}</Button>
        </div>
      </div>
    </div>
  );
}

function ColorField({ label, value, onChange }: { label: string; value: string; onChange: (v: string) => void }) {
  return (
    <label className="space-y-1 text-xs">
      <span className="block text-[var(--color-muted)]">{label}</span>
      <input type="color" value={value} onChange={(e) => onChange(e.target.value)} className="h-8 w-12 cursor-pointer rounded border border-[var(--color-border)] bg-transparent" />
    </label>
  );
}

function NumField({ label, value, onChange }: { label: string; value: number; onChange: (v: number) => void }) {
  return (
    <label className="space-y-1 text-xs">
      <span className="block text-[var(--color-muted)]">{label}</span>
      <input type="number" value={value} onChange={(e) => onChange(+e.target.value)} className="w-20 rounded-md border border-[var(--color-border)] bg-[var(--color-surface)] px-2 py-1 text-sm outline-none" />
    </label>
  );
}
