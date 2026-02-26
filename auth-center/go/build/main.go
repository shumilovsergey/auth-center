package main

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"embed"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html/template"
	"image/color"
	"io/fs"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/joho/godotenv"
	"github.com/mr-tron/base58"
	"github.com/skip2/go-qrcode"
)

// ── embedded web files ────────────────────────────────────────────────────

//go:embed web
var webFiles embed.FS

// ── config ────────────────────────────────────────────────────────────────

var (
	botToken       string
	botUsername    string
	webhookSecret  string
	appTokens      map[string]bool
	directRedirect string

	googleClientID     string
	googleClientSecret string
	googleCallbackURL  string

	googleStates   = make(map[string]googleState)
	googleStatesMu sync.Mutex

	httpClient = &http.Client{Timeout: 10 * time.Second}
)

type googleState struct {
	Redirect  string
	CreatedAt time.Time
}

// ── stores ────────────────────────────────────────────────────────────────

type Session struct {
	Status    string
	User      map[string]any
	CreatedAt time.Time
	Redirect  string
	Code      string
}

type Code struct {
	User      map[string]any
	Method    string
	CreatedAt time.Time
}

var (
	sessions   = make(map[string]*Session)
	sessionsMu sync.Mutex

	nonces   = make(map[string]string)
	noncesMu sync.Mutex

	codes   = make(map[string]*Code)
	codesMu sync.Mutex
)

const (
	sessionTTL = 5 * time.Minute
	codeTTL    = 60 * time.Second
)

// ── helpers ───────────────────────────────────────────────────────────────

func randToken(n int) string {
	b := make([]byte, n)
	rand.Read(b) //nolint:errcheck
	return base64.RawURLEncoding.EncodeToString(b)
}

func randHex(n int) string {
	b := make([]byte, n)
	rand.Read(b) //nolint:errcheck
	return hex.EncodeToString(b)
}

func jsonOK(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v) //nolint:errcheck
}

func jsonErr(w http.ResponseWriter, msg string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": msg}) //nolint:errcheck
}

func makeQR(url string) (string, error) {
	qr, err := qrcode.New(url, qrcode.Medium)
	if err != nil {
		return "", err
	}
	qr.BackgroundColor = color.RGBA{R: 8, G: 8, B: 15, A: 255}    // #08080f
	qr.ForegroundColor = color.RGBA{R: 196, G: 181, B: 253, A: 255} // #c4b5fd
	png, err := qr.PNG(256)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(png), nil
}

func cleanSessions() {
	sessionsMu.Lock()
	defer sessionsMu.Unlock()
	for k, v := range sessions {
		if time.Since(v.CreatedAt) > sessionTTL {
			delete(sessions, k)
		}
	}
}

func cleanCodes() {
	codesMu.Lock()
	defer codesMu.Unlock()
	for k, v := range codes {
		if time.Since(v.CreatedAt) > codeTTL {
			delete(codes, k)
		}
	}
}

func newCode(user map[string]any, method string) string {
	cleanCodes()
	c := randToken(32)
	codesMu.Lock()
	codes[c] = &Code{User: user, Method: method, CreatedAt: time.Now()}
	codesMu.Unlock()
	return c
}

func sendTG(chatID int64, text string) {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", botToken)
	body, _ := json.Marshal(map[string]any{"chat_id": chatID, "text": text})
	client := &http.Client{Timeout: 5 * time.Second}
	client.Post(url, "application/json", bytes.NewReader(body)) //nolint:errcheck
}

// ── template ──────────────────────────────────────────────────────────────

var indexTmpl *template.Template

func initTemplate() {
	src, err := webFiles.ReadFile("web/index.html")
	if err != nil {
		log.Fatalf("index.html not found: %v", err)
	}
	indexTmpl = template.Must(template.New("index").Parse(string(src)))
}

// ── handlers ──────────────────────────────────────────────────────────────

func handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	redirectURL := r.URL.Query().Get("redirect")
	if redirectURL == "" && directRedirect != "" {
		http.Redirect(w, r, directRedirect, http.StatusFound)
		return
	}
	rdJSON, _ := json.Marshal(redirectURL)
	indexTmpl.Execute(w, struct{ RedirectURL template.JS }{ //nolint:errcheck
		RedirectURL: template.JS(rdJSON),
	})
}

