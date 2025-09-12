package middleware

import (
    "net/http"

    "yourapp/internal/auth"
    "yourapp/internal/repo"

    "github.com/google/uuid"
)

// OptionalAuth reads the session cookie if present and valid, loads user and
// injects session, user, and org into context. It never returns 401; on any
// failure it simply passes the request through unauthenticated.
func OptionalAuth(r repo.Repo) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
            s := auth.ReadSession(req)
            if s == nil {
                next.ServeHTTP(w, req)
                return
            }
            // Basic sanity: ensure org is set; if not, try to pick one
            if s.ActiveOrg == uuid.Nil {
                if org, err := r.PickUserOrg(req.Context(), s.UserID); err == nil {
                    s.ActiveOrg = org.ID
                }
            }
            if u, err := r.GetUserByID(req.Context(), s.UserID); err == nil {
                ctx := auth.WithSession(req.Context(), s)
                ctx = auth.WithUser(ctx, &u)
                ctx = auth.WithOrg(ctx, s.ActiveOrg)
                next.ServeHTTP(w, req.WithContext(ctx))
                return
            }
            // If user load fails, continue unauthenticated
            next.ServeHTTP(w, req)
        })
    }
}

