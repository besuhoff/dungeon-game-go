package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/besuhoff/dungeon-game-go/internal/auth"
	"github.com/besuhoff/dungeon-game-go/internal/config"
	"github.com/besuhoff/dungeon-game-go/internal/db"
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
	
	// Setup HTTP routes
	http.HandleFunc("/ws", gameServer.HandleWebSocket)
	http.HandleFunc("/api/v1/auth/google/url", googleAuth.HandleGetAuthURL)
	http.HandleFunc("/api/v1/auth/google/callback", googleAuth.HandleCallback)
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Prepare address
	addr := fmt.Sprintf("%s:%s", *host, *port)
	
	// Start HTTP/HTTPS server
	go func() {
		if *useTLS || *certFile != "" {
			if *certFile == "" || *keyFile == "" {
				log.Fatal("TLS enabled but certificate or key file not provided. Use -cert and -key flags or TLS_CERT and TLS_KEY environment variables.")
			}
			log.Printf("Starting game server with TLS on %s", addr)
			if err := http.ListenAndServeTLS(addr, *certFile, *keyFile, nil); err != nil {
				log.Fatal("ListenAndServeTLS error: ", err)
			}
		} else {
			log.Printf("Starting game server on %s", addr)
			if err := http.ListenAndServe(addr, nil); err != nil {
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

	log.Println("Shutting down server...")
	gameServer.Shutdown()
}
