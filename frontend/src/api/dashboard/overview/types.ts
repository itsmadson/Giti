export interface ServiceHealth {
  name: string;
  online: boolean;
}

export interface OverviewStats {
  workspaces: number;
  layers: number;
  services: ServiceHealth[];
}