// POST /qr-session
func handleQRSession(w http.ResponseWriter, r *http.Request) {
	cleanSessions()
	var body struct {
		Redirect string `json:"redirect"`
	}
	json.NewDecoder(r.Body).Decode(&body) //nolint:errcheck

	tok := randToken(32)
	sessionsMu.Lock()
	sessions[tok] = &Session{
		Status:    "pending",
		CreatedAt: time.Now(),
		Redirect:  body.Redirect,
	}
	sessionsMu.Unlock()

	tmeURL := fmt.Sprintf("https://t.me/%s?start=%s", botUsername, tok)
	qr, err := makeQR(tmeURL)
	if err != nil {
		jsonErr(w, "qr error", http.StatusInternalServerError)
		return
	}
	jsonOK(w, map[string]string{"token": tok, "qr": qr, "url": tmeURL})
}

// GET /poll/{token}
func handlePoll(w http.ResponseWriter, r *http.Request) {
	tok := r.PathValue("token")

	sessionsMu.Lock()
	sess, ok := sessions[tok]
	if !ok {
		sessionsMu.Unlock()
		jsonErr(w, "expired", http.StatusNotFound)
		return
	}
	if time.Since(sess.CreatedAt) > sessionTTL {
		delete(sessions, tok)
		sessionsMu.Unlock()
		jsonErr(w, "expired", http.StatusNotFound)
		return
	}
	resp := map[string]any{"status": sess.Status, "user": sess.User}
	if sess.Status == "authenticated" && sess.Redirect != "" {
		resp["code"] = sess.Code
		resp["redirect"] = sess.Redirect
	}
	sessionsMu.Unlock()

	jsonOK(w, resp)
}

// POST /webhook
func handleWebhook(w http.ResponseWriter, r *http.Request) {
	if webhookSecret != "" {
		if r.Header.Get("X-Telegram-Bot-Api-Secret-Token") != webhookSecret {
			w.WriteHeader(http.StatusForbidden)
			return
		}
	}

	var update struct {
		Message struct {
			Text string `json:"text"`
			From struct {
				ID        int64  `json:"id"`
				FirstName string `json:"first_name"`
				LastName  string `json:"last_name"`
				Username  string `json:"username"`
			} `json:"from"`
		} `json:"message"`
	}
	if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	text := update.Message.Text
	from := update.Message.From

	if strings.HasPrefix(text, "/start ") {
		tok := strings.TrimSpace(strings.TrimPrefix(text, "/start "))

		sessionsMu.Lock()
		sess, ok := sessions[tok]
		if ok && sess.Status == "pending" && time.Since(sess.CreatedAt) <= sessionTTL {
			user := map[string]any{
				"id":         from.ID,
				"first_name": from.FirstName,
				"last_name":  from.LastName,
				"username":   from.Username,
			}
			sess.Status = "authenticated"
			sess.User = user
			if sess.Redirect != "" {
				sess.Code = newCode(user, "telegram")
			}
			sessionsMu.Unlock()
			go sendTG(from.ID, "You are authenticated!")
		} else {
			expired := ok && time.Since(sess.CreatedAt) > sessionTTL
			if expired {
				delete(sessions, tok)
			}
			sessionsMu.Unlock()
			if ok {
				go sendTG(from.ID, "This QR code has expired.")
			}
		}
	}

	w.WriteHeader(http.StatusOK)
}

