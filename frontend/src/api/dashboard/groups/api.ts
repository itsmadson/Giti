import { apiFetch, apiJson, apiDelete } from "@/api/client";

export interface GroupMember {
  layer: string;
  style: string;
}

export interface LayerGroup {
  workspace: string;
  name: string;
  title: string;
  abstract: string;
  mode: string;
  srs: string;
  bounds?: number[];
  members: GroupMember[];
}

export function listGroups(): Promise<LayerGroup[]> {
  return apiFetch<LayerGroup[]>("/api/v1/layergroups");
}

export function getGroup(ws: string, name: string): Promise<LayerGroup> {
  return apiFetch<LayerGroup>(`/api/v1/layergroups/${encodeURIComponent(ws)}/${encodeURIComponent(name)}`);
}

export function saveGroup(g: LayerGroup): Promise<void> {
  return apiJson("/api/v1/layergroups", g);
}

export function deleteGroup(ws: string, name: string): Promise<void> {
  return apiDelete(`/api/v1/layergroups/${encodeURIComponent(ws)}/${encodeURIComponent(name)}`);
}

export function computeGroupBounds(ws: string, name: string): Promise<{ bounds: number[] | null }> {
  return apiJson(`/api/v1/layergroups/${encodeURIComponent(ws)}/${encodeURIComponent(name)}/bounds`, {});
}
