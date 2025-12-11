package server

import (
	"context"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/besuhoff/dungeon-game-go/internal/auth"
	"github.com/besuhoff/dungeon-game-go/internal/config"
	"github.com/besuhoff/dungeon-game-go/internal/db"
	"github.com/besuhoff/dungeon-game-go/internal/game"
	"github.com/besuhoff/dungeon-game-go/internal/protocol"
	"github.com/besuhoff/dungeon-game-go/internal/types"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins in development
	},
}

// Session represents a game session with its engine
type Session struct {
	ID                string
	Name              string
	Engine            *game.Engine
	PlayerCount       int
	mu                sync.Mutex
	lastSaveTime      time.Time
	deadPlayerTracked map[string]bool // Track which player deaths have been recorded
}

// GameServer manages the game and all clients
type GameServer struct {
	clients    map[string]*WebsocketClient
	sessions   map[string]*Session // sessionID -> Session
	register   chan *WebsocketClient
	unregister chan *WebsocketClient
	broadcast  chan []byte
	shutdown   chan struct{}
	mu         sync.RWMutex
	running    bool
}

// NewGameServer creates a new game server
func NewGameServer() *GameServer {
	return &GameServer{
		clients:    make(map[string]*WebsocketClient),
		sessions:   make(map[string]*Session),
		register:   make(chan *WebsocketClient),
		unregister: make(chan *WebsocketClient),
		broadcast:  make(chan []byte, 256),
		shutdown:   make(chan struct{}),
		running:    false,
	}
}

// Run starts the game server loop
func (gs *GameServer) Run() {
	gs.running = true
	ticker := time.NewTicker(config.GameLoopInterval)
	defer ticker.Stop()

	for {
		select {
		case <-gs.shutdown:
			log.Println("Game server loop shutting down...")
			return

		case client := <-gs.register:
			gs.registerClient(client)

		case client := <-gs.unregister:
			gs.unregisterClient(client)

		case message := <-gs.broadcast:
			gs.broadcastMessage(message)

		case <-ticker.C:
			// Update all active sessions
			gs.mu.RLock()
			for _, session := range gs.sessions {
				session.Engine.Update()
				if (session.lastSaveTime.IsZero() || time.Since(session.lastSaveTime) > config.SessionSaveInterval) && session.PlayerCount > 0 {
					gs.mu.RUnlock()
					gs.saveSessionToDatabase(session)
					gs.mu.RLock()
				}

				// Check for player deaths and update leaderboard
				for _, player := range session.Engine.GetAllPlayers() {
					session.mu.Lock()
					isTracked := session.deadPlayerTracked[player.ID]
					session.mu.Unlock()

					if !player.IsAlive && !isTracked {
						log.Printf("Player %s (ID: %s) died! Score: %d, Kills: %d", player.Username, player.ID, player.Score, player.Kills)

						// Mark this death as tracked to avoid duplicate entries
						session.mu.Lock()
						session.deadPlayerTracked[player.ID] = true
						session.mu.Unlock()

						// Update player score in leaderboard
						go func(p *types.Player, sessID, sessName string) {
							ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
							defer cancel()

							userID, err := primitive.ObjectIDFromHex(p.ID)
							if err != nil {
								log.Printf("Updating leaderboard: invalid player ID %s: %v", p.ID, err)
								return
							}

							leaderboardRepo := db.NewLeaderboardRepository()
							entry := &db.LeaderboardEntry{
								UserID:      userID,
								Username:    p.Username,
								SessionID:   sessID,
								SessionName: sessName,
								Score:       p.Score,
								Kills:       p.Kills,
							}
							if err := leaderboardRepo.UpsertEntry(ctx, entry); err != nil {
								log.Printf("Failed to update leaderboard entry for player %s: %v", p.Username, err)
							} else {
								log.Printf("Leaderboard updated for player %s: score=%d, kills=%d", p.Username, p.Score, p.Kills)
							}
						}(player, session.ID, session.Name)
					} else if player.IsAlive {
						// Reset tracking when player respawns
						session.mu.Lock()
						delete(session.deadPlayerTracked, player.ID)
						session.mu.Unlock()
					}
				}
			}
			gs.mu.RUnlock()

			// Broadcast game state for each session
			gs.broadcastAllSessionStates()
		}
	}
}

