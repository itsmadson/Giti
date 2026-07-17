import { apiFetch, apiJson, apiPut, apiDelete } from "@/api/client";
import type { Style, StyleContent, ValidationResult } from "./types";

export function listStyles(): Promise<Style[]> {
  return apiFetch<Style[]>("/api/v1/styles");
}

export function getStyle(name: string): Promise<StyleContent> {
  return apiFetch<StyleContent>(`/api/v1/styles/${encodeURIComponent(name)}`);
}

export function createStyle(name: string, format: string, content: string): Promise<{ name: string }> {
  return apiJson<{ name: string }>("/api/v1/styles", { name, format, content });
}

export function updateStyle(name: string, format: string, content: string): Promise<void> {
  return apiPut(`/api/v1/styles/${encodeURIComponent(name)}`, { format, content });
}

export function deleteStyle(name: string): Promise<void> {
  return apiDelete(`/api/v1/styles/${encodeURIComponent(name)}`);
}

export function validateStyle(format: string, content: string): Promise<ValidationResult> {
  return apiJson<ValidationResult>("/api/v1/styles/validate", { format, content });
}

export function generateStyle(geomType: string, color?: string): Promise<{ sld: string }> {
  return apiJson<{ sld: string }>("/api/v1/styles/generate", { geomType, color });
}
