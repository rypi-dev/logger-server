package main

import (
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
	"github.com/joho/godotenv"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"logger-server/internal"
)

func main() {
	// Chargement configuration
	apiKey := os.Getenv("LOGGER_API_KEY")
	if apiKey == "" {
		log.Fatal("LOGGER_API_KEY is not set")
	}

	dbPath := os.Getenv("LOGGER_DB_PATH")
	if dbPath == "" {
		dbPath = "logs.sqlite"
	}

	maxRows := 10000

	// Initialiser le logger SQLite
	sqlLogger, err := internal.NewSQLiteLogger(dbPath, maxRows)
	if err != nil {
		log.Fatalf("failed to initialize SQLite logger: %v", err)
	}
	defer sqlLogger.Close()

	// Initialiser rate limiter : 100 requêtes / minute / IP
	rateLimiter := internal.NewRateLimiter(100, time.Minute)
	defer rateLimiter.Stop()

	// Créer le handler principal
	handler := internal.NewHandler(sqlLogger)

	r := handler.Router()
	r.Handle("/metrics", promhttp.Handler())

	// Chaîne des middlewares : RateLimit → APIKey → Handler
	mux := rateLimiter.Middleware(
		internal.ApiKeyMiddleware(apiKey, r),
	)

	// Configuration serveur HTTP
	srv := &http.Server{
		Addr:         ":8080",
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Gestion arrêt propre
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	go func() {
		log.Println("Logger server is running on :8080")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	// Attendre signal pour arrêter
	<-stop
	log.Println("Shutdown signal received. Shutting down...")

	// Shutdown propre (timeout)
	shutdownTimeout := 5 * time.Second
	shutdownCh := make(chan struct{})

	go func() {
		srv.Close()
		close(shutdownCh)
	}()

	select {
	case <-shutdownCh:
		log.Println("Shutdown complete.")
	case <-time.After(shutdownTimeout):
		log.Println("Shutdown timed out.")
	}
}