import { apiFetch } from "@/api/client";
import type { Layer, LayerDetail } from "./types";

export function listLayers(): Promise<Layer[]> {
  return apiFetch<Layer[]>("/api/v1/layers");
}

export function getLayerDetail(ws: string, name: string): Promise<LayerDetail> {
  return apiFetch<LayerDetail>(
    `/api/v1/layers/${encodeURIComponent(ws)}/${encodeURIComponent(name)}`,
  );
}
