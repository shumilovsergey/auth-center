package main

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/gorilla/sessions"
	"github.com/joho/godotenv"
)

// ── embedded web files ────────────────────────────────────────────────────

//go:embed web
var webFiles embed.FS

// ── config ────────────────────────────────────────────────────────────────

var (
	authURL      string
	authInternal string
	appURL       string
	appToken     string
	store        *sessions.CookieStore
	tmpl         *template.Template
	httpClient   = &http.Client{}
)

// ── template data ─────────────────────────────────────────────────────────

type pageData struct {
	User   map[string]any
	Method string
	Error  string
}

// userID formats the user ID regardless of whether it came back as
// float64 (Telegram — JSON number) or string (Solana — JSON string).
func userID(user map[string]any) string {
	if user == nil {
		return ""
	}
	switch v := user["id"].(type) {
	case float64:
		return fmt.Sprintf("%.0f", v)
	case string:
		return v
	}
	return ""
}

// fullName joins first_name and last_name, ignoring empty parts.
func fullName(user map[string]any) string {
	parts := []string{}
	for _, k := range []string{"first_name", "last_name"} {
		if s, ok := user[k].(string); ok && s != "" {
			parts = append(parts, s)
		}
	}
	return strings.Join(parts, " ")
}

// ── template ──────────────────────────────────────────────────────────────

func initTemplate() {
	src, err := webFiles.ReadFile("web/index.html")
	if err != nil {
		log.Fatalf("web/index.html not found: %v", err)
	}
	funcs := template.FuncMap{
		"userID":   userID,
		"fullName": fullName,
	}
	tmpl = template.Must(template.New("index").Funcs(funcs).Parse(string(src)))
}

// ── session helpers ───────────────────────────────────────────────────────

func getUser(r *http.Request) (map[string]any, string) {
	sess, _ := store.Get(r, "s")
	userJSON, _ := sess.Values["user"].(string)
	method, _ := sess.Values["method"].(string)
	if userJSON == "" {
		return nil, ""
	}
	var user map[string]any
	json.Unmarshal([]byte(userJSON), &user) //nolint:errcheck
	return user, method
}

func saveUser(w http.ResponseWriter, r *http.Request, user map[string]any, method string) {
	sess, _ := store.Get(r, "s")
	b, _ := json.Marshal(user)
	sess.Values["user"] = string(b)
	sess.Values["method"] = method
	sess.Save(r, w) //nolint:errcheck
}

func clearUser(w http.ResponseWriter, r *http.Request) {
	sess, _ := store.Get(r, "s")
	sess.Values = map[any]any{}
	sess.Save(r, w) //nolint:errcheck
}

// ── handlers ──────────────────────────────────────────────────────────────

func handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	code := r.URL.Query().Get("code")
	var errMsg string

	if code != "" {
		body, _ := json.Marshal(map[string]string{
			"code":      code,
			"app_token": appToken,
		})
		resp, err := httpClient.Post(
			authInternal+"/exchange",
			"application/json",
			bytes.NewReader(body),
		)
		if err != nil {
			errMsg = "could not reach auth center"
		} else {
			defer resp.Body.Close()
			var data map[string]any
			json.NewDecoder(resp.Body).Decode(&data) //nolint:errcheck
			if data["ok"] == true {
				user, _ := data["user"].(map[string]any)
				method, _ := data["method"].(string)
				saveUser(w, r, user, method)
				http.Redirect(w, r, "/", http.StatusFound)
				return
			} else if e, ok := data["error"].(string); ok {
				errMsg = e
			} else {
				errMsg = "exchange failed"
			}
		}
	}

	user, method := getUser(r)
	tmpl.Execute(w, pageData{User: user, Method: method, Error: errMsg}) //nolint:errcheck
}

func handleLogin(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, authURL+"/?redirect="+appURL+"/", http.StatusFound)
}

func handleLogout(w http.ResponseWriter, r *http.Request) {
	clearUser(w, r)
	http.Redirect(w, r, "/", http.StatusFound)
}

// ── main ──────────────────────────────────────────────────────────────────

func main() {
	godotenv.Load() //nolint:errcheck

	authURL = os.Getenv("AUTH_URL")
	authInternal = os.Getenv("AUTH_INTERNAL")
	appURL = os.Getenv("APP_URL")
	appToken = os.Getenv("APP_TOKEN")

	secretKey := os.Getenv("SECRET_KEY")
	if secretKey == "" {
		secretKey = "dev-secret"
	}
	store = sessions.NewCookieStore([]byte(secretKey))
	store.Options = &sessions.Options{
		Path:     "/",
		MaxAge:   86400 * 30,
		HttpOnly: true,
	}

	initTemplate()

	webFS, _ := fs.Sub(webFiles, "web")
	fileServer := http.FileServer(http.FS(webFS))

	mux := http.NewServeMux()
	mux.HandleFunc("GET /", handleIndex)
	mux.HandleFunc("GET /login", handleLogin)
	mux.HandleFunc("GET /logout", handleLogout)
	mux.Handle("GET /favicon.svg", fileServer)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8890"
	}
	log.Printf("listening on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, mux))
}
