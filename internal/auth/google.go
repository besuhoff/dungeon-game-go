package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/besuhoff/dungeon-game-go/internal/config"
	"github.com/besuhoff/dungeon-game-go/internal/db"
	"go.mongodb.org/mongo-driver/mongo"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

// GoogleAuthHandler handles Google OAuth authentication
type GoogleAuthHandler struct {
	config   *oauth2.Config
	userRepo *db.UserRepository
}

// NewGoogleAuthHandler creates a new Google auth handler
func NewGoogleAuthHandler() *GoogleAuthHandler {
	return &GoogleAuthHandler{
		config: &oauth2.Config{
			ClientID:     config.AppConfig.GoogleClientID,
			ClientSecret: config.AppConfig.GoogleClientSecret,
			RedirectURL:  config.AppConfig.APIBaseURL + "/api/v1/auth/google/callback",
			Scopes: []string{
				"openid",
				"email",
				"profile",
			},
			Endpoint: google.Endpoint,
		},
		userRepo: db.NewUserRepository(),
	}
}

// GetAuthURLResponse represents the response for auth URL
type GetAuthURLResponse struct {
	URL   string `json:"url"`
	State string `json:"state"`
}

// HandleGetAuthURL returns the Google OAuth URL
func (h *GoogleAuthHandler) HandleGetAuthURL(w http.ResponseWriter, r *http.Request) {
	// Generate random state for CSRF protection
	state, err := generateRandomState()
	if err != nil {
		http.Error(w, "Failed to generate state", http.StatusInternalServerError)
		return
	}

	authURL := h.config.AuthCodeURL(state, oauth2.AccessTypeOffline)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(GetAuthURLResponse{
		URL:   authURL,
		State: state,
	})
}

// HandleCallback handles the OAuth callback from Google
func (h *GoogleAuthHandler) HandleCallback(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")

	if code == "" || state == "" {
		http.Error(w, "Missing code or state", http.StatusBadRequest)
		return
	}

	// Exchange code for token
	ctx := context.Background()
	token, err := h.config.Exchange(ctx, code)
	if err != nil {
		http.Error(w, "Failed to exchange token", http.StatusInternalServerError)
		return
	}

	// Get user info from Google
	userInfo, err := h.getUserInfo(ctx, token)
	if err != nil {
		http.Error(w, "Failed to get user info", http.StatusInternalServerError)
		return
	}

	// Find or create user in database
	user, err := h.userRepo.FindByGoogleID(ctx, userInfo.ID)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			// Create new user
			username := userInfo.Email
			if len(username) > 0 {
				// Extract username from email
				if idx := len(username); idx > 0 {
					for i, c := range username {
						if c == '@' {
							username = username[:i]
							break
						}
					}
				}
			}

			user = &db.User{
				Email:    userInfo.Email,
				GoogleID: userInfo.ID,
				Username: username,
			}

			if err := h.userRepo.Create(ctx, user); err != nil {
				http.Error(w, "Failed to create user", http.StatusInternalServerError)
				return
			}
		} else {
			http.Error(w, "Database error", http.StatusInternalServerError)
			return
		}
	}

	// Generate JWT token
	jwtToken, err := GenerateToken(user.ID)
	if err != nil {
		http.Error(w, "Failed to generate token", http.StatusInternalServerError)
		return
	}

	// Redirect to frontend with token
	redirectURL := fmt.Sprintf("%s?token=%s", config.AppConfig.FrontendURL, url.QueryEscape(jwtToken))
	http.Redirect(w, r, redirectURL, http.StatusFound)
}

// GoogleUserInfo represents user information from Google
type GoogleUserInfo struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	Name  string `json:"name"`
}

// getUserInfo fetches user information from Google
func (h *GoogleAuthHandler) getUserInfo(ctx context.Context, token *oauth2.Token) (*GoogleUserInfo, error) {
	client := h.config.Client(ctx, token)
	
	// Use the token to get user info
	resp, err := client.Get("https://www.googleapis.com/oauth2/v2/userinfo")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var userInfo GoogleUserInfo
	if err := json.Unmarshal(data, &userInfo); err != nil {
		return nil, err
	}

	return &userInfo, nil
}

// generateRandomState generates a random state string for CSRF protection
func generateRandomState() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

// HandleGetUser returns the current authenticated user's information
func (h *GoogleAuthHandler) HandleGetUser(w http.ResponseWriter, r *http.Request) {
	// Extract token from Authorization header
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		http.Error(w, "Missing authorization header", http.StatusUnauthorized)
		return
	}

	// Remove "Bearer " prefix
	token := authHeader
	if len(authHeader) > 7 && authHeader[:7] == "Bearer " {
		token = authHeader[7:]
	}

	// Validate JWT token
	userID, err := ValidateToken(token)
	if err != nil {
		http.Error(w, "Invalid token", http.StatusUnauthorized)
		return
	}

	// Fetch user from database
	ctx := context.Background()
	user, err := h.userRepo.FindByID(ctx, userID)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			http.Error(w, "User not found", http.StatusNotFound)
		} else {
			http.Error(w, "Database error", http.StatusInternalServerError)
		}
		return
	}

	// Return user info
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(user)
}
