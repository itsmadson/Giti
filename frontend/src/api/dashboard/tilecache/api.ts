import { apiFetch, apiJson, apiPut, apiDelete } from "@/api/client";

export interface Gridset {
  name: string;
  srs: string;
  extent?: number[];
  tileSize: number;
  levels: number;
}
export interface BlobStore {
  name: string;
  type: string;
  config?: Record<string, string>;
  isDefault: boolean;
}
export interface DiskQuota {
  policy: string;
  maxBytes: number;
}

export const listGridsets = () => apiFetch<Gridset[]>("/api/v1/gwc/gridsets");
export const saveGridset = (g: Gridset) => apiJson<void>("/api/v1/gwc/gridsets", g);
export const deleteGridset = (name: string) => apiDelete(`/api/v1/gwc/gridsets/${encodeURIComponent(name)}`);

export const listBlobStores = () => apiFetch<BlobStore[]>("/api/v1/gwc/blobstores");
export const saveBlobStore = (b: BlobStore) => apiJson<void>("/api/v1/gwc/blobstores", b);
export const deleteBlobStore = (name: string) => apiDelete(`/api/v1/gwc/blobstores/${encodeURIComponent(name)}`);

export const getQuota = () => apiFetch<DiskQuota>("/api/v1/gwc/quota");
export const setQuota = (q: DiskQuota) => apiPut("/api/v1/gwc/quota", q);
