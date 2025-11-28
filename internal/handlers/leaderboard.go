package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/besuhoff/dungeon-game-go/internal/db"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
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
	Username  string `json:"username"`
	Score     int    `json:"score"`
	SessionID string `json:"session_id"`
	CreatedAt string `json:"created_at"`
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

// calculateScore calculates the score for a player based on their state
func calculateScore(player db.PlayerState) int {
	score := 0
	
	// Base score from player score field
	score += player.Score
	
	// Add health/lives
	score += player.Lives * 10
	
	// Add kills bonus
	score += player.Kills * 50
	
	// Bonus for being alive
	if player.IsAlive {
		score += 100
	}
	
	// Add money bonus
	score += player.Money
	
	return score
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

	timeframe := r.URL.Query().Get("timeframe")
	if timeframe == "" {
		timeframe = "all"
	}

	ctx := context.Background()
	
	// Build filter based on timeframe
	filter := bson.M{"is_active": true}
	if timeframe != "all" {
		var duration time.Duration
		switch timeframe {
		case "weekly":
			duration = 7 * 24 * time.Hour
		case "monthly":
			duration = 30 * 24 * time.Hour
		default:
			duration = 0
		}
		
		if duration > 0 {
			cutoffTime := time.Now().Add(-duration)
			filter["created_at"] = bson.M{"$gte": cutoffTime}
		}
	}

	// Get all matching sessions
	collection := db.GetDatabase().Collection("game_sessions")
	cursor, err := collection.Find(ctx, filter, options.Find().SetLimit(int64(limit*10)))
	if err != nil {
		http.Error(w, "Failed to fetch leaderboard", http.StatusInternalServerError)
		return
	}
	defer cursor.Close(ctx)

	var sessions []db.GameSession
	if err := cursor.All(ctx, &sessions); err != nil {
		http.Error(w, "Failed to decode sessions", http.StatusInternalServerError)
		return
	}

	// Calculate scores and build leaderboard
	entries := make([]LeaderboardEntry, 0)
	for _, session := range sessions {
		for _, player := range session.Players {
			score := calculateScore(player)
			
			entries = append(entries, LeaderboardEntry{
				Username:  player.Name,
				Score:     score,
				SessionID: session.ID.Hex(),
				CreatedAt: session.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
			})
		}
	}

	// Sort by score (descending)
	for i := 0; i < len(entries); i++ {
		for j := i + 1; j < len(entries); j++ {
			if entries[j].Score > entries[i].Score {
				entries[i], entries[j] = entries[j], entries[i]
			}
		}
	}

	// Limit results
	if len(entries) > limit {
		entries = entries[:limit]
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(entries)
}

// HandleGetUserStats returns statistics for a specific user
func (h *LeaderboardHandler) HandleGetUserStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract user ID from URL
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/leaderboard/user/")
	userIDStr := strings.TrimSpace(path)
	
	userID, err := primitive.ObjectIDFromHex(userIDStr)
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	ctx := context.Background()
	
	// Find user
	user, err := h.userRepo.FindByID(ctx, userID)
	if err != nil {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	// Find all sessions where user participated
	collection := db.GetDatabase().Collection("game_sessions")
	playerKey := "players." + userID.Hex()
	filter := bson.M{playerKey: bson.M{"$exists": true}}
	
	cursor, err := collection.Find(ctx, filter, options.Find().SetSort(bson.M{"created_at": -1}))
	if err != nil {
		http.Error(w, "Failed to fetch user sessions", http.StatusInternalServerError)
		return
	}
	defer cursor.Close(ctx)

	var sessions []db.GameSession
	if err := cursor.All(ctx, &sessions); err != nil {
		http.Error(w, "Failed to decode sessions", http.StatusInternalServerError)
		return
	}

	// Calculate statistics
	stats := UserStats{
		TotalGames: len(sessions),
		RecentScores: make([]struct {
			Score     int    `json:"score"`
			SessionID string `json:"session_id"`
			CreatedAt string `json:"created_at"`
		}, 0),
	}

	totalScore := 0
	for _, session := range sessions {
		if player, ok := session.Players[user.ID.Hex()]; ok {
			score := calculateScore(player)
			totalScore += score
			
			if score > stats.HighestScore {
				stats.HighestScore = score
			}
			
			// Add to recent scores (limit to 10)
			if len(stats.RecentScores) < 10 {
				stats.RecentScores = append(stats.RecentScores, struct {
					Score     int    `json:"score"`
					SessionID string `json:"session_id"`
					CreatedAt string `json:"created_at"`
				}{
					Score:     score,
					SessionID: session.ID.Hex(),
					CreatedAt: session.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
				})
			}
		}
	}

	if stats.TotalGames > 0 {
		stats.AverageScore = float64(totalScore) / float64(stats.TotalGames)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}
