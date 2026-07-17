"use client";

import { useEffect, useRef, useState } from "react";
import { basemaps, basemapStyle, gitiMvtTiles, type BasemapId } from "@/lib/basemaps";
import "maplibre-gl/dist/maplibre-gl.css";

const TEAL = "#2FA7A1";
const INK = "#1E4E8C";

// Add the layer's MVT vector source plus fill/line/circle draw layers, so any
// geometry (polygon/line/point) renders regardless of the declared type. The
// tiles service reprojects to Web Mercator via ST_AsMVT, so this is correct in
// the map's native projection (WMS in Giti only serves EPSG:4326).
function addOverlay(map: import("maplibre-gl").Map, layer: string) {
  const srcId = `giti-${layer}`;
  const srcLayer = layer.split(":").pop() ?? layer;
  if (map.getSource(srcId)) return;
  map.addSource(srcId, { type: "vector", tiles: [gitiMvtTiles(layer)] });
  map.addLayer({
    id: srcId + "-fill", type: "fill", source: srcId, "source-layer": srcLayer,
    filter: ["==", ["geometry-type"], "Polygon"],
    paint: { "fill-color": TEAL, "fill-opacity": 0.4, "fill-outline-color": INK },
  });
  map.addLayer({
    id: srcId + "-line", type: "line", source: srcId, "source-layer": srcLayer,
    filter: ["in", ["geometry-type"], ["literal", ["LineString", "Polygon"]]],
    paint: { "line-color": INK, "line-width": 1 },
  });
  map.addLayer({
    id: srcId + "-pt", type: "circle", source: srcId, "source-layer": srcLayer,
    filter: ["==", ["geometry-type"], "Point"],
    paint: { "circle-radius": 5, "circle-color": TEAL, "circle-stroke-color": INK, "circle-stroke-width": 1 },
  });
}

export function LayerPreviewMap({ layer, bbox, className }: {
  layer: string; // ws:name
  geomType?: string;
  bbox?: number[]; // minx,miny,maxx,maxy (EPSG:4326)
  className?: string;
}) {
  const ref = useRef<HTMLDivElement>(null);
  const mapRef = useRef<import("maplibre-gl").Map | null>(null);
  const [basemap, setBasemap] = useState<BasemapId>("osm");
  const [err, setErr] = useState("");

  useEffect(() => {
    let cancelled = false;
    let ro: ResizeObserver | null = null;
    (async () => {
      const maplibre = await import("maplibre-gl");
      if (cancelled || !ref.current || mapRef.current) return;
      const center: [number, number] =
        bbox && bbox.length === 4 ? [(bbox[0] + bbox[2]) / 2, (bbox[1] + bbox[3]) / 2] : [53, 32];
      const map = new maplibre.Map({ container: ref.current, style: basemapStyle("osm"), center, zoom: 4 });
      mapRef.current = map;
      map.addControl(new maplibre.NavigationControl({}), "top-right");
      map.on("error", (e) => setErr(e.error?.message ?? "map error"));
      map.on("load", () => {
        map.resize();
        addOverlay(map, layer);
        if (bbox && bbox.length === 4) {
          map.fitBounds([[bbox[0], bbox[1]], [bbox[2], bbox[3]]], { padding: 40, duration: 0, maxZoom: 13 });
        }
      });
      ro = new ResizeObserver(() => map.resize());
      ro.observe(ref.current);
      setTimeout(() => map.resize(), 300);
    })();
    return () => {
      cancelled = true;
      ro?.disconnect();
      mapRef.current?.remove();
      mapRef.current = null;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [layer]);

  // Re-fit when the bbox arrives async (after the dialog opened).
  useEffect(() => {
    const map = mapRef.current;
    if (!map || !bbox || bbox.length !== 4) return;
    const fit = () => map.fitBounds([[bbox[0], bbox[1]], [bbox[2], bbox[3]]], { padding: 40, duration: 0, maxZoom: 13 });
    if (map.isStyleLoaded()) fit();
    else map.once("load", fit);
  }, [bbox]);

  function switchBasemap(id: BasemapId) {
    setBasemap(id);
    const map = mapRef.current;
    if (!map) return;
    map.setStyle(basemapStyle(id));
    map.once("styledata", () => addOverlay(map, layer));
  }

  return (
    <div className={"relative " + (className ?? "")}>
      <div ref={ref} className="h-full w-full" />
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
      {err && <div className="absolute bottom-2 start-2 z-10 rounded bg-[var(--color-err)] px-2 py-1 text-xs text-white">{err}</div>}
    </div>
  );
}
