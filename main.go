package main

import (
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/besuhoff/dungeon-game-go/internal/server"
)

func main() {
	// Create game server
	gameServer := server.NewGameServer()
	
	// Start game loop in background
	go gameServer.Run()

	// Setup HTTP routes
	http.HandleFunc("/ws", gameServer.HandleWebSocket)
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Start HTTP server
	port := ":8080"
	log.Printf("Starting game server on %s", port)
	
	go func() {
		if err := http.ListenAndServe(port, nil); err != nil {
			log.Fatal("ListenAndServe error: ", err)
		}
	}()

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	log.Println("Shutting down server...")
	gameServer.Shutdown()
}
