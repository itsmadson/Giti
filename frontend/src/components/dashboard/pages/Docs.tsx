"use client";

import { useEffect, useState } from "react";
import { Copy, Check, ExternalLink } from "lucide-react";
import { Card } from "@/components/ui/Card";

function useOrigin() {
  const [o, setO] = useState("http://localhost");
  useEffect(() => {
    setO(process.env.NEXT_PUBLIC_API_BASE || window.location.origin);
  }, []);
  return o;
}

function Code({ children }: { children: string }) {
  const [done, setDone] = useState(false);
  return (
    <div className="group relative">
      <pre className="overflow-x-auto rounded-lg border border-[var(--color-border)] bg-[var(--color-surface-2)] p-3 pe-10 font-mono text-xs leading-relaxed">
        {children}
      </pre>
      <button
        onClick={() => {
          navigator.clipboard.writeText(children);
          setDone(true);
          setTimeout(() => setDone(false), 1200);
        }}
        className="absolute end-2 top-2 rounded-md p-1 text-[var(--color-muted)] opacity-0 transition-opacity hover:bg-[var(--color-surface)] group-hover:opacity-100"
      >
        {done ? <Check size={14} className="text-[var(--color-ok)]" /> : <Copy size={14} />}
      </button>
    </div>
  );
}

function Section({ id, title, children }: { id: string; title: string; children: React.ReactNode }) {
  return (
    <section id={id} className="scroll-mt-20 space-y-3">
      <h2 className="font-display text-lg font-semibold">{title}</h2>
      {children}
    </section>
  );
}

