import { apiFetch, apiPost, apiJson, apiPut, apiDelete } from "@/api/client";
import type { Store, StoreTable, PgConnection, StoreType, StoreReq, TestResult } from "./types";

export function listStoreTypes(): Promise<StoreType[]> {
  return apiFetch<StoreType[]>("/api/v1/store-types");
}

export function createStore(req: StoreReq): Promise<{ name: string }> {
  return apiJson<{ name: string }>("/api/v1/stores", req);
}

export function updateStore(ws: string, name: string, req: StoreReq): Promise<void> {
  return apiPut(`/api/v1/stores/${encodeURIComponent(ws)}/${encodeURIComponent(name)}`, req);
}

export function deleteStore(ws: string, name: string, recurse = false): Promise<void> {
  return apiDelete(
    `/api/v1/stores/${encodeURIComponent(ws)}/${encodeURIComponent(name)}?recurse=${recurse}`,
  );
}

export function testStore(
  req: Partial<StoreReq> & { type: string; connection: Record<string, string> },
): Promise<TestResult> {
  return apiJson<TestResult>("/api/v1/stores/test", req);
}

export function listStores(): Promise<Store[]> {
  return apiFetch<Store[]>("/api/v1/stores");
}

export function listStoreTables(ws: string, store: string): Promise<StoreTable[]> {
  return apiFetch<StoreTable[]>(
    `/api/v1/stores/${encodeURIComponent(ws)}/${encodeURIComponent(store)}/tables`,
  );
}

/** createPgStore registers a PostGIS datastore via the /rest compat API. */
export function createPgStore(
  ws: string,
  name: string,
  conn: PgConnection,
): Promise<void> {
  const entry = Object.entries(conn)
    .filter(([, v]) => v !== "")
    .map(([k, v]) => ({ "@key": k, $: v }));
  return apiPost(`/giti/rest/workspaces/${encodeURIComponent(ws)}/datastores`, {
    dataStore: {
      name,
      type: "PostGIS",
      enabled: true,
      connectionParameters: { entry },
    },
  });
}

/** publishTable publishes one store table as a featuretype + layer. */
export function publishTable(ws: string, store: string, table: string): Promise<void> {
  return apiPost(
    `/giti/rest/workspaces/${encodeURIComponent(ws)}/datastores/${encodeURIComponent(store)}/featuretypes`,
    { featureType: { name: table, nativeName: table, enabled: true } },
  );
}
