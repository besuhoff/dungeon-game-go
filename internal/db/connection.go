package db

import (
	"context"
	"log"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var Client *mongo.Client
var Database *mongo.Database

// Connect establishes a connection to MongoDB
func Connect(mongoURL string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	clientOptions := options.Client().ApplyURI(mongoURL)
	client, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		return err
	}

	// Ping the database to verify connection
	if err := client.Ping(ctx, nil); err != nil {
		return err
	}

	Client = client
	Database = client.Database("dungeon_game")

	log.Println("Connected to MongoDB successfully")

	// Create indexes
	if err := createIndexes(ctx); err != nil {
		log.Printf("Warning: Failed to create indexes: %v", err)
	}

	return nil
}

// createIndexes creates necessary database indexes
func createIndexes(ctx context.Context) error {
	// User indexes
	userCollection := Database.Collection("users")
	_, err := userCollection.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "email", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys:    bson.D{{Key: "google_id", Value: 1}},
			Options: options.Index().SetUnique(true).SetSparse(true),
		},
	})
	if err != nil {
		return err
	}

	// GameSession indexes
	sessionCollection := Database.Collection("game_sessions")
	_, err = sessionCollection.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys: bson.D{{Key: "host_id", Value: 1}},
		},
		{
			Keys: bson.D{{Key: "is_active", Value: 1}},
		},
	})
	if err != nil {
		return err
	}

	// Leaderboard indexes
	leaderboardCollection := Database.Collection("leaderboard")
	_, err = leaderboardCollection.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "user_id", Value: 1}, {Key: "session_id", Value: 1}},
			Options: options.Index().SetUnique(true), // Unique per user per session
		},
		{
			Keys: bson.D{{Key: "score", Value: -1}}, // For sorting by score descending
		},
		{
			Keys: bson.D{{Key: "session_id", Value: 1}, {Key: "score", Value: -1}}, // For per-session leaderboards
		},
		{
			Keys: bson.D{{Key: "user_id", Value: 1}, {Key: "updated_at", Value: -1}}, // For user history
		},
	})

	return err
}

// Disconnect closes the MongoDB connection
func Disconnect() error {
	if Client != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return Client.Disconnect(ctx)
	}
	return nil
}

// GetDatabase returns the database instance
func GetDatabase() *mongo.Database {
	return Database
}
