package httpapi

import (
	"net/http"
	"path"
)

type healthBody struct {
	Status string `json:"status"`
}

// New returns the service handler. An empty webDir serves the API only.
func New(webDir string) http.Handler {
	mux := http.NewServeMux()

	route(mux, http.MethodGet, "/health", health)
	route(mux, http.MethodPost, "/api/v1/calculate", calculate)

	// Every other path falls through to one handler. There are no client-side
	// routes, so a path that is not a file is a 404 with the envelope; an unknown
	// API path lands here too, rather than needing a pattern of its own.
	if webDir != "" {
		mux.Handle("/", staticHandler(webDir))
	} else {
		mux.HandleFunc("/", notFound)
	}

	// Logging outside recovery, so a recovered panic is logged with the 500 the
	// client actually received rather than with no status.
	return logRequests(recoverPanics(limitBody(cleanPaths(mux))))
}

// cleanPaths answers a path net/http would redirect before any handler runs
// with the same 404 envelope as any other unknown path.
func cleanPaths(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != path.Clean(r.URL.Path) {
			notFound(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// route registers handler for one method on path, and for every other method on
// the same path a 405 envelope: net/http's own 405 is plain text.
func route(mux *http.ServeMux, method, path string, handler http.HandlerFunc) {
	mux.HandleFunc(method+" "+path, handler)
	allowed := method
	if method == http.MethodGet {
		allowed = "GET, HEAD"
	}
	mux.HandleFunc(path, methodNotAllowedFor(allowed))
}

func health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, healthBody{Status: "ok"})
}

func notFound(w http.ResponseWriter, r *http.Request) {
	writeCode(w, codeNotFound, "no endpoint at "+r.URL.Path)
}

func methodNotAllowedFor(allowed string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeMethodNotAllowed(w, allowed, "only "+allowed+" is supported at "+r.URL.Path)
	}
}

func writeMethodNotAllowed(w http.ResponseWriter, allowed, message string) {
	w.Header().Set("Allow", allowed)
	writeCode(w, codeMethodNotAllowed, message)
}
