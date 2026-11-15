package httpapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vedhavyas/sezzle-calculator/backend/internal/calculator"
)

func serve(handler http.Handler, method, target, body string, headers ...map[string]string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, target, strings.NewReader(body))
	for _, header := range headers {
		for name, value := range header {
			request.Header.Set(name, value)
		}
	}
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	return recorder
}

func errorCode(t *testing.T, recorder *httptest.ResponseRecorder) string {
	t.Helper()
	var body struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("response body is not the error envelope: %v (%s)", err, recorder.Body.String())
	}
	if body.Error.Message == "" {
		t.Fatalf("error envelope carries no message: %s", recorder.Body.String())
	}
	return body.Error.Code
}

func errorMessage(t *testing.T, recorder *httptest.ResponseRecorder) string {
	t.Helper()
	var body struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("response body is not the error envelope: %v", err)
	}
	return body.Error.Message
}

func TestCalculateEndpoint(t *testing.T) {
	cases := []struct {
		name        string
		body        string
		headers     map[string]string
		wantStatus  int
		wantResult  string
		wantCode    string
		wantMessage string
	}{
		{name: "add with strings", body: `{"operation":"add","operands":["2","3"]}`, wantStatus: 200, wantResult: "5"},
		{name: "add with numbers", body: `{"operation":"add","operands":[2,3]}`, wantStatus: 200, wantResult: "5"},
		{name: "add keeps decimals exact", body: `{"operation":"add","operands":["0.1",0.2]}`, wantStatus: 200, wantResult: "0.3"},
		{name: "subtract", body: `{"operation":"subtract","operands":["1","2"]}`, wantStatus: 200, wantResult: "-1"},
		{name: "multiply", body: `{"operation":"multiply","operands":["0.1","0.2"]}`, wantStatus: 200, wantResult: "0.02"},
		{name: "divide", body: `{"operation":"divide","operands":["10","4"]}`, wantStatus: 200, wantResult: "2.5"},
		{name: "power", body: `{"operation":"power","operands":["2","10"]}`, wantStatus: 200, wantResult: "1024"},
		{name: "sqrt", body: `{"operation":"sqrt","operands":["16"]}`, wantStatus: 200, wantResult: "4"},
		{name: "percent", body: `{"operation":"percent","operands":["15","200"]}`, wantStatus: 200, wantResult: "30"},
		{name: "exponent notation", body: `{"operation":"add","operands":["1e3","1"]}`, wantStatus: 200, wantResult: "1001"},
		{name: "explicit plus", body: `{"operation":"add","operands":["+5","1"]}`, wantStatus: 200, wantResult: "6"},
		{name: "leading dot", body: `{"operation":"add","operands":[".5","1"]}`, wantStatus: 200, wantResult: "1.5"},
		{name: "trailing dot", body: `{"operation":"add","operands":["5.","1"]}`, wantStatus: 200, wantResult: "6"},
		{name: "operand at the length cap", body: `{"operation":"add","operands":["` + strings.Repeat("1", 100) + `","1"]}`, wantStatus: 200, wantResult: strings.Repeat("1", 99) + "2"},
		{name: "content type not enforced", body: `{"operation":"add","operands":["1","1"]}`, headers: map[string]string{"Content-Type": "text/plain"}, wantStatus: 200, wantResult: "2"},
		{name: "divide by zero", body: `{"operation":"divide","operands":["10","0"]}`, wantStatus: 422, wantCode: calculator.CodeDivisionByZero},
		{name: "negative sqrt", body: `{"operation":"sqrt","operands":["-4"]}`, wantStatus: 422, wantCode: calculator.CodeNegativeSqrt},
		{name: "non-real power", body: `{"operation":"power","operands":["-8","0.5"]}`, wantStatus: 422, wantCode: calculator.CodeUndefinedPower},
		{name: "large exponent accepted", body: `{"operation":"power","operands":["2","1001"]}`, wantStatus: 200, wantResult: "21430172143725346418968500981200036211228096234110672148875007767407021022498722449863967576313917162551893458351062936503742905713846280871969155149397149607869135549648461970842149210124742283755908364306092949967163882534797535118331087892154125829142392955373084335320859663305248773674411336138752"},
		{name: "result too large", body: `{"operation":"power","operands":["1e1000","99.5"]}`, wantStatus: 422, wantCode: calculator.CodeResultTooLarge},
		{name: "unknown operation", body: `{"operation":"sqrtt","operands":["1"]}`, wantStatus: 422, wantCode: calculator.CodeUnknownOperation, wantMessage: "supported: add, divide"},
		{name: "missing operation", body: `{"operands":["1","2"]}`, wantStatus: 422, wantCode: calculator.CodeUnknownOperation, wantMessage: "operation is required"},
		{name: "wrong operand count", body: `{"operation":"add","operands":["1"]}`, wantStatus: 422, wantCode: calculator.CodeInvalidOperandCount, wantMessage: "exactly 2"},
		{name: "missing operands", body: `{"operation":"add"}`, wantStatus: 422, wantCode: calculator.CodeInvalidOperandCount},
		{name: "null operands", body: `{"operation":"add","operands":null}`, wantStatus: 422, wantCode: calculator.CodeInvalidOperandCount, wantMessage: "exactly 2"},
		{name: "operand not a number", body: `{"operation":"add","operands":["1","abc"]}`, wantStatus: 400, wantCode: codeInvalidOperand, wantMessage: "string or a number"},
		{name: "operand with surrounding spaces", body: `{"operation":"add","operands":["1"," 5 "]}`, wantStatus: 400, wantCode: codeInvalidOperand},
		{name: "empty operand", body: `{"operation":"add","operands":["1",""]}`, wantStatus: 400, wantCode: codeInvalidOperand},
		{name: "null operand", body: `{"operation":"add","operands":["1",null]}`, wantStatus: 400, wantCode: codeInvalidOperand, wantMessage: "is null"},
		{name: "operand is an object", body: `{"operation":"add","operands":["1",{"a":1}]}`, wantStatus: 400, wantCode: codeInvalidOperand, wantMessage: "string or a number"},
		{name: "operand too long", body: `{"operation":"add","operands":["` + strings.Repeat("1", 101) + `","1"]}`, wantStatus: 422, wantCode: codeOperandTooLong},
		{name: "more operands than any operation takes", body: `{"operation":"add","operands":["1","2","3"]}`, wantStatus: 422, wantCode: calculator.CodeInvalidOperandCount, wantMessage: "at most 2"},
		{name: "malformed json", body: `{"operation":`, wantStatus: 400, wantCode: codeInvalidJSON},
		{name: "empty body", body: ``, wantStatus: 400, wantCode: codeInvalidJSON},
		{name: "truncated body", body: `{"operation":"add","operands":["1","2"`, wantStatus: 400, wantCode: codeInvalidJSON},
		{name: "truncated after the object", body: `{"operation":"add","operands":["1","2"]`, wantStatus: 400, wantCode: codeInvalidJSON},
		{name: "unknown field", body: `{"operation":"add","operands":["1","2"],"operand":["3"]}`, wantStatus: 400, wantCode: codeInvalidJSON},
		{name: "trailing data", body: `{"operation":"add","operands":["1","2"]} {}`, wantStatus: 400, wantCode: codeInvalidJSON},
	}

	handler := New("")
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			recorder := serve(handler, http.MethodPost, "/api/v1/calculate", tc.body, tc.headers)

			if recorder.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d (body %s)", recorder.Code, tc.wantStatus, recorder.Body.String())
			}
			if contentType := recorder.Header().Get("Content-Type"); contentType != "application/json; charset=utf-8" {
				t.Fatalf("Content-Type = %q", contentType)
			}
			if tc.wantCode != "" {
				if got := errorCode(t, recorder); got != tc.wantCode {
					t.Fatalf("code = %q, want %q", got, tc.wantCode)
				}
				if tc.wantMessage != "" && !strings.Contains(errorMessage(t, recorder), tc.wantMessage) {
					t.Fatalf("message = %q, want it to contain %q", errorMessage(t, recorder), tc.wantMessage)
				}
				return
			}
			var body struct {
				Result string `json:"result"`
			}
			if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
				t.Fatalf("success body is not JSON: %v", err)
			}
			if body.Result != tc.wantResult {
				t.Fatalf("result = %q, want %q", body.Result, tc.wantResult)
			}
		})
	}
}

