package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

var buildTime = "unknown"

const defaultAppName = "auth-miniapp"

// Link flags auth-center puts in front of the session token in ?startapp=.
// A QR was scanned, so the browser waiting for the login is on another machine;
// a button was pressed on the machine that is waiting. Kept in step with
// linkFromQR / linkFromButton in auth-center's telegram.go.
const (
	fromQR     = "q"
	fromButton = "b"
)

func linkSource(flag string) string {
	if flag == fromQR {
		return "qr"
	}
	return "button"
}

var appName string

//go:embed web
var webFiles embed.FS

var (
	// botToken verifies initData signatures. Unlike the PoC this app grew out
	// of, it is mandatory: this service does not report an identity, it asserts
	// one to auth-center, and an unverified assertion would be a login bypass.
	botToken string

	// authInternal is auth-center's base URL for the server-to-server call.
	authInternal string

	// authSecret is derived from botToken at startup — see authcenter.go.
	authSecret string
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

// noStore keeps the Telegram in-app browser from serving a stale page. Its
// cache is not something a phone user can clear, and a stale script.js would
// talk to an endpoint that has moved on.
func noStore(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		h.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v) //nolint:errcheck
}

// ── routes ────────────────────────────────────────────────────────────────────

func handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	src, err := webFiles.ReadFile("web/index.html")
	if err != nil {
		http.Error(w, "index.html missing", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Write(src) //nolint:errcheck
}

type authRequest struct {
	InitData string `json:"init_data"`
}

type authResponse struct {
	OK       bool   `json:"ok"`
	UserID   int64  `json:"user_id,omitempty"`
	Name     string `json:"name,omitempty"`
	Username string `json:"username,omitempty"`
	// Redirect and Code are the way back: the app the login started from, and a
	// one-time code minted for this page alone. The page navigates itself there
	// rather than closing, because closing leaves the user staring at a
	// collapsed tab with nothing saying the login worked.
	Redirect string `json:"redirect,omitempty"`
	Code     string `json:"code,omitempty"`
	Error    string `json:"error,omitempty"`
}

// handleAuth is the whole service. The page hands over the raw initData string;
// this proves our bot signed it, reads the session token out of the signed
// start_param, and reports the identity to auth-center. Nothing here is
// trusted before validateInitData returns.
func handleAuth(w http.ResponseWriter, r *http.Request) {
	var req authRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, authResponse{Error: "bad json"})
		return
	}
	if req.InitData == "" {
		log.Print("auth: empty init_data (page opened outside Telegram?)")
		writeJSON(w, http.StatusBadRequest, authResponse{
			Error: "empty init_data — open this page from inside Telegram",
		})
		return
	}

	data, err := validateInitData(req.InitData, botToken)
	if err != nil {
		log.Printf("auth: REJECTED: %v", err)
		writeJSON(w, http.StatusUnauthorized, authResponse{Error: err.Error()})
		return
	}
	user, err := parseUser(req.InitData)
	if err != nil {
		log.Printf("auth: signature ok but user unreadable: %v", err)
		writeJSON(w, http.StatusBadRequest, authResponse{Error: err.Error()})
		return
	}

	// The session travels in ?startapp=, which Telegram covers with the
	// signature — so by this line it is as trustworthy as the user id itself.
	// auth-center puts one flag character in front of the token saying how the
	// link was handed over, so the token is everything after the first
	// character. See linkFromQR / linkFromButton in auth-center's telegram.go.
	startParam := data.Get("start_param")
	if len(startParam) < 2 {
		log.Printf("auth: uid=%d start_param=%q — link carried no session", user.ID, startParam)
		writeJSON(w, http.StatusBadRequest, authResponse{
			Error: "в ссылке нет сессии входа — начните со страницы входа",
		})
		return
	}
	from, sessionToken := startParam[:1], startParam[1:]
	if from != fromQR && from != fromButton {
		log.Printf("auth: uid=%d unknown link flag %q", user.ID, from)
		writeJSON(w, http.StatusBadRequest, authResponse{
			Error: "ссылка входа не распознана",
		})
		return
	}

	bound, err := bindSession(sessionToken, from == fromQR, user)
	if err != nil {
		log.Printf("auth: uid=%d bind failed: %v", user.ID, err)
		writeJSON(w, http.StatusBadGateway, authResponse{Error: err.Error()})
		return
	}

	name := strings.TrimSpace(user.FirstName + " " + user.LastName)
	log.Printf("auth: BOUND uid=%d username=%q name=%q via=%s session=%s back=%q",
		user.ID, user.Username, name, linkSource(from), sessionToken, bound.Redirect)

	writeJSON(w, http.StatusOK, authResponse{
		OK: true, UserID: user.ID, Name: name, Username: user.Username,
		Redirect: bound.Redirect, Code: bound.Code,
	})
}

// ── main ──────────────────────────────────────────────────────────────────────

func main() {
	log.SetFlags(log.Ldate | log.Ltime | log.LUTC)
	godotenv.Load() //nolint:errcheck

	appName = os.Getenv("APP_NAME")
	if appName == "" {
		appName = defaultAppName
	}

	if len(os.Args) == 2 && (os.Args[1] == "--version" || os.Args[1] == "--info") {
		fmt.Printf("%s built: %s\n", appName, buildTime)
		os.Exit(0)
	}

	botToken = os.Getenv("BOT_TOKEN")
	authInternal = os.Getenv("AUTH_INTERNAL")

	// Both are refused at startup rather than per request. A mini app that
	// boots without them looks healthy and fails only at the moment someone
	// tries to log in, which is the worst time to find out.
	if botToken == "" {
		log.Fatal("BOT_TOKEN is required — without it initData cannot be verified")
	}
	if authInternal == "" {
		log.Fatal("AUTH_INTERNAL is required — it is where the verified identity goes")
	}
	authSecret = deriveSecret(botToken)

	webFS, _ := fs.Sub(webFiles, "web")
	fileServer := http.FileServer(http.FS(webFS))

	mux := http.NewServeMux()
	mux.HandleFunc("GET /", handleIndex)
	mux.HandleFunc("POST /api/auth", handleAuth)
	// Optional self-hosted copy of the Telegram SDK: absent by default, in
	// which case this 404s at once and the page falls back to telegram.org.
	// Dropping telegram-web-app.js into build/web/ removes the dependency on a
	// host that is not reachable from every network.
	mux.Handle("GET /telegram-web-app.js", noStore(fileServer))
	mux.Handle("GET /favicon.svg", fileServer)
	mux.Handle("GET /style.css", noStore(fileServer))
	mux.Handle("GET /script.js", noStore(fileServer))

	port := os.Getenv("PORT")
	if port == "" {
		port = "8892"
	}
	log.Printf("start app=%s port=%s auth=%s", appName, port, authInternal)
	log.Fatal(http.ListenAndServe(":"+port, logMiddleware(mux)))
}
