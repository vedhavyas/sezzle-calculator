package httpapi

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/vedhavyas/sezzle-calculator/backend/internal/calculator"
)

func calculate(w http.ResponseWriter, r *http.Request) {
	req, err := decodeRequest(r.Body)
	if err != nil {
		writeRefusal(w, r, err)
		return
	}
	result, err := calculator.Calculate(req.Operation, req.decimals())
	if err != nil {
		writeRefusal(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, resultBody{Result: result.String()})
}

func writeRefusal(w http.ResponseWriter, r *http.Request, err error) {
	var requestErr *requestError
	if errors.As(err, &requestErr) {
		if requestErr.detail != nil {
			slog.Warn("request refused", "method", r.Method, "path", r.URL.Path, "code", requestErr.code, "detail", requestErr.detail)
		}
		writeCode(w, requestErr.code, requestErr.message)
		return
	}

	var calcErr *calculator.Error
	if errors.As(err, &calcErr) {
		writeCode(w, calcErr.Code, calcErr.Message)
		return
	}

	slog.Error("unexpected error while handling a request", "method", r.Method, "path", r.URL.Path, "error", err)
	writeCode(w, codeInternal, "the server failed to process the request")
}
