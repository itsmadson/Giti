//! SLD 1.0 / 1.1 parser → Style model. Matches local element names so both
//! `sld:` (1.0) and `se:` (1.1) prefixes work.

use roxmltree::{Document, Node};

#[derive(Debug, Clone, Default)]
pub struct Style {
    pub rules: Vec<Rule>,
}

#[derive(Debug, Clone, Copy, PartialEq)]
pub enum CmpOp {
    Gt,
    Ge,
    Lt,
    Le,
    Eq,
    Ne,
    Like,
}

#[derive(Debug, Clone)]
pub struct RuleFilter {
    pub prop: String,
    pub op: CmpOp,
    pub value: String,
}

impl RuleFilter {
    /// matches evaluates the comparison against a feature's string attributes.
    pub fn matches(&self, attrs: &std::collections::HashMap<String, String>) -> bool {
        let v = match attrs.get(&self.prop) {
            Some(v) => v.as_str(),
            None => return false,
        };
        if let (Ok(a), Ok(b)) = (v.parse::<f64>(), self.value.parse::<f64>()) {
            return match self.op {
                CmpOp::Gt => a > b,
                CmpOp::Ge => a >= b,
                CmpOp::Lt => a < b,
                CmpOp::Le => a <= b,
                CmpOp::Eq => (a - b).abs() < f64::EPSILON,
                CmpOp::Ne => (a - b).abs() >= f64::EPSILON,
                CmpOp::Like => v.contains(&self.value),
            };
        }
        match self.op {
            CmpOp::Eq => v == self.value,
            CmpOp::Ne => v != self.value,
            CmpOp::Like => v.to_lowercase().contains(&self.value.to_lowercase().replace('*', "")),
            CmpOp::Gt => v > self.value.as_str(),
            CmpOp::Ge => v >= self.value.as_str(),
            CmpOp::Lt => v < self.value.as_str(),
            CmpOp::Le => v <= self.value.as_str(),
        }
    }
}

#[derive(Debug, Clone, Default)]
pub struct Rule {
    pub min_scale: Option<f64>,
    pub max_scale: Option<f64>,
    pub filter: Option<RuleFilter>,
    pub symbolizers: Vec<Symbolizer>,
}

// parse_rule_filter reads the first comparison inside a Rule's <ogc:Filter>.
fn parse_rule_filter(rule_node: Node) -> Option<RuleFilter> {
    let filt = child(rule_node, "Filter")?;
    for c in filt.children() {
        let op = match c.tag_name().name() {
            "PropertyIsGreaterThan" => CmpOp::Gt,
            "PropertyIsGreaterThanOrEqualTo" => CmpOp::Ge,
            "PropertyIsLessThan" => CmpOp::Lt,
            "PropertyIsLessThanOrEqualTo" => CmpOp::Le,
            "PropertyIsEqualTo" => CmpOp::Eq,
            "PropertyIsNotEqualTo" => CmpOp::Ne,
            "PropertyIsLike" => CmpOp::Like,
            _ => continue,
        };
        let prop = child(c, "PropertyName").and_then(|p| p.text()).unwrap_or("").trim().to_string();
        let value = child(c, "Literal").and_then(|p| p.text()).unwrap_or("").trim().to_string();
        if !prop.is_empty() {
            return Some(RuleFilter { prop, op, value });
        }
    }
    None
}

#[derive(Debug, Clone)]
pub enum Symbolizer {
    Point {
        fill: [u8; 4],
        size: f32,
        well_known: String,
    },
    Line {
        stroke: [u8; 4],
        width: f32,
    },
    Polygon {
        fill: [u8; 4],
        stroke: [u8; 4],
        stroke_width: f32,
    },
    Text {
        property: String,
        fill: [u8; 4],
        size: f32,
    },
    Raster {
        opacity: f32,
    },
}

fn hex_rgba(hex: &str, alpha: u8) -> [u8; 4] {
    let h = hex.trim_start_matches('#');
    if h.len() >= 6 {
        let r = u8::from_str_radix(&h[0..2], 16).unwrap_or(0);
        let g = u8::from_str_radix(&h[2..4], 16).unwrap_or(0);
        let b = u8::from_str_radix(&h[4..6], 16).unwrap_or(0);
        return [r, g, b, alpha];
    }
    [0, 0, 0, alpha]
}

