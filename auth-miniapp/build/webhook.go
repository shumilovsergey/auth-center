package main

// The bot half of auth-miniapp: an inbound webhook and the two replies it can
// make. It is entirely optional and entirely separate from the login — set no
// APP_URL and this file registers no route, opens no connection to Telegram,
// and the mini app behaves exactly as it did before.
//
// Worth knowing before turning it on: until this file existed, auth-miniapp
// made NO outbound call to Telegram at all — it only verified signatures it
// was handed. That property is why auth-proxy is not in the login path any
// more. Registering a webhook brings back a dependency on api.telegram.org
// being reachable from this host. Nothing in the login breaks if it is not —
// setWebhook fails, this file says so in the log, and every other route
// carries on — but the bot then answers nobody.

import (
	"bytes"
	"crypto/hmac"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

// webhookPath is both the route this app serves and the tail of the URL it
// hands to Telegram. Keeping it in one constant is what lets APP_URL be the
// app's own address rather than a path someone has to keep in step.
const webhookPath = "/webhook"

// What a /start gets in return. The whole vocabulary of this bot.
//
// Plain text on purpose: Telegram linkifies @sergey_showmelove and
// sh-development.ru by itself, so there is no parse_mode here and nothing to
// escape — and nothing that turns into a broken message the day somebody adds
// an underscore or an asterisk to this text.
const greeting = `Привет и добро пожаловать в sh-development 👋
Меня зовут Сергей Шумилов 😎

Снизу слева - кнопка с моими приложениями

Мой Telegram - @sergey_showmelove
Мой сайт - sh-development.ru

Если у вас возникнут вопросы или предложения - you are welcome! 🙂`

// Telegram delivers updates one at a time per bot and waits for the response
// before sending the next, so the handler answers first and works after.
// This bounds the working half.
const botAPITimeout = 10 * time.Second

var (
	// appURL is this app's own public base URL. The bot half is the only thing
	// using it today — it is what setWebhook is told to call — but the value
	// says nothing about webhooks, so anything else that needs to know where
	// this service lives can read the same variable. Empty means the bot half
	// is off.
	appURL string

	// webhookSecret is what tells a real update from a forged one. Telegram
	// echoes it in X-Telegram-Bot-Api-Secret-Token on every delivery.
	//
	// It has no environment variable on purpose. The value is generated at
	// startup and registered in the same setWebhook call that announces the
	// URL, so the two can never drift and there is no credential for anyone to
	// leak, rotate or forget. The cost is that two instances of this app
	// pointed at one bot would each overwrite the other's registration — which
	// is already true of setWebhook itself, secret or no secret.
	webhookSecret string
)

var botClient = &http.Client{Timeout: botAPITimeout}

// botAPIBase is a variable rather than a constant for one reason: the tests
// point it at a local stub and assert on what this file actually sends. There
// is no environment variable for it — a real deployment has no business
// talking to anything else. (An air-gapped host would route through a proxy at
// the network level, the way auth-proxy once did, not by rewriting this.)
var botAPIBase = "https://api.telegram.org"

// ── the Bot API ───────────────────────────────────────────────────────────────

// callTelegram posts to one Bot API method and reports whether Telegram was
// happy. Note what is NOT in the error text: the URL, which carries the bot
// token. A token in a log line is a token in a backup, and this one is full
// control of the bot.
func callTelegram(method string, payload map[string]any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("%s: %w", method, err)
	}

	resp, err := botClient.Post(
		botAPIBase+"/bot"+botToken+"/"+method,
		"application/json",
		bytes.NewReader(body),
	)
	if err != nil {
		return fmt.Errorf("%s: telegram unreachable", method)
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<10))
	var out struct {
		OK          bool   `json:"ok"`
		Description string `json:"description"`
	}
	json.Unmarshal(raw, &out) //nolint:errcheck
	if !out.OK {
		if out.Description == "" {
			out.Description = strings.TrimSpace(string(raw))
		}
		// Telegram's own wording is kept as-is: "message can't be deleted" and
		// "chat not found" are different problems with the same status code.
		return fmt.Errorf("%s: telegram said %d: %s", method, resp.StatusCode, out.Description)
	}
	return nil
}

// ── registration ──────────────────────────────────────────────────────────────

// initWebhook decides whether the bot half runs at all, and returns true when
// the route should be served.
func initWebhook() bool {
	if appURL == "" {
		log.Print("webhook: APP_URL not set — bot disabled, no outbound calls to telegram")
		return false
	}

	secret, err := newWebhookSecret()
	if err != nil {
		log.Printf("webhook: no randomness for a secret (%v) — bot disabled", err)
		return false
	}
	webhookSecret = secret

	// Registration is deliberately not on the startup path. It is an outbound
	// call to a third party, and a login must not wait on one: the moment
	// ListenAndServe is up, /api/auth works whether or not Telegram ever picks
	// up the phone.
	go registerWebhook()
	return true
}

