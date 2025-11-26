package server

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/google/uuid"
	"github.com/besuhoff/dungeon-game-go/internal/game"
	"github.com/besuhoff/dungeon-game-go/internal/types"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins in development
	},
}

// Client represents a connected client
type Client struct {
	ID       string
	Username string
	Conn     *websocket.Conn
	Send     chan []byte
	Server   *GameServer
}

// GameServer manages the game and all clients
type GameServer struct {
	clients    map[string]*Client
	engine     *game.Engine
	register   chan *Client
	unregister chan *Client
	broadcast  chan []byte
	mu         sync.RWMutex
	running    bool
}

// NewGameServer creates a new game server
func NewGameServer() *GameServer {
	return &GameServer{
		clients:    make(map[string]*Client),
		engine:     game.NewEngine(),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		broadcast:  make(chan []byte, 256),
		running:    false,
	}
}

// Run starts the game server loop
func (gs *GameServer) Run() {
	gs.running = true
	ticker := time.NewTicker(16 * time.Millisecond) // ~60 FPS
	defer ticker.Stop()

	for gs.running {
		select {
		case client := <-gs.register:
			gs.registerClient(client)

		case client := <-gs.unregister:
			gs.unregisterClient(client)

		case message := <-gs.broadcast:
			gs.broadcastMessage(message)

		case <-ticker.C:
			gs.engine.Update()
			gs.broadcastGameState()
		}
	}
}

// Shutdown gracefully shuts down the server
func (gs *GameServer) Shutdown() {
	gs.running = false
	gs.mu.Lock()
	defer gs.mu.Unlock()

	for _, client := range gs.clients {
		close(client.Send)
		client.Conn.Close()
	}
}

func (gs *GameServer) registerClient(client *Client) {
	gs.mu.Lock()
	gs.clients[client.ID] = client
	gs.mu.Unlock()

	// Add player to game
	player := gs.engine.AddPlayer(client.ID, client.Username)

	// Notify all clients about new player
	msg := types.Message{
		Type: types.MsgTypePlayerJoin,
		Payload: types.PlayerJoinPayload{
			Player: player,
		},
	}
	gs.broadcastJSON(msg)

	// Send current game state to new player
	gameState := gs.engine.GetGameState()
	stateMsg := types.Message{
		Type:    types.MsgTypeGameState,
		Payload: gameState,
	}
	client.SendJSON(stateMsg)

	log.Printf("Player %s (%s) joined", client.Username, client.ID)
}

func (gs *GameServer) unregisterClient(client *Client) {
	gs.mu.Lock()
	if _, ok := gs.clients[client.ID]; ok {
		delete(gs.clients, client.ID)
		close(client.Send)
		client.Conn.Close()
	}
	gs.mu.Unlock()

	// Remove player from game
	gs.engine.RemovePlayer(client.ID)

	// Notify all clients
	msg := types.Message{
		Type: types.MsgTypePlayerLeave,
		Payload: types.PlayerLeavePayload{
			PlayerID: client.ID,
		},
	}
	gs.broadcastJSON(msg)

	log.Printf("Player %s (%s) left", client.Username, client.ID)
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

func (gs *GameServer) broadcastJSON(v interface{}) {
	data, err := json.Marshal(v)
	if err != nil {
		log.Printf("Error marshaling message: %v", err)
		return
	}
	gs.broadcast <- data
}

func (gs *GameServer) broadcastGameState() {
	gameState := gs.engine.GetGameState()
	msg := types.Message{
		Type:    types.MsgTypeGameState,
		Payload: gameState,
	}
	gs.broadcastJSON(msg)
}

// HandleWebSocket handles WebSocket connections
func (gs *GameServer) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}

	client := &Client{
		ID:       uuid.New().String(),
		Username: "Player_" + uuid.New().String()[:8],
		Conn:     conn,
		Send:     make(chan []byte, 256),
		Server:   gs,
	}

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
		_, message, err := c.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error: %v", err)
			}
			break
		}

		c.handleMessage(message)
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

			if err := c.Conn.WriteMessage(websocket.TextMessage, message); err != nil {
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

func (c *Client) handleMessage(data []byte) {
	var msg types.Message
	if err := json.Unmarshal(data, &msg); err != nil {
		log.Printf("Error unmarshaling message: %v", err)
		return
	}

	switch msg.Type {
	case types.MsgTypeConnect:
		var payload types.ConnectPayload
		if err := remarshal(msg.Payload, &payload); err == nil && payload.Username != "" {
			c.Username = payload.Username
			// Update player username in engine
			if player, exists := c.Server.engine.GetPlayer(c.ID); exists {
				player.Username = payload.Username
			}
		}

	case types.MsgTypeInput:
		var payload types.InputPayload
		if err := remarshal(msg.Payload, &payload); err == nil {
			c.Server.engine.UpdatePlayerInput(c.ID, payload)
		}

	case types.MsgTypeShoot:
		var payload types.ShootPayload
		if err := remarshal(msg.Payload, &payload); err == nil {
			c.Server.engine.Shoot(c.ID, payload.Direction)
		}
	}
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

// Helper function to remarshal interface{} to specific type
func remarshal(from interface{}, to interface{}) error {
	data, err := json.Marshal(from)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, to)
}
