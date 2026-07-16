import { apiFetch } from "@/api/client";
import type { Layer } from "./types";

export function listLayers(): Promise<Layer[]> {
  return apiFetch<Layer[]>("/api/v1/layers");
}
