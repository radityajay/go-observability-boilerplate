package middleware

import (
	"fmt"
	"net/http"
	"time"

	"github.com/radityajayantara/go-observability-boilerplate/internal/observability"
)

// MetricsMiddleware records Prometheus metrics for each HTTP request.
func MetricsMiddleware(m *observability.Metrics) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			m.RequestsInFlight.Inc()
			defer m.RequestsInFlight.Dec()

			rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(rec, r)

			duration := time.Since(start).Seconds()
			status := fmt.Sprintf("%d", rec.status)

			m.RequestsTotal.WithLabelValues(r.Method, r.URL.Path, status).Inc()
			m.RequestDuration.WithLabelValues(r.Method, r.URL.Path).Observe(duration)
		})
	}
}
