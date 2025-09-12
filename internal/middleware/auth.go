package middleware

import (
	"net/http"

	"yourapp/internal/auth"
	"yourapp/internal/repo"

	"github.com/google/uuid"
)

// RequireAuth authenticates using the "session" cookie (auth.ReadSession),
// then loads the user by Session.UserID from the repo and injects both
// session and user into the context.
func RequireAuth(r repo.Repo) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			s := auth.ReadSession(req)
			if s == nil {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}

			user, err := r.GetUserByID(req.Context(), s.UserID)
			if err != nil {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}

			if s.ActiveOrg == uuid.Nil {
				http.Error(w, "organization not found for session", http.StatusUnauthorized)
				return
			}

			ctx := auth.WithSession(req.Context(), s)
			ctx = auth.WithUser(ctx, &user)
			//inject org from session
			ctx = auth.WithOrg(ctx, s.ActiveOrg)

			next.ServeHTTP(w, req.WithContext(ctx))
		})
	}
}