func TestRouting(t *testing.T) {
	handler := New("")

	for _, method := range []string{http.MethodGet, http.MethodPut, http.MethodDelete, http.MethodHead} {
		recorder := serve(handler, method, "/api/v1/calculate", "")
		if recorder.Code != http.StatusMethodNotAllowed {
			t.Fatalf("%s status = %d, want 405", method, recorder.Code)
		}
		if allow := recorder.Header().Get("Allow"); allow != http.MethodPost {
			t.Fatalf("%s Allow = %q, want POST", method, allow)
		}
		if got := errorCode(t, recorder); got != codeMethodNotAllowed {
			t.Fatalf("%s code = %q, want %q", method, got, codeMethodNotAllowed)
		}
	}

	for _, target := range []string{"/api", "/api/v1/nope", "/nope"} {
		recorder := serve(handler, http.MethodPost, target, "{}")
		if recorder.Code != http.StatusNotFound {
			t.Fatalf("%s status = %d, want 404", target, recorder.Code)
		}
		if got := errorCode(t, recorder); got != codeNotFound {
			t.Fatalf("%s code = %q, want %q", target, got, codeNotFound)
		}
	}
}

func TestBodyLimit(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{name: "oversized valid json", body: `{"operation":"add","operands":["` + strings.Repeat("1", 2<<20) + `","2"]}`},
		{name: "oversized garbage", body: strings.Repeat("x", 2<<20)},
		{name: "whitespace just over the limit", body: strings.Repeat(" ", maxBodyBytes+1)},
		{name: "valid json plus trailing spaces over the limit", body: `{"operation":"add","operands":["1","2"]}` + strings.Repeat(" ", maxBodyBytes)},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			recorder := serve(New(""), http.MethodPost, "/api/v1/calculate", tc.body)
			if recorder.Code != http.StatusRequestEntityTooLarge {
				t.Fatalf("status = %d, want 413", recorder.Code)
			}
			if got := errorCode(t, recorder); got != codeRequestTooLarge {
				t.Fatalf("code = %q, want %q", got, codeRequestTooLarge)
			}
		})
	}
}

