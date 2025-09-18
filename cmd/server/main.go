// cmd/server/main.go
package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/go-chi/chi/v5"
	//mux_middleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors" // <-- cors
	"github.com/jackc/pgx/v5/pgxpool"

	"yourapp/internal/auth"
	"yourapp/internal/config"
	db "yourapp/internal/db/gen"
	"yourapp/internal/handlers"
	"yourapp/internal/logging"
	"yourapp/internal/middleware"
	"yourapp/internal/models"
	"yourapp/internal/repo"
	"yourapp/internal/session"
)

func main() {
	// --- Load config (config.yaml + env overrides) ---
	cfg := config.Load()

	// --- Logger ---
	// Configure slog from config: logging.level, logging.format
	logging.Setup(cfg.Logging.Level, cfg.Logging.Format == "json")

	// Configure session cookie security (dev often needs Secure=false)
	auth.SetCookieSecurity(cfg.Security.Session.CookieSecure)
	// Configure SameSite policy
	auth.SetCookieSameSite(cfg.Security.Session.SameSite)

	// --- Background session sweeper ---
	interval := cfg.Security.Session.SweeperInterval
	if interval <= 0 {
		interval = 5 * time.Minute
	}
	go session.DefaultStore.StartSweeper(context.Background(), interval)

	// --- Connect to Postgres ---
	ctx := context.Background()
	slog.Debug("connecting to database")
	pool, err := pgxpool.New(ctx, cfg.Database.URL)
	if err != nil {
		slog.Error("db connect error", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		slog.Error("db ping error", "err", err)
		os.Exit(1)
	}
	slog.Debug("database connection ready")

	// sqlc queries + repo wrapper
	q := db.New(pool)
	r := repo.NewWithDB(pool, q)

	// --- Setup OAuth/OIDC providers ---
	providers := auth.SetupProviders(cfg)

	// --- Router ---
	mux := chi.NewRouter()

	// Ensure request ID then log requests with slog
	mux.Use(middleware.RequestID(cfg.Security.RequestID.TrustHeader))
	mux.Use(middleware.EnrichLogger)
	mux.Use(middleware.SlogRequestLogger)
	// Enforce MFA for local accounts if enabled
	mux.Use(middleware.MFAEnforce(r, cfg.Security.MFA.LocalRequired))
	if cfg.Security.RateLimit.Enabled {
		mux.Use(middleware.RateLimitWith(cfg.Security.RateLimit.RequestsPerMinute, cfg.Security.RateLimit.Burst, cfg.Security.RateLimit.TTL))
	}
	if cfg.Security.Denylist.Enabled {
		mux.Use(middleware.Denylist)
	}

	// --- CORS middleware ---
	mux.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"http://localhost:5500", "http://localhost:3000", "http://127.0.0.1:5500", "http://127.0.0.1:3000", "http://127.0.0.1:8080"}, // adjust as needed
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300, // Maximum value not ignored by browsers
	}))

	// OAuth/OIDC routes
	slog.Debug("oauth providers configured", "providers", providers)
	mux.Get("/auth/{provider}", auth.StartHandler(providers, r))
	mux.Get("/auth/{provider}/callback", auth.CallbackHandler(providers, r, cfg))

	// Local auth routes
	mux.Post("/auth/signup", auth.SignupHandler(r))
	mux.Post("/auth/login", auth.LoginHandler(r))
	mux.Post("/auth/logout", auth.LogoutHandler())
	mux.Post("/auth/link/password", auth.LinkWithPasswordHandler(r, cfg))
	mux.Post("/auth/set-password", auth.SetPasswordHandler(r, cfg))
	mux.Get("/auth/mfa/totp/setup", auth.TOTPSetupBeginHandler(r))
	mux.Post("/auth/mfa/totp/verify", auth.TOTPSetupVerifyHandler(r))
	mux.Put("/auth/profile", auth.UpdateProfileHandler(r))

	// Invite routes (stubs): create requires Owner role; accept is public
	mux.With(middleware.RequireAuth(r), middleware.RequireRole(r, models.RoleOwner)).
		Post("/auth/invite", auth.InviteCreateHandler(r, cfg))
	mux.Post("/auth/invite/accept", auth.InviteAcceptHandler(r))

	// main.go (add inside main())

	// --- Protected routes ---
	// All routes below require authentication

	mux.Handle("/auth/me", auth.ProfileHandler(r))
	mux.Handle("/auth/me/full", auth.FullProfileHandler(r))

	// Example protected routes by org/role
	mux.Route("/orgs/{slug}", func(sr chi.Router) {
		sr.With(middleware.RequireRole(r, models.RoleViewer)).
			Get("/projects", func(w http.ResponseWriter, _ *http.Request) {
				w.Write([]byte("list projects"))
			})
		sr.With(middleware.RequireRole(r, models.RoleAdmin, models.RoleOwner)).
			Post("/projects", func(w http.ResponseWriter, _ *http.Request) {
				w.Write([]byte("create project"))
			})
	})

	// Work orders and tasks routes
	handlers.RegisterRoutes(mux, r)

	// Serve static files from ./static at /static/*
	mux.Handle("/static/*", http.StripPrefix("/static/", http.FileServer(http.Dir("./static/"))))

	// Health root
	mux.Get("/", func(w http.ResponseWriter, r *http.Request) {
		//w.Write([]byte("OK"))
		http.ServeFile(w, r, "./static/test.html")
	})

	// Convenience routes to static pages (serve directly, no redirect)
	mux.Get("/invite", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "./static/invite/index.html")
	})
	mux.Get("/invite/accept", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "./static/invite/accept.html")
	})
	mux.Get("/mfa", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "./static/mfa/index.html")
	})

	// --- Start server ---
	addr := "127.0.0.1:8080"
	if v := os.Getenv("PORT"); v != "" {
		addr = ":" + v
	}
	slog.Info("listening", "addr", addr, "base_url", cfg.BaseURL)
	if err := http.ListenAndServe(addr, mux); err != nil {
		slog.Error("server error", "err", err)
		os.Exit(1)
	}
}
