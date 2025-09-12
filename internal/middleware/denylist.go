package middleware

import (
    "net/http"
    "yourapp/internal/auth"
    "yourapp/internal/security"
)

// Denylist blocks requests for users marked as denied.
func Denylist(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if sess, ok := auth.SessionFromContext(r.Context()); ok && sess != nil {
            if security.IsUserDenied(sess.UserID) {
                http.Error(w, "forbidden", http.StatusForbidden)
                return
            }
        }
        next.ServeHTTP(w, r)
    })
}

