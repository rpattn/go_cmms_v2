package middleware

import (
    "context"
    "net/http"
    "github.com/google/uuid"
)

// ctxKeyRequestID is the context key type for request IDs.
type ctxKeyRequestID struct{}

// GetRequestID returns the request id from context if set.
func GetRequestID(ctx context.Context) (string, bool) {
    v := ctx.Value(ctxKeyRequestID{})
    if s, ok := v.(string); ok && s != "" {
        return s, true
    }
    return "", false
}

// RequestID ensures each request has a request ID.
// If trustHeader is true, it uses X-Request-ID when present; otherwise it always generates a new one.
func RequestID(trustHeader bool) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            rid := ""
            if trustHeader {
                rid = r.Header.Get("X-Request-ID")
            }
            if rid == "" {
                rid = uuid.NewString()
            }
            ctx := context.WithValue(r.Context(), ctxKeyRequestID{}, rid)
            w.Header().Set("X-Request-ID", rid)
            next.ServeHTTP(w, r.WithContext(ctx))
        })
    }
}
