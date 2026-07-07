package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/joho/godotenv"
)

var (
	buildTime     = "unknown"
	authCenterURL string
	httpClient    = &http.Client{Timeout: 10 * time.Second}
)

// ── request logging ───────────────────────────────────────────────────────────

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (sw *statusWriter) WriteHeader(status int) {
	sw.status = status
	sw.ResponseWriter.WriteHeader(status)
}

func logMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sw, r)
		log.Printf("%s %s %d %s", r.Method, r.URL.Path, sw.status, time.Since(start).Round(time.Millisecond))
	})
}

// ── handlers ──────────────────────────────────────────────────────────────────

// GET /health
func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{ //nolint:errcheck
		"status":   "ok",
		"upstream": authCenterURL,
	})
}

// ── main ──────────────────────────────────────────────────────────────────────

func main() {
	if len(os.Args) == 2 && (os.Args[1] == "--version" || os.Args[1] == "--info") {
		fmt.Printf("auth-proxy built: %s\n", buildTime)
		os.Exit(0)
	}

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

	grafanaBotToken = os.Getenv("GRAFANA_BOT_TOKEN")
	grafanaChatID = os.Getenv("GRAFANA_CHAT_ID")
	grafanaAlertSecret = os.Getenv("GRAFANA_ALERT_SECRET")

	mux := http.NewServeMux()
	mux.HandleFunc("POST /webhook", handleWebhook)
	mux.HandleFunc("POST /tg-api/{path...}", handleTelegramAPI)
	mux.HandleFunc("GET /{$}", handleHealth) // root — deploy healthcheck probes GET /
	mux.HandleFunc("GET /health", handleHealth)

	if grafanaBotToken != "" && grafanaChatID != "" && grafanaAlertSecret != "" {
		mux.HandleFunc("POST /alert", handleGrafanaAlert)
		log.Printf("grafana alerts enabled chat_id=%s", grafanaChatID)
	} else {
		log.Printf("grafana alerts disabled (set GRAFANA_BOT_TOKEN, GRAFANA_CHAT_ID, GRAFANA_ALERT_SECRET)")
	}

	log.Printf("listening on :%s upstream=%s", port, authCenterURL)
	log.Fatal(http.ListenAndServe(":"+port, logMiddleware(mux)))
}
