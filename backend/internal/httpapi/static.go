package httpapi

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"syscall"
)

// staticHandler serves the built frontend. There are no client-side routes, so
// "/" serves the app shell and every other path must be a file; anything else
// is a 404 rather than the shell.
func staticHandler(webDir string) http.Handler {
	fsys := os.DirFS(webDir)
	files := http.FileServerFS(fsys)
	root := canonicalRoot(webDir)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requested := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if requested == "" || requested == "index.html" {
			if !servesFiles(r.Method) {
				writeMethodNotAllowed(w, "GET, HEAD", "only GET and HEAD serve static files")
				return
			}
			serveIndex(w, r, fsys)
			return
		}

		info, err := fs.Stat(fsys, requested)
		switch {
		case err == nil && info.IsDir():
			writeCode(w, codeNotFound, "no file at "+r.URL.Path)
			return
		case err == nil:
			if !servesFiles(r.Method) {
				writeMethodNotAllowed(w, "GET, HEAD", "only GET and HEAD serve static files")
				return
			}
			// os.DirFS is lexical: a symlink inside the build directory could
			// point anywhere. Refuse one that resolves outside the root.
			if !insideRoot(root, requested) {
				writeCode(w, codeNotFound, "no file at "+r.URL.Path)
				return
			}
			// Stat succeeding is not proof of readability: open once so a permission
			// fault becomes a logged 500 rather than FileServerFS's plain-text 403.
			file, openErr := fsys.Open(requested)
			if openErr != nil {
				if isMissing(openErr) {
					writeCode(w, codeNotFound, "no file at "+r.URL.Path)
					return
				}
				slog.Error("static file unavailable", "method", r.Method, "path", logText(r.URL.Path), "error", openErr)
				writeCode(w, codeInternal, "the frontend files could not be read")
				return
			}
			_ = file.Close()
			serveFile(w, r, info, files)
			return
		case isMissing(err):
			writeCode(w, codeNotFound, "no file at "+r.URL.Path)
			return
		default:
			slog.Error("static file unavailable", "method", r.Method, "path", logText(r.URL.Path), "error", err)
			writeCode(w, codeInternal, "the frontend files could not be read")
			return
		}
	})
}

func servesFiles(method string) bool {
	return method == http.MethodGet || method == http.MethodHead
}

// serveFile sets cache validators and serves the whole file. Range is dropped:
// a multi-range header can turn a one-megabyte asset into tens of megabytes of
// response, and these are small static files.
func serveFile(w http.ResponseWriter, r *http.Request, info fs.FileInfo, files http.Handler) {
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("ETag", fmt.Sprintf(`"%x-%x"`, info.Size(), info.ModTime().UnixNano()))
	r.Header.Del("Range")
	files.ServeHTTP(w, r)
}

// canonicalRoot resolves the web directory once, so a symlink inside it can be
// checked against the real path rather than the lexical one.
func canonicalRoot(webDir string) string {
	root, err := filepath.Abs(webDir)
	if err != nil {
		return webDir
	}
	if resolved, err := filepath.EvalSymlinks(root); err == nil {
		return resolved
	}
	return root
}

func insideRoot(root, requested string) bool {
	resolved, err := filepath.EvalSymlinks(filepath.Join(root, requested))
	if err != nil {
		return false
	}
	return resolved == root || strings.HasPrefix(resolved, root+string(os.PathSeparator))
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
		slog.Error("could not read index.html", "method", r.Method, "path", logText(r.URL.Path), "error", err)
		writeCode(w, codeInternal, "the frontend files could not be read")
		return
	}
	sum := sha256.Sum256(index)
	etag := fmt.Sprintf(`"%x"`, sum[:16])
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("ETag", etag)
	if r.Header.Get("If-None-Match") == etag {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(index)
}