func TestRecoverPanics(t *testing.T) {
	t.Run("before any write the client gets the envelope", func(t *testing.T) {
		handler := recoverPanics(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
			panic("boom")
		}))
		recorder := serve(handler, http.MethodGet, "/", "")
		if recorder.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want 500", recorder.Code)
		}
		if got := errorCode(t, recorder); got != codeInternal {
			t.Fatalf("code = %q, want %q", got, codeInternal)
		}
	})

	t.Run("after a partial write the connection is aborted, not appended to", func(t *testing.T) {
		handler := recoverPanics(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"result":"hel`))
			panic("boom")
		}))
		defer func() {
			if recovered := recover(); recovered != http.ErrAbortHandler {
				t.Fatalf("want ErrAbortHandler once a response is committed, got %v", recovered)
			}
		}()
		serve(handler, http.MethodGet, "/", "")
	})
}

func TestStaticServing(t *testing.T) {
	parent := t.TempDir()
	webDir := filepath.Join(parent, "web")
	if err := os.MkdirAll(filepath.Join(webDir, "assets"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(webDir, "index.html"), []byte("<html>app</html>"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(webDir, "assets", "app.js"), []byte("console.log(1)"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(parent, "secret.txt"), []byte("SECRET-OUTSIDE-WEB-DIR"), 0o644); err != nil {
		t.Fatal(err)
	}
	handler := New(webDir)

	t.Run("root serves the app shell without caching", func(t *testing.T) {
		recorder := serve(handler, http.MethodGet, "/", "")
		if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), "<html>app</html>") {
			t.Fatalf("status = %d body = %q", recorder.Code, recorder.Body.String())
		}
		if cache := recorder.Header().Get("Cache-Control"); cache != "no-cache" {
			t.Fatalf("Cache-Control = %q, want no-cache", cache)
		}
	})

	t.Run("an asset is served as itself", func(t *testing.T) {
		recorder := serve(handler, http.MethodGet, "/assets/app.js", "")
		if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), "console.log(1)") {
			t.Fatalf("status = %d body = %q", recorder.Code, recorder.Body.String())
		}
	})

	t.Run("a directory is a 404, not a listing or the shell", func(t *testing.T) {
		for _, accept := range []string{"text/html", "*/*"} {
			recorder := serve(handler, http.MethodGet, "/assets/", "", map[string]string{"Accept": accept})
			if recorder.Code != http.StatusNotFound {
				t.Fatalf("Accept %q: status = %d, want 404", accept, recorder.Code)
			}
			if body := recorder.Body.String(); strings.Contains(body, "app.js") || strings.Contains(body, "<html>app</html>") {
				t.Fatalf("Accept %q: a directory request returned a listing or the shell", accept)
			}
		}
	})

	t.Run("a method other than GET or HEAD is refused for static files", func(t *testing.T) {
		recorder := serve(handler, http.MethodPost, "/", "")
		if recorder.Code != http.StatusMethodNotAllowed {
			t.Fatalf("status = %d, want 405", recorder.Code)
		}
		if allow := recorder.Header().Get("Allow"); allow != "GET, HEAD" {
			t.Fatalf("Allow = %q, want GET, HEAD", allow)
		}
	})

	t.Run("an unknown path is the 404 envelope, never the shell", func(t *testing.T) {
		for _, target := range []string{"/some/deep/link", "/user/john.doe", "/api/v1/nope"} {
			recorder := serve(handler, http.MethodGet, target, "", map[string]string{"Accept": "text/html,application/xhtml+xml"})
			if recorder.Code != http.StatusNotFound {
				t.Fatalf("%s: status = %d, want 404", target, recorder.Code)
			}
			if strings.Contains(recorder.Body.String(), "<html>app</html>") {
				t.Fatalf("%s: answered with the app shell", target)
			}
			if got := errorCode(t, recorder); got != codeNotFound {
				t.Fatalf("%s: code = %q", target, got)
			}
		}
	})

	t.Run("a client-shaped path is a 404, not a 500", func(t *testing.T) {
		recorder := serve(handler, http.MethodGet, "/"+strings.Repeat("a", 300), "", map[string]string{"Accept": "*/*"})
		if recorder.Code != http.StatusNotFound {
			t.Fatalf("overlong path: status = %d, want 404", recorder.Code)
		}
		recorder = serve(handler, http.MethodGet, "/index.html/child.js", "", map[string]string{"Accept": "*/*"})
		if recorder.Code != http.StatusNotFound {
			t.Fatalf("path through a file: status = %d, want 404", recorder.Code)
		}
	})

	t.Run("an unreadable index is a 500, not a missing build", func(t *testing.T) {
		index := filepath.Join(webDir, "index.html")
		if err := os.Chmod(index, 0o000); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(index, 0o644) })

		recorder := serve(handler, http.MethodGet, "/", "")
		if recorder.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want 500", recorder.Code)
		}
		if got := errorCode(t, recorder); got != codeInternal {
			t.Fatalf("code = %q, want %q", got, codeInternal)
		}
	})

	t.Run("a missing asset is a 404 envelope", func(t *testing.T) {
		for _, target := range []string{"/assets/missing.js", "/some/deep/link"} {
			recorder := serve(handler, http.MethodGet, target, "", map[string]string{"Accept": "*/*"})
			if recorder.Code != http.StatusNotFound {
				t.Fatalf("%s: status = %d, want 404", target, recorder.Code)
			}
			if strings.Contains(recorder.Body.String(), "<html>app</html>") {
				t.Fatalf("%s: answered with the app shell", target)
			}
			if got := errorCode(t, recorder); got != codeNotFound {
				t.Fatalf("%s: code = %q", target, got)
			}
		}
	})

	t.Run("traversal is not served", func(t *testing.T) {
		for _, target := range []string{"/../secret.txt", "/../../etc/passwd"} {
			if body := serve(handler, http.MethodGet, target, "").Body.String(); strings.Contains(body, "SECRET-OUTSIDE-WEB-DIR") {
				t.Fatalf("%s: a traversal request returned a file from outside the web directory", target)
			}
			if body := serve(staticHandler(webDir), http.MethodGet, target, "").Body.String(); strings.Contains(body, "SECRET-OUTSIDE-WEB-DIR") {
				t.Fatalf("%s: the file layer returned a file from outside the web directory", target)
			}
		}
	})

	t.Run("a file that cannot be read is a 500, not a 404", func(t *testing.T) {
		locked := filepath.Join(webDir, "locked")
		if err := os.Mkdir(locked, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(locked, "app.js"), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(locked, 0o000); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(locked, 0o755) })

		recorder := serve(handler, http.MethodGet, "/locked/app.js", "", map[string]string{"Accept": "*/*"})
		if recorder.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want 500", recorder.Code)
		}
		if got := errorCode(t, recorder); got != codeInternal {
			t.Fatalf("code = %q, want %q", got, codeInternal)
		}
	})

	t.Run("a file that cannot be read is a 500 envelope, not the file server's 403 text", func(t *testing.T) {
		path := filepath.Join(webDir, "locked.txt")
		if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(path, 0o000); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(path, 0o644) })

		recorder := serve(handler, http.MethodGet, "/locked.txt", "", map[string]string{"Accept": "*/*"})
		if recorder.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want 500", recorder.Code)
		}
		if got := errorCode(t, recorder); got != codeInternal {
			t.Fatalf("code = %q, want %q", got, codeInternal)
		}
	})

	t.Run("a web dir without an index reports 404 rather than an empty page", func(t *testing.T) {
		for _, target := range []string{"/", "/deep/link"} {
			recorder := serve(New(t.TempDir()), http.MethodGet, target, "", map[string]string{"Accept": "text/html"})
			if recorder.Code != http.StatusNotFound {
				t.Fatalf("%s: status = %d, want 404", target, recorder.Code)
			}
			if got := errorCode(t, recorder); got != codeNotFound {
				t.Fatalf("%s: code = %q, want %q", target, got, codeNotFound)
			}
		}
	})
}

