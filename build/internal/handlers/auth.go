package handlers

import (
	"encoding/json"
	"net/http"

	"auth-center/internal/crypto"
	"auth-center/internal/database"

	"golang.org/x/crypto/bcrypt"
)

type AuthHandler struct {
	db *database.DB
}

func NewAuthHandler(db *database.DB) *AuthHandler {
	return &AuthHandler{db: db}
}

type AuthRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type AuthResponse struct {
	Status    string `json:"status"`
	PublicKey string `json:"public_key"`
}

type VerifyRequest struct {
	Username  string `json:"username"`
	PublicKey string `json:"public_key"`
}

type VerifyResponse struct {
	Valid     bool   `json:"valid"`
	UserID    int64  `json:"user_id,omitempty"`
	Username  string `json:"username,omitempty"`
	CreatedAt string `json:"created_at,omitempty"`
}

type RefreshRequest struct {
	Username     string `json:"username"`
	OldPublicKey string `json:"old_public_key"`
}

type RefreshResponse struct {
	PublicKey string `json:"public_key"`
}

// Login authenticates an existing user
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	var req AuthRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	if req.Username == "" || req.Password == "" {
		http.Error(w, `{"error":"username and password required"}`, http.StatusBadRequest)
		return
	}

	user, err := h.db.GetUserByUsername(req.Username)
	if err != nil {
		http.Error(w, `{"error":"database error"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	if user == nil {
		json.NewEncoder(w).Encode(AuthResponse{
			Status:    "invalid",
			PublicKey: "",
		})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		json.NewEncoder(w).Encode(AuthResponse{
			Status:    "invalid",
			PublicKey: "",
		})
		return
	}

	privateKey := crypto.BytesToPrivateKey(user.PrivateKey)
	token, err := crypto.GenerateToken(privateKey, req.Username)
	if err != nil {
		http.Error(w, `{"error":"failed to generate token"}`, http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(AuthResponse{
		Status:    "authenticated",
		PublicKey: token,
	})
}

// Register creates a new user
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	var req AuthRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	if req.Username == "" || req.Password == "" {
		http.Error(w, `{"error":"username and password required"}`, http.StatusBadRequest)
		return
	}

	user, err := h.db.GetUserByUsername(req.Username)
	if err != nil {
		http.Error(w, `{"error":"database error"}`, http.StatusInternalServerError)
		return
	}

	if user != nil {
		http.Error(w, `{"error":"user already exists"}`, http.StatusConflict)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), 12)
	if err != nil {
		http.Error(w, `{"error":"failed to hash password"}`, http.StatusInternalServerError)
		return
	}

	privateKey, publicKey, err := crypto.GenerateKeyPair()
	if err != nil {
		http.Error(w, `{"error":"failed to generate keys"}`, http.StatusInternalServerError)
		return
	}

	token, err := crypto.GenerateToken(privateKey, req.Username)
	if err != nil {
		http.Error(w, `{"error":"failed to generate token"}`, http.StatusInternalServerError)
		return
	}

	_, err = h.db.CreateUser(
		req.Username,
		string(hashedPassword),
		crypto.PrivateKeyToBytes(privateKey),
		crypto.PublicKeyToBase64(publicKey),
	)
	if err != nil {
		http.Error(w, `{"error":"failed to create user"}`, http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(AuthResponse{
		Status:    "created",
		PublicKey: token,
	})
}

// Verify checks if a token is valid for a user
func (h *AuthHandler) Verify(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	var req VerifyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	if req.Username == "" || req.PublicKey == "" {
		http.Error(w, `{"error":"username and public_key required"}`, http.StatusBadRequest)
		return
	}

	user, err := h.db.GetUserByUsername(req.Username)
	if err != nil {
		http.Error(w, `{"error":"database error"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	if user == nil {
		json.NewEncoder(w).Encode(VerifyResponse{Valid: false})
		return
	}

	valid, err := crypto.VerifyToken(user.PublicKey, req.PublicKey, req.Username)
	if err != nil || !valid {
		json.NewEncoder(w).Encode(VerifyResponse{Valid: false})
		return
	}

	json.NewEncoder(w).Encode(VerifyResponse{
		Valid:     true,
		UserID:    user.ID,
		Username:  user.Username,
		CreatedAt: user.CreatedAt,
	})
}

// Refresh generates a new token using an old valid one
func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	var req RefreshRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	if req.Username == "" || req.OldPublicKey == "" {
		http.Error(w, `{"error":"username and old_public_key required"}`, http.StatusBadRequest)
		return
	}

	user, err := h.db.GetUserByUsername(req.Username)
	if err != nil {
		http.Error(w, `{"error":"database error"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	if user == nil {
		http.Error(w, `{"error":"user not found"}`, http.StatusNotFound)
		return
	}

	// Verify old token first
	valid, err := crypto.VerifyToken(user.PublicKey, req.OldPublicKey, req.Username)
	if err != nil || !valid {
		http.Error(w, `{"error":"invalid old token"}`, http.StatusUnauthorized)
		return
	}

	// Generate new token
	privateKey := crypto.BytesToPrivateKey(user.PrivateKey)
	token, err := crypto.GenerateToken(privateKey, req.Username)
	if err != nil {
		http.Error(w, `{"error":"failed to generate token"}`, http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(RefreshResponse{PublicKey: token})
}