func newWebhookSecret() (string, error) {
	// Telegram allows 1–256 chars of A-Z a-z 0-9 _ - in a secret token, which
	// base64url without padding stays inside.
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func registerWebhook() {
	// APP_URL is this app's address; the path is ours to know. Someone who
	// writes the full URL including /webhook gets the same result rather than
	// a doubled path — a forgiving read costs one line and saves a support
	// round-trip.
	url := strings.TrimRight(appURL, "/")
	if !strings.HasSuffix(url, webhookPath) {
		url += webhookPath
	}

	err := callTelegram("setWebhook", map[string]any{
		"url":          url,
		"secret_token": webhookSecret,
		// Only messages are wanted, and asking for only what is handled keeps
		// Telegram from delivering edits, reactions and channel posts that
		// this file would answer with silence.
		"allowed_updates": []string{"message"},
		// Whatever piled up while the service was down is not worth acting on:
		// a greeting owed since yesterday is noise, and a message older than
		// 48 hours cannot be deleted anyway.
		"drop_pending_updates": true,
	})
	if err != nil {
		log.Printf("webhook: NOT registered: %v", err)
		return
	}
	log.Printf("webhook: registered url=%s", url)
}

// ── the handler ───────────────────────────────────────────────────────────────

type tgChat struct {
	ID int64 `json:"id"`
	// Type is "private", "group", "supergroup" or "channel". It is the only
	// thing standing between this bot and a group it was added to — see
	// handleMessage.
	Type string `json:"type"`
}

type tgMessage struct {
	MessageID int64  `json:"message_id"`
	Chat      tgChat `json:"chat"`
	Text      string `json:"text"`
}

type tgUpdate struct {
	UpdateID int64      `json:"update_id"`
	Message  *tgMessage `json:"message"`
}

// handleWebhook answers Telegram, then acts.
//
// Every ending here is a 200, including the malformed ones. A non-2xx makes
// Telegram redeliver the same update, and an update this app could not read
// once it will not read on the fifth attempt — the only thing a retry would
// buy is the same log line again. The one exception is the secret check: a
// caller who is not Telegram gets 403 and no hint about what was wrong.
func handleWebhook(w http.ResponseWriter, r *http.Request) {
	// The empty check is not redundant with the route registration that already
	// depends on it: hmac.Equal("", "") is true, so a future refactor that
	// mounted this route unconditionally would otherwise accept every caller.
	if webhookSecret == "" ||
		!hmac.Equal([]byte(r.Header.Get("X-Telegram-Bot-Api-Secret-Token")), []byte(webhookSecret)) {
		log.Printf("webhook: REJECTED from=%s — secret mismatch", r.RemoteAddr)
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	var upd tgUpdate
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&upd); err != nil {
		log.Printf("webhook: unreadable update: %v", err)
		w.WriteHeader(http.StatusOK)
		return
	}
	if upd.Message == nil {
		w.WriteHeader(http.StatusOK)
		return
	}

	// Answer first, work after. Telegram holds the next update until this
	// response arrives, so doing the Bot API call inline would make one slow
	// round-trip delay the following user's. The message is a copy by now and
	// borrows nothing from the request.
	w.WriteHeader(http.StatusOK)
	go handleMessage(*upd.Message)
}

// handleMessage is the whole behaviour: greet a /start, delete anything else.
func handleMessage(m tgMessage) {
	// Deleting is destructive and this bot deletes by default, so it does it
	// only where the rule was meant to apply: a one-to-one chat with the bot.
	// Added to a group, it would otherwise start clearing other people's
	// messages — and with admin rights it would succeed. Drop this guard if a
	// group is ever a place this bot belongs.
	if m.Chat.Type != "private" {
		log.Printf("webhook: ignored chat=%d type=%s", m.Chat.ID, m.Chat.Type)
		return
	}

	if isStartCommand(m.Text) {
		if err := callTelegram("sendMessage", map[string]any{
			"chat_id": m.Chat.ID,
			"text":    greeting,
		}); err != nil {
			log.Printf("webhook: greet chat=%d failed: %v", m.Chat.ID, err)
			return
		}
		log.Printf("webhook: start chat=%d — greeted", m.Chat.ID)
		return
	}

	if err := callTelegram("deleteMessage", map[string]any{
		"chat_id":    m.Chat.ID,
		"message_id": m.MessageID,
	}); err != nil {
		// Expected failures live here too: Telegram refuses to delete anything
		// older than 48 hours, which is why this is a log line and not a retry.
		log.Printf("webhook: delete chat=%d msg=%d failed: %v", m.Chat.ID, m.MessageID, err)
		return
	}
	log.Printf("webhook: deleted chat=%d msg=%d", m.Chat.ID, m.MessageID)
}

// isStartCommand recognises the /start Telegram itself sends, in every shape it
// arrives in: bare, with a deeplink payload, and addressed to a specific bot.
//
// The prefix alone is not enough — "/startgame" begins with "/start" and is a
// different command. What follows has to be nothing, or a separator.
func isStartCommand(text string) bool {
	const cmd = "/start"
	if !strings.HasPrefix(text, cmd) {
		return false
	}
	rest := text[len(cmd):]
	if rest == "" {
		return true
	}
	switch rest[0] {
	case ' ', '\t', '\n', '@':
		return true
	}
	return false
}