func TestHealth(t *testing.T) {
	handler := New("")

	recorder := serve(handler, http.MethodGet, "/health", "")
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", recorder.Code)
	}
	if body := recorder.Body.String(); !strings.Contains(body, `"status":"ok"`) {
		t.Fatalf("body = %q, want a status of ok", body)
	}

	recorder = serve(handler, http.MethodPost, "/health", "")
	if recorder.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST status = %d, want 405", recorder.Code)
	}
	if allow := recorder.Header().Get("Allow"); allow != http.MethodGet {
		t.Fatalf("Allow = %q, want GET", allow)
	}
}

func TestRequestLogging(t *testing.T) {
	var logged bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logged, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })

	handler := logRequests(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	}))
	serve(handler, http.MethodGet, "/anything", "")

	if line := logged.String(); !strings.Contains(line, "status=418") || !strings.Contains(line, "path=/anything") {
		t.Fatalf("log line = %q, want the status and path", line)
	}
}

func TestRecoveredPanicIsLoggedWithTheStatusTheClientGot(t *testing.T) {
	var logged bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logged, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })

	handler := logRequests(recoverPanics(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("boom")
	})))
	recorder := serve(handler, http.MethodGet, "/panic", "")
	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", recorder.Code)
	}
	if line := logged.String(); !strings.Contains(line, "status=500") {
		t.Fatalf("log line = %q, want the 500 the client received", line)
	}
}

