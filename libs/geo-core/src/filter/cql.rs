//! ECQL parser: hand-rolled lexer + recursive descent. Ported from the Go
//! `libs/ogc-kit/filter/cql.go` — grammar and WKT capture are identical.

use super::ast::{Expr, Lit};

#[derive(Debug, Clone, PartialEq)]
enum Kind {
    Eof,
    Ident,
    Str,
    Number,
    Punct,
    Op,
}

#[derive(Debug, Clone)]
struct Token {
    kind: Kind,
    text: String,
    up: String,
}

struct Lexer {
    src: Vec<char>,
    pos: usize,
}

fn is_ident_start(c: char) -> bool {
    c == '_' || c.is_ascii_alphabetic()
}
fn is_ident_part(c: char) -> bool {
    is_ident_start(c) || c.is_ascii_digit() || c == ':' || c == '.'
}

fn is_spatial_op(up: &str) -> bool {
    matches!(
        up,
        "INTERSECTS"
            | "WITHIN"
            | "CONTAINS"
            | "DISJOINT"
            | "TOUCHES"
            | "CROSSES"
            | "OVERLAPS"
            | "EQUALS"
    )
}
fn is_geom_keyword(up: &str) -> bool {
    matches!(
        up,
        "POINT"
            | "LINESTRING"
            | "POLYGON"
            | "MULTIPOINT"
            | "MULTILINESTRING"
            | "MULTIPOLYGON"
            | "GEOMETRYCOLLECTION"
    )
}

impl Lexer {
    fn new(s: &str) -> Lexer {
        Lexer {
            src: s.chars().collect(),
            pos: 0,
        }
    }

    fn peek(&self) -> Option<char> {
        self.src.get(self.pos).copied()
    }

    fn next_token(&mut self) -> Result<Token, String> {
        while let Some(c) = self.peek() {
            if c == ' ' || c == '\t' || c == '\n' || c == '\r' {
                self.pos += 1;
            } else {
                break;
            }
        }
        let c = match self.peek() {
            None => {
                return Ok(Token {
                    kind: Kind::Eof,
                    text: String::new(),
                    up: String::new(),
                })
            }
            Some(c) => c,
        };
        match c {
            '(' | ')' | ',' => {
                self.pos += 1;
                Ok(punct(c))
            }
            '=' => {
                self.pos += 1;
                Ok(op("="))
            }
            '<' => {
                self.pos += 1;
                match self.peek() {
                    Some('>') => {
                        self.pos += 1;
                        Ok(op("<>"))
                    }
                    Some('=') => {
                        self.pos += 1;
                        Ok(op("<="))
                    }
                    _ => Ok(op("<")),
                }
            }
            '>' => {
                self.pos += 1;
                match self.peek() {
                    Some('=') => {
                        self.pos += 1;
                        Ok(op(">="))
                    }
                    _ => Ok(op(">")),
                }
            }
            '\'' => self.lex_string(),
            c if c.is_ascii_digit() || c == '-' || c == '+' || c == '.' => self.lex_number(),
            c if is_ident_start(c) => Ok(self.lex_ident()),
            _ => Err(format!("unexpected character {c:?}")),
        }
    }

    fn lex_string(&mut self) -> Result<Token, String> {
        self.pos += 1;
        let mut b = String::new();
        while let Some(c) = self.peek() {
            if c == '\'' {
                if self.src.get(self.pos + 1) == Some(&'\'') {
                    b.push('\'');
                    self.pos += 2;
                    continue;
                }
                self.pos += 1;
                return Ok(Token {
                    kind: Kind::Str,
                    text: b,
                    up: String::new(),
                });
            }
            b.push(c);
            self.pos += 1;
        }
        Err("unterminated string literal".into())
    }

    fn lex_number(&mut self) -> Result<Token, String> {
        let start = self.pos;
        if matches!(self.peek(), Some('-') | Some('+')) {
            self.pos += 1;
        }
        let mut digits = false;
        while let Some(c) = self.peek() {
            if c.is_ascii_digit() {
                digits = true;
                self.pos += 1;
            } else if c == '.' || c == 'e' || c == 'E' || c == '-' || c == '+' {
                self.pos += 1;
            } else {
                break;
            }
        }
        if !digits {
            return Err("invalid number".into());
        }
        let text: String = self.src[start..self.pos].iter().collect();
        Ok(Token {
            kind: Kind::Number,
            text,
            up: String::new(),
        })
    }

    fn lex_ident(&mut self) -> Token {
        let start = self.pos;
        while let Some(c) = self.peek() {
            if is_ident_part(c) {
                self.pos += 1;
            } else {
                break;
            }
        }
        let text: String = self.src[start..self.pos].iter().collect();
        let up = text.to_uppercase();
        Token {
            kind: Kind::Ident,
            text,
            up,
        }
    }
}

