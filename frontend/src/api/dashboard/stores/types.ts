export interface Store {
  workspace: string;
  name: string;
  type: string; // PostGIS | GeoParquet | Shapefile | ...
  kind: string; // datastore | coveragestore
  enabled: boolean;
}

export interface StoreTable {
  name: string;
  geomType: string;
  srs: string;
  published: boolean;
}

export interface ParamField {
  key: string;
  label: string;
  type: string; // text | number | password | select
  default?: string;
  required: boolean;
}

export interface StoreType {
  type: string;
  kind: string; // datastore | coveragestore
  category: string; // Vector | Raster | Cascade
  label: string;
  params: ParamField[];
}

export interface StoreReq {
  workspace: string;
  name: string;
  type: string;
  kind?: string;
  description?: string;
  enabled: boolean;
  connection: Record<string, string>;
}

export interface TestResult {
  ok: boolean;
  error?: string;
}

// PostGIS connection form. host="self" targets Giti's own database.
export interface PgConnection {
  host: string;
  port: string;
  database: string;
  user: string;
  passwd: string;
  schema: string;
}
