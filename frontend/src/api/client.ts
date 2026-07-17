import { getToken } from "./auth/store";

const BASE = process.env.NEXT_PUBLIC_API_BASE ?? "";

export class ApiError extends Error {
  status: number;
  constructor(status: number, message: string) {
    super(message);
    this.status = status;
  }
}

function authHeaders(extra?: HeadersInit): HeadersInit {
  const h: Record<string, string> = {};
  const token = getToken();
  if (token) h["Authorization"] = `Bearer ${token}`;
  return { ...h, ...(extra as Record<string, string>) };
}

/** apiFetch performs a JSON request and returns the parsed body. */
export async function apiFetch<T>(path: string, opts: RequestInit = {}): Promise<T> {
  const res = await fetch(BASE + path, {
    ...opts,
    headers: authHeaders({ "Content-Type": "application/json", ...(opts.headers as object) }),
  });
  if (!res.ok) throw new ApiError(res.status, await safeText(res));
  if (res.status === 204) return undefined as T;
  return (await res.json()) as T;
}

/** apiSend posts a raw body (e.g. XML for the GeoServer REST API). */
export async function apiSend(
  path: string,
  body: string,
  contentType = "application/xml",
): Promise<void> {
  const res = await fetch(BASE + path, {
    method: "POST",
    headers: authHeaders({ "Content-Type": contentType }),
    body,
  });
  if (!res.ok && res.status !== 409) throw new ApiError(res.status, await safeText(res));
}

/** apiPost sends a JSON body and ignores the (possibly non-JSON) response.
 * Used for the GeoServer-compat /rest API, which replies with plain text. */
export async function apiPost(path: string, body: unknown): Promise<void> {
  const res = await fetch(BASE + path, {
    method: "POST",
    headers: authHeaders({ "Content-Type": "application/json" }),
    body: JSON.stringify(body),
  });
  if (!res.ok && res.status !== 409) throw new ApiError(res.status, await safeText(res));
}

/** apiJson posts a JSON body and parses a JSON response. */
export async function apiJson<T>(path: string, body: unknown): Promise<T> {
  const res = await fetch(BASE + path, {
    method: "POST",
    headers: authHeaders({ "Content-Type": "application/json" }),
    body: JSON.stringify(body),
  });
  if (!res.ok) throw new ApiError(res.status, await safeText(res));
  if (res.status === 204) return undefined as T;
  return (await res.json()) as T;
}

export async function apiPut(path: string, body: unknown): Promise<void> {
  const res = await fetch(BASE + path, {
    method: "PUT",
    headers: authHeaders({ "Content-Type": "application/json" }),
    body: JSON.stringify(body),
  });
  if (!res.ok) throw new ApiError(res.status, await safeText(res));
}

export async function apiPatch(path: string, body: unknown): Promise<void> {
  const res = await fetch(BASE + path, {
    method: "PATCH",
    headers: authHeaders({ "Content-Type": "application/json" }),
    body: JSON.stringify(body),
  });
  if (!res.ok) throw new ApiError(res.status, await safeText(res));
}

export async function apiDelete(path: string): Promise<void> {
  const res = await fetch(BASE + path, { method: "DELETE", headers: authHeaders() });
  if (!res.ok && res.status !== 404) throw new ApiError(res.status, await safeText(res));
}

async function safeText(res: Response): Promise<string> {
  try {
    return (await res.text()) || res.statusText;
  } catch {
    return res.statusText;
  }
}
