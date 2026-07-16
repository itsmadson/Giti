import { apiFetch, apiSend } from "@/api/client";
import type { Workspace } from "./types";

export function listWorkspaces(): Promise<Workspace[]> {
  return apiFetch<Workspace[]>("/api/v1/workspaces");
}

export function createWorkspace(name: string): Promise<void> {
  return apiSend("/geoserver/rest/workspaces", `<workspace><name>${name}</name></workspace>`);
}
