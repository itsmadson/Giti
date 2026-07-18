// Visual style model → SLD 1.0 generator. Lets users build thematic styles
// (filters → colors/size/labels/zoom) without writing SLD by hand.

export type GeomKind = "point" | "line" | "polygon";
export type FilterOp = ">" | ">=" | "<" | "<=" | "=" | "!=" | "like";

export interface RuleFilter {
  column: string;
  op: FilterOp;
  value: string;
}

export interface StyleRule {
  name: string;
  filter?: RuleFilter;
  fill: string;
  fillOpacity: number;
  stroke: string;
  strokeWidth: number;
  size: number; // point mark size
  mark: "circle" | "square" | "triangle" | "star";
  labelColumn?: string;
  labelSize: number;
  labelColor: string;
  labelOpacity: number;
  labelHaloRadius: number;
  labelHaloColor: string;
  minZoom?: number; // rule visible at/above this zoom
  maxZoom?: number; // rule visible at/below this zoom
}

export interface StyleModel {
  geom: GeomKind;
  rules: StyleRule[];
}

export function newRule(geom: GeomKind): StyleRule {
  return {
    name: "Rule",
    fill: geom === "line" ? "#1E4E8C" : "#2FA7A1",
    fillOpacity: 0.6,
    stroke: "#1E4E8C",
    strokeWidth: geom === "line" ? 2 : 1,
    size: 8,
    mark: "circle",
    labelSize: 12,
    labelColor: "#1a1a1a",
    labelOpacity: 1,
    labelHaloRadius: 1,
    labelHaloColor: "#ffffff",
  };
}

// Web-Mercator scale denominator at a given zoom (approx, 96dpi).
function zoomToScale(z: number): number {
  return Math.round(559082264.028 / Math.pow(2, z));
}

const OP_EL: Record<FilterOp, string> = {
  ">": "PropertyIsGreaterThan",
  ">=": "PropertyIsGreaterThanOrEqualTo",
  "<": "PropertyIsLessThan",
  "<=": "PropertyIsLessThanOrEqualTo",
  "=": "PropertyIsEqualTo",
  "!=": "PropertyIsNotEqualTo",
  like: "PropertyIsLike",
};

function esc(s: string): string {
  return s.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;").replace(/"/g, "&quot;");
}

function filterXml(f: RuleFilter): string {
  const el = OP_EL[f.op];
  const attrs = f.op === "like" ? ` wildCard="*" singleChar="." escapeChar="!"` : "";
  const val = f.op === "like" ? esc(f.value) : esc(f.value);
  return (
    `<ogc:Filter><ogc:${el}${attrs}>` +
    `<ogc:PropertyName>${esc(f.column)}</ogc:PropertyName>` +
    `<ogc:Literal>${val}</ogc:Literal>` +
    `</ogc:${el}></ogc:Filter>`
  );
}

function symbolizer(geom: GeomKind, r: StyleRule): string {
  const fill = `<Fill><CssParameter name="fill">${r.fill}</CssParameter>` +
    `<CssParameter name="fill-opacity">${r.fillOpacity}</CssParameter></Fill>`;
  const stroke = `<Stroke><CssParameter name="stroke">${r.stroke}</CssParameter>` +
    `<CssParameter name="stroke-width">${r.strokeWidth}</CssParameter></Stroke>`;
  if (geom === "polygon") return `<PolygonSymbolizer>${fill}${stroke}</PolygonSymbolizer>`;
  if (geom === "line") return `<LineSymbolizer><Stroke>` +
    `<CssParameter name="stroke">${r.stroke}</CssParameter>` +
    `<CssParameter name="stroke-width">${r.strokeWidth}</CssParameter></Stroke></LineSymbolizer>`;
  return (
    `<PointSymbolizer><Graphic><Mark>` +
    `<WellKnownName>${r.mark}</WellKnownName>${fill}${stroke}</Mark>` +
    `<Size>${r.size}</Size></Graphic></PointSymbolizer>`
  );
}

function labelXml(r: StyleRule): string {
  if (!r.labelColumn) return "";
  const halo =
    r.labelHaloRadius > 0
      ? `<Halo><Radius>${r.labelHaloRadius}</Radius>` +
        `<Fill><CssParameter name="fill">${r.labelHaloColor}</CssParameter></Fill></Halo>`
      : "";
  return (
    `<TextSymbolizer><Label><ogc:PropertyName>${esc(r.labelColumn)}</ogc:PropertyName></Label>` +
    `<Font><CssParameter name="font-size">${r.labelSize}</CssParameter></Font>` +
    `<Fill><CssParameter name="fill">${r.labelColor}</CssParameter>` +
    `<CssParameter name="fill-opacity">${r.labelOpacity}</CssParameter></Fill>` +
    halo +
    `</TextSymbolizer>`
  );
}

// generateSld renders the visual model to an SLD 1.0 document.
export function generateSld(name: string, model: StyleModel): string {
  const rulesXml = model.rules
    .map((r) => {
      const parts = [`<Name>${esc(r.name)}</Name>`];
      if (r.filter && r.filter.column) parts.push(filterXml(r.filter));
      // MaxScaleDenominator = visible when zoomed IN past minZoom
      if (r.minZoom != null) parts.push(`<MaxScaleDenominator>${zoomToScale(r.minZoom)}</MaxScaleDenominator>`);
      if (r.maxZoom != null) parts.push(`<MinScaleDenominator>${zoomToScale(r.maxZoom)}</MinScaleDenominator>`);
      parts.push(symbolizer(model.geom, r));
      parts.push(labelXml(r));
      return `<Rule>${parts.join("")}</Rule>`;
    })
    .join("");
  return (
    `<?xml version="1.0" encoding="UTF-8"?>` +
    `<StyledLayerDescriptor version="1.0.0" xmlns="http://www.opengis.net/sld" ` +
    `xmlns:ogc="http://www.opengis.net/ogc" xmlns:xlink="http://www.w3.org/1999/xlink">` +
    `<NamedLayer><Name>${esc(name)}</Name><UserStyle><Title>${esc(name)}</Title>` +
    `<FeatureTypeStyle>${rulesXml}</FeatureTypeStyle></UserStyle></NamedLayer></StyledLayerDescriptor>`
  );
}
