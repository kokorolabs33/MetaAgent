package main

import (
	"context"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	"taskhub/internal/adapter"
	"taskhub/internal/audit"
	"taskhub/internal/auth"
	"taskhub/internal/config"
	"taskhub/internal/db"
	"taskhub/internal/events"
	"taskhub/internal/executor"
	"taskhub/internal/handlers"
	"taskhub/internal/orchestrator"
	"taskhub/internal/rbac"
	"taskhub/internal/seed"
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

	// Dev seed (creates test user + org + session for local development)
	seed.DevSeedAndLog(ctx, pool, sessionStore)

	// RBAC
	rbacMw := &rbac.Middleware{DB: pool}

	// Handlers
	orgH := &handlers.OrgHandler{DB: pool}
	memberH := &handlers.MemberHandler{DB: pool}
	agentH := &handlers.AgentHandler{DB: pool}

	// Execution engine
	eventStore := &events.Store{DB: pool}
	broker := events.NewBroker()
	auditLogger := &audit.Logger{DB: pool}
	orch := &orchestrator.Orchestrator{}

	adapters := map[string]adapter.AgentAdapter{
		"http_poll": adapter.NewHTTPPollAdapter(),
		"native":    adapter.NewNativeAdapter(),
	}

	exec := &executor.DAGExecutor{
		DB: pool, Broker: broker, EventStore: eventStore,
		Audit: auditLogger, Orchestrator: orch, Adapters: adapters,
	}
	exec.Recover(ctx)

	taskH := &handlers.TaskHandler{DB: pool, Executor: exec, EventStore: eventStore, Audit: auditLogger}
	msgH := &handlers.MessageHandler{DB: pool, Executor: exec, EventStore: eventStore, Broker: broker}
	streamH := &handlers.StreamHandler{EventStore: eventStore, Broker: broker}

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

			// Agents
			r.Get("/agents", agentH.List)
			r.With(rbac.RequireRole("admin")).Post("/agents", agentH.Create)
			r.Get("/agents/{id}", agentH.Get)
			r.With(rbac.RequireRole("admin")).Put("/agents/{id}", agentH.Update)
			r.With(rbac.RequireRole("admin")).Delete("/agents/{id}", agentH.Delete)
			r.Post("/agents/{id}/healthcheck", agentH.Healthcheck)
			r.Post("/agents/test-endpoint", agentH.TestEndpoint)

			// Tasks
			r.With(rbac.RequireRole("member")).Post("/tasks", taskH.Create)
			r.Get("/tasks", taskH.List)
			r.Get("/tasks/{id}", taskH.Get)
			r.Post("/tasks/{id}/cancel", taskH.Cancel)
			r.Get("/tasks/{id}/cost", taskH.GetCost)
			r.Get("/tasks/{id}/subtasks", taskH.ListSubtasks)

			// Messages (Group Chat)
			r.Get("/tasks/{id}/messages", msgH.List)
			r.With(rbac.RequireRole("member")).Post("/tasks/{id}/messages", msgH.Send)

			// SSE Event Stream
			r.Get("/tasks/{id}/events", streamH.Stream)
		})
	})

	log.Printf("TaskHub V2 listening on :%s", cfg.Port)
	if err := http.ListenAndServe(":"+cfg.Port, r); err != nil {
		log.Fatalf("listen: %v", err)
	}
}