fn local(n: &Node, name: &str) -> bool {
    n.is_element() && n.tag_name().name() == name
}

fn child<'a, 'input>(n: Node<'a, 'input>, name: &str) -> Option<Node<'a, 'input>> {
    n.children().find(|c| local(c, name))
}

/// css_params collects CssParameter/SvgParameter name→value pairs under a node.
fn css_params(n: Node) -> std::collections::HashMap<String, String> {
    let mut m = std::collections::HashMap::new();
    for c in n.children() {
        let name = c.tag_name().name();
        if name == "CssParameter" || name == "SvgParameter" {
            if let Some(k) = c.attribute("name") {
                m.insert(k.to_string(), c.text().unwrap_or("").trim().to_string());
            }
        }
    }
    m
}

fn parse_fill(n: Node) -> [u8; 4] {
    if let Some(fill) = child(n, "Fill") {
        let p = css_params(fill);
        let alpha = p
            .get("fill-opacity")
            .and_then(|v| v.parse::<f32>().ok())
            .map(|o| (o * 255.0) as u8)
            .unwrap_or(255);
        if let Some(hex) = p.get("fill") {
            return hex_rgba(hex, alpha);
        }
    }
    [128, 128, 128, 255]
}

fn parse_stroke(n: Node) -> ([u8; 4], f32) {
    if let Some(stroke) = child(n, "Stroke") {
        let p = css_params(stroke);
        let alpha = p
            .get("stroke-opacity")
            .and_then(|v| v.parse::<f32>().ok())
            .map(|o| (o * 255.0) as u8)
            .unwrap_or(255);
        let color = p
            .get("stroke")
            .map(|h| hex_rgba(h, alpha))
            .unwrap_or([0, 0, 0, 255]);
        let width = p
            .get("stroke-width")
            .and_then(|v| v.parse::<f32>().ok())
            .unwrap_or(1.0);
        return (color, width);
    }
    ([0, 0, 0, 255], 1.0)
}

fn parse_symbolizer(n: Node) -> Option<Symbolizer> {
    match n.tag_name().name() {
        "PolygonSymbolizer" => {
            let fill = parse_fill(n);
            let (stroke, stroke_width) = parse_stroke(n);
            Some(Symbolizer::Polygon {
                fill,
                stroke,
                stroke_width,
            })
        }
        "LineSymbolizer" => {
            let (stroke, width) = parse_stroke(n);
            Some(Symbolizer::Line { stroke, width })
        }
        "PointSymbolizer" => {
            let graphic = child(n, "Graphic")?;
            let mark = child(graphic, "Mark");
            let fill = mark.map(parse_fill).unwrap_or([255, 0, 0, 255]);
            let well_known = mark
                .and_then(|m| child(m, "WellKnownName"))
                .and_then(|w| w.text())
                .unwrap_or("circle")
                .to_string();
            let size = child(graphic, "Size")
                .and_then(|s| s.text())
                .and_then(|t| t.trim().parse().ok())
                .unwrap_or(6.0);
            Some(Symbolizer::Point {
                fill,
                size,
                well_known,
            })
        }
        "TextSymbolizer" => {
            let property = child(n, "Label")
                .and_then(|l| child(l, "PropertyName").or(Some(l)))
                .and_then(|p| p.text())
                .unwrap_or("")
                .trim()
                .to_string();
            let fill = parse_fill(n);
            let size = child(n, "Font")
                .map(css_params)
                .and_then(|p| p.get("font-size").and_then(|v| v.parse().ok()))
                .unwrap_or(12.0);
            Some(Symbolizer::Text {
                property,
                fill,
                size,
            })
        }
        "RasterSymbolizer" => {
            let opacity = child(n, "Opacity")
                .and_then(|o| o.text())
                .and_then(|t| t.trim().parse().ok())
                .unwrap_or(1.0);
            Some(Symbolizer::Raster { opacity })
        }
        _ => None,
    }
}

