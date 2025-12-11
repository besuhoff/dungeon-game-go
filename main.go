package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/besuhoff/dungeon-game-go/internal/auth"
	"github.com/besuhoff/dungeon-game-go/internal/config"
	"github.com/besuhoff/dungeon-game-go/internal/db"
	"github.com/besuhoff/dungeon-game-go/internal/handlers"
	"github.com/besuhoff/dungeon-game-go/internal/server"
)

var (
	host     = flag.String("host", getEnv("HOST", "localhost"), "Host to listen on")
	port     = flag.String("port", getEnv("PORT", "8080"), "Port to listen on")
	certFile = flag.String("cert", getEnv("TLS_CERT", ""), "TLS certificate file (required for HTTPS)")
	keyFile  = flag.String("key", getEnv("TLS_KEY", ""), "TLS key file (required for HTTPS)")
	useTLS   = flag.Bool("tls", getEnv("USE_TLS", "") == "true", "Enable TLS/HTTPS")
)

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// CORS middleware
func corsMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Parse frontend domain from config
		frontendDomain := config.AppConfig.FrontendURL
		if idx := strings.Index(frontendDomain, "://"); idx != -1 {
			if pathIdx := strings.Index(frontendDomain[idx+3:], "/"); pathIdx != -1 {
				frontendDomain = frontendDomain[:idx+3+pathIdx]
			}
		}
		w.Header().Set("Access-Control-Allow-Origin", frontendDomain)
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Allow-Credentials", "true")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next(w, r)
	}
}

func main() {
	flag.Parse()

	// Load configuration
	cfg := config.LoadConfig()

	// Connect to MongoDB
	if err := db.Connect(cfg.MongoDBURL); err != nil {
		log.Fatal("Failed to connect to MongoDB: ", err)
	}
	defer db.Disconnect()

	log.Println("MongoDB connected successfully")

	// Create game server
	gameServer := server.NewGameServer()

	// Start game loop in background
	go gameServer.Run()

	// Setup auth handlers
	googleAuth := auth.NewGoogleAuthHandler()
	sessionHandler := handlers.NewSessionHandler()
	leaderboardHandler := handlers.NewLeaderboardHandler()

	// Setup HTTP routes
	http.HandleFunc("/ws", gameServer.HandleWebSocket)

	// Auth endpoints
	http.HandleFunc("/api/v1/auth/google/url", corsMiddleware(googleAuth.HandleGetAuthURL))
	http.HandleFunc("/api/v1/auth/google/callback", googleAuth.HandleCallback)
	http.HandleFunc("/api/v1/auth/user", corsMiddleware(googleAuth.HandleGetUser))

	// Session endpoints
	http.HandleFunc("/api/v1/sessions", corsMiddleware(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			sessionHandler.HandleCreateSession(w, r)
		} else if r.Method == http.MethodGet {
			sessionHandler.HandleListSessions(w, r)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}))
	http.HandleFunc("/api/v1/sessions/", corsMiddleware(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/join") {
			sessionHandler.HandleJoinSession(w, r)
		} else if r.Method == http.MethodDelete {
			sessionHandler.HandleDeleteSession(w, r)
		} else {
			http.Error(w, "Not found", http.StatusNotFound)
		}
	}))

	// Leaderboard endpoints
	http.HandleFunc("/api/v1/leaderboard/global", corsMiddleware(leaderboardHandler.HandleGetGlobalLeaderboard))

	// Health check
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Prepare address
	addr := fmt.Sprintf("%s:%s", *host, *port)

	// Create HTTP server with proper configuration
	httpServer := &http.Server{
		Addr:         addr,
		Handler:      nil, // Uses DefaultServeMux
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start HTTP/HTTPS server
	go func() {
		if *useTLS || *certFile != "" {
			if *certFile == "" || *keyFile == "" {
				log.Fatal("TLS enabled but certificate or key file not provided. Use -cert and -key flags or TLS_CERT and TLS_KEY environment variables.")
			}
			log.Printf("Starting game server with TLS on %s", addr)
			if err := httpServer.ListenAndServeTLS(*certFile, *keyFile); err != nil && err != http.ErrServerClosed {
				log.Fatal("ListenAndServeTLS error: ", err)
			}
		} else {
			log.Printf("Starting game server on %s", addr)
			if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				log.Fatal("ListenAndServe error: ", err)
			}
		}
	}()

	log.Println("Server started successfully")
	if *useTLS {
		log.Printf("WebSocket (JSON): wss://your-domain:%s/ws", *port)
		log.Printf("WebSocket (Binary): wss://your-domain:%s/ws?protocol=binary", *port)
	} else {
		log.Printf("WebSocket (JSON): ws://localhost:%s/ws", *port)
		log.Printf("WebSocket (Binary): ws://localhost:%s/ws?protocol=binary", *port)
	}

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	log.Println("Received shutdown signal, shutting down gracefully...")

	// Shutdown game server first (save sessions, close websockets)
	gameServer.Shutdown()

	// Shutdown HTTP server with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(ctx); err != nil {
		log.Printf("HTTP server shutdown error: %v", err)
	} else {
		log.Println("HTTP server shut down successfully")
	}

	log.Println("Server stopped")
}
