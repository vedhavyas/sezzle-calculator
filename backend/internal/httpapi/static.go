package httpapi

import (
	"errors"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"path"
	"strings"
	"syscall"
)

// staticHandler serves the built frontend. There are no client-side routes, so
// "/" serves the app shell and every other path must be a file; anything else
// is a 404 rather than the shell.
func staticHandler(webDir string) http.Handler {
	fsys := os.DirFS(webDir)
	files := http.FileServerFS(fsys)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			writeCode(w, codeMethodNotAllowed, "only GET and HEAD serve static files")
			return
		}
		requested := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if requested == "" || requested == "index.html" {
			serveIndex(w, r, fsys)
			return
		}

		info, err := fs.Stat(fsys, requested)
		switch {
		case err == nil && info.IsDir():
			writeCode(w, codeNotFound, "no file at "+r.URL.Path)
			return
		case err == nil:
			// Stat succeeding is not proof of readability: open once so a permission
			// fault becomes a logged 500 rather than FileServerFS's plain-text 403.
			file, openErr := fsys.Open(requested)
			if openErr != nil {
				if isMissing(openErr) {
					writeCode(w, codeNotFound, "no file at "+r.URL.Path)
					return
				}
				slog.Error("static file unavailable", "method", r.Method, "path", r.URL.Path, "error", openErr)
				writeCode(w, codeInternal, "the frontend files could not be read")
				return
			}
			_ = file.Close()
			files.ServeHTTP(w, r)
			return
		case isMissing(err):
			writeCode(w, codeNotFound, "no file at "+r.URL.Path)
			return
		default:
			slog.Error("static file unavailable", "method", r.Method, "path", r.URL.Path, "error", err)
			writeCode(w, codeInternal, "the frontend files could not be read")
			return
		}
	})
}

// isMissing covers the client-shaped failures a path can produce: a path that is
// too long or runs through a file is not a server fault.
func isMissing(err error) bool {
	if errors.Is(err, fs.ErrNotExist) || errors.Is(err, fs.ErrInvalid) {
		return true
	}
	var pathErr *fs.PathError
	if errors.As(err, &pathErr) {
		for _, target := range []error{syscall.ENOTDIR, syscall.ENAMETOOLONG, syscall.EINVAL} {
			if errors.Is(pathErr.Err, target) {
				return true
			}
		}
	}
	return false
}

func serveIndex(w http.ResponseWriter, r *http.Request, fsys fs.FS) {
	index, err := fs.ReadFile(fsys, "index.html")
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			writeCode(w, codeNotFound, "the frontend build is missing")
			return
		}
		slog.Error("could not read index.html", "method", r.Method, "path", r.URL.Path, "error", err)
		writeCode(w, codeInternal, "the frontend files could not be read")
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	_, _ = w.Write(index)
}
