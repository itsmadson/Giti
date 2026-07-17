import { apiFetch, apiPatch, apiJson } from "@/api/client";
import type { Layer, LayerDetail, FeatureTypePatch, LayerPatch, SRSInfo } from "./types";

export function listLayers(): Promise<Layer[]> {
  return apiFetch<Layer[]>("/api/v1/layers");
}

export function getLayerDetail(ws: string, name: string): Promise<LayerDetail> {
  return apiFetch<LayerDetail>(
    `/api/v1/layers/${encodeURIComponent(ws)}/${encodeURIComponent(name)}`,
  );
}

export function patchLayer(ws: string, name: string, patch: LayerPatch): Promise<void> {
  return apiPatch(`/api/v1/layers/${encodeURIComponent(ws)}/${encodeURIComponent(name)}`, patch);
}

export function patchFeatureType(ws: string, name: string, patch: FeatureTypePatch): Promise<void> {
  return apiPatch(`/api/v1/featuretypes/${encodeURIComponent(ws)}/${encodeURIComponent(name)}`, patch);
}

export function computeBbox(ws: string, name: string): Promise<{ bbox?: number[]; featureCount: number }> {
  return apiJson(`/api/v1/layers/${encodeURIComponent(ws)}/${encodeURIComponent(name)}/bbox`, {});
}

export function getSRS(code: string): Promise<SRSInfo> {
  return apiFetch<SRSInfo>(`/api/v1/srs/${encodeURIComponent(code)}`);
}