// Shutdown gracefully shuts down the server
func (gs *GameServer) Shutdown() {
	log.Println("Starting graceful shutdown...")

	// Signal the Run loop to stop
	close(gs.shutdown)

	// Give it a moment to process
	time.Sleep(100 * time.Millisecond)

	gs.mu.Lock()
	defer gs.mu.Unlock()

	// Close all client connections gracefully
	log.Printf("Closing %d client connections...", len(gs.clients))
	for id, client := range gs.clients {
		// Send close message to client
		client.Conn.WriteControl(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseNormalClosure, "Server shutting down"),
			time.Now().Add(time.Second))
		client.Conn.Close()
		delete(gs.clients, id)
	}

	// Save all active sessions to database
	log.Printf("Saving %d active sessions to database...", len(gs.sessions))
	ctx := context.Background()
	sessionRepo := db.NewGameSessionRepository()

	for sessionID, session := range gs.sessions {
		if sessionObjID, err := primitive.ObjectIDFromHex(sessionID); err == nil {
			if dbSession, err := sessionRepo.FindByID(ctx, sessionObjID); err == nil {
				session.Engine.SaveToSession(dbSession)
				sessionRepo.Update(ctx, dbSession)
				log.Printf("Saved session %s", sessionID)
			}
		}
	}

	log.Println("Graceful shutdown complete")
}

func (gs *GameServer) registerClient(client *WebsocketClient) {
	gs.mu.Lock()

	gs.clients[client.ID] = client

	// Get or create session
	session, exists := gs.sessions[client.SessionID]
	if !exists {
		// Create new session
		session = &Session{
			ID:                client.SessionID,
			Name:              client.SessionName,
			Engine:            game.NewEngine(client.SessionID),
			PlayerCount:       0,
			deadPlayerTracked: make(map[string]bool),
		}
		gs.sessions[client.SessionID] = session

		// Try to load existing session from database
		ctx := context.Background()
		sessionRepo := db.NewGameSessionRepository()

		if sessionID, err := primitive.ObjectIDFromHex(client.SessionID); err == nil {
			if dbSession, err := sessionRepo.FindByID(ctx, sessionID); err == nil {
				log.Printf("Loading existing session %s from database", client.SessionID)
				session.Engine.LoadFromSession(dbSession)
				session.lastSaveTime = time.Now()
			} else {
				log.Printf("Creating new session %s", client.SessionID)
			}
		}
	}

	session.mu.Lock()
	session.PlayerCount++
	playerCount := session.PlayerCount
	session.mu.Unlock()

	// Unlock before calling methods that need to acquire locks
	gs.mu.Unlock()

	// Add player to game engine
	player := session.Engine.AddPlayer(client.UserID.Hex(), client.Username)

	// Update user's current session in database
	ctx := context.Background()
	userRepo := db.NewUserRepository()
	if user, err := userRepo.FindByID(ctx, client.UserID); err == nil {
		user.CurrentSession = client.SessionID
		userRepo.Update(ctx, user)
	}

	gs.broadcastPlayerJoinedMessage(client.SessionID, player)

	client.SendGameState(session.Engine.GetGameStateForPlayer(player.ID))

	log.Printf("Player %s (%s) joined session %s (players: %d)",
		client.Username, client.UserID.Hex(), client.SessionID, playerCount)
}

func (gs *GameServer) saveSessionToDatabase(session *Session) {
	ctx := context.Background()
	sessionRepo := db.NewGameSessionRepository()
	if sessionObjectID, err := primitive.ObjectIDFromHex(session.ID); err == nil {
		// Load or create database session
		dbSession, err := sessionRepo.FindByID(ctx, sessionObjectID)
		if err != nil {
			// Create new session
			dbSession = &db.GameSession{
				ID:         sessionObjectID,
				Name:       "Session " + session.ID[:8],
				MaxPlayers: 10,
				IsActive:   true,
			}
			sessionRepo.Create(ctx, dbSession)
		}

		// Save engine state to session
		session.Engine.SaveToSession(dbSession)
		sessionRepo.Update(ctx, dbSession)
		session.lastSaveTime = time.Now()

		log.Printf("Session %s saved to database", session.ID)
	}
}

func (gs *GameServer) unregisterClient(client *WebsocketClient) {
	gs.mu.Lock()
	_, exists := gs.clients[client.ID]
	if exists {
		delete(gs.clients, client.ID)
	}

	session, sessionExists := gs.sessions[client.SessionID]
	gs.mu.Unlock()

	if !exists {
		return
	}

	if !sessionExists {
		return
	}

	// Remove player from game engine
	session.Engine.RemovePlayer(client.ID)

	// Decrement player count
	session.mu.Lock()
	session.PlayerCount--
	playerCount := session.PlayerCount
	session.mu.Unlock()

	// Clear user's current session in database
	ctx := context.Background()
	userRepo := db.NewUserRepository()
	if user, err := userRepo.FindByID(ctx, client.UserID); err == nil {
		user.CurrentSession = ""
		userRepo.Update(ctx, user)
	}

	// If this was the last player, save session to database and clear from memory
	if playerCount == 0 {
		log.Printf("Last player left session %s, saving to database", client.SessionID)

		// Save session to database
		gs.saveSessionToDatabase(session)

		// Remove session from memory
		gs.mu.Lock()
		delete(gs.sessions, client.SessionID)
		gs.mu.Unlock()

		// Clear engine state
		session.Engine.Clear()
	} else {
		gs.broadcastPlayerLeftMessage(client.SessionID, client.ID)
	}

	log.Printf("Player %s (%s) left session %s (remaining: %d)",
		client.Username, client.ID, client.SessionID, playerCount)
}

