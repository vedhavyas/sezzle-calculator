package httpapi

import (
	"log/slog"
	"net/http"
	"runtime/debug"
	"time"
)

const maxBodyBytes = 1 << 20

func limitBody(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
		next.ServeHTTP(w, r)
	})
}

// responseRecorder remembers the status, so logging can report it and recovery can
// tell "nothing sent yet" from "a response is already on the wire".
type responseRecorder struct {
	http.ResponseWriter
	status int
}

func (r *responseRecorder) WriteHeader(status int) {
	if r.status != 0 {
		return
	}
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func (r *responseRecorder) Write(body []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	return r.ResponseWriter.Write(body)
}

func (r *responseRecorder) written() bool { return r.status != 0 }

func logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		recorder := &responseRecorder{ResponseWriter: w}
		defer func() {
			slog.Info("request",
				"method", r.Method,
				"path", r.URL.Path,
				"status", recorder.status,
				"duration", time.Since(started).Round(time.Microsecond).String(),
			)
		}()
		next.ServeHTTP(recorder, r)
	})
}

// recoverPanics answers with the JSON envelope when nothing has been written, and
// aborts the connection when a response is already committed: a second header
// cannot replace the first, and appending would produce a 200 with broken JSON.
func recoverPanics(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		recorder := &responseRecorder{ResponseWriter: w}
		defer func() {
			recovered := recover()
			if recovered == nil {
				return
			}
			if recovered == http.ErrAbortHandler {
				panic(recovered)
			}
			slog.Error(
				"panic while serving",
				"method", r.Method,
				"path", r.URL.Path,
				"panic", recovered,
				"stack", string(debug.Stack()),
			)
			if recorder.written() {
				panic(http.ErrAbortHandler)
			}
			writeCode(recorder, codeInternal, "the server failed to process the request")
		}()
		next.ServeHTTP(recorder, r)
	})
}
