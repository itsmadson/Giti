//! OGC filter parsing (CQL) and PostGIS SQL compilation. Kept byte-identical to
//! the Go `libs/ogc-kit/filter` implementation via a shared golden corpus.

pub mod ast;
pub mod cql;
pub mod sql;

#[cfg(test)]
mod corpus;

pub use ast::{Expr, Lit};
pub use cql::parse_cql;
pub use sql::{to_sql, Arg};