fn punct(c: char) -> Token {
    Token {
        kind: Kind::Punct,
        text: c.to_string(),
        up: String::new(),
    }
}
fn op(s: &str) -> Token {
    Token {
        kind: Kind::Op,
        text: s.to_string(),
        up: String::new(),
    }
}

fn strip_prefix(name: &str) -> String {
    match name.find(':') {
        Some(i) => name[i + 1..].to_string(),
        None => name.to_string(),
    }
}

struct Parser {
    lex: Lexer,
    tok: Token,
}

/// parse_cql parses an ECQL filter string into an Expr.
pub fn parse_cql(s: &str) -> Result<Expr, String> {
    let mut lex = Lexer::new(s);
    let tok = lex.next_token()?;
    let mut p = Parser { lex, tok };
    if p.tok.kind == Kind::Eof {
        return Err("empty filter".into());
    }
    let e = p.parse_or()?;
    if p.tok.kind != Kind::Eof {
        return Err(format!("unexpected {:?} at end of filter", p.tok.text));
    }
    Ok(e)
}

impl Parser {
    fn advance(&mut self) -> Result<(), String> {
        self.tok = self.lex.next_token()?;
        Ok(())
    }

    fn expect(&mut self, text: &str) -> Result<(), String> {
        if self.tok.text != text {
            return Err(format!("expected {text:?}, got {:?}", self.tok.text));
        }
        self.advance()
    }

    fn parse_or(&mut self) -> Result<Expr, String> {
        let mut left = self.parse_and()?;
        while self.tok.kind == Kind::Ident && self.tok.up == "OR" {
            self.advance()?;
            let right = self.parse_and()?;
            left = Expr::Logic {
                op: "OR".into(),
                exprs: vec![left, right],
            };
        }
        Ok(left)
    }

    fn parse_and(&mut self) -> Result<Expr, String> {
        let mut left = self.parse_unary()?;
        while self.tok.kind == Kind::Ident && self.tok.up == "AND" {
            self.advance()?;
            let right = self.parse_unary()?;
            left = Expr::Logic {
                op: "AND".into(),
                exprs: vec![left, right],
            };
        }
        Ok(left)
    }

    fn parse_unary(&mut self) -> Result<Expr, String> {
        if self.tok.kind == Kind::Ident && self.tok.up == "NOT" {
            self.advance()?;
            let e = self.parse_unary()?;
            return Ok(Expr::Not(Box::new(e)));
        }
        self.parse_primary()
    }

    fn parse_primary(&mut self) -> Result<Expr, String> {
        if self.tok.kind == Kind::Punct && self.tok.text == "(" {
            self.advance()?;
            let e = self.parse_or()?;
            if !(self.tok.kind == Kind::Punct && self.tok.text == ")") {
                return Err("expected )".into());
            }
            self.advance()?;
            return Ok(e);
        }
        if self.tok.kind != Kind::Ident {
            return Err(format!("expected identifier, got {:?}", self.tok.text));
        }
        let up = self.tok.up.clone();
        match up.as_str() {
            "INCLUDE" => {
                self.advance()?;
                Ok(Expr::Include)
            }
            "EXCLUDE" => {
                self.advance()?;
                Ok(Expr::Not(Box::new(Expr::Include)))
            }
            "BBOX" => self.parse_bbox(),
            _ if is_spatial_op(&up) => self.parse_spatial(&up),
            "DWITHIN" => self.parse_dwithin(false),
            "BEYOND" => self.parse_dwithin(true),
            _ => self.parse_predicate(),
        }
    }

