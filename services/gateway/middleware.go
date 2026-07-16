package main

import (
	"net"
	"net/http"
	"sync"

	"github.com/geoson/geoson/libs/ogc-kit/ows"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"golang.org/x/time/rate"
)

var (
	reqTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "geoson_gateway_requests_total",
		Help: "OWS requests by service and status code.",
	}, []string{"service", "code"})
	reqSeconds = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name: "geoson_gateway_request_seconds",
		Help: "OWS request latency.",
	}, []string{"service"})
)

type statusWriter struct {
	http.ResponseWriter
	code int
}

func (s *statusWriter) WriteHeader(code int) {
	s.code = code
	s.ResponseWriter.WriteHeader(code)
}

func metricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		service := ows.ParseKVP(r.URL.Query()).Service
		if service == "" {
			service = "unknown"
		}
		sw := &statusWriter{ResponseWriter: w, code: 200}
		timer := prometheus.NewTimer(reqSeconds.WithLabelValues(service))
		next.ServeHTTP(sw, r)
		timer.ObserveDuration()
		reqTotal.WithLabelValues(service, http.StatusText(sw.code)).Inc()
	})
}

func metricsHandler() http.Handler { return promhttp.Handler() }

// rateLimitMiddleware enforces a per-client-IP token bucket.
// limit <= 0 disables limiting.
func rateLimitMiddleware(limit float64, burst int, next http.Handler) http.Handler {
	if limit <= 0 {
		return next
	}
	var mu sync.Mutex
	limiters := map[string]*rate.Limiter{}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip, _, _ := net.SplitHostPort(r.RemoteAddr)
		mu.Lock()
		l, ok := limiters[ip]
		if !ok {
			l = rate.NewLimiter(rate.Limit(limit), burst)
			limiters[ip] = l
		}
		mu.Unlock()
		if !l.Allow() {
			// Plain 429, not an OWS exception: WMS exception formats force
			// HTTP 200, which would hide throttling from load balancers.
			http.Error(w, "Too many requests", http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}
