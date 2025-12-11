package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/besuhoff/dungeon-game-go/internal/db"
)

// LeaderboardHandler handles leaderboard-related HTTP requests
type LeaderboardHandler struct {
	sessionRepo *db.GameSessionRepository
	userRepo    *db.UserRepository
}

// NewLeaderboardHandler creates a new leaderboard handler
func NewLeaderboardHandler() *LeaderboardHandler {
	return &LeaderboardHandler{
		sessionRepo: db.NewGameSessionRepository(),
		userRepo:    db.NewUserRepository(),
	}
}

// LeaderboardEntry represents an entry in the leaderboard
type LeaderboardEntry struct {
	Username    string `json:"username"`
	Score       int    `json:"score"`
	SessionID   string `json:"sessionId"`
	SessionName string `json:"sessionName"`
	CreatedAt   string `json:"createdAt"`
}

// UserStats represents user statistics
type UserStats struct {
	TotalGames   int     `json:"total_games"`
	HighestScore int     `json:"highest_score"`
	AverageScore float64 `json:"average_score"`
	RecentScores []struct {
		Score     int    `json:"score"`
		SessionID string `json:"session_id"`
		CreatedAt string `json:"created_at"`
	} `json:"recent_scores"`
}

// HandleGetGlobalLeaderboard returns the global leaderboard
func (h *LeaderboardHandler) HandleGetGlobalLeaderboard(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse query parameters
	limitStr := r.URL.Query().Get("limit")
	limit := 100
	if limitStr != "" {
		if val, err := strconv.Atoi(limitStr); err == nil && val > 0 {
			limit = val
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	leaderboardRepo := db.NewLeaderboardRepository()
	dbEntries, err := leaderboardRepo.GetTopScores(ctx, limit)
	if err != nil {
		http.Error(w, "Failed to fetch leaderboard", http.StatusInternalServerError)
		return
	}

	// Convert to response format
	entries := make([]LeaderboardEntry, len(dbEntries))
	for i, entry := range dbEntries {
		entries[i] = LeaderboardEntry{
			Username:    entry.Username,
			Score:       entry.Score,
			SessionID:   entry.SessionID,
			SessionName: entry.SessionName,
			CreatedAt:   entry.UpdatedAt.Format(time.RFC3339),
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(entries)
}
