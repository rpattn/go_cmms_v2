package middleware

import (
    "net/http"
    "strings"

    "yourapp/internal/auth"
    "yourapp/internal/repo"
)

// MFAEnforce enforces TOTP for local accounts if localRequired is true.
// It permits access to TOTP setup/verify and logout endpoints even when MFA is required but not yet set.
func MFAEnforce(r repo.Repo, localRequired bool) func(http.Handler) http.Handler {
    // Allowed paths when user must setup MFA
    allowed := map[string]struct{}{
        "/auth/mfa/totp/setup":  {},
        "/auth/mfa/totp/verify": {},
        "/auth/logout":          {},
    }

    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
            if !localRequired {
                next.ServeHTTP(w, req)
                return
            }

            // Only applies to authenticated local users
            s := auth.ReadSession(req)
            if s == nil || s.Provider != "local" {
                next.ServeHTTP(w, req)
                return
            }

            // If user has TOTP, allow
            if r.UserHasTOTP(req.Context(), s.UserID) {
                next.ServeHTTP(w, req)
                return
            }

            // Allow TOTP setup/verify/logout without blocking
            path := req.URL.Path
            if _, ok := allowed[path]; ok {
                next.ServeHTTP(w, req)
                return
            }
            // Also allow static and root
            if strings.HasPrefix(path, "/static/") || path == "/" {
                next.ServeHTTP(w, req)
                return
            }

            // Friendlier flow for browsers: if Accept header prefers HTML, redirect to setup page.
            if strings.Contains(req.Header.Get("Accept"), "text/html") {
                http.Redirect(w, req, "/static/mfa/index.html", http.StatusFound)
                return
            }
            // Otherwise return JSON for API clients
            w.Header().Set("Content-Type", "application/json")
            w.WriteHeader(http.StatusForbidden)
            _, _ = w.Write([]byte(`{"error":"mfa_required"}`))
        })
    }
}
