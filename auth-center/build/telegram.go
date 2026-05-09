package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image/color"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/skip2/go-qrcode"
)

// ── config ────────────────────────────────────────────────────────────────────

var (
	botToken      string
	botUsername   string
	webhookSecret string
	telegramAPIURL string
)

func initTelegram() {
	botToken = os.Getenv("BOT_TOKEN")
	botUsername = os.Getenv("BOT_USERNAME")
	webhookSecret = os.Getenv("WEBHOOK_SECRET")
	telegramAPIURL = os.Getenv("TELEGRAM_API_URL")
	if telegramAPIURL == "" {
		telegramAPIURL = "https://api.telegram.org"
	}
}

// ── session store ─────────────────────────────────────────────────────────────

type Session struct {
	Status    string
	User      map[string]any
	CreatedAt time.Time
	Redirect  string
	Code      string
}

var (
	sessions   = make(map[string]*Session)
	sessionsMu sync.Mutex
)

func cleanSessions() {
	sessionsMu.Lock()
	defer sessionsMu.Unlock()
	for k, v := range sessions {
		if time.Since(v.CreatedAt) > sessionTTL {
			delete(sessions, k)
		}
	}
}

// ── helpers ───────────────────────────────────────────────────────────────────

func makeQR(url string) (string, error) {
	qr, err := qrcode.New(url, qrcode.Medium)
	if err != nil {
		return "", err
	}
	qr.BackgroundColor = color.RGBA{R: 8, G: 8, B: 15, A: 255}
	qr.ForegroundColor = color.RGBA{R: 196, G: 181, B: 253, A: 255}
	png, err := qr.PNG(256)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(png), nil
}

func sendTG(chatID int64, text string, redirectURL string) {
	apiURL := fmt.Sprintf("%s/bot%s/sendMessage", telegramAPIURL, botToken)
	payload := map[string]any{"chat_id": chatID, "text": text}
	if redirectURL != "" {
		payload["text"] = text + "\n\n" + redirectURL
		payload["link_preview_options"] = map[string]any{
			"url":             redirectURL,
			"show_above_text": true,
		}
	}
	body, _ := json.Marshal(payload)
	client := &http.Client{Timeout: 5 * time.Second}
	client.Post(apiURL, "application/json", bytes.NewReader(body)) //nolint:errcheck
}

// ── handlers ──────────────────────────────────────────────────────────────────

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
			redirect := sess.Redirect
			if redirect != "" {
				sess.Code = newCode(user, "telegram")
			}
			sessionsMu.Unlock()
			log.Printf("telegram uid=%d name=%q", from.ID, strings.TrimSpace(from.FirstName+" "+from.LastName))
			go sendTG(from.ID, "You are authenticated!", redirect)
		} else {
			expired := ok && time.Since(sess.CreatedAt) > sessionTTL
			if expired {
				delete(sessions, tok)
			}
			sessionsMu.Unlock()
			if ok {
				go sendTG(from.ID, "This QR code has expired.", "")
			}
		}
	}

	w.WriteHeader(http.StatusOK)
}
