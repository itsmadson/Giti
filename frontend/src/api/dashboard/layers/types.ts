export interface Layer {
  workspace: string;
  name: string;
  type: string; // VECTOR | RASTER
  defaultStyle: string;
}

export interface LayerAttribute {
  name: string;
  type: string;
}

export interface LayerDetail {
  workspace: string;
  name: string;
  type: string;
  srs: string;
  store: string;
  table: string;
  geomColumn: string;
  geomType: string;
  defaultStyle: string;
  attributes: LayerAttribute[];
  bbox?: number[]; // minx,miny,maxx,maxy (EPSG:4326)
  featureCount: number;
  title: string;
  abstract: string;
  keywords: string[];
  declaredSrs: string;
  srsHandling: string;
  queryable: boolean;
  opaque: boolean;
  advertised: boolean;
  alternateStyles: string[];
  timeColumn: string;
  elevationColumn: string;
}

export interface FeatureTypePatch {
  title?: string;
  abstract?: string;
  keywords?: string[];
  srs?: string;
  declaredSrs?: string;
  srsHandling?: string;
  timeColumn?: string;
  elevationColumn?: string;
}

export interface LayerPatch {
  defaultStyle?: string;
  alternateStyles?: string[];
  queryable?: boolean;
  opaque?: boolean;
  advertised?: boolean;
  enabled?: boolean;
}

export interface SRSInfo {
  code: string;
  name: string;
}
