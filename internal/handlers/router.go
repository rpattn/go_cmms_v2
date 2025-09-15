// internal/handlers/router.go
package handlers

import (
    tables "yourapp/internal/handlers/tables"
    "yourapp/internal/handlers/admin"
    "yourapp/internal/handlers/users"
    "yourapp/internal/middleware"
    "yourapp/internal/repo"

    "github.com/go-chi/chi/v5"
)

func RegisterRoutes(mux *chi.Mux, r repo.Repo) {
    u := users.New(r)
    t := tables.New(r)

    mux.Route("/users", func(sr chi.Router) {
        // Apply auth to the whole group ONCE
        sr.Use(middleware.RequireAuth(r))

        sr.Post("/search", u.Search)
    })

    // Generic EAV table search routes
    mux.Route("/tables", func(sr chi.Router) {
        sr.Use(middleware.RequireAuth(r))
        // Create and list org-scoped user tables
        sr.Get("/", t.List)
        sr.Get("/indexed-fields", t.IndexedFields)
        sr.Post("/", t.Create)
        sr.Delete("/{table}", t.Delete)
        sr.Post("/{table}/columns", t.AddColumn)
        sr.Delete("/{table}/columns/{column}", t.RemoveColumn)
        sr.Post("/{table}/rows", t.AddRow)
        sr.Delete("/{table}/rows/{row_id}", t.DeleteRow)
        sr.Post("/{table}/rows/indexed", t.LookupIndexed)
        sr.Post("/rows/lookup", t.LookupRow)
        sr.Post("/{table}/search", t.Search)
    })

	// Admin routes
	mux.Route("/admin", func(sr chi.Router) {
		sr.Use(middleware.RequireAuth(r))
		sr.Get("/sessions", admin.ListSessionsHandler(r))
	})
}