    fn parse_predicate(&mut self) -> Result<Expr, String> {
        let prop_name = strip_prefix(&self.tok.text);
        let prop = Expr::Property(prop_name.clone());
        self.advance()?;

        if self.tok.kind == Kind::Op {
            let op = self.tok.text.clone();
            self.advance()?;
            let val = self.parse_value()?;
            return Ok(Expr::Compare {
                op,
                left: Box::new(prop),
                right: Box::new(val),
            });
        }

        if self.tok.kind != Kind::Ident {
            return Err(format!("expected operator after {prop_name:?}"));
        }
        let mut negate = false;
        if self.tok.up == "NOT" {
            negate = true;
            self.advance()?;
            if self.tok.kind != Kind::Ident {
                return Err("expected LIKE/IN/BETWEEN after NOT".into());
            }
        }
        match self.tok.up.as_str() {
            "LIKE" | "ILIKE" => {
                let ci = self.tok.up == "ILIKE";
                self.advance()?;
                if self.tok.kind != Kind::Str {
                    return Err("LIKE requires a string pattern".into());
                }
                let pat = self.tok.text.clone();
                self.advance()?;
                Ok(Expr::Like {
                    prop: Box::new(prop),
                    pattern: pat,
                    ci,
                    negate,
                })
            }
            "IN" => {
                self.advance()?;
                if !(self.tok.kind == Kind::Punct && self.tok.text == "(") {
                    return Err("IN requires (".into());
                }
                let mut vals = Vec::new();
                loop {
                    self.advance()?;
                    let v = self.parse_value()?;
                    vals.push(v);
                    if self.tok.kind == Kind::Punct && self.tok.text == "," {
                        continue;
                    }
                    break;
                }
                if !(self.tok.kind == Kind::Punct && self.tok.text == ")") {
                    return Err("IN requires )".into());
                }
                self.advance()?;
                Ok(Expr::In {
                    prop: Box::new(prop),
                    values: vals,
                    negate,
                })
            }
            "BETWEEN" => {
                self.advance()?;
                let lo = self.parse_value()?;
                if !(self.tok.kind == Kind::Ident && self.tok.up == "AND") {
                    return Err("BETWEEN requires AND".into());
                }
                self.advance()?;
                let hi = self.parse_value()?;
                Ok(Expr::Between {
                    prop: Box::new(prop),
                    lo: Box::new(lo),
                    hi: Box::new(hi),
                    negate,
                })
            }
            "IS" => {
                if negate {
                    return Err("unexpected NOT before IS".into());
                }
                self.advance()?;
                let mut is_neg = false;
                if self.tok.kind == Kind::Ident && self.tok.up == "NOT" {
                    is_neg = true;
                    self.advance()?;
                }
                if !(self.tok.kind == Kind::Ident && self.tok.up == "NULL") {
                    return Err("expected NULL".into());
                }
                self.advance()?;
                Ok(Expr::IsNull {
                    prop: Box::new(prop),
                    negate: is_neg,
                })
            }
            _ => Err(format!("unexpected token {:?}", self.tok.text)),
        }
    }

    fn parse_value(&mut self) -> Result<Expr, String> {
        match self.tok.kind {
            Kind::Str => {
                let v = self.tok.text.clone();
                self.advance()?;
                Ok(Expr::Lit(Lit::Str(v)))
            }
            Kind::Number => {
                let f: f64 = self
                    .tok
                    .text
                    .parse()
                    .map_err(|_| "invalid number".to_string())?;
                self.advance()?;
                Ok(Expr::Lit(Lit::Num(f)))
            }
            Kind::Ident => {
                let up = self.tok.up.clone();
                if up == "TRUE" || up == "FALSE" {
                    self.advance()?;
                    return Ok(Expr::Lit(Lit::Bool(up == "TRUE")));
                }
                let name = strip_prefix(&self.tok.text);
                self.advance()?;
                Ok(Expr::Property(name))
            }
            _ => Err(format!("expected value, got {:?}", self.tok.text)),
        }
    }

    fn parse_num(&mut self) -> Result<f64, String> {
        if self.tok.kind != Kind::Number {
            return Err(format!("expected number, got {:?}", self.tok.text));
        }
        let f: f64 = self
            .tok
            .text
            .parse()
            .map_err(|_| "invalid number".to_string())?;
        self.advance()?;
        Ok(f)
    }

    fn parse_bbox(&mut self) -> Result<Expr, String> {
        self.advance()?;
        self.expect("(")?;
        if self.tok.kind != Kind::Ident {
            return Err("BBOX requires a property".into());
        }
        let prop = strip_prefix(&self.tok.text);
        self.advance()?;
        let mut nums = [0f64; 4];
        for n in nums.iter_mut() {
            self.expect(",")?;
            *n = self.parse_num()?;
        }
        let mut srs = String::new();
        if self.tok.kind == Kind::Punct && self.tok.text == "," {
            self.advance()?;
            if self.tok.kind != Kind::Str {
                return Err("BBOX srs must be a string".into());
            }
            srs = self.tok.text.clone();
            self.advance()?;
        }
        self.expect(")")?;
        Ok(Expr::BBox {
            prop,
            minx: nums[0],
            miny: nums[1],
            maxx: nums[2],
            maxy: nums[3],
            srs,
        })
    }