// POST /solana/nonce
func handleSolanaNonce(w http.ResponseWriter, r *http.Request) {
	var body struct {
		PublicKey string `json:"public_key"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || strings.TrimSpace(body.PublicKey) == "" {
		jsonErr(w, "missing public_key", http.StatusBadRequest)
		return
	}
	nonce := fmt.Sprintf("Sign in to Auth Center\nNonce: %s", randHex(16))
	noncesMu.Lock()
	nonces[body.PublicKey] = nonce
	noncesMu.Unlock()
	jsonOK(w, map[string]string{"nonce": nonce})
}

// POST /solana/auth
func handleSolanaAuth(w http.ResponseWriter, r *http.Request) {
	var body struct {
		PublicKey string `json:"public_key"`
		Signature string `json:"signature"`
		Nonce     string `json:"nonce"`
		Redirect  string `json:"redirect"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonErr(w, "no data", http.StatusBadRequest)
		return
	}
	if body.PublicKey == "" || body.Signature == "" || body.Nonce == "" {
		jsonErr(w, "missing fields", http.StatusBadRequest)
		return
	}

	noncesMu.Lock()
	expected, ok := nonces[body.PublicKey]
	noncesMu.Unlock()
	if !ok || expected != body.Nonce {
		jsonErr(w, "invalid or expired nonce", http.StatusForbidden)
		return
	}

	pubKeyBytes, err := base58.Decode(body.PublicKey)
	if err != nil || len(pubKeyBytes) != 32 {
		jsonErr(w, "invalid public key", http.StatusBadRequest)
		return
	}
	sigBytes, err := base64.StdEncoding.DecodeString(body.Signature)
	if err != nil {
		jsonErr(w, "invalid signature encoding", http.StatusBadRequest)
		return
	}
	if !ed25519.Verify(ed25519.PublicKey(pubKeyBytes), []byte(body.Nonce), sigBytes) {
		jsonErr(w, "invalid signature", http.StatusForbidden)
		return
	}

	noncesMu.Lock()
	delete(nonces, body.PublicKey)
	noncesMu.Unlock()

	resp := map[string]any{"ok": true, "public_key": body.PublicKey}
	if body.Redirect != "" {
		user := map[string]any{"id": body.PublicKey}
		resp["code"] = newCode(user, "solana")
		resp["redirect"] = body.Redirect
	}
	jsonOK(w, resp)
}

