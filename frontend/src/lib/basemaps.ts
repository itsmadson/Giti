import type { StyleSpecification } from "maplibre-gl";

// A dependency-free raster basemap (OSM) — dark/light variants via a tint layer.
export function baseStyle(dark: boolean): StyleSpecification {
  return {
    version: 8,
    sources: {
      osm: {
        type: "raster",
        tiles: ["https://tile.openstreetmap.org/{z}/{x}/{y}.png"],
        tileSize: 256,
        attribution: "© OpenStreetMap contributors",
      },
    },
    layers: [
      { id: "bg", type: "background", paint: { "background-color": dark ? "#0B1220" : "#F5F8FC" } },
      { id: "osm", type: "raster", source: "osm", paint: { "raster-opacity": dark ? 0.55 : 0.9 } },
    ],
  };
}

// Giti MVT source URL template for a published vector layer.
export function gitiMvtTiles(layer: string): string {
  const base = process.env.NEXT_PUBLIC_API_BASE ?? "";
  return `${base}/tiles/${layer}/{z}/{x}/{y}.pbf`;
}
