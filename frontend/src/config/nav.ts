export type NavItem = { key: string; href: string; icon: string };
export type NavGroup = { labelKey: string; items: NavItem[] };

export const navGroups: NavGroup[] = [
  {
    labelKey: "navgroup.data",
    items: [
      { key: "nav.overview", href: "/dashboard", icon: "overview" },
      { key: "nav.workspaces", href: "/dashboard/workspaces", icon: "workspaces" },
      { key: "nav.stores", href: "/dashboard/stores", icon: "stores" },
      { key: "nav.layers", href: "/dashboard/layers", icon: "layers" },
      { key: "nav.layerGroups", href: "/dashboard/layer-groups", icon: "layers" },
      { key: "nav.styles", href: "/dashboard/styles", icon: "styles" },
    ],
  },
  {
    labelKey: "navgroup.tiles",
    items: [{ key: "nav.tileCache", href: "/dashboard/tile-cache", icon: "tileCache" }],
  },
  {
    labelKey: "navgroup.services",
    items: [
      { key: "nav.wps", href: "/dashboard/wps", icon: "wps" },
      { key: "nav.conversions", href: "/dashboard/conversions", icon: "conversions" },
    ],
  },
  {
    labelKey: "navgroup.security",
    items: [{ key: "nav.security", href: "/dashboard/security", icon: "security" }],
  },
  {
    labelKey: "navgroup.system",
    items: [
      { key: "nav.status", href: "/dashboard/status", icon: "overview" },
      { key: "nav.docs", href: "/dashboard/docs", icon: "overview" },
      { key: "nav.settings", href: "/dashboard/settings", icon: "settings" },
    ],
  },
];
