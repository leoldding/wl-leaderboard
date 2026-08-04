package main

import (
	"context"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	authclient "github.com/leoldding/auth-middleware/client"
	authmiddleware "github.com/leoldding/auth-middleware/middleware"

	"github.com/leoldding/wl-leaderboard/internal/awsclients"
	"github.com/leoldding/wl-leaderboard/internal/config"
	"github.com/leoldding/wl-leaderboard/internal/handlers"
	"github.com/leoldding/wl-leaderboard/internal/middlewares"
)

func main() {
	clients, err := awsclients.New()
	if err != nil {
		log.Fatalf("failed to initialize AWS clients: %v", err)
	}

	cfg, err := config.Load(context.Background(), clients.SSM)
	if err != nil {
		log.Fatal(err)
	}

	authClient := authclient.NewAuthClient(cfg.AuthServiceURL)

	leaderboardHandler := handlers.NewLeaderboardHandler(
		clients,
		cfg.DynamoDBTable,
	)

	r := chi.NewRouter()

	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(authmiddleware.RequestID)

	r.Get("/api/v1/leaderboard/health", healthcheck)

	r.Route("/api/v1/leaderboard", func(r chi.Router) {
		r.Use(middlewares.CORS())

		r.Get("/", leaderboardHandler.GetRows)
		r.With(authmiddleware.AuthCheck(authClient)).
			Post("/", leaderboardHandler.AddRow)
		r.With(authmiddleware.AuthCheck(authClient)).
			Put("/{id}", leaderboardHandler.UpdateRow)
		r.With(authmiddleware.AuthCheck(authClient)).
			Delete("/{id}", leaderboardHandler.DeleteRow)
	})

	log.Println("leaderboard service running on :8085")

	if err := http.ListenAndServe(":8085", r); err != nil {
		log.Fatalf("server exited: %v", err)
	}
}

func healthcheck(w http.ResponseWriter, _ *http.Request) {
	w.Write([]byte("leaderboard service is healthy"))
}