func TestErrorEnvelopeEdges(t *testing.T) {
	t.Run("an unmapped code cannot reach the client as a zero status", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		writeCode(recorder, "not_a_real_code", "ignored")
		if recorder.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want 500", recorder.Code)
		}
		if got := errorCode(t, recorder); got != codeInternal {
			t.Fatalf("code = %q, want %q", got, codeInternal)
		}
	})

	t.Run("a request error reports its message", func(t *testing.T) {
		err := &requestError{code: codeInvalidOperand, message: "operand broken"}
		if err.Error() != "operand broken" {
			t.Fatalf("Error() = %q, want the message", err.Error())
		}
	})
}

type failingReader struct{}

func (failingReader) Read([]byte) (int, error) { return 0, errors.New("read failed") }

func TestRequestReadingEdges(t *testing.T) {
	t.Run("a body that cannot be read is invalid_json, not a panic", func(t *testing.T) {
		if _, err := decodeRequest(failingReader{}); err == nil {
			t.Fatal("want an error from an unreadable body")
		}
	})

	t.Run("an unexpected error type still answers with the envelope", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		writeRefusal(recorder, httptest.NewRequest(http.MethodPost, "/api/v1/calculate", nil), errors.New("boom"))
		if recorder.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want 500", recorder.Code)
		}
		if got := errorCode(t, recorder); got != codeInternal {
			t.Fatalf("code = %q, want %q", got, codeInternal)
		}
	})

	t.Run("a response that cannot be encoded is a 500, not a half body", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		writeJSON(recorder, http.StatusOK, make(chan int))
		if recorder.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want 500", recorder.Code)
		}
		if got := errorCode(t, recorder); got != codeInternal {
			t.Fatalf("code = %q, want %q", got, codeInternal)
		}
	})
}

