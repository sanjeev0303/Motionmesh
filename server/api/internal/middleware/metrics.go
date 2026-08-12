package middleware

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/motionmesh/server/shared/metrics"
)

// MetricsMiddleware records HTTP request duration and status codes.
func MetricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Use a response writer wrapper to capture the status code
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

		next.ServeHTTP(ww, r)

		duration := time.Since(start).Seconds()
		status := ww.Status()

		// Fallback to 200 if not set
		if status == 0 {
			status = 200
		}

		// Get matched route pattern from Chi context (e.g., /videos/{id})
		routeContext := chi.RouteContext(r.Context())
		path := r.URL.Path
		if routeContext != nil && routeContext.RoutePattern() != "" {
			path = routeContext.RoutePattern()
		}

		// Handle status as string
		statusStr := http.StatusText(status)
		if statusStr == "" {
			statusStr = "unknown"
		} else {
			// convert 200 to "200"
			statusStr = "200" // we can just use the int as string
			statusStr = string(rune(status + '0')) // hackish, let's just use fmt or strconv? actually we should just use strconv.Itoa below.
		}

		importStr := func(i int) string {
			// fast path for common status codes
			switch i {
			case 200:
				return "200"
			case 201:
				return "201"
			case 400:
				return "400"
			case 401:
				return "401"
			case 403:
				return "403"
			case 404:
				return "404"
			case 429:
				return "429"
			case 500:
				return "500"
			default:
				// this is fine for metrics, not high allocation normally
				importStr := func(n int) string {
					buf := [11]byte{}
					pos := len(buf)
					i := int64(n)
					signed := i < 0
					if signed {
						i = -i
					}
					for {
						pos--
						buf[pos] = '0' + byte(i%10)
						i /= 10
						if i == 0 {
							break
						}
					}
					if signed {
						pos--
						buf[pos] = '-'
					}
					return string(buf[pos:])
				}
				return importStr(i)
			}
		}

		metrics.APIRequestsTotal.WithLabelValues(r.Method, path, importStr(status)).Inc()
		metrics.APIRequestDuration.WithLabelValues(r.Method, path).Observe(duration)
	})
}
