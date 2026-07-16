package ows

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWMS111Exception(t *testing.T) {
	rec := httptest.NewRecorder()
	WriteException(rec, "WMS", "1.1.1", "", ServiceError{Code: "InvalidParameterValue", Locator: "layers", Message: "no such layer"})
	body := rec.Body.String()
	if rec.Code != 200 || rec.Header().Get("Content-Type") != "application/vnd.ogc.se_xml" {
		t.Fatalf("code=%d ct=%s", rec.Code, rec.Header().Get("Content-Type"))
	}
	for _, want := range []string{
		`<!DOCTYPE ServiceExceptionReport SYSTEM "http://schemas.opengis.net/wms/1.1.1/WMS_exception_1_1_1.dtd">`,
		`<ServiceExceptionReport version="1.1.1">`,
		`code="InvalidParameterValue"`, `locator="layers"`, `no such layer`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %q in %s", want, body)
		}
	}
}

func TestWMS130Exception(t *testing.T) {
	rec := httptest.NewRecorder()
	WriteException(rec, "WMS", "1.3.0", "", ServiceError{Code: "LayerNotDefined", Message: "x"})
	body := rec.Body.String()
	if rec.Header().Get("Content-Type") != "text/xml" {
		t.Fatalf("ct=%s", rec.Header().Get("Content-Type"))
	}
	if !strings.Contains(body, `xmlns="http://www.opengis.net/ogc"`) ||
		!strings.Contains(body, `version="1.3.0"`) {
		t.Fatalf("body=%s", body)
	}
}

func TestWFS20Exception(t *testing.T) {
	rec := httptest.NewRecorder()
	WriteException(rec, "WFS", "2.0.0", "", ServiceError{Code: "OperationNotSupported", Locator: "request", Message: "nope", Status: 400})
	body := rec.Body.String()
	if rec.Code != 400 || rec.Header().Get("Content-Type") != "application/xml" {
		t.Fatalf("code=%d ct=%s", rec.Code, rec.Header().Get("Content-Type"))
	}
	for _, want := range []string{
		`xmlns:ows="http://www.opengis.net/ows/1.1"`, `version="2.0.0"`,
		`exceptionCode="OperationNotSupported"`, `<ows:ExceptionText>nope</ows:ExceptionText>`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %q in %s", want, body)
		}
	}
}

func TestWFS10Exception(t *testing.T) {
	rec := httptest.NewRecorder()
	WriteException(rec, "WFS", "1.0.0", "", ServiceError{Code: "MissingParameterValue", Message: "typename"})
	if !strings.Contains(rec.Body.String(), `version="1.2.0"`) {
		t.Fatalf("body=%s", rec.Body.String())
	}
}

func TestJSONException(t *testing.T) {
	rec := httptest.NewRecorder()
	WriteException(rec, "WMS", "1.3.0", "application/json", ServiceError{Code: "NoApplicableCode", Message: "boom"})
	body := rec.Body.String()
	if rec.Header().Get("Content-Type") != "application/json" {
		t.Fatalf("ct=%s", rec.Header().Get("Content-Type"))
	}
	for _, want := range []string{`"version":"1.3.0"`, `"code":"NoApplicableCode"`, `"text":"boom"`} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %q in %s", want, body)
		}
	}
}