/// parse_sld reads an SLD document into a Style.
pub fn parse_sld(xml: &str) -> Result<Style, String> {
    let doc = Document::parse(xml).map_err(|e| e.to_string())?;
    let mut style = Style::default();
    for rule_node in doc.descendants().filter(|n| local(n, "Rule")) {
        let mut rule = Rule::default();
        if let Some(mn) = child(rule_node, "MinScaleDenominator") {
            rule.min_scale = mn.text().and_then(|t| t.trim().parse().ok());
        }
        if let Some(mx) = child(rule_node, "MaxScaleDenominator") {
            rule.max_scale = mx.text().and_then(|t| t.trim().parse().ok());
        }
        rule.filter = parse_rule_filter(rule_node);
        for c in rule_node.children() {
            if let Some(sym) = parse_symbolizer(c) {
                rule.symbolizers.push(sym);
            }
        }
        if !rule.symbolizers.is_empty() {
            style.rules.push(rule);
        }
    }
    Ok(style)
}

/// default_style_for returns a fallback style per geometry type.
pub fn default_style_for(geom_type: &str) -> Style {
    let g = geom_type.to_lowercase();
    let sym = if g.contains("point") {
        Symbolizer::Point {
            fill: [255, 0, 0, 255],
            size: 6.0,
            well_known: "circle".into(),
        }
    } else if g.contains("line") {
        Symbolizer::Line {
            stroke: [0, 0, 255, 255],
            width: 1.0,
        }
    } else {
        Symbolizer::Polygon {
            fill: [170, 170, 170, 255],
            stroke: [0, 0, 0, 255],
            stroke_width: 1.0,
        }
    };
    Style {
        rules: vec![Rule {
            symbolizers: vec![sym],
            ..Default::default()
        }],
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    const POLY_SLD: &str = r#"<StyledLayerDescriptor xmlns="http://www.opengis.net/sld">
      <NamedLayer><UserStyle><FeatureTypeStyle><Rule>
        <PolygonSymbolizer>
          <Fill><CssParameter name="fill">#FF0000</CssParameter></Fill>
          <Stroke><CssParameter name="stroke">#000000</CssParameter>
                  <CssParameter name="stroke-width">2</CssParameter></Stroke>
        </PolygonSymbolizer>
      </Rule></FeatureTypeStyle></UserStyle></NamedLayer></StyledLayerDescriptor>"#;

    #[test]
    fn parse_polygon() {
        let s = parse_sld(POLY_SLD).unwrap();
        assert_eq!(s.rules.len(), 1);
        match &s.rules[0].symbolizers[0] {
            Symbolizer::Polygon {
                fill, stroke_width, ..
            } => {
                assert_eq!(*fill, [0xFF, 0, 0, 0xFF]);
                assert_eq!(*stroke_width, 2.0);
            }
            other => panic!("got {other:?}"),
        }
    }

    #[test]
    fn parse_point_and_text() {
        let sld = r#"<StyledLayerDescriptor xmlns="http://www.opengis.net/sld">
          <NamedLayer><UserStyle><FeatureTypeStyle><Rule>
            <PointSymbolizer><Graphic><Mark><WellKnownName>circle</WellKnownName>
              <Fill><CssParameter name="fill">#00FF00</CssParameter></Fill></Mark>
              <Size>8</Size></Graphic></PointSymbolizer>
            <TextSymbolizer><Label><PropertyName>name</PropertyName></Label>
              <Font><CssParameter name="font-size">12</CssParameter></Font></TextSymbolizer>
          </Rule></FeatureTypeStyle></UserStyle></NamedLayer></StyledLayerDescriptor>"#;
        let s = parse_sld(sld).unwrap();
        assert!(s.rules[0]
            .symbolizers
            .iter()
            .any(|s| matches!(s, Symbolizer::Point { size, .. } if *size == 8.0)));
        assert!(s.rules[0]
            .symbolizers
            .iter()
            .any(|s| matches!(s, Symbolizer::Text { property, .. } if property == "name")));
    }

    #[test]
    fn default_fallbacks() {
        assert!(!default_style_for("Polygon").rules.is_empty());
        assert!(!default_style_for("Point").rules.is_empty());
    }
}
