package main

import (
	"embed"
	"fmt"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/joho/godotenv"
)

var buildTime = "unknown"

// defaultAppName is the fallback when APP_NAME is unset, so a service file that
// forgets the variable still starts and still gets auth-client.db rather than a
// database named after the empty string.
//
// ── FORKING THIS TEMPLATE: change this to your app's name. ──
const defaultAppName = "auth-client"

// appName is the app's runtime identity: the --version banner, the start log
// line, and (when DB_PATH is unset) the database filename all derive from it.
var appName string

//go:embed web
var webFiles embed.FS

var (
	authURL      string
	authInternal string
	appURL       string
	appToken     string
	tmpl         *template.Template
	httpClient   = &http.Client{}
)

type pageData struct {
	User  *User
	Error string
}

func initTemplate() {
	src, err := webFiles.ReadFile("web/index.html")
	if err != nil {
		log.Fatalf("web/index.html not found: %v", err)
	}
	tmpl = template.Must(template.New("index").Parse(string(src)))
}

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

// cacheStatic wraps a handler with a 30-day immutable cache header.
func cacheStatic(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "public, max-age=2592000") // 30 days
		h.ServeHTTP(w, r)
	})
}

// ── app routes ────────────────────────────────────────────────────────────────
// Add your app-specific handlers here.

func handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	if code := r.URL.Query().Get("code"); code != "" {
		handleCallback(w, r, code)
		return
	}
	var user *User
	if uid := sessionUserID(r); uid != 0 {
		user, _ = getUserByID(uid)
	}
	tmpl.Execute(w, pageData{User: user}) //nolint:errcheck
}

// ── main ──────────────────────────────────────────────────────────────────────

func main() {
	log.SetFlags(log.Ldate | log.Ltime | log.LUTC)
	godotenv.Load() //nolint:errcheck

	// Resolved before anything else: --version prints it, and the database is
	// named after it. Everything downstream reads appName, never a literal.
	appName = os.Getenv("APP_NAME")
	if appName == "" {
		appName = defaultAppName
	}

	if len(os.Args) == 2 && (os.Args[1] == "--version" || os.Args[1] == "--info") {
		fmt.Printf("%s built: %s\n", appName, buildTime)
		os.Exit(0)
	}

	// An explicit DB_PATH always wins — that is how a compose mount puts the file
	// on a named volume. With nothing set the name follows the app instead of a
	// literal, so a service file copied to the next app cannot leave it quietly
	// writing into auth-client.db. db.go reads DB_PATH, so setting it here keeps
	// that shared template file untouched and makes its own fallback unreachable.
	if os.Getenv("DB_PATH") == "" {
		os.Setenv("DB_PATH", appName+".db") //nolint:errcheck
	}

	authURL = os.Getenv("AUTH_URL")
	authInternal = os.Getenv("AUTH_INTERNAL")
	appURL = os.Getenv("APP_URL")
	appToken = os.Getenv("APP_TOKEN")

	secretKey := os.Getenv("SECRET_KEY")
	if secretKey == "" {
		secretKey = "dev-secret"
	}
	jwtSecret = []byte(secretKey)

	initDB()
	initTemplate()

	webFS, _ := fs.Sub(webFiles, "web")
	fileServer := http.FileServer(http.FS(webFS))

	mux := http.NewServeMux()
	mux.HandleFunc("GET /", handleIndex)
	mux.HandleFunc("GET /login", handleLogin)
	mux.HandleFunc("GET /logout", handleLogout)
	mux.HandleFunc("GET /apps", handleOpenApps)
	mux.Handle("GET /favicon.svg", fileServer)
	mux.Handle("GET /background.webp", cacheStatic(fileServer))
	mux.Handle("GET /style.css", fileServer)
	mux.Handle("GET /script.js", fileServer)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8890"
	}
	log.Printf("start app=%s db=%s port=%s", appName, os.Getenv("DB_PATH"), port)
	log.Fatal(http.ListenAndServe(":"+port, logMiddleware(mux)))
}
