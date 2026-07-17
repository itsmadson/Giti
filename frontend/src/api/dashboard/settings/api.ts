import { apiFetch, apiPut } from "@/api/client";

export interface GlobalSettings {
  organization?: string;
  contactName?: string;
  contactEmail?: string;
  serviceTitle?: string;
  serviceAbstract?: string;
  proxyBaseUrl?: string;
}

export function getSettings(): Promise<GlobalSettings> {
  return apiFetch<GlobalSettings>("/api/v1/settings");
}

export function saveSettings(s: GlobalSettings): Promise<void> {
  return apiPut("/api/v1/settings", s);
}