function ParamTable({ rows }: { rows: [string, string, string][] }) {
  return (
    <div className="overflow-hidden rounded-lg border border-[var(--color-border)]">
      <table className="w-full text-sm">
        <thead>
          <tr className="border-b border-[var(--color-border)] text-start text-xs uppercase tracking-wide text-[var(--color-muted)]">
            <th className="px-3 py-2 text-start">Parameter</th>
            <th className="px-3 py-2 text-start">Req.</th>
            <th className="px-3 py-2 text-start">Meaning</th>
          </tr>
        </thead>
        <tbody>
          {rows.map(([p, req, m]) => (
            <tr key={p} className="border-b border-[var(--color-border)] last:border-0">
              <td className="px-3 py-1.5 font-mono text-xs">{p}</td>
              <td className="px-3 py-1.5 text-xs">{req}</td>
              <td className="px-3 py-1.5 text-[var(--color-muted)]">{m}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

const toc = [
  ["start", "Getting started"],
  ["services", "OGC service endpoints"],
  ["wms", "WMS — maps"],
  ["wfs", "WFS — features"],
  ["tiles", "WMTS / XYZ — tiles"],
  ["wcs", "WCS — coverages"],
  ["csw", "CSW — catalogue"],
  ["wps", "WPS — processing"],
  ["ogcapi", "OGC API - Features"],
  ["rest", "REST / JSON API"],
  ["recipes", "Recipes"],
];

export function Docs() {
  const o = useOrigin();
  const cap = (svc: string, path: string) => `${o}${path}?service=${svc}&request=GetCapabilities`;
  return (
    <div className="mx-auto grid max-w-5xl gap-6 lg:grid-cols-[180px_1fr]">
      <nav className="hidden lg:block">
        <div className="sticky top-4 space-y-1 text-sm">
          {toc.map(([id, label]) => (
            <a key={id} href={`#${id}`} className="block rounded px-2 py-1 text-[var(--color-muted)] hover:text-[var(--color-text)]">
              {label}
            </a>
          ))}
        </div>
      </nav>

      <div className="min-w-0 space-y-8">
        <div>
          <h1 className="font-display text-2xl font-semibold">Giti API & usage docs</h1>
          <p className="mt-1 text-sm text-[var(--color-muted)]">
            Every OGC service and REST endpoint Giti exposes, with copy-paste examples.
          </p>
        </div>

        <Section id="start" title="Getting started">
          <Card className="space-y-2 p-4 text-sm text-[var(--color-muted)]">
            <p><b className="text-[var(--color-text)]">1. Add data</b> — Stores → Add store. Connect a PostGIS database, or upload a file (GeoJSON → PostGIS, GeoTIFF → raster coverage).</p>
            <p><b className="text-[var(--color-text)]">2. Publish</b> — pick tables to publish, or the upload auto-publishes a layer.</p>
            <p><b className="text-[var(--color-text)]">3. Style</b> — Layers → Edit → Publishing → New visual style. Build rules (colours, labels, conditions, zoom) with no SLD needed.</p>
            <p><b className="text-[var(--color-text)]">4. Preview</b> — Layers → Preview: see the layer on a map with selectable basemaps and styles.</p>
            <p><b className="text-[var(--color-text)]">5. Serve</b> — every layer is instantly available over WMS/WFS/WMTS (and WCS for rasters). Base URL: <code className="font-mono">{o}</code></p>
          </Card>
        </Section>

        <Section id="services" title="OGC service endpoints">
          <div className="overflow-hidden rounded-lg border border-[var(--color-border)]">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b border-[var(--color-border)] text-start text-xs uppercase tracking-wide text-[var(--color-muted)]">
                  <th className="px-3 py-2 text-start">Service</th>
                  <th className="px-3 py-2 text-start">Endpoint</th>
                  <th className="px-3 py-2 text-start">Capabilities</th>
                </tr>
              </thead>
              <tbody>
                {[
                  ["WMS 1.1.1/1.3.0", "/giti/wms", cap("WMS", "/giti/wms")],
                  ["WFS 1.0/1.1/2.0", "/giti/wfs", cap("WFS", "/giti/wfs")],
                  ["WMTS 1.0", "/giti/gwc/service/wmts", cap("WMTS", "/giti/gwc/service/wmts")],
                  ["WCS 2.0", "/giti/wcs", cap("WCS", "/giti/wcs")],
                  ["CSW 2.0.2", "/giti/csw", cap("CSW", "/giti/csw")],
                  ["WPS 1.0", "/giti/wps", cap("WPS", "/giti/wps")],
                  ["OGC API - Features", "/api/v1/ogc/features", `${o}/api/v1/ogc/features`],
                  ["XYZ vector tiles", "/tiles/{layer}/{z}/{x}/{y}.pbf", ""],
                ].map(([name, ep, url]) => (
                  <tr key={name} className="border-b border-[var(--color-border)] last:border-0">
                    <td className="px-3 py-1.5">{name}</td>
                    <td className="px-3 py-1.5 font-mono text-xs">{ep}</td>
                    <td className="px-3 py-1.5">
                      {url ? (
                        <a href={url} target="_blank" rel="noreferrer" className="inline-flex items-center gap-1 text-xs text-[var(--color-primary)] hover:underline">
                          open <ExternalLink size={11} />
                        </a>
                      ) : (
                        <span className="text-xs text-[var(--color-muted)]">—</span>
                      )}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </Section>

        <Section id="wms" title="WMS — maps (raster/vector rendering)">
          <p className="text-sm text-[var(--color-muted)]">GetMap renders a map image; GetFeatureInfo queries a pixel; on-the-fly reprojection to any advertised CRS.</p>
          <ParamTable rows={[
            ["LAYERS", "yes", "workspace:layer(s), comma-separated"],
            ["STYLES", "yes", "style name(s); empty = default"],
            ["CRS / SRS", "yes", "EPSG:4326, EPSG:3857, …"],
            ["BBOX", "yes", "minx,miny,maxx,maxy (1.3.0 geographic = lat,lon)"],
            ["WIDTH / HEIGHT", "yes", "image size in px (≤10000)"],
            ["FORMAT", "yes", "image/png · image/jpeg · image/webp"],
            ["TRANSPARENT", "no", "true / false"],
            ["CQL_FILTER", "no", "ECQL filter, e.g. pop>100000"],
            ["FILTER", "no", "OGC Filter (XML)"],
            ["TIME / ELEVATION", "no", "dimension value or start/end range"],
            ["SLD_BODY / SLD", "no", "inline SLD, or an SLD URL"],
            ["EXCEPTIONS", "no", "XML · INIMAGE · BLANK · application/json"],
          ]} />
          <Code>{`${o}/giti/wms?service=WMS&version=1.1.1&request=GetMap&layers=WS:LAYER&styles=&srs=EPSG:3857&bbox=5000000,3900000,5900000,4800000&width=768&height=768&format=image/png&transparent=true`}</Code>
        </Section>

        <Section id="wfs" title="WFS — features (vector data + editing)">
          <p className="text-sm text-[var(--color-muted)]">GetFeature returns features (GeoJSON/GML); Transaction edits them; SRSNAME reprojects output.</p>
          <ParamTable rows={[
            ["typeNames", "yes", "workspace:layer"],
            ["outputFormat", "no", "application/json · GML3 · GML32 · csv · KML"],
            ["count / startIndex", "no", "paging"],
            ["srsName", "no", "reproject output to this CRS"],
            ["bbox", "no", "minx,miny,maxx,maxy[,crs]"],
            ["CQL_FILTER / FILTER", "no", "ECQL text / OGC Filter XML"],
            ["sortBy", "no", "col A|D"],
            ["propertyName", "no", "subset of columns"],
          ]} />
          <Code>{`${o}/giti/wfs?service=WFS&version=2.0.0&request=GetFeature&typeNames=WS:LAYER&count=10&outputFormat=application/json&srsName=EPSG:3857&CQL_FILTER=pop>100000`}</Code>
          <p className="text-xs text-[var(--color-muted)]">Also: GetPropertyValue, LockFeature (enforced), stored query GetFeatureById, Transaction (Insert/Update/Delete).</p>
        </Section>

        <Section id="tiles" title="WMTS / XYZ — vector tiles">
          <p className="text-sm text-[var(--color-muted)]">MVT vector tiles with a content-addressed cache. Use directly in MapLibre/OpenLayers.</p>
          <Code>{`${o}/tiles/WS:LAYER/{z}/{x}/{y}.pbf

# WMTS KVP
${o}/giti/gwc/service/wmts?service=WMTS&version=1.0.0&request=GetTile&layer=WS:LAYER&tilematrixset=EPSG:3857&tilematrix=6&tilerow=25&tilecol=41&format=application/vnd.mapbox-vector-tile`}</Code>
        </Section>

        <Section id="wcs" title="WCS — coverages (raster)">
          <Code>{`# describe
${o}/giti/wcs?service=WCS&version=2.0.1&request=DescribeCoverage&coverageId=WS__LAYER

# subset → PNG or GeoTIFF
${o}/giti/wcs?service=WCS&version=2.0.1&request=GetCoverage&coverageId=WS__LAYER&subset=Long(50,55)&subset=Lat(30,35)&format=image/png`}</Code>
        </Section>

        <Section id="csw" title="CSW — catalogue (metadata search)">
          <Code>{`${o}/giti/csw?service=CSW&version=2.0.2&request=GetRecords&typeNames=csw:Record&resultType=results&maxRecords=10&constraint=AnyText LIKE '%25iran%25'`}</Code>
        </Section>

        <Section id="wps" title="WPS — processing">
          <p className="text-sm text-[var(--color-muted)]">Geospatial processes (buffer, clip, intersection, …), sync or async.</p>
          <Code>{`${o}/giti/wps?service=WPS&version=1.0.0&request=DescribeProcess&identifier=giti:buffer`}</Code>
        </Section>

        <Section id="ogcapi" title="OGC API - Features (modern JSON)">
          <ParamTable rows={[
            ["limit / offset", "no", "paging"],
            ["bbox", "no", "minx,miny,maxx,maxy"],
            ["filter", "no", "CQL2 / CQL text"],
          ]} />
          <Code>{`${o}/api/v1/ogc/features/collections
${o}/api/v1/ogc/features/collections/WS:LAYER/items?limit=100&filter=pop>1000000&bbox=44,25,63,40`}</Code>
        </Section>

        <Section id="rest" title="REST / JSON API (/api/v1)">
          <p className="text-sm text-[var(--color-muted)]">The console uses these; call them from your own apps (send the bearer token from login).</p>
          <ParamTable rows={[
            ["GET /api/v1/store-types", "", "supported store types + connection schema"],
            ["GET/POST /api/v1/stores", "", "list / create stores"],
            ["GET /api/v1/stores/{ws}/{store}/tables", "", "introspect publishable tables"],
            ["GET /api/v1/layers", "", "all published layers"],
            ["GET/PATCH /api/v1/layers/{ws}/{name}", "", "layer detail / edit"],
            ["POST /api/v1/ingest", "", "upload GeoJSON → PostGIS layer"],
            ["POST /api/v1/convert/coverage", "", "upload GeoTIFF → raster coverage"],
            ["GET/POST /api/v1/styles", "", "styles; /validate, /generate"],
            ["GET/POST /api/v1/layergroups", "", "layer groups"],
            ["/api/v1/gwc/*", "", "gridsets, blobstores, quota, per-layer cache"],
            ["/api/v1/settings", "", "global service metadata"],
            ["POST /api/v1/auth/login", "", "get a JWT bearer token"],
          ]} />
        </Section>

        <Section id="recipes" title="Recipes">
          <p className="text-sm font-medium">Get features as GeoJSON, reprojected + filtered</p>
          <Code>{`curl "${o}/giti/wfs?service=WFS&version=2.0.0&request=GetFeature&typeNames=WS:LAYER&outputFormat=application/json&srsName=EPSG:3857&CQL_FILTER=area>1000"`}</Code>
          <p className="text-sm font-medium">Ingest a GeoJSON file</p>
          <Code>{`curl -F file=@data.geojson "${o}/api/v1/ingest?workspace=myws&name=mylayer"`}</Code>
          <p className="text-sm font-medium">Add a layer to MapLibre (vector tiles)</p>
          <Code>{`map.addSource('l', { type:'vector', tiles:['${o}/tiles/WS:LAYER/{z}/{x}/{y}.pbf'] });
map.addLayer({ id:'l', type:'fill', source:'l', 'source-layer':'LAYER',
  paint:{ 'fill-color':'#2FA7A1', 'fill-opacity':0.4 } });`}</Code>
        </Section>
      </div>
    </div>
  );
}
