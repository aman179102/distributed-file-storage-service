package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/distributed-file-storage/service/src/api"
	"github.com/distributed-file-storage/service/src/config"
	"github.com/distributed-file-storage/service/src/domain"
	"github.com/distributed-file-storage/service/src/infrastructure/auth"
	"github.com/distributed-file-storage/service/src/infrastructure/database"
	"golang.org/x/crypto/bcrypt"
)

type AuthHandler struct {
	userRepo   *database.UserRepository
	jwtManager *auth.JWTManager
	cfg        *config.Config
}

func NewAuthHandler(userRepo *database.UserRepository, jwtManager *auth.JWTManager, cfg *config.Config) *AuthHandler {
	return &AuthHandler{
		userRepo:   userRepo,
		jwtManager: jwtManager,
		cfg:        cfg,
	}
}

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type LoginResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int64  `json:"expires_in"`
	UserID       string `json:"user_id"`
	Username     string `json:"username"`
	Role         string `json:"role"`
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.WriteError(w, http.StatusBadRequest, "INVALID_INPUT", "invalid request body")
		return
	}

	if req.Username == "" || req.Password == "" {
		api.WriteError(w, http.StatusBadRequest, "INVALID_INPUT", "username and password are required")
		return
	}

	user, err := h.userRepo.GetByUsername(req.Username)
	if err != nil {
		api.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "invalid credentials")
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		api.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "invalid credentials")
		return
	}

	policies := user.Policies
	tokenPair, err := h.jwtManager.GenerateTokenPair(user.ID, user.Username, user.Role, policies)
	if err != nil {
		slog.Error("failed to generate tokens", "error", err)
		api.WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to generate tokens")
		return
	}

	resp := LoginResponse{
		AccessToken:  tokenPair.AccessToken,
		RefreshToken: tokenPair.RefreshToken,
		TokenType:    "Bearer",
		ExpiresIn:    tokenPair.ExpiresIn,
		UserID:       user.ID,
		Username:     user.Username,
		Role:         user.Role,
	}

	api.WriteJSON(w, http.StatusOK, resp)
}

func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	var req struct {
		RefreshToken string `json:"refresh_token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.WriteError(w, http.StatusBadRequest, "INVALID_INPUT", "invalid request body")
		return
	}

	if req.RefreshToken == "" {
		api.WriteError(w, http.StatusBadRequest, "INVALID_INPUT", "refresh token is required")
		return
	}

	tokenPair, err := h.jwtManager.RefreshToken(req.RefreshToken)
	if err != nil {
		api.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "invalid or expired refresh token")
		return
	}

	api.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"access_token":  tokenPair.AccessToken,
		"refresh_token": tokenPair.RefreshToken,
		"token_type":    "Bearer",
		"expires_in":    tokenPair.ExpiresIn,
	})
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	api.WriteJSON(w, http.StatusOK, map[string]string{"message": "logged out successfully"})
}

func (h *AuthHandler) CreateUser(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Email    string `json:"email"`
		Password string `json:"password"`
		Role     string `json:"role"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.WriteError(w, http.StatusBadRequest, "INVALID_INPUT", "invalid request body")
		return
	}

	if req.Username == "" || req.Email == "" || req.Password == "" {
		api.WriteError(w, http.StatusBadRequest, "INVALID_INPUT", "username, email, and password are required")
		return
	}

	if req.Role == "" {
		req.Role = "user"
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), h.cfg.Auth.BcryptCost)
	if err != nil {
		slog.Error("failed to hash password", "error", err)
		api.WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to create user")
		return
	}

	now := time.Now().UTC()
	user := &domain.User{
		ID:           uuid.New().String(),
		Username:     req.Username,
		Email:        req.Email,
		PasswordHash: string(hashedPassword),
		Role:         req.Role,
		Enabled:      true,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if err := h.userRepo.Create(user); err != nil {
		api.WriteError(w, http.StatusConflict, "ALREADY_EXISTS", "username or email already exists")
		return
	}

	api.WriteJSON(w, http.StatusCreated, map[string]interface{}{
		"id":       user.ID,
		"username": user.Username,
		"email":    user.Email,
		"role":     user.Role,
	})
}

func (h *AuthHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
	users, err := h.userRepo.List(0, 100)
	if err != nil {
		slog.Error("failed to list users", "error", err)
		api.WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to list users")
		return
	}

	type userResponse struct {
		ID        string    `json:"id"`
		Username  string    `json:"username"`
		Email     string    `json:"email"`
		Role      string    `json:"role"`
		Enabled   bool      `json:"enabled"`
		CreatedAt time.Time `json:"created_at"`
	}

	var userResponses []userResponse
	for _, u := range users {
		userResponses = append(userResponses, userResponse{
			ID:        u.ID,
			Username:  u.Username,
			Email:     u.Email,
			Role:      u.Role,
			Enabled:   u.Enabled,
			CreatedAt: u.CreatedAt,
		})
	}

	api.WriteJSON(w, http.StatusOK, map[string]interface{}{"users": userResponses})
}
