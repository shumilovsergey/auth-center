package main

import (
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"time"
)

// githubAllowedIPs gates the /github/* surface: only these client IPs may use
// the proxy as a GitHub relay. Empty = the whole feature is off (see main.go).
// The client IP is trusted from X-Forwarded-For — nginx must set it to the real
// caller and strip any spoofed value; this proxy does not police that.
var githubAllowedIPs []string

// git clone and binary downloads run far longer than the shared 10s client
// allows, so github traffic gets its own generous timeout. Redirects are
// followed by default, which covers raw.githubusercontent.com's CDN hops.
var githubClient = &http.Client{Timeout: 5 * time.Minute}

// hop-by-hop headers are connection-scoped and must not be forwarded verbatim.
var hopHeaders = map[string]bool{
	"Connection":          true,
	"Keep-Alive":          true,
	"Proxy-Authenticate":  true,
	"Proxy-Authorization": true,
	"Te":                  true,
	"Trailer":             true,
	"Transfer-Encoding":   true,
	"Upgrade":             true,
}

func copyHeaders(dst, src http.Header) {
	for k, vv := range src {
		if hopHeaders[http.CanonicalHeaderKey(k)] {
			continue
		}
		for _, v := range vv {
			dst.Add(k, v)
		}
	}
}

// clientIP prefers the first entry of X-Forwarded-For (the original caller),
// falling back to the TCP peer when the header is absent.
func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if i := strings.IndexByte(xff, ','); i >= 0 {
			return strings.TrimSpace(xff[:i])
		}
		return strings.TrimSpace(xff)
	}
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}
	return r.RemoteAddr
}

// parseCSV splits a comma-separated env value into trimmed, non-empty entries.
func parseCSV(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func githubIPAllowed(r *http.Request) bool {
	ip := clientIP(r)
	for _, allowed := range githubAllowedIPs {
		if ip == allowed {
			return true
		}
	}
	return false
}

// GET|POST /github/{path...} — IP-gated reverse proxy to GitHub for a box that
// can't reach github.com directly. Two upstreams, chosen by prefix:
//
//	/github/raw/{owner}/{repo}/{ref}/{path}  → raw.githubusercontent.com/...  (deploy binaries)
//	/github/{owner}/{repo}.git/{path}        → github.com/...                 (ansible git pull)
//
// The path, method, query string, and body are passed through untouched; the
// upstream response (status, headers, body) is streamed back verbatim.
func handleGitHub(w http.ResponseWriter, r *http.Request) {
	if !githubIPAllowed(r) {
		log.Printf("github error=forbidden ip=%s path=%s", clientIP(r), r.URL.Path)
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	path := r.PathValue("path")
	var target string
	if raw, ok := strings.CutPrefix(path, "raw/"); ok {
		target = "https://raw.githubusercontent.com/" + raw
	} else {
		target = "https://github.com/" + path
	}
	if r.URL.RawQuery != "" {
		target += "?" + r.URL.RawQuery
	}

	// r.Context() ties the upstream call to the client: if the caller aborts
	// (e.g. a cancelled git fetch), the upstream request is cancelled too.
	req, err := http.NewRequestWithContext(r.Context(), r.Method, target, r.Body)
	if err != nil {
		log.Printf("github error=build_request path=%s: %v", path, err)
		http.Error(w, "proxy error", http.StatusInternalServerError)
		return
	}
	copyHeaders(req.Header, r.Header)
	req.ContentLength = r.ContentLength // git-upload-pack POSTs a sized body

	resp, err := githubClient.Do(req)
	if err != nil {
		log.Printf("github error=upstream path=%s: %v", path, err)
		http.Error(w, "upstream unreachable", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	copyHeaders(w.Header(), resp.Header)
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body) //nolint:errcheck
}
