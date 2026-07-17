// Package style provides SLD validation and default-style generation for the
// admin editor. Validation is structural (well-formedness + root element);
// deep semantic checks happen at render time in the wms service.
package style

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"strings"
)

// ValidationError is one problem found in a style document.
type ValidationError struct {
	Line    int    `json:"line"`
	Message string `json:"message"`
}

// ValidationResult is the outcome of validating a style.
type ValidationResult struct {
	OK     bool              `json:"ok"`
	Errors []ValidationError `json:"errors"`
}

// Validate checks a style document for the given format (sld|css|ysld|mbstyle).
func Validate(format, content string) ValidationResult {
	switch strings.ToLower(format) {
	case "sld", "":
		return validateSLD(content)
	case "mbstyle":
		if !json.Valid([]byte(content)) {
			return ValidationResult{OK: false, Errors: []ValidationError{{Line: 0, Message: "invalid JSON"}}}
		}
		return ValidationResult{OK: true}
	case "css", "ysld", "geocss":
		if strings.TrimSpace(content) == "" {
			return ValidationResult{OK: false, Errors: []ValidationError{{Line: 0, Message: "empty style"}}}
		}
		return ValidationResult{OK: true}
	default:
		return ValidationResult{OK: false, Errors: []ValidationError{{Line: 0, Message: "unknown format " + format}}}
	}
}

func validateSLD(content string) ValidationResult {
	dec := xml.NewDecoder(strings.NewReader(content))
	var rootLocal string
	for {
		tok, err := dec.Token()
		if err != nil {
			if err.Error() == "EOF" {
				break
			}
			if se, ok := err.(*xml.SyntaxError); ok {
				return ValidationResult{OK: false, Errors: []ValidationError{{Line: se.Line, Message: se.Msg}}}
			}
			return ValidationResult{OK: false, Errors: []ValidationError{{Line: 0, Message: err.Error()}}}
		}
		if se, ok := tok.(xml.StartElement); ok && rootLocal == "" {
			rootLocal = se.Name.Local
			break
		}
	}
	if rootLocal != "StyledLayerDescriptor" {
		return ValidationResult{OK: false, Errors: []ValidationError{{Line: 1,
			Message: fmt.Sprintf("root element must be StyledLayerDescriptor, got %q", rootLocal)}}}
	}
	return ValidationResult{OK: true}
}

// GenerateDefault returns a minimal SLD for a geometry type with the given fill.
func GenerateDefault(geomType, color string) string {
	if color == "" {
		color = "#2FA7A1"
	}
	g := strings.ToUpper(geomType)
	var symbolizer string
	switch {
	case strings.Contains(g, "POLYGON"):
		symbolizer = fmt.Sprintf(`<PolygonSymbolizer><Fill><CssParameter name="fill">%s</CssParameter></Fill>`+
			`<Stroke><CssParameter name="stroke">#1E4E8C</CssParameter></Stroke></PolygonSymbolizer>`, color)
	case strings.Contains(g, "LINE"):
		symbolizer = fmt.Sprintf(`<LineSymbolizer><Stroke><CssParameter name="stroke">%s</CssParameter>`+
			`<CssParameter name="stroke-width">1.5</CssParameter></Stroke></LineSymbolizer>`, color)
	default:
		symbolizer = fmt.Sprintf(`<PointSymbolizer><Graphic><Mark><WellKnownName>circle</WellKnownName>`+
			`<Fill><CssParameter name="fill">%s</CssParameter></Fill></Mark><Size>8</Size></Graphic></PointSymbolizer>`, color)
	}
	return `<?xml version="1.0" encoding="UTF-8"?>` +
		`<StyledLayerDescriptor xmlns="http://www.opengis.net/sld" version="1.0.0">` +
		`<NamedLayer><Name>default</Name><UserStyle><Title>Generated</Title><FeatureTypeStyle><Rule>` +
		symbolizer +
		`</Rule></FeatureTypeStyle></UserStyle></NamedLayer></StyledLayerDescriptor>`
}
