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

// Absolute origin for API calls. MapLibre's tile fetch needs absolute URLs.
function apiOrigin(): string {
  const base = process.env.NEXT_PUBLIC_API_BASE ?? "";
  if (base) return base;
  return typeof window !== "undefined" ? window.location.origin : "";
}

// Giti MVT source URL template for a published vector layer (absolute).
export function gitiMvtTiles(layer: string): string {
  return `${apiOrigin()}/tiles/${layer}/{z}/{x}/{y}.pbf`;
}

// Sample OGC service URLs for a published layer (for the Layers "Services" column).
export function serviceSamples(layer: string, bbox?: number[]) {
  const o = apiOrigin();
  const bb = bbox && bbox.length === 4 ? bbox.join(",") : "-180,-90,180,90";
  return {
    wms: `${o}/giti/wms?service=WMS&version=1.1.1&request=GetMap&layers=${layer}&styles=&srs=EPSG:4326&bbox=${bb}&width=768&height=576&format=image/png`,
    wmsCaps: `${o}/giti/wms?service=WMS&version=1.1.1&request=GetCapabilities`,
    wfs: `${o}/giti/wfs?service=WFS&version=2.0.0&request=GetFeature&typeNames=${layer}&count=10&outputFormat=application/json`,
    wfsCaps: `${o}/giti/wfs?service=WFS&version=2.0.0&request=GetCapabilities`,
  };
}

// Selectable raster basemaps for the preview/map. Google endpoints are the
// public unofficial tile hosts — fine for internal/dev use.
export type BasemapId =
  | "osm"
  | "carto-light"
  | "carto-dark"
  | "esri-imagery"
  | "google-streets"
  | "google-satellite"
  | "google-hybrid";

type BasemapDef = { id: BasemapId; label: string; tiles: string[]; attribution: string; bg: string };

export const basemaps: BasemapDef[] = [
  { id: "osm", label: "OpenStreetMap", tiles: ["https://tile.openstreetmap.org/{z}/{x}/{y}.png"], attribution: "© OpenStreetMap", bg: "#F5F8FC" },
  { id: "carto-light", label: "Carto Light", tiles: ["https://a.basemaps.cartocdn.com/light_all/{z}/{x}/{y}.png"], attribution: "© Carto", bg: "#F5F8FC" },
  { id: "carto-dark", label: "Carto Dark", tiles: ["https://a.basemaps.cartocdn.com/dark_all/{z}/{x}/{y}.png"], attribution: "© Carto", bg: "#0B1220" },
  { id: "esri-imagery", label: "Esri Satellite", tiles: ["https://server.arcgisonline.com/ArcGIS/rest/services/World_Imagery/MapServer/tile/{z}/{y}/{x}"], attribution: "© Esri", bg: "#0B1220" },
  { id: "google-streets", label: "Google Streets", tiles: ["https://mt1.google.com/vt/lyrs=m&x={x}&y={y}&z={z}"], attribution: "© Google", bg: "#F5F8FC" },
  { id: "google-satellite", label: "Google Satellite", tiles: ["https://mt1.google.com/vt/lyrs=s&x={x}&y={y}&z={z}"], attribution: "© Google", bg: "#0B1220" },
  { id: "google-hybrid", label: "Google Hybrid", tiles: ["https://mt1.google.com/vt/lyrs=y&x={x}&y={y}&z={z}"], attribution: "© Google", bg: "#0B1220" },
];

export function basemapStyle(id: BasemapId): StyleSpecification {
  const b = basemaps.find((x) => x.id === id) ?? basemaps[0];
  return {
    version: 8,
    sources: {
      base: { type: "raster", tiles: b.tiles, tileSize: 256, attribution: b.attribution },
    },
    layers: [
      { id: "bg", type: "background", paint: { "background-color": b.bg } },
      { id: "base", type: "raster", source: "base" },
    ],
  };
}
