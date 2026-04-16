package main

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/joho/godotenv"
)

var (
	authCenterURL string
	httpClient    = &http.Client{Timeout: 10 * time.Second}
)

// POST /webhook — forward to auth-center as-is
func handleWebhook(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	log.Printf("webhook: incoming from %s", r.RemoteAddr)

	req, err := http.NewRequest("POST", authCenterURL+"/webhook", r.Body)
	if err != nil {
		log.Printf("webhook: failed to build request: %v", err)
		http.Error(w, "proxy error", http.StatusInternalServerError)
		return
	}
	req.Header.Set("Content-Type", r.Header.Get("Content-Type"))
	if secret := r.Header.Get("X-Telegram-Bot-Api-Secret-Token"); secret != "" {
		req.Header.Set("X-Telegram-Bot-Api-Secret-Token", secret)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		log.Printf("webhook: upstream error: %v", err)
		http.Error(w, "upstream unreachable", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body) //nolint:errcheck

	log.Printf("webhook: forwarded → %d (%s)", resp.StatusCode, time.Since(start).Round(time.Millisecond))
}

// POST /tg-api/{path...} — forward Telegram API calls from auth-center to Telegram
func handleTelegramAPI(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	path := r.PathValue("path")
	target := "https://api.telegram.org/" + path

	req, err := http.NewRequest("POST", target, r.Body)
	if err != nil {
		log.Printf("tg-api: failed to build request: %v", err)
		http.Error(w, "proxy error", http.StatusInternalServerError)
		return
	}
	req.Header.Set("Content-Type", r.Header.Get("Content-Type"))

	resp, err := httpClient.Do(req)
	if err != nil {
		log.Printf("tg-api: upstream error: %v", err)
		http.Error(w, "upstream unreachable", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body) //nolint:errcheck

	log.Printf("tg-api: %s → %d (%s)", path, resp.StatusCode, time.Since(start).Round(time.Millisecond))
}

// GET /health — liveness check
func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{ //nolint:errcheck
		"status":   "ok",
		"upstream": authCenterURL,
	})
}

func main() {
	log.SetFlags(log.Ldate | log.Ltime | log.LUTC)

	godotenv.Load() //nolint:errcheck

	authCenterURL = os.Getenv("AUTH_CENTER_URL")
	if authCenterURL == "" {
		log.Fatal("AUTH_CENTER_URL is required")
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /webhook", handleWebhook)
	mux.HandleFunc("POST /tg-api/{path...}", handleTelegramAPI)
	mux.HandleFunc("GET /health", handleHealth)

	log.Printf("auth-proxy starting — port=%s upstream=%s", port, authCenterURL)
	log.Fatal(http.ListenAndServe(":"+port, mux))
}
