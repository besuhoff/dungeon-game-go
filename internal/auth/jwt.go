package auth

import (
	"errors"
	"time"

	"github.com/besuhoff/dungeon-game-go/internal/config"
	"github.com/golang-jwt/jwt/v5"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Claims represents the JWT claims
type Claims struct {
	UserID string `json:"sub"`
	jwt.RegisteredClaims
}

// GenerateToken generates a new JWT token for a user
func GenerateToken(userID primitive.ObjectID) (string, error) {
	expirationTime := time.Now().Add(time.Duration(config.AppConfig.AccessTokenExpireMinutes) * time.Minute)
	
	claims := &Claims{
		UserID: userID.Hex(),
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(config.AppConfig.SecretKey))
}

// ValidateToken validates a JWT token and returns the user ID
func ValidateToken(tokenString string) (primitive.ObjectID, error) {
	claims := &Claims{}

	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("invalid signing method")
		}
		return []byte(config.AppConfig.SecretKey), nil
	})

	if err != nil {
		return primitive.NilObjectID, err
	}

	if !token.Valid {
		return primitive.NilObjectID, errors.New("invalid token")
	}

	userID, err := primitive.ObjectIDFromHex(claims.UserID)
	if err != nil {
		return primitive.NilObjectID, errors.New("invalid user ID in token")
	}

	return userID, nil
}
