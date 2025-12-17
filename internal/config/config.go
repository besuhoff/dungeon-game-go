package config

import (
	"log"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	MongoDBURL               string
	SecretKey                string
	GoogleClientID           string
	GoogleClientSecret       string
	APIBaseURL               string
	FrontendURL              string
	AccessTokenExpireMinutes int
	Port                     string
	UseTLS                   bool
	TLSCert                  string
	TLSKey                   string
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
		MongoDBURL:               getEnvOrDefault("MONGODB_URL", ""),
		SecretKey:                getEnvOrDefault("SECRET_KEY", ""),
		GoogleClientID:           getEnvOrDefault("GOOGLE_CLIENT_ID", ""),
		GoogleClientSecret:       getEnvOrDefault("GOOGLE_CLIENT_SECRET", ""),
		APIBaseURL:               getEnvOrDefault("API_BASE_URL", "http://localhost:8080"),
		FrontendURL:              getEnvOrDefault("FRONTEND_URL", "http://localhost:9000"),
		AccessTokenExpireMinutes: expireMinutes,
		Port:                     getEnvOrDefault("PORT", "8080"),
		UseTLS:                   useTLS,
		TLSCert:                  getEnvOrDefault("TLS_CERT", ""),
		TLSKey:                   getEnvOrDefault("TLS_KEY", ""),
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

// Constants
const (
	// Player constants
	PlayerLives = 6.0
	PlayerSpeed = 300.0 // Units per second

	PlayerSize          = 24.0
	PlayerRadius        = PlayerSize / 2
	PlayerGunEndOffsetX = -10.0
	PlayerGunEndOffsetY = 20.0
	PlayerTorchOffsetX  = 7.0
	PlayerTorchOffsetY  = 11.0

	PlayerRotationSpeed            = 180.0 // Degrees per second
	PlayerInvulnerabilityTime      = 1.0   // Seconds
	PlayerSpawnInvulnerabilityTime = 3.0   // Seconds after spawn
	PlayerReward                   = 100.0 // Money for killing enemy

	// Blaster constants
	BlasterBulletDamage       = 1
	BlasterBulletSize         = 8.0
	BlasterBulletRadius       = BlasterBulletSize / 2
	BlasterBulletSpeed        = 420.0 // Units per second
	BlasterBulletLifetime     = 3 * time.Second
	BlasterShootDelay         = 0.2 // Seconds
	BlasterMaxBullets         = 6
	BlasterBulletRechargeTime = 1.0 // Seconds per bullet

	// Shotgun constants
	ShotgunShootDelay         = 0.2 // Seconds
	ShotgunMaxBullets         = 2
	ShotgunBulletRechargeTime = 2    // Seconds
	ShotgunSpreadAngle        = 30.0 // Degrees
	ShotgunNumPellets         = 8
	ShotgunDamage             = 2.0
	ShotgunRange              = 200.0

	// Rocket Launcher constants
	RocketLauncherShootDelay     = 1.5   // Seconds
	RocketLauncherBulletSpeed    = 300.0 // Units per second
	RocketLauncherDamage         = 2
	RocketLauncherDamageRadius   = 100.0
	RocketLauncherBulletLifetime = 5 * time.Second

	// Railgun constants
	RailgunShootDelay = 1.0 // Seconds
	RailgunDamage     = 3.0
	RailgunRange      = SightRadius

	// Enemy constants
	EnemySpeed = 120.0 // Units per second

	EnemySize          = 24.0
	EnemyRadius        = EnemySize / 2
	EnemyGunEndOffsetX = -1.0
	EnemyGunEndOffsetY = 26.0

	EnemyLives             = 1.0
	EnemyShootDelay        = 1.0   // Seconds
	EnemyBulletSpeed       = 240.0 // Units per second
	EnemyDeathTraceTime    = 5.0   // Seconds
	EnemyReward            = 10.0  // Money reward
	EnemyDropChance        = 0.3   // 30% chance to drop bonus
	EnemyDropChanceGoggles = 0.2   // 20% chance to drop goggles if dropping bonus

	// Bonus constants
	AidKitSize        = 32.0
	AidKitHealAmount  = 1.0
	GogglesSize       = 32.0
	GogglesActiveTime = 20.0 // Seconds

	// World constants
	ChunkSize            = 2000.0
	SightRadius          = 1500.0
	WallWidth            = 30.0
	MinWallsPerKiloPixel = 7
	ShopSize             = 64.0

	// Vision constants
	TorchRadius                = 200.0
	NightVisionDetectionRadius = 100.0

	// Session constants
	SessionSaveInterval      = 5 * time.Minute
	DeadEntitiesCacheTimeout = 5 * time.Second
	GameLoopInterval         = time.Second / 30
)
