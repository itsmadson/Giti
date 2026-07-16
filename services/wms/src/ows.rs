//! WMS OWS layer: KVP parsing, version negotiation, exception rendering.

use axum::http::header::CONTENT_TYPE;
use axum::response::{IntoResponse, Response};
use std::collections::HashMap;

/// Kvp holds request parameters keyed by upper-cased name (case-insensitive).
pub struct Kvp(HashMap<String, String>);

impl Kvp {
    pub fn parse(query: &str) -> Kvp {
        let mut m = HashMap::new();
        for (k, v) in form_urlencoded::parse(query.as_bytes()) {
            m.insert(k.to_uppercase(), v.into_owned());
        }
        Kvp(m)
    }
    pub fn get(&self, key: &str) -> Option<&str> {
        self.0.get(&key.to_uppercase()).map(|s| s.as_str())
    }
    pub fn service(&self) -> String {
        self.get("SERVICE").unwrap_or("").to_uppercase()
    }
    pub fn version(&self) -> String {
        self.get("VERSION").unwrap_or("").to_string()
    }
    pub fn request(&self) -> String {
        self.get("REQUEST").unwrap_or("").to_string()
    }
}

fn cmp_ver(a: &str, b: &str) -> std::cmp::Ordering {
    let pa: Vec<u32> = a.split('.').filter_map(|s| s.parse().ok()).collect();
    let pb: Vec<u32> = b.split('.').filter_map(|s| s.parse().ok()).collect();
    pa.cmp(&pb)
}

/// negotiate_wms picks a supported version: exact, else highest below, else lowest.
pub fn negotiate_wms(requested: &str) -> String {
    let supported = ["1.3.0", "1.1.1"]; // newest first
    if requested.is_empty() {
        return supported[0].to_string();
    }
    for v in supported {
        if cmp_ver(v, requested) != std::cmp::Ordering::Greater {
            return v.to_string();
        }
    }
    supported[supported.len() - 1].to_string()
}

fn xml_escape(s: &str) -> String {
    s.replace('&', "&amp;")
        .replace('<', "&lt;")
        .replace('>', "&gt;")
}

/// exception_response renders a WMS ServiceExceptionReport for the version.
pub fn exception_response(version: &str, code: &str, locator: &str, msg: &str) -> Response {
    let loc = if locator.is_empty() {
        String::new()
    } else {
        format!(" locator=\"{locator}\"")
    };
    let cd = if code.is_empty() {
        String::new()
    } else {
        format!(" code=\"{code}\"")
    };
    if version == "1.3.0" {
        let body = format!(
            r#"<?xml version="1.0" encoding="UTF-8" standalone="no"?>
<ServiceExceptionReport version="1.3.0" xmlns="http://www.opengis.net/ogc" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:schemaLocation="http://www.opengis.net/ogc http://schemas.opengis.net/wms/1.3.0/exceptions_1_3_0.xsd">
  <ServiceException{cd}{loc}>{}</ServiceException>
</ServiceExceptionReport>
"#,
            xml_escape(msg)
        );
        return ([(CONTENT_TYPE, "text/xml")], body).into_response();
    }
    let body = format!(
        r#"<?xml version="1.0" encoding="UTF-8" standalone="no"?>
<!DOCTYPE ServiceExceptionReport SYSTEM "http://schemas.opengis.net/wms/1.1.1/WMS_exception_1_1_1.dtd">
<ServiceExceptionReport version="1.1.1">
  <ServiceException{cd}{loc}>{}</ServiceException>
</ServiceExceptionReport>
"#,
        xml_escape(msg)
    );
    ([(CONTENT_TYPE, "application/vnd.ogc.se_xml")], body).into_response()
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn kvp_case_insensitive() {
        let k = Kvp::parse("service=wms&VeRsIoN=1.3.0&ReQuEsT=GetMap&LAYERS=topp:roads");
        assert_eq!(k.service(), "WMS");
        assert_eq!(k.version(), "1.3.0");
        assert_eq!(k.request(), "GetMap");
        assert_eq!(k.get("layers"), Some("topp:roads"));
    }

    #[test]
    fn negotiate() {
        assert_eq!(negotiate_wms(""), "1.3.0");
        assert_eq!(negotiate_wms("1.1.1"), "1.1.1");
        assert_eq!(negotiate_wms("1.3.0"), "1.3.0");
        assert_eq!(negotiate_wms("1.2.0"), "1.1.1");
    }
}
