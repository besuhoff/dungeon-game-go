package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/besuhoff/dungeon-game-go/internal/auth"
	"github.com/besuhoff/dungeon-game-go/internal/config"
	"github.com/besuhoff/dungeon-game-go/internal/db"
	"github.com/besuhoff/dungeon-game-go/internal/types"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// SessionHandler handles session-related HTTP requests
type SessionHandler struct {
	sessionRepo *db.GameSessionRepository
	userRepo    *db.UserRepository
}

// NewSessionHandler creates a new session handler
func NewSessionHandler() *SessionHandler {
	return &SessionHandler{
		sessionRepo: db.NewGameSessionRepository(),
		userRepo:    db.NewUserRepository(),
	}
}

// CreateSessionRequest represents the request body for creating a session
type CreateSessionRequest struct {
	Name       string `json:"name"`
	MaxPlayers int    `json:"max_players"`
	IsPrivate  bool   `json:"is_private"`
	Password   string `json:"password,omitempty"`
}

// SessionResponse represents a game session response
type SessionResponse struct {
	ID            string                    `json:"id"`
	Name          string                    `json:"name"`
	Host          UserResponse              `json:"host"`
	MaxPlayers    int                       `json:"max_players"`
	IsPrivate     bool                      `json:"is_private"`
	WorldMap      map[string]db.Chunk       `json:"world_map"`
	SharedObjects map[string]db.WorldObject `json:"shared_objects"`
	GameState     map[string]interface{}    `json:"game_state"`
	PlayerRoles   map[string]string         `json:"player_roles"`
	Players       map[string]db.PlayerState `json:"players"`
	CreatedAt     string                    `json:"created_at"`
	IsActive      bool                      `json:"is_active"`
}

// UserResponse represents a user in responses
type UserResponse struct {
	ID             string `json:"id"`
	Email          string `json:"email"`
	Username       string `json:"username"`
	GoogleID       string `json:"google_id,omitempty"`
	IsActive       bool   `json:"is_active"`
	CurrentSession string `json:"current_session,omitempty"`
	CreatedAt      string `json:"created_at"`
}

// getCurrentUser extracts and validates the JWT token, returning the user
func (h *SessionHandler) getCurrentUser(r *http.Request) (*db.User, error) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return nil, http.ErrNoCookie
	}

	token := strings.TrimPrefix(authHeader, "Bearer ")
	userID, err := auth.ValidateToken(token)
	if err != nil {
		return nil, err
	}

	ctx := context.Background()
	return h.userRepo.FindByID(ctx, userID)
}

// HandleCreateSession creates a new game session
func (h *SessionHandler) HandleCreateSession(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	user, err := h.getCurrentUser(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req CreateSessionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Name == "" || len(req.Name) > 50 {
		http.Error(w, "Name must be between 1 and 50 characters", http.StatusBadRequest)
		return
	}

	if req.MaxPlayers == 0 {
		req.MaxPlayers = 10
	}

	ctx := context.Background()
	session := &db.GameSession{
		Name:       req.Name,
		HostID:     user.ID,
		MaxPlayers: req.MaxPlayers,
		IsPrivate:  req.IsPrivate,
		Password:   req.Password,
		Players:    map[string]db.PlayerState{},
	}

	if err := h.sessionRepo.Create(ctx, session); err != nil {
		http.Error(w, "Failed to create session", http.StatusInternalServerError)
		return
	}

	// Update user's current session
	user.CurrentSession = session.ID.Hex()
	h.userRepo.Update(ctx, user)

	response := h.sessionToResponse(session, user)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response)
}

