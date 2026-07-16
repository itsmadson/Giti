import { listWorkspaces } from "@/api/dashboard/workspaces/api";
import { listLayers } from "@/api/dashboard/layers/api";
import type { OverviewStats, ServiceHealth } from "./types";

// The gateway is reachable at the same origin; each OGC service answers at its
// virtual path. We probe a representative endpoint per service.
const probes: { name: string; path: string }[] = [
  { name: "gateway", path: "/healthz" },
  { name: "wfs", path: "/geoserver/wfs?service=WFS&version=2.0.0&request=GetCapabilities" },
  { name: "wms", path: "/geoserver/wms?service=WMS&version=1.3.0&request=GetCapabilities" },
  { name: "wps", path: "/geoserver/wps?service=WPS&version=1.0.0&request=GetCapabilities" },
];

async function probe(path: string): Promise<boolean> {
  try {
    const res = await fetch(path, { method: "GET" });
    return res.ok;
  } catch {
    return false;
  }
}

export async function getOverview(): Promise<OverviewStats> {
  const [workspaces, layers] = await Promise.all([
    listWorkspaces().catch(() => []),
    listLayers().catch(() => []),
  ]);
  const services: ServiceHealth[] = await Promise.all(
    probes.map(async (p) => ({ name: p.name, online: await probe(p.path) })),
  );
  return { workspaces: workspaces.length, layers: layers.length, services };
}
