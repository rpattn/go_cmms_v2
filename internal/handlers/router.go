// internal/handlers/router.go
package handlers

import (
	"yourapp/internal/handlers/admin"
	"yourapp/internal/handlers/users"
	"yourapp/internal/middleware"
	"yourapp/internal/repo"

	"github.com/go-chi/chi/v5"
)

func RegisterRoutes(mux *chi.Mux, r repo.Repo) {
	u := users.New(r)

	mux.Route("/users", func(sr chi.Router) {
		// Apply auth to the whole group ONCE
		sr.Use(middleware.RequireAuth(r))

		sr.Post("/search", u.Search)
	})

	// Admin routes
	mux.Route("/admin", func(sr chi.Router) {
		sr.Use(middleware.RequireAuth(r))
		sr.Get("/sessions", admin.ListSessionsHandler(r))
	})
}
