package main

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"github.com/geoson/geoson/libs/ogc-kit/ows"
)

type backends struct {
	byService map[string]*url.URL
}

func newBackends(getenv func(string) string) backends {
	b := backends{byService: map[string]*url.URL{}}
	for svc, env := range map[string]string{
		"WMS": "GEOSON_WMS_URL", "WFS": "GEOSON_WFS_URL",
		"WMTS": "GEOSON_TILES_URL", "WPS": "GEOSON_WPS_URL",
	} {
		if v := getenv(env); v != "" {
			if u, err := url.Parse(v); err == nil {
				b.byService[svc] = u
			}
		}
	}
	return b
}

// endpointService maps URL endpoint segments to implied services.
var endpointService = map[string]string{
	"wms": "WMS", "wfs": "WFS", "wps": "WPS", "wmts": "WMTS", "gwc": "WMTS",
}

// parsePath splits /geoserver/[{ws}/[{layer}/]]{endpoint} into parts.
func parsePath(path string) (ws, layer, endpoint string) {
	path = strings.TrimPrefix(path, "/geoserver")
	path = strings.Trim(path, "/")
	segs := strings.Split(path, "/")
	for i, s := range segs {
		ls := strings.ToLower(s)
		if _, ok := endpointService[ls]; ok || ls == "ows" {
			endpoint = ls
			segs = segs[:i]
			break
		}
	}
	if len(segs) > 0 {
		ws = segs[0]
	}
	if len(segs) > 1 {
		layer = segs[1]
	}
	return ws, layer, endpoint
}

func newDispatcher(b backends) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		wsName, layer, endpoint := parsePath(r.URL.Path)

		var req ows.Request
		var bodyCopy []byte
		if r.Method == http.MethodPost {
			bodyCopy, _ = io.ReadAll(io.LimitReader(r.Body, 64<<20))
			var err error
			req, err = ows.ParseXML(bytes.NewReader(bodyCopy))
			if err != nil {
				ows.WriteException(w, "", "", "", ows.ServiceError{
					Code: ows.CodeNoApplicableCode, Message: err.Error(), Status: 400})
				return
			}
		} else {
			req = ows.ParseKVP(r.URL.Query())
		}

		service := req.Service
		if service == "" {
			if implied, ok := endpointService[endpoint]; ok {
				service = implied
			}
		}
		exceptions := req.Get("EXCEPTIONS")
		if service == "" {
			ows.WriteException(w, "", "", exceptions, ows.ServiceError{
				Code: ows.CodeMissingParameterValue, Locator: "service",
				Message: "Could not determine service", Status: 400})
			return
		}
		version := ows.Negotiate(service, req.Version)
		if version == "" {
			ows.WriteException(w, "", "", exceptions, ows.ServiceError{
				Code: ows.CodeInvalidParameterValue, Locator: "service",
				Message: "No service: ( " + strings.ToLower(service) + " )", Status: 400})
			return
		}
		if req.Request == "" {
			ows.WriteException(w, service, version, exceptions, ows.ServiceError{
				Code: ows.CodeMissingParameterValue, Locator: "request",
				Message: "Could not determine request", Status: 400})
			return
		}
		backend, ok := b.byService[service]
		if !ok {
			ows.WriteException(w, service, version, exceptions, ows.ServiceError{
				Code:    ows.CodeNoApplicableCode,
				Message: "Service " + service + " is not available", Status: 503})
			return
		}

		proxy := httputil.NewSingleHostReverseProxy(backend)
		r.Header.Set("X-Geoson-Workspace", wsName)
		r.Header.Set("X-Geoson-Layer", layer)
		r.Header.Set("X-Geoson-Version", version)
		r.URL.Path = "/" + strings.ToLower(service)
		if bodyCopy != nil {
			r.Body = io.NopCloser(bytes.NewReader(bodyCopy))
			r.ContentLength = int64(len(bodyCopy))
		}
		proxy.ServeHTTP(w, r)
	})
}
