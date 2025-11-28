package server

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"google.golang.org/protobuf/proto"

	"github.com/besuhoff/dungeon-game-go/internal/auth"
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

// Client represents a connected client
type Client struct {
	ID         string
	UserID     primitive.ObjectID // MongoDB User ID
	Username   string
	SessionID  string             // Game session ID
	Conn       *websocket.Conn
	Send       chan []byte
	Server     *GameServer
	UseBinary  bool // Whether client prefers binary protocol
}

// Session represents a game session with its engine
type Session struct {
	ID         string
	Engine     *game.Engine
	PlayerCount int
	mu         sync.Mutex
}

// GameServer manages the game and all clients
type GameServer struct {
	clients    map[string]*Client
	sessions   map[string]*Session // sessionID -> Session
	register   chan *Client
	unregister chan *Client
	broadcast  chan []byte
	shutdown   chan struct{}
	mu         sync.RWMutex
	running    bool
}

// NewGameServer creates a new game server
func NewGameServer() *GameServer {
	return &GameServer{
		clients:    make(map[string]*Client),
		sessions:   make(map[string]*Session),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		broadcast:  make(chan []byte, 256),
		shutdown:   make(chan struct{}),
		running:    false,
	}
}

// Run starts the game server loop
func (gs *GameServer) Run() {
	gs.running = true
	ticker := time.NewTicker(50 * time.Millisecond) // ~20 FPS
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

func (gs *GameServer) registerClient(client *Client) {
	gs.mu.Lock()

	gs.clients[client.ID] = client

	// Get or create session
	session, exists := gs.sessions[client.SessionID]
	if !exists {
		// Create new session
		session = &Session{
			ID:          client.SessionID,
			Engine:      game.NewEngine(client.SessionID),
			PlayerCount: 0,
		}
		gs.sessions[client.SessionID] = session

		// Try to load existing session from database
		ctx := context.Background()
		sessionRepo := db.NewGameSessionRepository()
		
		if sessionID, err := primitive.ObjectIDFromHex(client.SessionID); err == nil {
			if dbSession, err := sessionRepo.FindByID(ctx, sessionID); err == nil {
				log.Printf("Loading existing session %s from database", client.SessionID)
				session.Engine.LoadFromSession(dbSession)
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
	player := session.Engine.AddPlayer(client.ID, client.Username)

	// Update user's current session in database
	ctx := context.Background()
	userRepo := db.NewUserRepository()
	if user, err := userRepo.FindByID(ctx, client.UserID); err == nil {
		user.CurrentSession = client.SessionID
		userRepo.Update(ctx, user)
	}

	// Notify all clients in this session about new player
	msg := types.Message{
		Type: types.MsgTypePlayerJoin,
		Payload: types.PlayerJoinPayload{
			Player: player,
		},
	}
	gs.broadcastToSession(client.SessionID, msg)

	// Send current game state to new player
	gameState := session.Engine.GetGameState()

	if client.UseBinary {
		client.sendBinaryGameState(gameState)
	} else {
		client.sendJSONGameState(gameState)
	}

	log.Printf("Player %s (%s) joined session %s (players: %d)", 
		client.Username, client.ID, client.SessionID, playerCount)
}

func (gs *GameServer) unregisterClient(client *Client) {
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
		
		sessionRepo := db.NewGameSessionRepository()
		if sessionID, err := primitive.ObjectIDFromHex(client.SessionID); err == nil {
			// Load or create database session
			dbSession, err := sessionRepo.FindByID(ctx, sessionID)
			if err != nil {
				// Create new session
				dbSession = &db.GameSession{
					ID:         sessionID,
					Name:       "Session " + client.SessionID[:8],
					HostID:     client.UserID,
					MaxPlayers: 10,
					IsActive:   true,
				}
				sessionRepo.Create(ctx, dbSession)
			}
			
			// Save engine state to session
			session.Engine.SaveToSession(dbSession)
			sessionRepo.Update(ctx, dbSession)
			
			log.Printf("Session %s saved to database", client.SessionID)
		}
		
		// Remove session from memory
		gs.mu.Lock()
		delete(gs.sessions, client.SessionID)
		gs.mu.Unlock()
		
		// Clear engine state
		session.Engine.Clear()
	} else {
		// Notify remaining clients in this session
		msg := types.Message{
			Type: types.MsgTypePlayerLeave,
			Payload: types.PlayerLeavePayload{
				PlayerID: client.ID,
			},
		}
		gs.broadcastToSession(client.SessionID, msg)
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

func (gs *GameServer) broadcastToSession(sessionID string, v interface{}) {
	data, err := json.Marshal(v)
	if err != nil {
		log.Printf("Error marshaling message: %v", err)
		return
	}

	gs.mu.RLock()
	defer gs.mu.RUnlock()

	for _, client := range gs.clients {
		if client.SessionID == sessionID {
			select {
			case client.Send <- data:
			default:
				// Client buffer full, skip
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
		// Get delta instead of full state
		delta := session.Engine.GetGameStateDelta()
		
		// Only broadcast if there are changes
		if delta.IsEmpty() {
			continue
		}
		
		gs.mu.RLock()
		for _, client := range gs.clients {
			if client.SessionID == sessionID {
				if client.UseBinary {
					client.sendBinaryGameStateDelta(delta)
				} else {
					client.sendJSONGameStateDelta(delta)
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

	// Get session ID from query parameter or use user's current session
	sessionID := r.URL.Query().Get("sessionId")
	if sessionID == "" {
		// Try to use user's current session
		if user.CurrentSession != "" {
			sessionID = user.CurrentSession
		} else {
			// Create a new session
			sessionRepo := db.NewGameSessionRepository()
			newSession := &db.GameSession{
				Name:       user.Username + "'s Game",
				HostID:     user.ID,
				MaxPlayers: 10,
				IsActive:   true,
			}
			if err := sessionRepo.Create(ctx, newSession); err != nil {
				http.Error(w, "Failed to create session", http.StatusInternalServerError)
				return
			}
			sessionID = newSession.ID.Hex()
			log.Printf("Created new session %s for user %s", sessionID, user.Username)
		}
	}

	// Upgrade to WebSocket
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}

	// Check if client wants binary protocol (via query parameter)
	useBinary := r.URL.Query().Get("protocol") == "binary"

	client := &Client{
		ID:        uuid.New().String(),
		UserID:    user.ID,
		Username:  user.Username,
		SessionID: sessionID,
		Conn:      conn,
		Send:      make(chan []byte, 256),
		Server:    gs,
		UseBinary: useBinary,
	}

	log.Printf("New client connected (ID: %s, User: %s, Session: %s, Binary: %v)", 
		client.ID, client.Username, client.SessionID, useBinary)

	// Start client goroutines
	go client.writePump()
	go client.readPump()

	// Register client
	gs.register <- client
}

// Client methods

func (c *Client) readPump() {
	defer func() {
		c.Server.unregister <- c
	}()

	c.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.Conn.SetPongHandler(func(string) error {
		c.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		messageType, message, err := c.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error: %v", err)
			}
			break
		}

		// Handle binary or text messages
		if messageType == websocket.BinaryMessage {
			c.handleBinaryMessage(message)
		} else {
			c.handleJSONMessage(message)
		}
	}
}

func (c *Client) writePump() {
	ticker := time.NewTicker(54 * time.Second)
	defer func() {
		ticker.Stop()
		c.Conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.Send:
			c.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			// Send as binary or text based on client preference
			msgType := websocket.TextMessage
			if c.UseBinary {
				msgType = websocket.BinaryMessage
			}

			if err := c.Conn.WriteMessage(msgType, message); err != nil {
				return
			}

		case <-ticker.C:
			c.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (c *Client) handleJSONMessage(data []byte) {
	var msg types.Message
	if err := json.Unmarshal(data, &msg); err != nil {
		log.Printf("Error unmarshaling JSON message: %v", err)
		return
	}

	// Get session engine
	c.Server.mu.RLock()
	session, exists := c.Server.sessions[c.SessionID]
	c.Server.mu.RUnlock()
	
	if !exists {
		log.Printf("Session %s not found for client %s", c.SessionID, c.ID)
		return
	}

	switch msg.Type {
	case types.MsgTypeConnect:
		var payload types.ConnectPayload
		if err := remarshal(msg.Payload, &payload); err == nil && payload.Username != "" {
			c.Username = payload.Username
			// Update player username in engine
			if player, exists := session.Engine.GetPlayer(c.ID); exists {
				player.Username = payload.Username
			}
		}

	case types.MsgTypeInput:
		var payload types.InputPayload
		if err := remarshal(msg.Payload, &payload); err == nil {
			session.Engine.UpdatePlayerInput(c.ID, payload)
		}

	case types.MsgTypeShoot:
		var payload types.ShootPayload
		if err := remarshal(msg.Payload, &payload); err == nil {
			session.Engine.Shoot(c.ID, payload.Direction)
		}
	}
}

func (c *Client) handleBinaryMessage(data []byte) {
	var msg protocol.GameMessage
	if err := proto.Unmarshal(data, &msg); err != nil {
		log.Printf("Error unmarshaling binary message: %v", err)
		return
	}

	// Get session engine
	c.Server.mu.RLock()
	session, exists := c.Server.sessions[c.SessionID]
	c.Server.mu.RUnlock()
	
	if !exists {
		log.Printf("Session %s not found for client %s", c.SessionID, c.ID)
		return
	}

	switch msg.Type {
	case protocol.MessageType_CONNECT:
		if connect := msg.GetConnect(); connect != nil {
			payload := protocol.FromProtoConnect(connect)
			if payload.Username != "" {
				c.Username = payload.Username
				if player, exists := session.Engine.GetPlayer(c.ID); exists {
					player.Username = payload.Username
				}
			}
		}

	case protocol.MessageType_INPUT:
		if input := msg.GetInput(); input != nil {
			payload := protocol.FromProtoInput(input)
			session.Engine.UpdatePlayerInput(c.ID, payload)
		}

	case protocol.MessageType_SHOOT:
		if shoot := msg.GetShoot(); shoot != nil {
			payload := protocol.FromProtoShoot(shoot)
			session.Engine.Shoot(c.ID, payload.Direction)
		}
	}
}

func (c *Client) sendJSONGameState(gameState types.GameState) {
	stateMsg := types.Message{
		Type:    types.MsgTypeGameState,
		Payload: gameState,
	}
	c.SendJSON(stateMsg)
}

func (c *Client) sendJSONGameStateDelta(delta types.GameStateDelta) {
	deltaMsg := types.Message{
		Type:    types.MsgTypeGameStateDelta,
		Payload: delta,
	}
	c.SendJSON(deltaMsg)
}

func (c *Client) SendJSON(v interface{}) {
	data, err := json.Marshal(v)
	if err != nil {
		log.Printf("Error marshaling message: %v", err)
		return
	}
	select {
	case c.Send <- data:
	default:
		// Buffer full
	}
}

func (c *Client) sendBinaryGameState(gameState types.GameState) {
	protoState := protocol.ToProtoGameState(gameState)
	msg := &protocol.GameMessage{
		Type: protocol.MessageType_GAME_STATE,
		Payload: &protocol.GameMessage_GameState{
			GameState: protoState,
		},
	}

	data, err := proto.Marshal(msg)
	if err != nil {
		log.Printf("Error marshaling binary game state: %v", err)
		return
	}

	select {
	case c.Send <- data:
	default:
		// Buffer full
	}
}

func (c *Client) sendBinaryGameStateDelta(delta types.GameStateDelta) {
	protoDelta := protocol.ToProtoGameStateDelta(delta)
	msg := &protocol.GameMessage{
		Type: protocol.MessageType_GAME_STATE_DELTA,
		Payload: &protocol.GameMessage_GameStateDelta{
			GameStateDelta: protoDelta,
		},
	}

	data, err := proto.Marshal(msg)
	if err != nil {
		log.Printf("Error marshaling binary game state delta: %v", err)
		return
	}

	select {
	case c.Send <- data:
	default:
		// Buffer full
	}
}

func (c *Client) SendBinary(msg *protocol.GameMessage) {
	data, err := proto.Marshal(msg)
	if err != nil {
		log.Printf("Error marshaling binary message: %v", err)
		return
	}
	select {
	case c.Send <- data:
	default:
		// Buffer full
	}
}

// Helper function to remarshal interface{} to specific type
func remarshal(from interface{}, to interface{}) error {
	data, err := json.Marshal(from)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, to)
}
