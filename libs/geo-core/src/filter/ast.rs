//! Filter AST — mirrors the Go `libs/ogc-kit/filter` AST so CQL behaves
//! identically across services.

#[derive(Debug, Clone, PartialEq)]
pub enum Lit {
    Str(String),
    Num(f64),
    Bool(bool),
}

#[derive(Debug, Clone, PartialEq)]
pub enum Expr {
    Property(String),
    Lit(Lit),
    Compare {
        op: String,
        left: Box<Expr>,
        right: Box<Expr>,
    },
    Logic {
        op: String,
        exprs: Vec<Expr>,
    },
    Not(Box<Expr>),
    Like {
        prop: Box<Expr>,
        pattern: String,
        ci: bool,
        negate: bool,
    },
    In {
        prop: Box<Expr>,
        values: Vec<Expr>,
        negate: bool,
    },
    Between {
        prop: Box<Expr>,
        lo: Box<Expr>,
        hi: Box<Expr>,
        negate: bool,
    },
    IsNull {
        prop: Box<Expr>,
        negate: bool,
    },
    BBox {
        prop: String,
        minx: f64,
        miny: f64,
        maxx: f64,
        maxy: f64,
        srs: String,
    },
    Spatial {
        op: String,
        prop: String,
        wkt: String,
    },
    DWithin {
        prop: String,
        wkt: String,
        distance: f64,
        units: String,
        beyond: bool,
    },
    Include,
}