func (gs *GameServer) broadcastMessage(message []byte) {
	gs.mu.RLock()
	defer gs.mu.RUnlock()

	for _, client := range gs.clients {
		select {
		case client.Send <- message:
		default:
			// Client buffer full, skip
		}
	}
}

func (gs *GameServer) broadcastPlayerJoinedMessage(sessionID string, player *types.Player) {
	msg := &protocol.GameMessage{
		Type: protocol.MessageType_PLAYER_JOIN,
		Payload: &protocol.GameMessage_PlayerJoin{
			PlayerJoin: &protocol.PlayerJoinMessage{
				Player: protocol.ToProtoPlayer(player),
			},
		},
	}

	gs.broadcastToSession(sessionID, msg, player.ID)
}

func (gs *GameServer) broadcastPlayerLeftMessage(sessionID string, playerID string) {
	msg := &protocol.GameMessage{
		Type: protocol.MessageType_PLAYER_LEAVE,
		Payload: &protocol.GameMessage_PlayerLeave{
			PlayerLeave: &protocol.PlayerLeaveMessage{
				PlayerId: playerID,
			},
		},
	}

	gs.broadcastToSession(sessionID, msg, playerID)
}

func (gs *GameServer) broadcastToSession(sessionID string, msg *protocol.GameMessage, excludeClientId string) {
	gs.mu.RLock()
	defer gs.mu.RUnlock()

	for _, client := range gs.clients {
		if client.SessionID == sessionID && client.UserID.Hex() != excludeClientId {
			if client.UseBinary {
				client.SendBinary(msg)
			} else {
				client.SendJSON(msg)
			}
		}
	}
}

func (gs *GameServer) broadcastAllSessionStates() {
	gs.mu.RLock()
	sessions := make(map[string]*Session)
	for id, session := range gs.sessions {
		sessions[id] = session
	}
	gs.mu.RUnlock()

	for sessionID, session := range sessions {
		// Send individualized delta to each player in the session
		gs.mu.RLock()
		for _, client := range gs.clients {
			if client.SessionID == sessionID {
				// Get player-specific delta (filtered to surrounding chunks)
				delta := session.Engine.GetGameStateDeltaForPlayer(client.UserID.Hex())

				// Only send if there are changes
				if !delta.IsEmpty() {
					client.SendGameStateDelta(delta)
				}
			}
		}
		gs.mu.RUnlock()
	}
}

// HandleWebSocket handles WebSocket connections
func (gs *GameServer) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	// Extract and validate JWT token from query parameters
	token := r.URL.Query().Get("token")
	if token == "" {
		// Check Authorization header as fallback
		authHeader := r.Header.Get("Authorization")
		if strings.HasPrefix(authHeader, "Bearer ") {
			token = strings.TrimPrefix(authHeader, "Bearer ")
		}
	}

	if token == "" {
		http.Error(w, "Unauthorized: missing token", http.StatusUnauthorized)
		return
	}

	// Validate JWT token
	userID, err := auth.ValidateToken(token)
	if err != nil {
		log.Printf("Token validation error: %v", err)
		http.Error(w, "Unauthorized: invalid token", http.StatusUnauthorized)
		return
	}

	// Fetch user from database
	ctx := context.Background()
	userRepo := db.NewUserRepository()
	user, err := userRepo.FindByID(ctx, userID)
	if err != nil {
		log.Printf("User lookup error: %v", err)
		http.Error(w, "Unauthorized: user not found", http.StatusUnauthorized)
		return
	}

	sessionID := r.URL.Query().Get("sessionId")
	sessionRepo := db.NewGameSessionRepository()
	sessionObjID, err := primitive.ObjectIDFromHex(sessionID)
	if err != nil {
		http.Error(w, "Invalid session ID", http.StatusBadRequest)
		return
	}

	session, err := sessionRepo.FindByID(ctx, sessionObjID)
	if err != nil {
		http.Error(w, "Session not found", http.StatusNotFound)
		return
	}

	// Upgrade to WebSocket
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}

	// Check if client wants binary protocol (via query parameter)
	useBinary := r.URL.Query().Get("protocol") == "binary"

	client := &WebsocketClient{
		ID:          uuid.New().String(),
		UserID:      user.ID,
		Username:    user.Username,
		SessionID:   sessionID,
		SessionName: session.Name,
		Conn:        conn,
		Send:        make(chan []byte, 256),
		Server:      gs,
		UseBinary:   useBinary,
	}

	log.Printf("New client connected (ID: %s, User: %s, Session: %s, Binary: %v)",
		client.ID, client.Username, client.SessionID, useBinary)

	// Start client goroutines
	go client.writePump()
	go client.readPump()

	// Register client
	gs.register <- client
}
