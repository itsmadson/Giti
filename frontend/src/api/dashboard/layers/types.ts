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
}