func TestResponseRecorderAndAbort(t *testing.T) {
	t.Run("a second WriteHeader is ignored", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		wrapped := &responseRecorder{ResponseWriter: recorder}
		wrapped.WriteHeader(http.StatusOK)
		wrapped.WriteHeader(http.StatusTeapot)
		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d, want the first one (200)", recorder.Code)
		}
	})

	t.Run("an intentional abort is re-panicked, not converted", func(t *testing.T) {
		handler := recoverPanics(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
			panic(http.ErrAbortHandler)
		}))
		defer func() {
			if recovered := recover(); recovered != http.ErrAbortHandler {
				t.Fatalf("want ErrAbortHandler to propagate, got %v", recovered)
			}
		}()
		serve(handler, http.MethodGet, "/", "")
	})
}

func TestTransportCodeLiterals(t *testing.T) {
	literals := map[string]string{
		codeInvalidJSON:      "invalid_json",
		codeInvalidOperand:   "invalid_operand",
		codeOperandTooLong:   "operand_too_long",
		codeNotFound:         "not_found",
		codeMethodNotAllowed: "method_not_allowed",
		codeRequestTooLarge:  "request_too_large",
		codeInternal:         "internal_error",
	}
	for got, want := range literals {
		if got != want {
			t.Fatalf("wire code %q, want %q", got, want)
		}
	}
}

