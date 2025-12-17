package db

import (
	"context"
	"time"

	"github.com/besuhoff/dungeon-game-go/internal/config"
	"github.com/besuhoff/dungeon-game-go/internal/types"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// User represents a user in the database
type User struct {
	ID             primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	Email          string             `bson:"email" json:"email"`
	GoogleID       string             `bson:"google_id,omitempty" json:"google_id,omitempty"`
	Username       string             `bson:"username" json:"username"`
	IsActive       bool               `bson:"is_active" json:"is_active"`
	CreatedAt      time.Time          `bson:"created_at" json:"created_at"`
	CurrentSession string             `bson:"current_session,omitempty" json:"current_session,omitempty"`
}

type InventoryItem struct {
	Type     int32 `bson:"type" json:"type"`
	Quantity int32 `bson:"quantity" json:"quantity"`
}

// PlayerState represents a player's state in a game session
type PlayerState struct {
	PlayerID                string           `bson:"player_id" json:"player_id"`
	Name                    string           `bson:"name" json:"name"`
	Position                Position         `bson:"position" json:"position"`
	Lives                   float32          `bson:"lives" json:"lives"`
	Score                   int              `bson:"score" json:"score"`
	Money                   int              `bson:"money" json:"money"`
	Kills                   int              `bson:"kills" json:"kills"`
	BulletsLeftByWeaponType map[string]int32 `bson:"bullets_left_by_weapon_type" json:"bullets_left_by_weapon_type"`
	InvulnerableTimer       float64          `bson:"invulnerable_timer" json:"invulnerable_timer"`
	NightVisionTimer        float64          `bson:"night_vision_timer" json:"night_vision_timer"`
	IsAlive                 bool             `bson:"is_alive" json:"is_alive"`
	IsConnected             bool             `bson:"is_connected" json:"is_connected"`
	LastUpdated             time.Time        `bson:"last_updated" json:"last_updated"`
	Inventory               []InventoryItem  `bson:"inventory" json:"inventory"`
	SelectedGunType         string           `bson:"selected_gun_type" json:"selected_gun_type"`
}

func (ps *PlayerState) Respawn() {
	ps.IsAlive = true
	ps.Lives = config.PlayerLives
	ps.BulletsLeftByWeaponType = map[string]int32{
		types.WeaponTypeBlaster: config.BlasterMaxBullets,
	}
	ps.InvulnerableTimer = config.PlayerSpawnInvulnerabilityTime
	ps.NightVisionTimer = 0
	ps.Kills = 0
	ps.Money = 0
	ps.Score = 0
	ps.Inventory = []InventoryItem{{Type: int32(types.InventoryItemBlaster), Quantity: 1}}
	ps.SelectedGunType = types.WeaponTypeBlaster
}

// Position represents x, y coordinates and rotation
type Position struct {
	X        float64 `bson:"x" json:"x"`
	Y        float64 `bson:"y" json:"y"`
	Rotation float64 `bson:"rotation" json:"rotation"`
}

// WorldObject represents an object in the game world
type WorldObject struct {
	ObjectID   string                 `bson:"object_id" json:"object_id"`
	Type       string                 `bson:"type" json:"type"` // wall, enemy, bonus, bullet
	X          float64                `bson:"x" json:"x"`
	Y          float64                `bson:"y" json:"y"`
	Properties map[string]interface{} `bson:"properties,omitempty" json:"properties,omitempty"`
	OwnerID    string                 `bson:"owner_id,omitempty" json:"owner_id,omitempty"`
}

// Chunk represents a chunk of the game world
type Chunk struct {
	ChunkID string                 `bson:"chunk_id" json:"chunk_id"`
	X       int                    `bson:"x" json:"x"`
	Y       int                    `bson:"y" json:"y"`
	Objects map[string]WorldObject `bson:"objects" json:"objects"`
}

// GameSession represents a multiplayer game session
type GameSession struct {
	ID            primitive.ObjectID     `bson:"_id,omitempty" json:"id"`
	Name          string                 `bson:"name" json:"name"`
	HostID        primitive.ObjectID     `bson:"host_id" json:"host_id"`
	Players       map[string]PlayerState `bson:"players" json:"players"`
	MaxPlayers    int                    `bson:"max_players" json:"max_players"`
	IsPrivate     bool                   `bson:"is_private" json:"is_private"`
	Password      string                 `bson:"password,omitempty" json:"-"`
	WorldMap      map[string]Chunk       `bson:"world_map" json:"world_map"`
	SharedObjects map[string]WorldObject `bson:"shared_objects" json:"shared_objects"`
	CreatedAt     time.Time              `bson:"created_at" json:"created_at"`
	LastUpdated   time.Time              `bson:"last_updated" json:"last_updated"`
	IsActive      bool                   `bson:"is_active" json:"is_active"`
}

// UserRepository provides database operations for users
type UserRepository struct {
	collection *mongo.Collection
}

// NewUserRepository creates a new user repository
func NewUserRepository() *UserRepository {
	return &UserRepository{
		collection: Database.Collection("users"),
	}
}

// FindByEmail finds a user by email
func (r *UserRepository) FindByEmail(ctx context.Context, email string) (*User, error) {
	var user User
	err := r.collection.FindOne(ctx, bson.M{"email": email}).Decode(&user)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// FindByGoogleID finds a user by Google ID
func (r *UserRepository) FindByGoogleID(ctx context.Context, googleID string) (*User, error) {
	var user User
	err := r.collection.FindOne(ctx, bson.M{"google_id": googleID}).Decode(&user)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// FindByID finds a user by ID
func (r *UserRepository) FindByID(ctx context.Context, id primitive.ObjectID) (*User, error) {
	var user User
	err := r.collection.FindOne(ctx, bson.M{"_id": id}).Decode(&user)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// Create creates a new user
func (r *UserRepository) Create(ctx context.Context, user *User) error {
	user.CreatedAt = time.Now()
	user.IsActive = true

	result, err := r.collection.InsertOne(ctx, user)
	if err != nil {
		return err
	}

	user.ID = result.InsertedID.(primitive.ObjectID)
	return nil
}

// Update updates a user
func (r *UserRepository) Update(ctx context.Context, user *User) error {
	_, err := r.collection.UpdateOne(
		ctx,
		bson.M{"_id": user.ID},
		bson.M{"$set": user},
	)
	return err
}

// GameSessionRepository provides database operations for game sessions
type GameSessionRepository struct {
	collection *mongo.Collection
}

// NewGameSessionRepository creates a new game session repository
func NewGameSessionRepository() *GameSessionRepository {
	return &GameSessionRepository{
		collection: Database.Collection("game_sessions"),
	}
}

// FindByID finds a game session by ID
func (r *GameSessionRepository) FindByID(ctx context.Context, id primitive.ObjectID) (*GameSession, error) {
	var session GameSession
	err := r.collection.FindOne(ctx, bson.M{"_id": id}).Decode(&session)
	if err != nil {
		return nil, err
	}
	return &session, nil
}

// FindActiveSessions finds all active game sessions
func (r *GameSessionRepository) FindActiveSessions(ctx context.Context) ([]GameSession, error) {
	cursor, err := r.collection.Find(ctx, bson.M{"is_active": true})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var sessions []GameSession
	if err := cursor.All(ctx, &sessions); err != nil {
		return nil, err
	}
	return sessions, nil
}

// Create creates a new game session
func (r *GameSessionRepository) Create(ctx context.Context, session *GameSession) error {
	session.CreatedAt = time.Now()
	session.LastUpdated = time.Now()
	session.IsActive = true

	if session.Players == nil {
		session.Players = make(map[string]PlayerState)
	}
	if session.WorldMap == nil {
		session.WorldMap = make(map[string]Chunk)
	}
	if session.SharedObjects == nil {
		session.SharedObjects = make(map[string]WorldObject)
	}

	result, err := r.collection.InsertOne(ctx, session)
	if err != nil {
		return err
	}

	session.ID = result.InsertedID.(primitive.ObjectID)
	return nil
}

// Update updates a game session
func (r *GameSessionRepository) Update(ctx context.Context, session *GameSession) error {
	session.LastUpdated = time.Now()

	_, err := r.collection.UpdateOne(
		ctx,
		bson.M{"_id": session.ID},
		bson.M{"$set": session},
	)
	return err
}

// Delete deletes a game session
func (r *GameSessionRepository) Delete(ctx context.Context, id primitive.ObjectID) error {
	_, err := r.collection.DeleteOne(ctx, bson.M{"_id": id})
	return err
}

// Leaderboard
type LeaderboardEntry struct {
	ID          primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	UserID      primitive.ObjectID `bson:"user_id" json:"user_id"`
	Username    string             `bson:"username" json:"username"`
	SessionID   string             `bson:"session_id" json:"session_id"`
	SessionName string             `bson:"session_name" json:"session_name"`
	Score       int                `bson:"score" json:"score"`
	Kills       int                `bson:"kills" json:"kills"`
	Deaths      int                `bson:"deaths" json:"deaths"`
	CreatedAt   time.Time          `bson:"created_at" json:"created_at"`
	UpdatedAt   time.Time          `bson:"updated_at" json:"updated_at"`
}

type LeaderboardRepository struct {
	collection *mongo.Collection
}

// UpsertEntry creates or updates a leaderboard entry for a user in a session
func (r *LeaderboardRepository) UpsertEntry(ctx context.Context, entry *LeaderboardEntry) error {
	filter := bson.M{
		"user_id":    entry.UserID,
		"session_id": entry.SessionID,
	}

	update := bson.M{
		"$max": bson.M{
			"score": entry.Score, // Only update if new score is higher
			"kills": entry.Kills, // Only update if new kills is higher
		},
		"$set": bson.M{
			"username":     entry.Username,
			"session_name": entry.SessionName,
			"updated_at":   time.Now(),
		},
		"$inc": bson.M{
			"deaths": 1,
		},
		"$setOnInsert": bson.M{
			"created_at": time.Now(),
		},
	}

	opts := options.Update().SetUpsert(true)
	_, err := r.collection.UpdateOne(ctx, filter, update, opts)
	return err
}

// GetTopScores returns the top N scores globally
func (r *LeaderboardRepository) GetTopScores(ctx context.Context, limit int) ([]LeaderboardEntry, error) {
	opts := options.Find().SetSort(bson.D{{Key: "score", Value: -1}}).SetLimit(int64(limit))
	cursor, err := r.collection.Find(ctx, bson.M{}, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var entries []LeaderboardEntry
	if err := cursor.All(ctx, &entries); err != nil {
		return nil, err
	}
	return entries, nil
}

// GetTopScoresBySession returns the top N scores for a specific session
func (r *LeaderboardRepository) GetTopScoresBySession(ctx context.Context, sessionID string, limit int) ([]LeaderboardEntry, error) {
	filter := bson.M{"session_id": sessionID}
	opts := options.Find().SetSort(bson.D{{Key: "score", Value: -1}}).SetLimit(int64(limit))
	cursor, err := r.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var entries []LeaderboardEntry
	if err := cursor.All(ctx, &entries); err != nil {
		return nil, err
	}
	return entries, nil
}

// GetUserStats returns statistics for a specific user
func (r *LeaderboardRepository) GetUserStats(ctx context.Context, userID primitive.ObjectID) (*LeaderboardEntry, error) {
	// Get the user's best score across all sessions
	opts := options.FindOne().SetSort(bson.D{{Key: "score", Value: -1}})
	var entry LeaderboardEntry
	err := r.collection.FindOne(ctx, bson.M{"user_id": userID}, opts).Decode(&entry)
	if err != nil {
		return nil, err
	}
	return &entry, nil
}

// GetUserSessionEntry returns a user's entry for a specific session
func (r *LeaderboardRepository) GetUserSessionEntry(ctx context.Context, userID primitive.ObjectID, sessionID string) (*LeaderboardEntry, error) {
	var entry LeaderboardEntry
	err := r.collection.FindOne(ctx, bson.M{
		"user_id":    userID,
		"session_id": sessionID,
	}).Decode(&entry)
	if err != nil {
		return nil, err
	}
	return &entry, nil
}

// NewLeaderboardRepository creates a new leaderboard repository
func NewLeaderboardRepository() *LeaderboardRepository {
	return &LeaderboardRepository{
		collection: Database.Collection("leaderboard"),
	}
}
