"use client";

import { useEffect, useRef, useState } from "react";
import { basemaps, basemapStyle, gitiMvtTiles, type BasemapId } from "@/lib/basemaps";
import "maplibre-gl/dist/maplibre-gl.css";

// Geometry-aware MapLibre layer spec for a vector MVT source.
function layerSpec(id: string, srcLayer: string, geomType: string) {
  const g = (geomType || "").toUpperCase();
  if (g.includes("POLYGON")) {
    return { id, type: "fill" as const, source: id, "source-layer": srcLayer,
      paint: { "fill-color": "#2FA7A1", "fill-opacity": 0.35, "fill-outline-color": "#1E4E8C" } };
  }
  if (g.includes("LINE")) {
    return { id, type: "line" as const, source: id, "source-layer": srcLayer,
      paint: { "line-color": "#2FA7A1", "line-width": 2 } };
  }
  return { id, type: "circle" as const, source: id, "source-layer": srcLayer,
    paint: { "circle-radius": 5, "circle-color": "#2FA7A1", "circle-stroke-color": "#1E4E8C", "circle-stroke-width": 1, "circle-opacity": 0.9 } };
}

export function LayerPreviewMap({ layer, geomType, bbox, className }: {
  layer: string; // ws:name
  geomType: string;
  bbox?: number[];
  className?: string;
}) {
  const ref = useRef<HTMLDivElement>(null);
  const mapRef = useRef<import("maplibre-gl").Map | null>(null);
  const [basemap, setBasemap] = useState<BasemapId>("osm");

  // Add the layer's MVT overlay + fit bounds. Called on load and after each
  // basemap style swap (setStyle clears added sources/layers).
  function addOverlay(map: import("maplibre-gl").Map) {
    const srcId = `giti-${layer}`;
    const srcLayer = layer.split(":").pop() ?? layer;
    if (map.getSource(srcId)) return;
    map.addSource(srcId, { type: "vector", tiles: [gitiMvtTiles(layer)] });
    map.addLayer(layerSpec(srcId, srcLayer, geomType));
  }

  useEffect(() => {
    let cancelled = false;
    (async () => {
      const maplibre = await import("maplibre-gl");
      if (cancelled || !ref.current || mapRef.current) return;
      const map = new maplibre.Map({
        container: ref.current,
        style: basemapStyle("osm"),
        center: bbox && bbox.length === 4 ? [(bbox[0] + bbox[2]) / 2, (bbox[1] + bbox[3]) / 2] : [51.4, 35.7],
        zoom: 4,
      });
      mapRef.current = map;
      map.addControl(new maplibre.NavigationControl({}), "top-right");
      map.on("load", () => {
        addOverlay(map);
        if (bbox && bbox.length === 4) {
          map.fitBounds([[bbox[0], bbox[1]], [bbox[2], bbox[3]]], { padding: 40, duration: 0, maxZoom: 14 });
        }
      });
    })();
    return () => {
      cancelled = true;
      mapRef.current?.remove();
      mapRef.current = null;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [layer, geomType]);

  function switchBasemap(id: BasemapId) {
    setBasemap(id);
    const map = mapRef.current;
    if (!map) return;
    map.setStyle(basemapStyle(id));
    map.once("styledata", () => addOverlay(map));
  }

  return (
    <div className={"relative " + (className ?? "")}>
      <div ref={ref} className="absolute inset-0" />
      <div className="absolute start-2 top-2 z-10">
        <select
          value={basemap}
          onChange={(e) => switchBasemap(e.target.value as BasemapId)}
          className="rounded-md border border-[var(--color-border)] bg-[var(--color-surface)] px-2 py-1 text-xs shadow-sm outline-none"
        >
          {basemaps.map((b) => (
            <option key={b.id} value={b.id}>{b.label}</option>
          ))}
        </select>
      </div>
    </div>
  );
}