// POST /exchange
func handleExchange(w http.ResponseWriter, r *http.Request) {
	cleanCodes()
	var body struct {
		Code     string `json:"code"`
		AppToken string `json:"app_token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonErr(w, "no data", http.StatusBadRequest)
		return
	}
	if len(appTokens) > 0 && !appTokens[body.AppToken] {
		jsonErr(w, "unauthorized", http.StatusForbidden)
		return
	}
	if body.Code == "" {
		jsonErr(w, "missing code", http.StatusBadRequest)
		return
	}

	codesMu.Lock()
	entry, ok := codes[body.Code]
	if !ok || time.Since(entry.CreatedAt) > codeTTL {
		if ok {
			delete(codes, body.Code)
		}
		codesMu.Unlock()
		jsonErr(w, "invalid or expired code", http.StatusForbidden)
		return
	}
	user := entry.User
	method := entry.Method
	delete(codes, body.Code)
	codesMu.Unlock()

	jsonOK(w, map[string]any{"ok": true, "user": user, "method": method})
}

// ── google oauth ──────────────────────────────────────────────────────────

// GET /google/login
func handleGoogleLogin(w http.ResponseWriter, r *http.Request) {
	redirectURL := r.URL.Query().Get("redirect")
	state := randToken(32)
	googleStatesMu.Lock()
	googleStates[state] = googleState{Redirect: redirectURL, CreatedAt: time.Now()}
	googleStatesMu.Unlock()

	params := url.Values{
		"client_id":     {googleClientID},
		"redirect_uri":  {googleCallbackURL},
		"response_type": {"code"},
		"scope":         {"openid email profile"},
		"state":         {state},
	}
	http.Redirect(w, r, "https://accounts.google.com/o/oauth2/v2/auth?"+params.Encode(), http.StatusFound)
}

// GET /google/callback
func handleGoogleCallback(w http.ResponseWriter, r *http.Request) {
	if errParam := r.URL.Query().Get("error"); errParam != "" {
		http.Error(w, "google auth error: "+errParam, http.StatusBadRequest)
		return
	}

	state := r.URL.Query().Get("state")
	authCode := r.URL.Query().Get("code")

	googleStatesMu.Lock()
	stateData, ok := googleStates[state]
	if ok {
		delete(googleStates, state)
	}
	googleStatesMu.Unlock()

	if !ok || time.Since(stateData.CreatedAt) > sessionTTL {
		http.Error(w, "invalid or expired state", http.StatusBadRequest)
		return
	}

	// exchange code for access token
	tokenBody, _ := json.Marshal(map[string]string{
		"code":          authCode,
		"client_id":     googleClientID,
		"client_secret": googleClientSecret,
		"redirect_uri":  googleCallbackURL,
		"grant_type":    "authorization_code",
	})
	tokenResp, err := httpClient.Post(
		"https://oauth2.googleapis.com/token",
		"application/json",
		bytes.NewReader(tokenBody),
	)
	if err != nil {
		http.Error(w, "token exchange failed", http.StatusInternalServerError)
		return
	}
	defer tokenResp.Body.Close()
	var tokenData map[string]any
	json.NewDecoder(tokenResp.Body).Decode(&tokenData) //nolint:errcheck

	if _, hasErr := tokenData["error"]; hasErr {
		http.Error(w, fmt.Sprintf("token error: %v", tokenData["error"]), http.StatusBadRequest)
		return
	}
	accessToken, _ := tokenData["access_token"].(string)

	// fetch user info
	req, _ := http.NewRequest("GET", "https://www.googleapis.com/oauth2/v3/userinfo", nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	userResp, err := httpClient.Do(req)
	if err != nil {
		http.Error(w, "userinfo fetch failed", http.StatusInternalServerError)
		return
	}
	defer userResp.Body.Close()
	var userInfo map[string]any
	json.NewDecoder(userResp.Body).Decode(&userInfo) //nolint:errcheck

	sub, _ := userInfo["sub"].(string)
	email, _ := userInfo["email"].(string)
	name, _ := userInfo["name"].(string)
	user := map[string]any{"id": sub, "email": email, "name": name}

	if stateData.Redirect != "" {
		oneTimeCode := newCode(user, "google")
		sep := "?"
		if strings.Contains(stateData.Redirect, "?") {
			sep = "&"
		}
		http.Redirect(w, r, stateData.Redirect+sep+"code="+oneTimeCode, http.StatusFound)
		return
	}

	target := directRedirect
	if target == "" {
		target = "/"
	}
	http.Redirect(w, r, target, http.StatusFound)
}

// ── main ──────────────────────────────────────────────────────────────────

func main() {
	godotenv.Load() //nolint:errcheck

	botToken = os.Getenv("BOT_TOKEN")
	botUsername = os.Getenv("BOT_USERNAME")
	webhookSecret = os.Getenv("WEBHOOK_SECRET")
	directRedirect = os.Getenv("DIRECT_REDIRECT")

	googleClientID = os.Getenv("GOOGLE_CLIENT_ID")
	googleClientSecret = os.Getenv("GOOGLE_CLIENT_SECRET")
	googleCallbackURL = os.Getenv("GOOGLE_CALLBACK_URL")
	if googleCallbackURL == "" {
		googleCallbackURL = "http://localhost:8886/google/callback"
	}

	appTokens = make(map[string]bool)
	for _, t := range strings.Split(os.Getenv("APP_TOKENS"), ",") {
		if t = strings.TrimSpace(t); t != "" {
			appTokens[t] = true
		}
	}

	initTemplate()

	webFS, _ := fs.Sub(webFiles, "web")
	fileServer := http.FileServer(http.FS(webFS))

	mux := http.NewServeMux()
	mux.HandleFunc("GET /", handleIndex)
	mux.HandleFunc("POST /qr-session", handleQRSession)
	mux.HandleFunc("GET /poll/{token}", handlePoll)
	mux.HandleFunc("POST /webhook", handleWebhook)
	mux.HandleFunc("POST /solana/nonce", handleSolanaNonce)
	mux.HandleFunc("POST /solana/auth", handleSolanaAuth)
	mux.HandleFunc("GET /google/login", handleGoogleLogin)
	mux.HandleFunc("GET /google/callback", handleGoogleCallback)
	mux.HandleFunc("POST /exchange", handleExchange)
	mux.Handle("GET /style.css", fileServer)
	mux.Handle("GET /script.js", fileServer)
	mux.Handle("GET /favicon.svg", fileServer)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8886"
	}
	log.Printf("listening on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, mux))
}
