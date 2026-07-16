//! Expr -> parameterized PostGIS SQL. Ported from Go `libs/ogc-kit/filter/sql.go`
//! — output must be byte-identical (proven by the shared golden corpus).

use super::ast::{Expr, Lit};

#[derive(Debug, Clone, PartialEq)]
pub enum Arg {
    Str(String),
    Num(f64),
    Bool(bool),
}

fn valid_ident(name: &str) -> bool {
    let mut chars = name.chars();
    match chars.next() {
        Some(c) if c == '_' || c.is_ascii_alphabetic() => {}
        _ => return false,
    }
    chars.all(|c| c == '_' || c.is_ascii_alphanumeric())
}

fn quote_ident(name: &str) -> Result<String, String> {
    if !valid_ident(name) {
        return Err(format!("invalid identifier {name:?}"));
    }
    Ok(format!("\"{name}\""))
}

struct Ctr {
    n: usize,
    args: Vec<Arg>,
}

impl Ctr {
    fn placeholder(&mut self, v: Arg) -> String {
        self.args.push(v);
        let p = format!("${}", self.n);
        self.n += 1;
        p
    }
}

fn lit_arg(e: &Expr) -> Result<Arg, String> {
    match e {
        Expr::Lit(Lit::Str(s)) => Ok(Arg::Str(s.clone())),
        Expr::Lit(Lit::Num(n)) => Ok(Arg::Num(*n)),
        Expr::Lit(Lit::Bool(b)) => Ok(Arg::Bool(*b)),
        _ => Err("expected literal value".into()),
    }
}

fn st_spatial(op: &str) -> String {
    // title-case: INTERSECTS -> St_Intersects? No — Go uses strings.Title(lower) =
    // "Intersects" -> "ST_Intersects". Match that exactly.
    let lower = op.to_lowercase();
    let mut c = lower.chars();
    let title = match c.next() {
        Some(f) => f.to_uppercase().collect::<String>() + c.as_str(),
        None => lower,
    };
    format!("ST_{title}")
}

/// to_sql compiles an Expr to a parameterized WHERE fragment. `start_arg` is the
/// first placeholder index (1 -> $1).
pub fn to_sql(e: &Expr, start_arg: usize) -> Result<(String, Vec<Arg>), String> {
    let mut ctr = Ctr {
        n: start_arg,
        args: Vec::new(),
    };
    let sql = emit(e, &mut ctr)?;
    Ok((sql, ctr.args))
}

fn emit(e: &Expr, ctr: &mut Ctr) -> Result<String, String> {
    match e {
        Expr::Include => Ok("TRUE".into()),
        Expr::Property(name) => quote_ident(name),
        Expr::Compare { op, left, right } => {
            let l = emit(left, ctr)?;
            match lit_arg(right) {
                Ok(v) => Ok(format!("{l} {op} {}", ctr.placeholder(v))),
                Err(_) => {
                    let r = emit(right, ctr)?;
                    Ok(format!("{l} {op} {r}"))
                }
            }
        }
        Expr::Logic { op, exprs } => {
            let mut parts = Vec::with_capacity(exprs.len());
            for sub in exprs {
                parts.push(emit(sub, ctr)?);
            }
            Ok(format!("({})", parts.join(&format!(" {op} "))))
        }
        Expr::Not(inner) => {
            let s = emit(inner, ctr)?;
            Ok(format!("NOT ({s})"))
        }
        Expr::Like {
            prop,
            pattern,
            ci,
            negate,
        } => {
            let col = emit(prop, ctr)?;
            let mut op = if *ci { "ILIKE" } else { "LIKE" }.to_string();
            if *negate {
                op = format!("NOT {op}");
            }
            Ok(format!(
                "{col} {op} {}",
                ctr.placeholder(Arg::Str(pattern.clone()))
            ))
        }
        Expr::In {
            prop,
            values,
            negate,
        } => {
            let col = emit(prop, ctr)?;
            let mut phs = Vec::with_capacity(values.len());
            for v in values {
                phs.push(ctr.placeholder(lit_arg(v)?));
            }
            let op = if *negate { "NOT IN" } else { "IN" };
            Ok(format!("{col} {op} ({})", phs.join(", ")))
        }
        Expr::Between {
            prop,
            lo,
            hi,
            negate,
        } => {
            let col = emit(prop, ctr)?;
            let lo = lit_arg(lo)?;
            let hi = lit_arg(hi)?;
            let op = if *negate { "NOT BETWEEN" } else { "BETWEEN" };
            Ok(format!(
                "{col} {op} {} AND {}",
                ctr.placeholder(lo),
                ctr.placeholder(hi)
            ))
        }
        Expr::IsNull { prop, negate } => {
            let col = emit(prop, ctr)?;
            if *negate {
                Ok(format!("{col} IS NOT NULL"))
            } else {
                Ok(format!("{col} IS NULL"))
            }
        }
        Expr::BBox {
            prop,
            minx,
            miny,
            maxx,
            maxy,
            ..
        } => {
            let col = quote_ident(prop)?;
            Ok(format!(
                "{col} && ST_MakeEnvelope({}, {}, {}, {})",
                ctr.placeholder(Arg::Num(*minx)),
                ctr.placeholder(Arg::Num(*miny)),
                ctr.placeholder(Arg::Num(*maxx)),
                ctr.placeholder(Arg::Num(*maxy))
            ))
        }
        Expr::Spatial { op, prop, wkt } => {
            let col = quote_ident(prop)?;
            Ok(format!(
                "{}({col}, ST_GeomFromText({}))",
                st_spatial(op),
                ctr.placeholder(Arg::Str(wkt.clone()))
            ))
        }
        Expr::DWithin {
            prop,
            wkt,
            distance,
            beyond,
            ..
        } => {
            let col = quote_ident(prop)?;
            let expr = format!(
                "ST_DWithin({col}, ST_GeomFromText({}), {})",
                ctr.placeholder(Arg::Str(wkt.clone())),
                ctr.placeholder(Arg::Num(*distance))
            );
            if *beyond {
                Ok(format!("NOT {expr}"))
            } else {
                Ok(expr)
            }
        }
        _ => Err(format!("cannot compile expression {e:?}")),
    }
}
