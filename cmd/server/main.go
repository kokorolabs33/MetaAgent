package main

import (
	"context"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	"taskhub/internal/auth"
	"taskhub/internal/config"
	"taskhub/internal/db"
	"taskhub/internal/handlers"
	"taskhub/internal/rbac"
)

func main() {
	ctx := context.Background()
	cfg := config.Load()

	// Database
	pool, err := db.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("db open: %v", err)
	}
	defer pool.Close()

	if err := db.RunMigrations(ctx, pool); err != nil {
		log.Fatalf("migrations: %v", err)
	}

	// Auth
	sessionStore := &auth.SessionStore{DB: pool}
	authMw := &auth.Middleware{Sessions: sessionStore}
	authH := &auth.Handler{
		DB:           pool,
		Sessions:     sessionStore,
		GoogleID:     cfg.GoogleClientID,
		GoogleSecret: cfg.GoogleSecret,
		FrontendURL:  cfg.FrontendURL,
	}

	// RBAC
	rbacMw := &rbac.Middleware{DB: pool}

	// Handlers
	orgH := &handlers.OrgHandler{DB: pool}
	memberH := &handlers.MemberHandler{DB: pool}

	// Router
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{cfg.FrontendURL},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Content-Type", "Authorization"},
		AllowCredentials: true,
	}))

	// Health check — no auth
	r.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	// Auth routes — no auth middleware
	r.Post("/api/auth/google/login", authH.GoogleLogin)
	r.Get("/api/auth/google/callback", authH.GoogleCallback)
	r.Post("/api/auth/logout", authH.Logout)

	// Authenticated routes
	r.Group(func(r chi.Router) {
		r.Use(authMw.RequireAuth)

		r.Get("/api/orgs", orgH.List)
		r.Post("/api/orgs", orgH.Create)

		// Org-scoped routes
		r.Route("/api/orgs/{org_id}", func(r chi.Router) {
			r.Use(rbacMw.RequireOrg)

			r.Get("/", orgH.Get)
			r.With(rbac.RequireRole("owner")).Put("/", orgH.Update)

			r.Get("/members", memberH.List)
			r.With(rbac.RequireRole("admin")).Post("/members", memberH.Invite)
			r.With(rbac.RequireRole("admin")).Put("/members/{uid}", memberH.UpdateRole)
			r.With(rbac.RequireRole("admin")).Delete("/members/{uid}", memberH.Remove)

			// TODO: agent routes
			// TODO: task routes
		})
	})

	log.Printf("TaskHub V2 listening on :%s", cfg.Port)
	if err := http.ListenAndServe(":"+cfg.Port, r); err != nil {
		log.Fatalf("listen: %v", err)
	}
}
