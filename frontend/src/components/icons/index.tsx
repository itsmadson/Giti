import {
  LayoutDashboard,
  FolderTree,
  Database,
  Layers as LayersIcon,
  Palette,
  Grid3x3,
  Shield,
  Cpu,
  FileUp,
  Settings,
  Map as MapIcon,
  Globe,
  type LucideIcon,
} from "lucide-react";

/** Semantic name → Lucide icon. Keep UI code free of icon-library names. */
export const icons: Record<string, LucideIcon> = {
  overview: LayoutDashboard,
  workspaces: FolderTree,
  stores: Database,
  layers: LayersIcon,
  styles: Palette,
  tileCache: Grid3x3,
  security: Shield,
  wps: Cpu,
  conversions: FileUp,
  settings: Settings,
  map: MapIcon,
  brand: Globe,
};
