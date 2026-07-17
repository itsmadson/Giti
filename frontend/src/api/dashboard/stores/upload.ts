import { ApiError } from "@/api/client";
import { getToken } from "@/api/auth/store";

const BASE = process.env.NEXT_PUBLIC_API_BASE ?? "";

// uploadFile stores a file on the server's data volume and returns its path,
// used to fill a file-backed store's connection url.
export async function uploadFile(file: File): Promise<{ path: string; name: string }> {
  const fd = new FormData();
  fd.append("file", file);
  const headers: Record<string, string> = {};
  const token = getToken();
  if (token) headers["Authorization"] = `Bearer ${token}`;
  const res = await fetch(BASE + "/api/v1/convert/upload", { method: "POST", headers, body: fd });
  if (!res.ok) throw new ApiError(res.status, await res.text());
  return (await res.json()) as { path: string; name: string };
}
