package main

import (
	"context"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"taskhub/internal/config"
	"taskhub/internal/db"
	"taskhub/internal/handlers"
	"taskhub/internal/master"
	oai "taskhub/internal/openai"
	"taskhub/internal/seed"
	"taskhub/internal/sse"
)

func main() {
	cfg := config.Load()

	database, err := db.Open(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer database.Close()

	if err := db.RunMigrations(database); err != nil {
		log.Fatalf("run migrations: %v", err)
	}
	log.Println("migrations OK")

	if err := seed.Run(context.Background(), database); err != nil {
		log.Fatalf("seed: %v", err)
	}

	openaiClient := oai.New(cfg.OpenAIAPIKey)
	broker := sse.NewBroker()

	masterAgent := &master.Agent{
		DB:     database,
		OpenAI: openaiClient,
		Broker: broker,
	}

	agentH := &handlers.AgentHandler{DB: database}
	taskH := &handlers.TaskHandler{DB: database, Master: masterAgent}
	channelH := &handlers.ChannelHandler{DB: database, Broker: broker}

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"http://localhost:3000"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Content-Type"},
		AllowCredentials: false,
		MaxAge:           300,
	}))

	r.Route("/api", func(r chi.Router) {
		r.Get("/agents", agentH.List)
		r.Post("/agents", agentH.Create)
		r.Get("/tasks", taskH.List)
		r.Post("/tasks", taskH.Create)
		r.Get("/tasks/{id}", taskH.Get)
		r.Get("/tasks/{id}/channel", taskH.GetChannel)
		r.Get("/channels/{id}", channelH.Get)
		r.Get("/channels/{id}/stream", channelH.Stream)
	})

	log.Printf("server listening on :%s", cfg.Port)
	if err := http.ListenAndServe(":"+cfg.Port, r); err != nil {
		log.Fatalf("listen: %v", err)
	}
}
