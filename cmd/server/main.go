package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	"taskhub/internal/a2a"
	"taskhub/internal/audit"
	"taskhub/internal/auth"
	"taskhub/internal/config"
	"taskhub/internal/db"
	"taskhub/internal/events"
	"taskhub/internal/executor"
	"taskhub/internal/handlers"
	"taskhub/internal/llm"
	"taskhub/internal/models"
	"taskhub/internal/orchestrator"
	"taskhub/internal/policy"
	"taskhub/internal/seed"
	"taskhub/internal/webhook"
)

func main() {
	ctx := context.Background()
	cfg := config.Load()

	log.Printf("TaskHub — mode: %s", cfg.Mode)
	if cfg.OpenAIAPIKey == "" {
		log.Println("WARNING: OPENAI_API_KEY not set -- task orchestration will fail")
	}

	// Database
	pool, err := db.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("db open: %v", err)
	}
	defer pool.Close()

	if err := db.RunMigrations(ctx, pool); err != nil {
		log.Fatalf("migrations: %v", err)
	}

	// Local mode: seed default user + org, no auth needed
	if cfg.IsLocal() {
		seed.LocalSeedAndLog(ctx, pool)
	}

	// Auth
	sessionStore := &auth.SessionStore{DB: pool}
	authMw := &auth.Middleware{Sessions: sessionStore, LocalMode: cfg.IsLocal()}
	authH := &auth.Handler{
		DB:           pool,
		Sessions:     sessionStore,
		GoogleID:     cfg.GoogleClientID,
		GoogleSecret: cfg.GoogleSecret,
		FrontendURL:  cfg.FrontendURL,
	}

	// Handlers
	a2aResolver := a2a.NewResolver()
	agentH := &handlers.AgentHandler{DB: pool, Resolver: a2aResolver} // Aggregator set below

	// Execution engine
	eventStore := &events.Store{DB: pool}
	broker := events.NewBroker()
	auditLogger := &audit.Logger{DB: pool}
	llmClient := llm.NewClient(cfg.OpenAIAPIKey)
	orch := &orchestrator.Orchestrator{LLM: llmClient}
	a2aClient := a2a.NewClient()
	policyEngine := policy.NewEngine(pool)

	webhookSender := webhook.NewSender(pool)

	exec := &executor.DAGExecutor{
		DB: pool, Broker: broker, EventStore: eventStore,
		Audit: auditLogger, Orchestrator: orch, A2AClient: a2aClient,
		PolicyEngine: policyEngine, WebhookSender: webhookSender,
	}
	exec.Recover(ctx)

	// A2A Server
	aggregator := a2a.NewAggregator(pool)
	agentH.Aggregator = aggregator
	a2aServer := &a2a.Server{
		DB:         pool,
		Aggregator: aggregator,
		BaseURL:    "http://localhost:" + cfg.Port,
		TaskExecutor: func(ctx context.Context, taskID string) error {
			var task models.Task
			err := pool.QueryRow(ctx,
				`SELECT id, title, description, status, created_by, replan_count,
				        COALESCE(template_id,''), COALESCE(template_version,0), created_at
				 FROM tasks WHERE id = $1`, taskID).
				Scan(&task.ID, &task.Title, &task.Description, &task.Status,
					&task.CreatedBy, &task.ReplanCount,
					&task.TemplateID, &task.TemplateVersion, &task.CreatedAt)
			if err != nil {
				return fmt.Errorf("load task %s: %w", taskID, err)
			}
			return exec.Execute(ctx, task)
		},
	}
	a2aConfigH := &handlers.A2AConfigHandler{
		DB:         pool,
		Aggregator: aggregator,
		BaseURL:    "http://localhost:" + cfg.Port,
	}

	// Start health checker (runs in background goroutine)
	healthChecker := &a2a.HealthChecker{
		DB:         pool,
		Resolver:   a2aResolver,
		Aggregator: aggregator,
		Broker:     broker,
		Interval:   2 * time.Minute,
	}
	go healthChecker.Start(ctx)

	webhookH := &handlers.WebhookHandler{DB: pool, Sender: webhookSender}
	policyH := &handlers.PolicyHandler{DB: pool}
	templateH := &handlers.TemplateHandler{DB: pool}
	evolH := &handlers.EvolutionHandler{DB: pool}
	taskH := &handlers.TaskHandler{DB: pool, Executor: exec, EventStore: eventStore, Audit: auditLogger}
	msgH := &handlers.MessageHandler{DB: pool, Executor: exec, EventStore: eventStore, Broker: broker}
	streamH := &handlers.StreamHandler{EventStore: eventStore, Broker: broker}
	convH := &handlers.ConversationHandler{
		DB: pool, Executor: exec, EventStore: eventStore,
		Broker: broker, Orchestrator: orch, Audit: auditLogger,
	}
	analyticsH := &handlers.AnalyticsHandler{DB: pool}
	traceH := &handlers.TraceHandler{DB: pool}
	agentHealthH := &handlers.AgentHealthHandler{DB: pool}
	agentStatusStreamH := &handlers.AgentStatusStreamHandler{Broker: broker, DB: pool}
	auditLogH := &handlers.AuditLogHandler{DB: pool}

	// Router
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{cfg.FrontendURL},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Content-Type", "Authorization", "Cookie"},
		AllowCredentials: true,
	}))

	// Health check
	r.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	// A2A public endpoints (no auth)
	r.Get("/.well-known/agent-card.json", a2aConfigH.ServeAgentCard)
	r.Post("/a2a", a2aServer.HandleJSONRPC)

	// Auth routes (public — no auth middleware)
	r.Post("/api/auth/login", authH.SimpleLogin)
	r.Post("/api/auth/google/login", authH.GoogleLogin)
	r.Get("/api/auth/google/callback", authH.GoogleCallback)
	r.Post("/api/auth/logout", authH.Logout)

	// All API routes — auth middleware handles local vs cloud
	r.Group(func(r chi.Router) {
		r.Use(authMw.RequireAuth)

		// Auth (authenticated)
		r.Get("/api/auth/me", authH.GetMe)

		// Conversations
		r.Get("/api/conversations", convH.List)
		r.Post("/api/conversations", convH.Create)
		r.Get("/api/conversations/{id}", convH.Get)
		r.Put("/api/conversations/{id}", convH.Update)
		r.Delete("/api/conversations/{id}", convH.Delete)
		r.Get("/api/conversations/{id}/messages", convH.GetMessages)
		r.Post("/api/conversations/{id}/messages", convH.SendMessage)
		r.Get("/api/conversations/{id}/tasks", convH.ListTasks)
		r.Get("/api/conversations/{id}/events", convH.Stream)

		// Agents
		r.Get("/api/agents", agentH.List)
		r.Post("/api/agents", agentH.Create)
		r.Get("/api/agents/health/overview", agentHealthH.GetOverview)
		r.Get("/api/agents/stream", agentStatusStreamH.Stream)
		r.Get("/api/agents/{id}", agentH.Get)
		r.Get("/api/agents/{id}/health", agentHealthH.GetHealth)
		r.Put("/api/agents/{id}", agentH.Update)
		r.Delete("/api/agents/{id}", agentH.Delete)
		r.Post("/api/agents/{id}/healthcheck", agentH.Healthcheck)
		r.Post("/api/agents/test-endpoint", agentH.TestEndpoint)
		r.Post("/api/agents/discover", agentH.Discover)

		// Tasks
		r.Post("/api/tasks", taskH.Create)
		r.Get("/api/tasks", taskH.List)
		r.Get("/api/tasks/{id}", taskH.Get)
		r.Post("/api/tasks/{id}/cancel", taskH.Cancel)
		r.Post("/api/tasks/{id}/approve", taskH.Approve)
		r.Get("/api/tasks/{id}/cost", taskH.GetCost)
		r.Get("/api/tasks/{id}/subtasks", taskH.ListSubtasks)

		// Messages
		r.Get("/api/tasks/{id}/messages", msgH.List)
		r.Post("/api/tasks/{id}/messages", msgH.Send)

		// SSE
		r.Get("/api/tasks/{id}/events", streamH.Stream)
		r.Get("/api/tasks/stream", streamH.MultiStream)

		// Task timeline
		r.Get("/api/tasks/{id}/timeline", traceH.GetTimeline)

		// Analytics
		r.Get("/api/analytics/dashboard", analyticsH.GetDashboard)
		r.Get("/api/analytics/agents/{id}/tasks", analyticsH.GetAgentTasks)

		// Audit logs
		r.Get("/api/audit-logs", auditLogH.List)

		// Webhooks
		r.Get("/api/webhooks", webhookH.List)
		r.Post("/api/webhooks", webhookH.Create)
		r.Put("/api/webhooks/{id}", webhookH.Update)
		r.Delete("/api/webhooks/{id}", webhookH.Delete)
		r.Post("/api/webhooks/{id}/test", webhookH.Test)

		// Policies
		r.Get("/api/policies", policyH.List)
		r.Post("/api/policies", policyH.Create)
		r.Put("/api/policies/{id}", policyH.Update)
		r.Delete("/api/policies/{id}", policyH.Delete)

		// Templates
		r.Get("/api/templates", templateH.List)
		r.Post("/api/templates", templateH.Create)
		r.Get("/api/templates/{id}", templateH.Get)
		r.Put("/api/templates/{id}", templateH.Update)
		r.Delete("/api/templates/{id}", templateH.Delete)
		r.Post("/api/templates/from-task/{task_id}", templateH.CreateFromTask)
		r.Post("/api/templates/{id}/rollback/{version}", templateH.Rollback)
		r.Get("/api/templates/{id}/executions", templateH.ListExecutions)
		r.Post("/api/templates/{id}/analyze", evolH.Analyze)

		// A2A config
		r.Get("/api/a2a-config", a2aConfigH.GetConfig)
		r.Put("/api/a2a-config", a2aConfigH.UpdateConfig)
		r.Post("/api/a2a-config/refresh-card", a2aConfigH.RefreshCard)
	})

	log.Printf("TaskHub listening on :%s", cfg.Port)
	if err := http.ListenAndServe(":"+cfg.Port, r); err != nil {
		log.Fatalf("listen: %v", err)
	}
}
