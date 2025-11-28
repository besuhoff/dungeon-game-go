package config

import (
	"log"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	MongoDBURL              string
	SecretKey               string
	GoogleClientID          string
	GoogleClientSecret      string
	APIBaseURL              string
	FrontendURL             string
	AccessTokenExpireMinutes int
	Port                    string
	UseTLS                  bool
	TLSCert                 string
	TLSKey                  string
}

var AppConfig *Config

// LoadConfig loads configuration from environment variables
func LoadConfig() *Config {
	// Load .env file if it exists
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using environment variables")
	}

	expireMinutes := 11520 // Default: 8 days
	if expireStr := os.Getenv("ACCESS_TOKEN_EXPIRE_MINUTES"); expireStr != "" {
		if val, err := strconv.Atoi(expireStr); err == nil {
			expireMinutes = val
		}
	}

	useTLS := false
	if tlsStr := os.Getenv("USE_TLS"); tlsStr == "true" {
		useTLS = true
	}

	config := &Config{
		MongoDBURL:              getEnvOrDefault("MONGODB_URL", ""),
		SecretKey:               getEnvOrDefault("SECRET_KEY", ""),
		GoogleClientID:          getEnvOrDefault("GOOGLE_CLIENT_ID", ""),
		GoogleClientSecret:      getEnvOrDefault("GOOGLE_CLIENT_SECRET", ""),
		APIBaseURL:              getEnvOrDefault("API_BASE_URL", "http://localhost:8080"),
		FrontendURL:             getEnvOrDefault("FRONTEND_URL", "http://localhost:9000"),
		AccessTokenExpireMinutes: expireMinutes,
		Port:                    getEnvOrDefault("PORT", "8080"),
		UseTLS:                  useTLS,
		TLSCert:                 getEnvOrDefault("TLS_CERT", ""),
		TLSKey:                  getEnvOrDefault("TLS_KEY", ""),
	}

	// Validate required fields
	if config.MongoDBURL == "" {
		log.Fatal("MONGODB_URL is required")
	}
	if config.SecretKey == "" {
		log.Fatal("SECRET_KEY is required")
	}
	if config.GoogleClientID == "" {
		log.Fatal("GOOGLE_CLIENT_ID is required")
	}
	if config.GoogleClientSecret == "" {
		log.Fatal("GOOGLE_CLIENT_SECRET is required")
	}

	AppConfig = config
	return config
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
