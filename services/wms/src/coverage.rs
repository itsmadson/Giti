//! GeoTIFF coverage reader + raster GetMap/GetCoverage rendering.
//! Pure-Rust (tiff crate); reads dimensions, the geotransform (ModelPixelScale +
//! ModelTiepoint), and RGB(A)/grayscale pixels, then samples into an output grid
//! for an arbitrary request bbox (with on-the-fly 4326↔3857 reprojection).

use std::io::Cursor;
use tiff::decoder::{Decoder, DecodingResult};
use tiny_skia::{Pixmap, PremultipliedColorU8};

const MODEL_PIXEL_SCALE: u16 = 33550;
const MODEL_TIEPOINT: u16 = 33922;
const GDAL_NODATA: u16 = 42113;

// read_ascii_tag pulls an ASCII (type 2) tag value from the IFD.
fn read_ascii_tag(b: &[u8], tag: u16) -> Option<String> {
    if b.len() < 8 {
        return None;
    }
    let le = &b[0..2] == b"II";
    let u16a = |o: usize| if le { u16::from_le_bytes([b[o], b[o + 1]]) } else { u16::from_be_bytes([b[o], b[o + 1]]) };
    let u32a = |o: usize| if le {
        u32::from_le_bytes([b[o], b[o + 1], b[o + 2], b[o + 3]])
    } else {
        u32::from_be_bytes([b[o], b[o + 1], b[o + 2], b[o + 3]])
    };
    let ifd = u32a(4) as usize;
    if ifd + 2 > b.len() {
        return None;
    }
    let n = u16a(ifd) as usize;
    for i in 0..n {
        let e = ifd + 2 + i * 12;
        if e + 12 > b.len() {
            break;
        }
        if u16a(e) == tag && u16a(e + 2) == 2 {
            let cnt = u32a(e + 4) as usize;
            let bytes = if cnt <= 4 { &b[e + 8..e + 8 + cnt] } else {
                let off = u32a(e + 8) as usize;
                if off + cnt > b.len() { return None; }
                &b[off..off + cnt]
            };
            return Some(String::from_utf8_lossy(bytes).trim_matches(char::from(0)).trim().to_string());
        }
    }
    None
}

// read_geo_doubles pulls a DOUBLE-typed tag's values straight from the IFD,
// since the tiff crate discards unknown (GeoTIFF) tags.
fn read_geo_doubles(b: &[u8], tag: u16) -> Option<Vec<f64>> {
    if b.len() < 8 {
        return None;
    }
    let le = &b[0..2] == b"II";
    let u16a = |o: usize| if le { u16::from_le_bytes([b[o], b[o + 1]]) } else { u16::from_be_bytes([b[o], b[o + 1]]) };
    let u32a = |o: usize| if le {
        u32::from_le_bytes([b[o], b[o + 1], b[o + 2], b[o + 3]])
    } else {
        u32::from_be_bytes([b[o], b[o + 1], b[o + 2], b[o + 3]])
    };
    let f64a = |o: usize| {
        let mut a = [0u8; 8];
        a.copy_from_slice(&b[o..o + 8]);
        if le { f64::from_le_bytes(a) } else { f64::from_be_bytes(a) }
    };
    let ifd = u32a(4) as usize;
    if ifd + 2 > b.len() {
        return None;
    }
    let n = u16a(ifd) as usize;
    for i in 0..n {
        let e = ifd + 2 + i * 12;
        if e + 12 > b.len() {
            break;
        }
        if u16a(e) == tag && u16a(e + 2) == 12 {
            let cnt = u32a(e + 4) as usize;
            let off = u32a(e + 8) as usize; // >1 double → always an offset
            let mut out = Vec::with_capacity(cnt);
            for k in 0..cnt {
                let p = off + k * 8;
                if p + 8 > b.len() {
                    return None;
                }
                out.push(f64a(p));
            }
            return Some(out);
        }
    }
    None
}

pub struct Coverage {
    pub width: u32,
    pub height: u32,
    ox: f64, // world x of pixel (0,0) upper-left
    oy: f64, // world y of pixel (0,0)
    sx: f64, // pixel size x (world units/px, +)
    sy: f64, // pixel size y (world units/px, +, applied downward)
    samples: usize,
    data: Vec<u8>, // interleaved, row-major
    nodata: Option<u8>,
    pub srid: i32,
}

