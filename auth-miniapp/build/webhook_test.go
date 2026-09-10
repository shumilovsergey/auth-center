package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// The bot has exactly one decision to make — greet or delete — and it makes it
// on this function. Getting it wrong is not a cosmetic failure: a /start read
// as "anything else" means the bot deletes the message that was supposed to
// start the conversation, and says nothing about it.
func TestIsStartCommand(t *testing.T) {
	cases := []struct {
		text string
		want bool
	}{
		// The shapes Telegram actually sends.
		{"/start", true},
		{"/start s0meSessionToken", true}, // deeplink payload
		{"/start@auth_center_bot", true},  // addressed to one bot in a group
		{"/start@auth_center_bot arg", true},
		{"/start\nsecond line", true},

		// Not a /start, however much it looks like one.
		{"/startgame", false}, // a different command with the same prefix
		{"/start_over", false},
		{"start", false},
		{" /start", false}, // Telegram does not pad commands
		{"/START", false},  // commands are case-sensitive
		{"привет", false},
		{"", false}, // a photo or a sticker: no text at all
	}

	for _, c := range cases {
		if got := isStartCommand(c.text); got != c.want {
			t.Errorf("isStartCommand(%q) = %v, want %v", c.text, got, c.want)
		}
	}
}

// The secret is the only thing between this route and anyone on the internet
// who can guess a hostname: without it, a stranger could make the bot greet
// people and delete message ids of their choosing.
func TestWebhookRejectsAForgedUpdate(t *testing.T) {
	prev := webhookSecret
	defer func() { webhookSecret = prev }()
	webhookSecret = "the-real-secret"

	body := `{"update_id":1,"message":{"message_id":7,"chat":{"id":42,"type":"private"},"text":"привет"}}`

	for _, sent := range []string{"", "wrong", "the-real-secre"} {
		req := httptest.NewRequest(http.MethodPost, webhookPath, strings.NewReader(body))
		if sent != "" {
			req.Header.Set("X-Telegram-Bot-Api-Secret-Token", sent)
		}
		rec := httptest.NewRecorder()
		handleWebhook(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Errorf("secret %q: got %d, want %d", sent, rec.Code, http.StatusForbidden)
		}
	}
}

// Telegram redelivers an update until it gets a 2xx. Anything this app cannot
// act on must therefore still answer 200, or one unreadable update becomes a
// retry loop that outlives everyone's patience.
func TestWebhookAnswers200ToWhatItCannotAct(t *testing.T) {
	prev := webhookSecret
	defer func() { webhookSecret = prev }()
	webhookSecret = "s"

	// No Telegram call is made for either of these, so the test needs no bot.
	cases := map[string]string{
		"not json":        `{{{`,
		"no message":      `{"update_id":1}`,
		"edited, ignored": `{"update_id":1,"edited_message":{"message_id":7}}`,
	}

	for name, body := range cases {
		req := httptest.NewRequest(http.MethodPost, webhookPath, strings.NewReader(body))
		req.Header.Set("X-Telegram-Bot-Api-Secret-Token", "s")
		rec := httptest.NewRecorder()
		handleWebhook(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("%s: got %d, want %d", name, rec.Code, http.StatusOK)
		}
	}
}

// stubTelegram stands in for api.telegram.org and records what it was asked to
// do, so the two behaviours can be asserted on the wire rather than by reading
// the code back.
type botCall struct {
	method  string
	payload map[string]any
}

func stubTelegram(t *testing.T, calls *[]botCall) func() {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]any
		json.NewDecoder(r.Body).Decode(&payload) //nolint:errcheck

		// "/bot<token>/<method>" — the last segment is what was called.
		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		*calls = append(*calls, botCall{method: parts[len(parts)-1], payload: payload})

		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ok":true,"result":true}`)) //nolint:errcheck
	}))

	prevBase, prevToken := botAPIBase, botToken
	botAPIBase, botToken = srv.URL, "test-token"

	return func() {
		botAPIBase, botToken = prevBase, prevToken
		srv.Close()
	}
}

// The two scenarios this bot exists for, end to end from a decoded update.
func TestBotGreetsStartAndDeletesEverythingElse(t *testing.T) {
	t.Run("/start is answered, not deleted", func(t *testing.T) {
		var calls []botCall
		defer stubTelegram(t, &calls)()

		handleMessage(tgMessage{
			MessageID: 7,
			Chat:      tgChat{ID: 42, Type: "private"},
			Text:      "/start",
		})

		if len(calls) != 1 || calls[0].method != "sendMessage" {
			t.Fatalf("got %+v, want one sendMessage", calls)
		}
		if got := calls[0].payload["text"]; got != greeting {
			t.Errorf("text = %v, want %q", got, greeting)
		}
		if got := calls[0].payload["chat_id"]; got != float64(42) {
			t.Errorf("chat_id = %v, want 42", got)
		}
	})

	t.Run("anything else is deleted", func(t *testing.T) {
		// A sticker carries no text at all, which is exactly the case a naive
		// "text == /start" check would still get right and a naive
		// "text is empty means ignore" check would not.
		for _, text := range []string{"привет", "/help", "/startgame", ""} {
			var calls []botCall
			done := stubTelegram(t, &calls)

			handleMessage(tgMessage{
				MessageID: 7,
				Chat:      tgChat{ID: 42, Type: "private"},
				Text:      text,
			})

			if len(calls) != 1 || calls[0].method != "deleteMessage" {
				t.Fatalf("%q: got %+v, want one deleteMessage", text, calls)
			}
			if got := calls[0].payload["message_id"]; got != float64(7) {
				t.Errorf("%q: message_id = %v, want 7", text, got)
			}
			done()
		}
	})

	// A bot that deletes by default must not do it in a room full of other
	// people's messages. Added to a group, it stays silent.
	t.Run("groups are left alone", func(t *testing.T) {
		for _, kind := range []string{"group", "supergroup", "channel"} {
			var calls []botCall
			done := stubTelegram(t, &calls)

			handleMessage(tgMessage{
				MessageID: 7,
				Chat:      tgChat{ID: -100, Type: kind},
				Text:      "что угодно",
			})

			if len(calls) != 0 {
				t.Errorf("%s: got %+v, want no calls at all", kind, calls)
			}
			done()
		}
	})
}
