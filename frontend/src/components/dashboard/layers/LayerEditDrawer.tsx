"use client";
import { useEffect, useState } from "react";
import { useT } from "@/i18n/provider";
import { Drawer } from "@/components/ui/Drawer";
import { Input, Select } from "@/components/ui/Field";
import { Button } from "@/components/ui/Button";
import { Badge } from "@/components/ui/Badge";
import { useToast } from "@/components/ui/Toast";
import { apiFetch } from "@/api/client";
import { getLayerDetail, patchLayer, patchFeatureType, computeBbox } from "@/api/dashboard/layers/api";
import type { LayerDetail } from "@/api/dashboard/layers/types";
import { StyleBuilder } from "@/components/dashboard/styles/StyleBuilder";

type Tab = "data" | "publishing" | "crs" | "feature";

export function LayerEditDrawer({ ws, name, open, onClose, onSaved }: {
  ws: string;
  name: string;
  open: boolean;
  onClose: () => void;
  onSaved: () => void;
}) {
  const { t } = useT();
  const { toast } = useToast();
  const [tab, setTab] = useState<Tab>("data");
  const [d, setD] = useState<LayerDetail | null>(null);
  const [styles, setStyles] = useState<string[]>([]);
  const [kw, setKw] = useState("");
  const [busy, setBusy] = useState(false);
  const [showBuilder, setShowBuilder] = useState(false);

  const loadStyles = () =>
    apiFetch<{ name: string }[]>("/api/v1/styles").then((s) => setStyles(s.map((x) => x.name))).catch(() => setStyles([]));
  useEffect(() => {
    if (!open) return;
    setTab("data");
    getLayerDetail(ws, name).then(setD).catch(() => setD(null));
    loadStyles();
  }, [open, ws, name]);

  if (!open || !d) return null;
  const set = (patch: Partial<LayerDetail>) => setD({ ...d, ...patch });

  async function save() {
    if (!d) return;
    setBusy(true);
    try {
      await patchFeatureType(ws, name, {
        title: d.title,
        abstract: d.abstract,
        keywords: d.keywords,
        declaredSrs: d.declaredSrs,
        srsHandling: d.srsHandling,
      });
      await patchLayer(ws, name, {
        defaultStyle: d.defaultStyle,
        alternateStyles: d.alternateStyles,
        queryable: d.queryable,
        opaque: d.opaque,
        advertised: d.advertised,
      });
      toast({ title: t("layerEdit.saved") });
      onSaved();
    } catch (e) {
      toast({ title: (e as Error).message, tone: "err" });
    } finally {
      setBusy(false);
    }
  }

  async function recompute() {
    const r = await computeBbox(ws, name);
    set({ bbox: r.bbox, featureCount: r.featureCount });
    toast({ title: t("layerEdit.bboxDone") });
  }

  const tabs: [Tab, string][] = [
    ["data", t("layerEdit.tabData")],
    ["publishing", t("layerEdit.tabPublishing")],
    ["crs", t("layerEdit.tabCrs")],
    ["feature", t("layerEdit.tabFeature")],
  ];

  return (
    <Drawer
      open={open}
      onClose={onClose}
      title={`${d.workspace}:${d.name}`}
      footer={
        <div className="flex justify-end gap-2">
          <Button variant="ghost" onClick={onClose}>{t("action.cancel")}</Button>
          <Button onClick={save} disabled={busy}>{busy ? t("common.loading") : t("action.save")}</Button>
        </div>
      }
    >
      <div className="mb-4 flex gap-1 border-b border-[var(--color-border)]">
        {tabs.map(([id, label]) => (
          <button
            key={id}
            onClick={() => setTab(id)}
            className={
              "px-3 py-2 text-sm " +
              (tab === id
                ? "border-b-2 border-[var(--color-primary)] text-[var(--color-text)]"
                : "text-[var(--color-muted)]")
            }
          >
            {label}
          </button>
        ))}
      </div>

      {tab === "data" && (
        <div className="space-y-3">
          <Input label={t("layerEdit.title")} value={d.title} onChange={(e) => set({ title: e.target.value })} />
          <label className="block space-y-1">
            <span className="text-xs font-medium text-[var(--color-muted)]">{t("layerEdit.abstract")}</span>
            <textarea
              value={d.abstract}
              onChange={(e) => set({ abstract: e.target.value })}
              className="min-h-24 w-full rounded-md border border-[var(--color-border)] bg-[var(--color-surface)] px-3 py-2 text-sm outline-none focus:border-[var(--color-primary)]"
            />
          </label>
          <div>
            <span className="text-xs font-medium text-[var(--color-muted)]">{t("layerEdit.keywords")}</span>
            <div className="mt-1 flex flex-wrap gap-1.5">
              {d.keywords?.map((k) => (
                <Badge key={k} tone="primary">
                  {k}
                  <button className="ms-1" onClick={() => set({ keywords: d.keywords.filter((x) => x !== k) })}>×</button>
                </Badge>
              ))}
            </div>
            <div className="mt-2 flex gap-2">
              <Input value={kw} onChange={(e) => setKw(e.target.value)} placeholder={t("layerEdit.addKeyword")} />
              <Button
                variant="ghost"
                onClick={() => {
                  if (kw.trim()) set({ keywords: [...(d.keywords ?? []), kw.trim()] });
                  setKw("");
                }}
              >
                +
              </Button>
            </div>
          </div>
        </div>
      )}

      {tab === "publishing" && (
        <div className="space-y-3">
          <div className="flex items-end gap-2">
            <div className="flex-1">
              <Select label={t("layers.style")} value={d.defaultStyle} onChange={(e) => set({ defaultStyle: e.target.value })}>
                {!styles.includes(d.defaultStyle) && d.defaultStyle && <option value={d.defaultStyle}>{d.defaultStyle}</option>}
                {styles.map((s) => (
                  <option key={s} value={s}>{s}</option>
                ))}
              </Select>
            </div>
            <Button variant="ghost" onClick={() => setShowBuilder(true)}>{t("builder.new")}</Button>
          </div>

          <div>
            <span className="text-xs font-medium text-[var(--color-muted)]">{t("layerEdit.altStyles")}</span>
            <div className="mt-1 max-h-32 space-y-1 overflow-auto rounded-md border border-[var(--color-border)] p-2">
              {styles.filter((s) => s !== d.defaultStyle).map((s) => (
                <label key={s} className="flex items-center gap-2 text-sm">
                  <input
                    type="checkbox"
                    checked={d.alternateStyles?.includes(s) ?? false}
                    onChange={(e) =>
                      set({
                        alternateStyles: e.target.checked
                          ? [...(d.alternateStyles ?? []), s]
                          : (d.alternateStyles ?? []).filter((x) => x !== s),
                      })
                    }
                    className="accent-[var(--color-primary)]"
                  />
                  <span className="font-mono text-xs">{s}</span>
                </label>
              ))}
              {styles.length === 0 && <span className="text-xs text-[var(--color-muted)]">—</span>}
            </div>
            <p className="mt-1 text-xs text-[var(--color-muted)]">{t("layerEdit.altHint")}</p>
          </div>

          <label className="flex items-center gap-2 text-sm">
            <input type="checkbox" checked={d.queryable} onChange={(e) => set({ queryable: e.target.checked })} className="accent-[var(--color-primary)]" />
            {t("layerEdit.queryable")}
          </label>
          <label className="flex items-center gap-2 text-sm">
            <input type="checkbox" checked={d.opaque} onChange={(e) => set({ opaque: e.target.checked })} className="accent-[var(--color-primary)]" />
            {t("layerEdit.opaque")}
          </label>
          <label className="flex items-center gap-2 text-sm">
            <input type="checkbox" checked={d.advertised} onChange={(e) => set({ advertised: e.target.checked })} className="accent-[var(--color-primary)]" />
            {t("layerEdit.advertised")}
          </label>
        </div>
      )}

      {tab === "crs" && (
        <div className="space-y-3">
          <Input label={t("layerEdit.nativeSrs")} value={d.srs} disabled />
          <div className="grid grid-cols-2 gap-3">
            <Input label={t("layerEdit.declaredSrs")} value={d.declaredSrs} onChange={(e) => set({ declaredSrs: e.target.value })} placeholder="EPSG:4326" />
            <Select label={t("layerEdit.srsHandling")} value={d.srsHandling} onChange={(e) => set({ srsHandling: e.target.value })}>
              <option value="FORCE">Force declared</option>
              <option value="REPROJECT">Reproject native to declared</option>
              <option value="KEEP">Keep native</option>
            </Select>
          </div>
          <div className="rounded-lg border border-[var(--color-border)] p-3">
            <div className="mb-2 flex items-center justify-between">
              <span className="text-xs font-medium uppercase tracking-wide text-[var(--color-muted)]">{t("layerDetail.bbox")}</span>
              <Button variant="ghost" onClick={recompute}>{t("layerEdit.computeBbox")}</Button>
            </div>
            <p className="font-mono text-xs text-[var(--color-muted)]">
              {d.bbox ? d.bbox.map((n) => n.toFixed(4)).join(", ") : "—"}
            </p>
            <p className="mt-1 text-xs text-[var(--color-muted)]">{t("layerDetail.features")}: {d.featureCount}</p>
          </div>
        </div>
      )}

      {tab === "feature" && (
        <div className="space-y-1">
          {d.attributes.map((a) => (
            <div key={a.name} className="flex justify-between rounded-md border border-[var(--color-border)] px-3 py-2 text-sm">
              <span className="font-mono">{a.name}</span>
              <span className="text-[var(--color-muted)]">{a.type}</span>
            </div>
          ))}
          {d.geomColumn && (
            <div className="flex justify-between rounded-md border border-[var(--color-border)] px-3 py-2 text-sm">
              <span className="font-mono">{d.geomColumn}</span>
              <span className="text-[var(--color-muted)]">{d.geomType}</span>
            </div>
          )}
        </div>
      )}

      {showBuilder && (
        <StyleBuilder
          layer={`${ws}:${name}`}
          geomType={d.geomType}
          columns={d.attributes.map((a) => a.name)}
          onClose={() => setShowBuilder(false)}
          onSaved={(styleName) => {
            setShowBuilder(false);
            set({ defaultStyle: styleName });
            loadStyles();
            toast({ title: t("builder.applied", { name: styleName }) });
          }}
        />
      )}
    </Drawer>
  );
}