    fn parse_wkt(&mut self) -> Result<String, String> {
        if !(self.tok.kind == Kind::Ident && is_geom_keyword(&self.tok.up)) {
            return Err(format!("expected WKT geometry, got {:?}", self.tok.text));
        }
        let mut b = String::new();
        b.push_str(&self.tok.up);
        self.advance()?;
        if !(self.tok.kind == Kind::Punct && self.tok.text == "(") {
            return Err("WKT requires (".into());
        }
        let mut depth = 0i32;
        b.push('(');
        depth += 1;
        self.advance()?;
        let mut first = true;
        while depth > 0 {
            if self.tok.kind == Kind::Eof {
                return Err("unterminated WKT".into());
            }
            if self.tok.kind == Kind::Punct && self.tok.text == "(" {
                b.push('(');
                depth += 1;
                first = true;
            } else if self.tok.kind == Kind::Punct && self.tok.text == ")" {
                b.push(')');
                depth -= 1;
            } else if self.tok.kind == Kind::Punct && self.tok.text == "," {
                b.push_str(", ");
                first = true;
            } else {
                if !first {
                    b.push(' ');
                }
                b.push_str(&self.tok.text);
                first = false;
            }
            if depth == 0 {
                break;
            }
            self.advance()?;
        }
        self.advance()?;
        Ok(b)
    }

    fn parse_spatial(&mut self, op: &str) -> Result<Expr, String> {
        self.advance()?;
        self.expect("(")?;
        if self.tok.kind != Kind::Ident {
            return Err(format!("{op} requires a property"));
        }
        let prop = strip_prefix(&self.tok.text);
        self.advance()?;
        self.expect(",")?;
        let wkt = self.parse_wkt()?;
        self.expect(")")?;
        Ok(Expr::Spatial {
            op: op.to_string(),
            prop,
            wkt,
        })
    }

    fn parse_dwithin(&mut self, beyond: bool) -> Result<Expr, String> {
        self.advance()?;
        self.expect("(")?;
        if self.tok.kind != Kind::Ident {
            return Err("DWITHIN requires a property".into());
        }
        let prop = strip_prefix(&self.tok.text);
        self.advance()?;
        self.expect(",")?;
        let wkt = self.parse_wkt()?;
        self.expect(",")?;
        let dist = self.parse_num()?;
        self.expect(",")?;
        if self.tok.kind != Kind::Ident {
            return Err("DWITHIN requires distance units".into());
        }
        let units = self.tok.text.clone();
        self.advance()?;
        self.expect(")")?;
        Ok(Expr::DWithin {
            prop,
            wkt,
            distance: dist,
            units,
            beyond,
        })
    }
}

#[cfg(test)]
mod tests {
    use super::super::ast::{Expr, Lit};
    use super::parse_cql;

    #[test]
    fn comparisons_and_precedence() {
        let e = parse_cql("a = 1 OR b = 2 AND c = 3").unwrap();
        match e {
            Expr::Logic { op, exprs } => {
                assert_eq!(op, "OR");
                assert!(matches!(&exprs[1], Expr::Logic { op, .. } if op == "AND"));
            }
            _ => panic!("got {e:?}"),
        }
    }

    #[test]
    fn like_in_between_null() {
        assert!(matches!(
            parse_cql("name LIKE 'ro%'").unwrap(),
            Expr::Like {
                ci: false,
                negate: false,
                ..
            }
        ));
        assert!(matches!(
            parse_cql("name ILIKE 'ro%'").unwrap(),
            Expr::Like { ci: true, .. }
        ));
        assert!(matches!(
            parse_cql("t IN ('a','b')").unwrap(),
            Expr::In { .. }
        ));
        assert!(matches!(
            parse_cql("n BETWEEN 1 AND 3").unwrap(),
            Expr::Between { .. }
        ));
        assert!(matches!(
            parse_cql("n IS NOT NULL").unwrap(),
            Expr::IsNull { negate: true, .. }
        ));
        assert!(matches!(parse_cql("INCLUDE").unwrap(), Expr::Include));
        assert!(matches!(
            parse_cql("active = true").unwrap(),
            Expr::Compare { right, .. } if matches!(*right, Expr::Lit(Lit::Bool(true)))
        ));
    }

    #[test]
    fn spatial() {
        assert!(matches!(
            parse_cql("BBOX(geom, -10, -20, 10, 20)").unwrap(),
            Expr::BBox { minx, maxy, .. } if minx == -10.0 && maxy == 20.0
        ));
        assert!(matches!(
            parse_cql("INTERSECTS(geom, POINT(1 2))").unwrap(),
            Expr::Spatial { op, wkt, .. } if op == "INTERSECTS" && wkt == "POINT(1 2)"
        ));
        assert!(matches!(
            parse_cql("DWITHIN(geom, POINT(0 0), 500, meters)").unwrap(),
            Expr::DWithin { distance, .. } if distance == 500.0
        ));
    }

    #[test]
    fn errors() {
        for bad in ["", "name =", "AND", "BBOX(geom,1,2,3)"] {
            assert!(parse_cql(bad).is_err(), "{bad:?} should error");
        }
    }
}
