//! Pixmap → PNG / JPEG / WebP encoders.

use image::{ImageEncoder, RgbaImage};
use tiny_skia::Pixmap;

fn to_rgba(px: &Pixmap) -> RgbaImage {
    // tiny-skia stores premultiplied RGBA; demultiply for encoders.
    let mut img = RgbaImage::new(px.width(), px.height());
    for (i, p) in px.pixels().iter().enumerate() {
        let x = (i as u32) % px.width();
        let y = (i as u32) / px.width();
        img.put_pixel(x, y, image::Rgba([p.red(), p.green(), p.blue(), p.alpha()]));
    }
    img
}

pub fn encode_png(px: &Pixmap) -> Result<Vec<u8>, String> {
    let img = to_rgba(px);
    let mut buf = Vec::new();
    image::codecs::png::PngEncoder::new(&mut buf)
        .write_image(
            img.as_raw(),
            img.width(),
            img.height(),
            image::ExtendedColorType::Rgba8,
        )
        .map_err(|e| e.to_string())?;
    Ok(buf)
}

pub fn encode_jpeg(px: &Pixmap, quality: u8) -> Result<Vec<u8>, String> {
    let img = to_rgba(px);
    let rgb = image::DynamicImage::ImageRgba8(img).to_rgb8();
    let mut buf = Vec::new();
    image::codecs::jpeg::JpegEncoder::new_with_quality(&mut buf, quality)
        .write_image(
            rgb.as_raw(),
            rgb.width(),
            rgb.height(),
            image::ExtendedColorType::Rgb8,
        )
        .map_err(|e| e.to_string())?;
    Ok(buf)
}

pub fn encode_webp(px: &Pixmap) -> Result<Vec<u8>, String> {
    let img = to_rgba(px);
    let mut buf = Vec::new();
    image::codecs::webp::WebPEncoder::new_lossless(&mut buf)
        .write_image(
            img.as_raw(),
            img.width(),
            img.height(),
            image::ExtendedColorType::Rgba8,
        )
        .map_err(|e| e.to_string())?;
    Ok(buf)
}

/// encode_for returns (bytes, content-type) for the requested FORMAT.
pub fn encode_for(format: &str, px: &Pixmap) -> (Vec<u8>, &'static str) {
    let f = format.to_lowercase();
    if f.contains("jpeg") || f.contains("jpg") {
        return (encode_jpeg(px, 85).unwrap_or_default(), "image/jpeg");
    }
    if f.contains("webp") {
        return (encode_webp(px).unwrap_or_default(), "image/webp");
    }
    (encode_png(px).unwrap_or_default(), "image/png")
}