impl Coverage {
    pub fn open(path: &str, srid: i32) -> Result<Coverage, String> {
        let bytes = std::fs::read(path).map_err(|e| format!("{path}: {e}"))?;
        let scale = read_geo_doubles(&bytes, MODEL_PIXEL_SCALE).ok_or("no ModelPixelScale")?;
        let tie = read_geo_doubles(&bytes, MODEL_TIEPOINT).ok_or("no ModelTiepoint")?;

        let mut dec = Decoder::new(Cursor::new(&bytes)).map_err(|e| e.to_string())?;
        let (width, height) = dec.dimensions().map_err(|e| e.to_string())?;
        if scale.len() < 2 || tie.len() < 6 {
            return Err("incomplete geotransform".into());
        }
        // tiepoint: (i,j,k, x,y,z) maps raster (i,j) → world (x,y)
        let (i, j, wx, wy) = (tie[0], tie[1], tie[3], tie[4]);
        let (sx, sy) = (scale[0], scale[1]);
        let ox = wx - i * sx;
        let oy = wy + j * sy; // world y decreases downward → origin above

        let samples = dec.colortype().map(sample_count).unwrap_or(3);
        let data = match dec.read_image().map_err(|e| e.to_string())? {
            DecodingResult::U8(v) => v,
            DecodingResult::U16(v) => v.iter().map(|&x| (x >> 8) as u8).collect(),
            _ => return Err("unsupported pixel type".into()),
        };

        let nodata = read_ascii_tag(&bytes, GDAL_NODATA)
            .and_then(|s| s.parse::<f64>().ok())
            .map(|v| v.round().clamp(0.0, 255.0) as u8);

        Ok(Coverage { width, height, ox, oy, sx, sy, samples, data, nodata, srid })
    }

    /// bounds returns the coverage extent [minx,miny,maxx,maxy] in its SRS.
    pub fn bounds(&self) -> [f64; 4] {
        let maxx = self.ox + self.width as f64 * self.sx;
        let miny = self.oy - self.height as f64 * self.sy;
        [self.ox, miny, maxx, self.oy]
    }

    fn sample(&self, col: i64, row: i64) -> Option<[u8; 4]> {
        if col < 0 || row < 0 || col as u32 >= self.width || row as u32 >= self.height {
            return None;
        }
        let idx = (row as usize * self.width as usize + col as usize) * self.samples;
        let d = &self.data;
        let px = match self.samples {
            1 => d.get(idx).map(|&g| [g, g, g, 255]),
            3 => d.get(idx + 2).map(|_| [d[idx], d[idx + 1], d[idx + 2], 255]),
            4 => d.get(idx + 3).map(|_| [d[idx], d[idx + 1], d[idx + 2], d[idx + 3]]),
            _ => None,
        }?;
        // GDAL nodata → transparent
        if let Some(nd) = self.nodata {
            if px[0] == nd && (self.samples < 3 || (px[1] == nd && px[2] == nd)) {
                return None;
            }
        }
        Some(px)
    }

    /// render samples the coverage into an out_w × out_h image for a request
    /// bbox [minx,miny,maxx,maxy] in EPSG:req_srid.
    pub fn render(&self, bbox: [f64; 4], req_srid: i32, out_w: u32, out_h: u32) -> Pixmap {
        let mut px = Pixmap::new(out_w.max(1), out_h.max(1)).unwrap();
        let pixels = px.pixels_mut();
        let (bw, bh) = (bbox[2] - bbox[0], bbox[3] - bbox[1]);
        for oy in 0..out_h {
            for ox in 0..out_w {
                // output pixel → world (request SRS)
                let wx_req = bbox[0] + (ox as f64 + 0.5) / out_w as f64 * bw;
                let wy_req = bbox[3] - (oy as f64 + 0.5) / out_h as f64 * bh;
                // reproject to the coverage's native SRS
                let (wx, wy) = reproject(wx_req, wy_req, req_srid, self.srid);
                let col = ((wx - self.ox) / self.sx).floor() as i64;
                let row = ((self.oy - wy) / self.sy).floor() as i64;
                if let Some(c) = self.sample(col, row) {
                    let i = (oy * out_w + ox) as usize;
                    let a = c[3];
                    pixels[i] = PremultipliedColorU8::from_rgba(
                        (c[0] as u16 * a as u16 / 255) as u8,
                        (c[1] as u16 * a as u16 / 255) as u8,
                        (c[2] as u16 * a as u16 / 255) as u8,
                        a,
                    )
                    .unwrap_or(pixels[i]);
                }
            }
        }
        px
    }
}

fn sample_count(ct: tiff::ColorType) -> usize {
    use tiff::ColorType::*;
    match ct {
        Gray(_) => 1,
        RGB(_) => 3,
        RGBA(_) => 4,
        _ => 3,
    }
}

// Web-Mercator ↔ WGS84 (metres ↔ degrees). Identity when srids match.
const R: f64 = 6_378_137.0;
fn reproject(x: f64, y: f64, from: i32, to: i32) -> (f64, f64) {
    if from == to {
        return (x, y);
    }
    match (from, to) {
        (3857, 4326) => {
            let lon = x / R * 180.0 / std::f64::consts::PI;
            let lat = (2.0 * (y / R).exp().atan() - std::f64::consts::FRAC_PI_2) * 180.0
                / std::f64::consts::PI;
            (lon, lat)
        }
        (4326, 3857) => {
            let mx = x * std::f64::consts::PI / 180.0 * R;
            let my = ((90.0 + y) * std::f64::consts::PI / 360.0).tan().ln() * R;
            (mx, my)
        }
        _ => (x, y),
    }
}
