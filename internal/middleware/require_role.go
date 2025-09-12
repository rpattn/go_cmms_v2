// internal/middleware/require_role.go
package middleware

import (
	"net/http"

	"yourapp/internal/auth"
	"yourapp/internal/models"
	"yourapp/internal/repo"
)

func RequireRole(r repo.Repo, allowed ...models.OrgRole) func(http.Handler) http.Handler {
	// Map role to integer level — adjust roles and levels as needed
	var roleLevels = map[models.OrgRole]int{
		models.RoleViewer: 1,
		models.RoleMember: 2,
		models.RoleAdmin:  3,
		models.RoleOwner:  4,
	}

	// Find the minimum allowed role level
	minAllowedLevel := 9999
	for _, role := range allowed {
		lvl, ok := roleLevels[role]
		if !ok {
			// If unknown role, skip or handle error
			continue
		}
		if lvl < minAllowedLevel {
			minAllowedLevel = lvl
		}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			sess, _ := auth.SessionFromContext(req.Context())
			if sess == nil {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}

			role, err := r.GetRole(req.Context(), sess.ActiveOrg, sess.UserID)
			if err != nil {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}

			userLevel, ok := roleLevels[role]
			if !ok {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}

			if userLevel < minAllowedLevel {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, req)
		})
	}
}
