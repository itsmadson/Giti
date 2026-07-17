"use client";

import { useEffect, useRef } from "react";
import { useTheme } from "next-themes";
import { baseStyle, gitiMvtTiles } from "@/lib/basemaps";
import "maplibre-gl/dist/maplibre-gl.css";

// Choose a MapLibre layer spec from the PostGIS geometry type.
function layerSpec(id: string, srcLayer: string, geomType: string) {
  const g = (geomType || "").toUpperCase();
  if (g.includes("POLYGON")) {
    return {
      id,
      type: "fill" as const,
      source: id,
      "source-layer": srcLayer,
      paint: {
        "fill-color": "#2FA7A1",
        "fill-opacity": 0.35,
        "fill-outline-color": "#1E4E8C",
      },
    };
  }
  if (g.includes("LINE")) {
    return {
      id,
      type: "line" as const,
      source: id,
      "source-layer": srcLayer,
      paint: { "line-color": "#2FA7A1", "line-width": 1.8 },
    };
  }
  return {
    id,
    type: "circle" as const,
    source: id,
    "source-layer": srcLayer,
    paint: { "circle-radius": 4, "circle-color": "#2FA7A1", "circle-opacity": 0.85 },
  };
}

export function LayerPreview({
  layer,
  geomType,
  bbox,
  className,
}: {
  layer: string; // ws:name
  geomType: string;
  bbox?: number[]; // minx,miny,maxx,maxy 4326
  className?: string;
}) {
  const { resolvedTheme } = useTheme();
  const dark = resolvedTheme !== "light";
  const ref = useRef<HTMLDivElement>(null);
  const mapRef = useRef<import("maplibre-gl").Map | null>(null);

  useEffect(() => {
    let cancelled = false;
    (async () => {
      const maplibre = await import("maplibre-gl");
      if (cancelled || !ref.current || mapRef.current) return;
      const map = new maplibre.Map({
        container: ref.current,
        style: baseStyle(dark),
        center: [51.4, 35.7],
        zoom: 3,
        attributionControl: false,
      });
      mapRef.current = map;
      map.addControl(new maplibre.NavigationControl({}), "top-right");
      map.on("load", () => {
        const srcId = `giti-${layer}`;
        const srcLayer = layer.split(":").pop() ?? layer;
        map.addSource(srcId, { type: "vector", tiles: [gitiMvtTiles(layer)] });
        map.addLayer(layerSpec(srcId, srcLayer, geomType));
        if (bbox && bbox.length === 4) {
          map.fitBounds([[bbox[0], bbox[1]], [bbox[2], bbox[3]]], {
            padding: 40,
            duration: 0,
            maxZoom: 14,
          });
        }
      });
    })();
    return () => {
      cancelled = true;
      mapRef.current?.remove();
      mapRef.current = null;
    };
  }, [layer, geomType, dark, bbox]);

  return <div ref={ref} className={className} />;
}
