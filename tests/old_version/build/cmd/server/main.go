package main

import (
	"encoding/json"
	"log"
	"net/http"

	"auth-center/internal/config"
	"auth-center/internal/database"
	"auth-center/internal/handlers"
	"auth-center/internal/middleware"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	db, err := database.New(cfg.DBPath)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	tokenAuth := middleware.NewTokenAuth(cfg.Tokens)
	authHandler := handlers.NewAuthHandler(db)

	mux := http.NewServeMux()
	mux.HandleFunc("/login", authHandler.Login)
	mux.HandleFunc("/register", authHandler.Register)
	mux.HandleFunc("/verify", authHandler.Verify)
	mux.HandleFunc("/refresh", authHandler.Refresh)

	authedHandler := tokenAuth.Middleware(mux)

	root := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			serveIndex(w, r)
		case "/health":
			serveHealth(w, r)
		default:
			authedHandler.ServeHTTP(w, r)
		}
	})

	log.Printf("Starting server on :%s", cfg.Port)
	if err := http.ListenAndServe(":"+cfg.Port, root); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

func serveHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func serveIndex(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	index := map[string]any{
		"service": "auth-center",
		"endpoints": []map[string]any{
			{
				"path":        "/register",
				"method":      "POST",
				"auth":        "X-Auth-Token",
				"description": "Create a new user",
				"request":     map[string]string{"username": "string", "password": "string"},
				"responses": map[string]any{
					"200": map[string]string{"status": "created", "public_key": "base64-token"},
					"409": map[string]string{"error": "user already exists"},
				},
			},
			{
				"path":        "/login",
				"method":      "POST",
				"auth":        "X-Auth-Token",
				"description": "Authenticate an existing user",
				"request":     map[string]string{"username": "string", "password": "string"},
				"responses": map[string]any{
					"200 (success)": map[string]string{"status": "authenticated", "public_key": "base64-token"},
					"200 (invalid)": map[string]string{"status": "invalid", "public_key": ""},
				},
			},
			{
				"path":        "/verify",
				"method":      "POST",
				"auth":        "X-Auth-Token",
				"description": "Check if a user token is valid",
				"request":     map[string]string{"username": "string", "public_key": "base64-token"},
				"responses": map[string]any{
					"200 (valid)":   map[string]any{"valid": true, "user_id": "int", "username": "string", "created_at": "datetime"},
					"200 (invalid)": map[string]any{"valid": false},
				},
			},
			{
				"path":        "/refresh",
				"method":      "POST",
				"auth":        "X-Auth-Token",
				"description": "Generate new token from a valid old one",
				"request":     map[string]string{"username": "string", "old_public_key": "base64-token"},
				"responses": map[string]any{
					"200": map[string]string{"public_key": "base64-new-token"},
					"401": map[string]string{"error": "invalid old token"},
				},
			},
			{
				"path":        "/health",
				"method":      "GET",
				"auth":        "none",
				"description": "Health check",
				"responses": map[string]any{
					"200": map[string]string{"status": "ok"},
				},
			},
		},
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.Encode(index)
}
