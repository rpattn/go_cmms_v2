package middleware

import (
    "net/http"
    "time"
    "log/slog"

    "yourapp/internal/auth"
)

type responseWriter struct {
    http.ResponseWriter
    status int
    bytes  int
}

func (w *responseWriter) WriteHeader(code int) {
    w.status = code
    w.ResponseWriter.WriteHeader(code)
}

func (w *responseWriter) Write(b []byte) (int, error) {
    if w.status == 0 {
        // Default to 200 if Write is called first
        w.status = http.StatusOK
    }
    n, err := w.ResponseWriter.Write(b)
    w.bytes += n
    return n, err
}

// SlogRequestLogger logs each HTTP request with structured fields using slog.
func SlogRequestLogger(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        start := time.Now()
        rw := &responseWriter{ResponseWriter: w}
        next.ServeHTTP(rw, r)
        dur := time.Since(start)

        // Enrich with request_id if present and user/org context when available.
        attrs := []any{
            "method", r.Method,
            "url", r.URL.String(),
            "status", rw.status,
            "duration", dur,
            "bytes", rw.bytes,
        }
        if rid := r.Context().Value(ctxKeyRequestID{}); rid != nil {
            attrs = append(attrs, "request_id", rid)
        }
        if sess, ok := auth.SessionFromContext(r.Context()); ok && sess != nil {
            attrs = append(attrs, "user_id", sess.UserID.String())
        } else if u, ok := auth.GetUserFromContext(r.Context()); ok && u != nil {
            attrs = append(attrs, "user_id", u.ID.String())
        }
        if org, ok := auth.OrgFromContext(r.Context()); ok {
            attrs = append(attrs, "org_id", org.String())
        }
        // Use logger from context if present to include EnrichLogger attrs
        slog.InfoContext(r.Context(), "request", attrs...)
    })
}
