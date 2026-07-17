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

// PostGIS connection form. host="self" targets Giti's own database.
export interface PgConnection {
  host: string;
  port: string;
  database: string;
  user: string;
  passwd: string;
  schema: string;
}
