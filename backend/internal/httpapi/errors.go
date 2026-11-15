package httpapi

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/vedhavyas/sezzle-calculator/backend/internal/calculator"
)

// Codes raised while reading a request, as opposed to the calculator's own refusals.
const (
	codeInvalidJSON      = "invalid_json"
	codeInvalidOperand   = "invalid_operand"
	codeOperandTooLong   = "operand_too_long"
	codeNotFound         = "not_found"
	codeMethodNotAllowed = "method_not_allowed"
	codeRequestTooLarge  = "request_too_large"
	codeInternal         = "internal_error"
)

var statusByCode = map[string]int{
	codeInvalidJSON:                    http.StatusBadRequest,
	codeInvalidOperand:                 http.StatusBadRequest,
	codeOperandTooLong:                 http.StatusUnprocessableEntity,
	calculator.CodeUnknownOperation:    http.StatusUnprocessableEntity,
	calculator.CodeInvalidOperandCount: http.StatusUnprocessableEntity,
	calculator.CodeDivisionByZero:      http.StatusUnprocessableEntity,
	calculator.CodeNegativeSqrt:        http.StatusUnprocessableEntity,
	calculator.CodeUndefinedPower:      http.StatusUnprocessableEntity,
	calculator.CodeResultTooLarge:      http.StatusUnprocessableEntity,
	codeNotFound:                       http.StatusNotFound,
	codeMethodNotAllowed:               http.StatusMethodNotAllowed,
	codeRequestTooLarge:                http.StatusRequestEntityTooLarge,
	codeInternal:                       http.StatusInternalServerError,
}

type errorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type errorBody struct {
	Error errorDetail `json:"error"`
}

// requestError is a refusal raised while reading a request. detail carries the
// underlying cause for the log without leaking library or reflection text to the client.
type requestError struct {
	code    string
	message string
	detail  error
}

func (e *requestError) Error() string { return e.message }

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, errorBody{Error: errorDetail{Code: code, Message: message}})
}

// writeJSON encodes before writing, so a response is never a status with a half-written body.
func writeJSON(w http.ResponseWriter, status int, body any) {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(body); err != nil {
		slog.Error("could not encode a response", "error", err)
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":{"code":"internal_error","message":"the server failed to process the request"}}`))
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write(buf.Bytes())
}

// writeCode resolves the status for a code so callers never pick one by hand.
func writeCode(w http.ResponseWriter, code, message string) {
	status, ok := statusByCode[code]
	if !ok {
		slog.Error("no status mapped for error code", "code", code)
		writeError(w, http.StatusInternalServerError, codeInternal, "the server failed to process the request")
		return
	}
	writeError(w, status, code, message)
}