// HandleListSessions lists all active sessions
func (h *SessionHandler) HandleListSessions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	_, err := h.getCurrentUser(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	ctx := context.Background()
	sessions, err := h.sessionRepo.FindActiveSessions(ctx)
	if err != nil {
		http.Error(w, "Failed to fetch sessions", http.StatusInternalServerError)
		return
	}

	responses := make([]SessionResponse, 0, len(sessions))
	for _, session := range sessions {
		host, err := h.userRepo.FindByID(ctx, session.HostID)
		if err != nil {
			continue
		}
		responses = append(responses, h.sessionToResponse(&session, host))
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(responses)
}

// HandleJoinSession joins an existing session
func (h *SessionHandler) HandleJoinSession(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	user, err := h.getCurrentUser(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Extract session ID from URL path
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/sessions/")
	sessionIDStr := strings.TrimSuffix(path, "/join")

	sessionID, err := primitive.ObjectIDFromHex(sessionIDStr)
	if err != nil {
		http.Error(w, "Invalid session ID", http.StatusBadRequest)
		return
	}

	var body struct {
		Password string `json:"password"`
	}
	json.NewDecoder(r.Body).Decode(&body)

	ctx := context.Background()
	session, err := h.sessionRepo.FindByID(ctx, sessionID)
	if err != nil {
		http.Error(w, "Session not found", http.StatusNotFound)
		return
	}

	playerID := user.ID.Hex()
	if len(session.Players) >= session.MaxPlayers {
		_, ok := session.Players[playerID]
		// Allow re-joining if already in session
		if !ok {
			http.Error(w, "Session is full", http.StatusBadRequest)
			return
		}
	}

	if session.IsPrivate && session.Password != body.Password {
		http.Error(w, "Invalid password", http.StatusForbidden)
		return
	}

	// Add player to session
	if _, ok := session.Players[playerID]; !ok {
		session.Players[playerID] = db.PlayerState{
			PlayerID:    playerID,
			Name:        user.Username,
			Position:    db.Position{X: 0, Y: 0, Rotation: 0},
			Lives:       config.PlayerLives,
			IsAlive:     true,
			IsConnected: false,
			BulletsLeftByWeaponType: map[string]int32{
				types.WeaponTypeBlaster: config.BlasterMaxBullets,
			},
			InvulnerableTimer: config.PlayerSpawnInvulnerabilityTime,
		}

		if err := h.sessionRepo.Update(ctx, session); err != nil {
			http.Error(w, "Failed to join session", http.StatusInternalServerError)
			return
		}
	}

	// Update user's current session
	user.CurrentSession = session.ID.Hex()
	h.userRepo.Update(ctx, user)

	// Prepare environment for the player

	host, _ := h.userRepo.FindByID(ctx, session.HostID)
	response := h.sessionToResponse(session, host)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// HandleDeleteSession leaves a session
func (h *SessionHandler) HandleDeleteSession(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	user, err := h.getCurrentUser(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Extract session ID from URL path
	sessionIDStr := strings.TrimPrefix(r.URL.Path, "/api/v1/sessions/")

	sessionID, err := primitive.ObjectIDFromHex(sessionIDStr)
	if err != nil {
		http.Error(w, "Invalid session ID", http.StatusBadRequest)
		return
	}

	ctx := context.Background()
	session, err := h.sessionRepo.FindByID(ctx, sessionID)
	if err != nil {
		http.Error(w, "Session not found", http.StatusNotFound)
		return
	}

	if session.HostID != user.ID {
		http.Error(w, "Only the host can delete the session", http.StatusForbidden)
		return
	}

	h.sessionRepo.Delete(ctx, session.ID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "Successfully deleted session"})
}

// sessionToResponse converts a session to a response object
func (h *SessionHandler) sessionToResponse(session *db.GameSession, host *db.User) SessionResponse {
	return SessionResponse{
		ID:   session.ID.Hex(),
		Name: session.Name,
		Host: UserResponse{
			ID:             host.ID.Hex(),
			Email:          host.Email,
			Username:       host.Username,
			GoogleID:       host.GoogleID,
			IsActive:       host.IsActive,
			CurrentSession: host.CurrentSession,
			CreatedAt:      host.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		},
		MaxPlayers:    session.MaxPlayers,
		IsPrivate:     session.IsPrivate,
		WorldMap:      session.WorldMap,
		SharedObjects: session.SharedObjects,
		Players:       session.Players,
		CreatedAt:     session.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		IsActive:      session.IsActive,
	}
}
