//! OGC Filter Encoding (FE 1.0 / 1.1) XML → geo_core::Expr.
//! Covers the comparison, logical, Like/Between/Null, and BBOX predicates used
//! by the WMS FILTER vendor parameter. Arbitrary GML-geometry spatial ops
//! (Within/Intersects with a polygon) are not yet supported.

use geo_core::filter::{Expr, Lit};
use roxmltree::{Document, Node};

fn local<'a>(n: Node<'a, 'a>, name: &str) -> Option<Node<'a, 'a>> {
    n.children().find(|c| c.is_element() && c.tag_name().name() == name)
}

fn first_element<'a>(n: Node<'a, 'a>) -> Option<Node<'a, 'a>> {
    n.children().find(|c| c.is_element())
}

fn text_of(n: Node) -> String {
    n.text().unwrap_or("").trim().to_string()
}

fn to_lit(s: &str) -> Expr {
    match s.parse::<f64>() {
        Ok(v) => Expr::Lit(Lit::Num(v)),
        Err(_) => Expr::Lit(Lit::Str(s.to_string())),
    }
}

// property + literal children of a binary comparison
fn prop_and_value(n: Node) -> Option<(Expr, Expr)> {
    let prop = local(n, "PropertyName").or_else(|| local(n, "ValueReference"))?;
    let lit = local(n, "Literal")?;
    Some((Expr::Property(text_of(prop)), to_lit(&text_of(lit))))
}

fn compare(n: Node, op: &str) -> Option<Expr> {
    let (l, r) = prop_and_value(n)?;
    Some(Expr::Compare { op: op.into(), left: Box::new(l), right: Box::new(r) })
}

// gml:Box/gml:Envelope → [minx,miny,maxx,maxy]
fn parse_envelope(n: Node) -> Option<[f64; 4]> {
    // gml:Envelope with lowerCorner/upperCorner ("x y")
    if let (Some(lc), Some(uc)) = (local(n, "lowerCorner"), local(n, "upperCorner")) {
        let lo: Vec<f64> = text_of(lc).split_whitespace().filter_map(|s| s.parse().ok()).collect();
        let hi: Vec<f64> = text_of(uc).split_whitespace().filter_map(|s| s.parse().ok()).collect();
        if lo.len() == 2 && hi.len() == 2 {
            return Some([lo[0], lo[1], hi[0], hi[1]]);
        }
    }
    // gml:Box with coordinates "minx,miny maxx,maxy"
    if let Some(coords) = local(n, "coordinates") {
        let pts: Vec<f64> = text_of(coords)
            .split(|c| c == ',' || c == ' ')
            .filter_map(|s| s.trim().parse().ok())
            .collect();
        if pts.len() == 4 {
            return Some([pts[0], pts[1], pts[2], pts[3]]);
        }
    }
    None
}

fn parse_bbox(n: Node) -> Option<Expr> {
    let prop = local(n, "PropertyName")
        .or_else(|| local(n, "ValueReference"))
        .map(text_of)
        .unwrap_or_default();
    let geo = n.children().find(|c| {
        c.is_element() && matches!(c.tag_name().name(), "Envelope" | "Box")
    })?;
    let b = parse_envelope(geo)?;
    Some(Expr::BBox { prop, minx: b[0], miny: b[1], maxx: b[2], maxy: b[3], srs: String::new() })
}

fn ogc_like_to_sql(pat: &str, wild: char, single: char) -> String {
    pat.chars()
        .map(|c| {
            if c == wild {
                '%'
            } else if c == single {
                '_'
            } else {
                c
            }
        })
        .collect()
}

fn node_to_expr(n: Node) -> Option<Expr> {
    match n.tag_name().name() {
        "PropertyIsEqualTo" => compare(n, "="),
        "PropertyIsNotEqualTo" => compare(n, "<>"),
        "PropertyIsGreaterThan" => compare(n, ">"),
        "PropertyIsGreaterThanOrEqualTo" => compare(n, ">="),
        "PropertyIsLessThan" => compare(n, "<"),
        "PropertyIsLessThanOrEqualTo" => compare(n, "<="),
        "PropertyIsLike" => {
            let prop = local(n, "PropertyName").or_else(|| local(n, "ValueReference"))?;
            let lit = local(n, "Literal")?;
            let wild = n.attribute("wildCard").and_then(|s| s.chars().next()).unwrap_or('*');
            let single = n.attribute("singleChar").and_then(|s| s.chars().next()).unwrap_or('.');
            Some(Expr::Like {
                prop: Box::new(Expr::Property(text_of(prop))),
                pattern: ogc_like_to_sql(&text_of(lit), wild, single),
                ci: false,
                negate: false,
            })
        }
        "PropertyIsBetween" => {
            let prop = local(n, "PropertyName").or_else(|| local(n, "ValueReference"))?;
            let lo = local(n, "LowerBoundary").and_then(first_element).map(text_of)?;
            let hi = local(n, "UpperBoundary").and_then(first_element).map(text_of)?;
            Some(Expr::Between {
                prop: Box::new(Expr::Property(text_of(prop))),
                lo: Box::new(to_lit(&lo)),
                hi: Box::new(to_lit(&hi)),
                negate: false,
            })
        }
        "PropertyIsNull" => {
            let prop = local(n, "PropertyName").or_else(|| local(n, "ValueReference"))?;
            Some(Expr::IsNull { prop: Box::new(Expr::Property(text_of(prop))), negate: false })
        }
        "And" | "Or" => {
            let op = if n.tag_name().name() == "And" { "AND" } else { "OR" };
            let exprs: Vec<Expr> = n.children().filter(|c| c.is_element()).filter_map(node_to_expr).collect();
            if exprs.is_empty() {
                None
            } else {
                Some(Expr::Logic { op: op.into(), exprs })
            }
        }
        "Not" => first_element(n).and_then(node_to_expr).map(|e| Expr::Not(Box::new(e))),
        "BBOX" => parse_bbox(n),
        "Filter" => first_element(n).and_then(node_to_expr),
        _ => None,
    }
}

/// parse_filter_xml parses an OGC Filter document into an Expr.
pub fn parse_filter_xml(xml: &str) -> Result<Expr, String> {
    let doc = Document::parse(xml).map_err(|e| e.to_string())?;
    let root = doc.root_element();
    let start = if root.tag_name().name() == "Filter" {
        first_element(root)
    } else {
        Some(root)
    };
    start
        .and_then(node_to_expr)
        .ok_or_else(|| "unsupported or empty filter".to_string())
}