func TestDecodingEdges(t *testing.T) {
	cases := []struct {
		name       string
		body       string
		wantStatus int
		wantCode   string
		wantResult string
	}{
		{
			name:       "operands before operation",
			body:       `{"operands":["2","3"],"operation":"add"}`,
			wantStatus: 200,
			wantResult: "5",
		},
		{
			name:       "three operands are refused without being parsed",
			body:       `{"operation":"add","operands":["1","2","3"]}`,
			wantStatus: 422,
			wantCode:   calculator.CodeInvalidOperandCount,
		},
		{
			name:       "operands that are not an array",
			body:       `{"operation":"add","operands":"1"}`,
			wantStatus: 400,
			wantCode:   codeInvalidJSON,
		},
		{
			name:       "a body that is not an object",
			body:       `["add","1","2"]`,
			wantStatus: 400,
			wantCode:   codeInvalidJSON,
		},
		{
			name:       "an unknown field",
			body:       `{"operation":"add","operands":["1","2"],"extra":true}`,
			wantStatus: 400,
			wantCode:   codeInvalidJSON,
		},
		{
			name:       "an operation that is not a string",
			body:       `{"operation":5,"operands":["1","2"]}`,
			wantStatus: 400,
			wantCode:   codeInvalidJSON,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			recorder := serve(New(""), http.MethodPost, "/api/v1/calculate", tc.body)
			if recorder.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d (%s)", recorder.Code, tc.wantStatus, recorder.Body.String())
			}
			if tc.wantCode != "" {
				if got := errorCode(t, recorder); got != tc.wantCode {
					t.Fatalf("code = %q, want %q", got, tc.wantCode)
				}
				return
			}
			if body := recorder.Body.String(); !strings.Contains(body, tc.wantResult) {
				t.Fatalf("body = %q, want %q", body, tc.wantResult)
			}
		})
	}
}

func TestRefusedOperandMessageDoesNotEchoTheOperand(t *testing.T) {
	long := strings.Repeat("x", 60)
	recorder := serve(New(""), http.MethodPost, "/api/v1/calculate", `{"operation":"add","operands":["1","`+long+`"]}`)

	message := errorMessage(t, recorder)
	if len(message) > 120 {
		t.Fatalf("message is %d characters, want a fixed message", len(message))
	}
	if strings.Contains(message, long) {
		t.Fatalf("message = %q, want the operand left out", message)
	}
}

func TestOperandCountBeyondArityIsRefused(t *testing.T) {
	operands := make([]string, 500)
	for i := range operands {
		operands[i] = `"1"`
	}
	body := `{"operation":"add","operands":[` + strings.Join(operands, ",") + `]}`

	recorder := serve(New(""), http.MethodPost, "/api/v1/calculate", body)
	if recorder.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422", recorder.Code)
	}
	if got := errorCode(t, recorder); got != calculator.CodeInvalidOperandCount {
		t.Fatalf("code = %q, want %q", got, calculator.CodeInvalidOperandCount)
	}
}

func TestBodyMustBeAJSONObject(t *testing.T) {
	for _, body := range []string{`null`, `["add","1","2"]`, `"add"`} {
		recorder := serve(New(""), http.MethodPost, "/api/v1/calculate", body)
		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("%s: status = %d, want 400", body, recorder.Code)
		}
		if got := errorCode(t, recorder); got != codeInvalidJSON {
			t.Fatalf("%s: code = %q, want %q", body, got, codeInvalidJSON)
		}
	}
}

func TestJSONNumbersKeepTheirExactDigits(t *testing.T) {
	// 2^53 + 1 is the first integer a float64 cannot represent.
	recorder := serve(New(""), http.MethodPost, "/api/v1/calculate", `{"operation":"add","operands":[9007199254740993,1]}`)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (%s)", recorder.Code, recorder.Body.String())
	}
	if body := recorder.Body.String(); !strings.Contains(body, "9007199254740994") {
		t.Fatalf("body = %q, want the exact sum", body)
	}
}

func TestEveryDomainCodeHasAStatus(t *testing.T) {
	domainCodes := []string{
		calculator.CodeUnknownOperation,
		calculator.CodeInvalidOperandCount,
		calculator.CodeDivisionByZero,
		calculator.CodeNegativeSqrt,
		calculator.CodeUndefinedPower,
		calculator.CodeResultTooLarge,
	}
	for _, code := range domainCodes {
		if _, ok := statusByCode[code]; !ok {
			t.Fatalf("domain code %q has no mapped status", code)
		}
	}
}
