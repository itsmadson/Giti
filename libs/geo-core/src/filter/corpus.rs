#[cfg(test)]
mod tests {
    use super::super::{parse_cql, to_sql, Arg};
    use serde_json::Value;

    #[test]
    fn golden_corpus_matches_go() {
        let path = concat!(
            env!("CARGO_MANIFEST_DIR"),
            "/../../tests/filter-corpus/corpus.json"
        );
        let raw = std::fs::read_to_string(path).expect("read corpus");
        let cases: Vec<Value> = serde_json::from_str(&raw).unwrap();
        assert!(cases.len() >= 30, "corpus too small: {}", cases.len());
        for c in &cases {
            let cql = c["cql"].as_str().unwrap();
            let want_sql = c["sql"].as_str().unwrap();
            let e = parse_cql(cql).unwrap_or_else(|err| panic!("{cql}: {err}"));
            let (sql, args) = to_sql(&e, 1).unwrap_or_else(|err| panic!("{cql}: {err}"));
            assert_eq!(sql, want_sql, "SQL mismatch for {cql}");
            let want_args = c["args"].as_array().unwrap();
            assert_eq!(args.len(), want_args.len(), "arg count for {cql}");
            for (got, want) in args.iter().zip(want_args) {
                assert!(
                    arg_eq(got, want),
                    "arg mismatch for {cql}: {got:?} vs {want}"
                );
            }
        }
    }

    fn arg_eq(got: &Arg, want: &Value) -> bool {
        match (got, want) {
            (Arg::Str(s), Value::String(w)) => s == w,
            (Arg::Bool(b), Value::Bool(w)) => b == w,
            (Arg::Num(n), Value::Number(w)) => (*n - w.as_f64().unwrap()).abs() < 1e-9,
            _ => false,
        }
    }
}
