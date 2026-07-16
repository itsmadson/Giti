"use client";

import { useEffect, useRef, useState } from "react";
import { useTheme } from "next-themes";
import { useSearchParams } from "next/navigation";
import { useT } from "@/i18n/provider";
import { listLayers } from "@/api/dashboard/layers/api";
import type { Layer } from "@/api/dashboard/layers/types";
import { baseStyle, gitiMvtTiles } from "@/lib/basemaps";
import { Card } from "@/components/ui/Card";
import "maplibre-gl/dist/maplibre-gl.css";

export function MapWorkspace() {
  const { t } = useT();
  const { resolvedTheme } = useTheme();
  const dark = resolvedTheme !== "light";
  const search = useSearchParams();
  const containerRef = useRef<HTMLDivElement>(null);
  const mapRef = useRef<import("maplibre-gl").Map | null>(null);
  const [layers, setLayers] = useState<Layer[]>([]);
  const [active, setActive] = useState<Set<string>>(new Set());

  useEffect(() => {
    listLayers()
      .then((ls) => {
        setLayers(ls);
        const pre = search.get("layer");
        if (pre) setActive(new Set([pre]));
      })
      .catch(() => setLayers([]));
  }, [search]);

  useEffect(() => {
    let cancelled = false;
    (async () => {
      const maplibre = await import("maplibre-gl");
      if (cancelled || !containerRef.current || mapRef.current) return;
      mapRef.current = new maplibre.Map({
        container: containerRef.current,
        style: baseStyle(dark),
        center: [51.4, 35.7],
        zoom: 4,
        attributionControl: false,
      });
      mapRef.current.addControl(new maplibre.NavigationControl({}), "top-right");
    })();
    return () => {
      cancelled = true;
      mapRef.current?.remove();
      mapRef.current = null;
    };
    // re-init on theme change to swap basemap
  }, [dark]);

  function toggle(id: string) {
    const map = mapRef.current;
    if (!map) return;
    const next = new Set(active);
    const srcId = `giti-${id}`;
    if (next.has(id)) {
      if (map.getLayer(srcId)) map.removeLayer(srcId);
      if (map.getSource(srcId)) map.removeSource(srcId);
      next.delete(id);
    } else {
      map.addSource(srcId, { type: "vector", tiles: [gitiMvtTiles(id)] });
      map.addLayer({
        id: srcId,
        type: "circle",
        source: srcId,
        "source-layer": id.split(":").pop() ?? id,
        paint: { "circle-radius": 4, "circle-color": "#2FA7A1", "circle-opacity": 0.85 },
      });
      next.add(id);
    }
    setActive(next);
  }

  return (
    <div className="relative -m-6 h-[calc(100vh-3.5rem)]">
      <div ref={containerRef} className="absolute inset-0" />
      <Card className="absolute end-4 top-4 w-64 p-4 backdrop-blur">
        <div className="mb-3 text-xs font-medium uppercase tracking-wide text-[var(--color-muted)]">
          {t("map.layers")}
        </div>
        {layers.length === 0 && (
          <p className="text-sm text-[var(--color-muted)]">{t("map.noLayers")}</p>
        )}
        <div className="space-y-1">
          {layers.map((l) => {
            const id = `${l.workspace}:${l.name}`;
            return (
              <label key={id} className="flex cursor-pointer items-center gap-2 text-sm">
                <input
                  type="checkbox"
                  checked={active.has(id)}
                  onChange={() => toggle(id)}
                  className="accent-[var(--color-primary)]"
                />
                <span className="font-mono text-xs">{id}</span>
              </label>
            );
          })}
        </div>
      </Card>
    </div>
  );
}
