package db

import (
	"context"
	"time"

	"github.com/besuhoff/dungeon-game-go/internal/config"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
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

// PlayerState represents a player's state in a game session
type PlayerState struct {
	PlayerID          string    `bson:"player_id" json:"player_id"`
	Name              string    `bson:"name" json:"name"`
	Position          Position  `bson:"position" json:"position"`
	Lives             int       `bson:"lives" json:"lives"`
	Score             int       `bson:"score" json:"score"`
	Money             int       `bson:"money" json:"money"`
	Kills             int       `bson:"kills" json:"kills"`
	BulletsLeft       int       `bson:"bullets_left" json:"bullets_left"`
	InvulnerableTimer float64   `bson:"invulnerable_timer" json:"invulnerable_timer"`
	NightVisionTimer  float64   `bson:"night_vision_timer" json:"night_vision_timer"`
	IsAlive           bool      `bson:"is_alive" json:"is_alive"`
	IsConnected       bool      `bson:"is_connected" json:"is_connected"`
	LastUpdated       time.Time `bson:"last_updated" json:"last_updated"`
}

func (ps *PlayerState) Respawn() {
	ps.IsAlive = true
	ps.Lives = config.PlayerLives
	ps.BulletsLeft = config.PlayerMaxBullets
	ps.InvulnerableTimer = config.PlayerSpawnInvulnerabilityTime
	ps.NightVisionTimer = 0
	ps.Kills = 0
	ps.Money = 0
	ps.Score = 0
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
	GameState     map[string]interface{} `bson:"game_state" json:"game_state"`
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
	if session.GameState == nil {
		session.GameState = make(map[string]interface{})
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
