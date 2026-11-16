package httpapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"unicode/utf8"

	"github.com/shopspring/decimal"

	"github.com/vedhavyas/sezzle-calculator/backend/internal/calculator"
)

const maxOperandLength = 100

const requestShape = `request body must be a JSON object with "operation" and "operands"`

type calculateRequest struct {
	Operation calculator.Operation `json:"operation"`
	Operands  operands             `json:"operands"`
}

// operands is the request's operand list. Decoding stops at the first operand
// past calculator.MaxOperands, so a body naming hundreds of thousands of them
// never allocates for them.
type operands []operand

var (
	errOperandsShape   = errors.New(`"operands" must be an array of strings or numbers`)
	errTooManyOperands = errors.New("too many operands")
)

func (o *operands) UnmarshalJSON(data []byte) error {
	if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		*o = nil
		return nil
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	if open, err := decoder.Token(); err != nil || open != json.Delim('[') {
		return errOperandsShape
	}
	var values operands
	for decoder.More() {
		if len(values) == calculator.MaxOperands {
			return errTooManyOperands
		}
		var item operand
		if err := decoder.Decode(&item); err != nil {
			return err
		}
		values = append(values, item)
	}
	*o = values
	return nil
}

// operand is one request operand. It exists so the two refusals the API
// promises are checked before decimal.Decimal decodes the value: decimal reads
// null as zero and has no length limit.
type operand decimal.Decimal

func (o *operand) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		return &requestError{code: codeInvalidOperand, message: "an operand is null; it must be a string or a number"}
	}
	// A quoted operand carries two extra bytes, so trim the quotes before
	// measuring the text.
	if utf8.RuneCount(bytes.Trim(data, `"`)) > maxOperandLength {
		return &requestError{
			code:    codeOperandTooLong,
			message: fmt.Sprintf("an operand is longer than %d characters", maxOperandLength),
		}
	}
	if err := (*decimal.Decimal)(o).UnmarshalJSON(data); err != nil {
		return &requestError{code: codeInvalidOperand, message: "an operand must be a string or a number this service can read", detail: err}
	}
	return nil
}

// decimals converts the wire operands into the values the calculator takes.
func (r calculateRequest) decimals() []decimal.Decimal {
	operands := make([]decimal.Decimal, len(r.Operands))
	for index := range r.Operands {
		operands[index] = decimal.Decimal(r.Operands[index])
	}
	return operands
}

type resultBody struct {
	Result string `json:"result"`
}

// decodeRequest reads the body through the size limiter before parsing, so an
// oversized body is a 413 even when its bytes are not valid JSON.
func decodeRequest(body io.Reader) (calculateRequest, error) {
	payload, err := io.ReadAll(body)
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			return calculateRequest{}, &requestError{code: codeRequestTooLarge, message: "request body is larger than 1 MiB"}
		}
		return calculateRequest{}, &requestError{code: codeInvalidJSON, message: "request body could not be read", detail: err}
	}

	if err := rejectDuplicateFields(payload); err != nil {
		return calculateRequest{}, &requestError{code: codeInvalidJSON, message: "request body must not repeat a field", detail: err}
	}

	var req *calculateRequest
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		return calculateRequest{}, decodeFailure(err, requestShape)
	}
	if req == nil {
		return calculateRequest{}, &requestError{code: codeInvalidJSON, message: requestShape}
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return calculateRequest{}, decodeFailure(err, "request body must hold exactly one JSON object")
	}
	return *req, nil
}

// rejectDuplicateFields refuses a body that repeats a top-level field; the
// standard decoder silently keeps the last value.
func rejectDuplicateFields(payload []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(payload))
	if open, err := decoder.Token(); err != nil || open != json.Delim('{') {
		return nil
	}
	seen := make(map[string]bool)
	for decoder.More() {
		keyToken, err := decoder.Token()
		if err != nil {
			return nil
		}
		key, ok := keyToken.(string)
		if !ok {
			return nil
		}
		if seen[key] {
			return fmt.Errorf("field %q is repeated", key)
		}
		seen[key] = true
		var value json.RawMessage
		if err := decoder.Decode(&value); err != nil {
			return nil
		}
	}
	return nil
}

// decodeFailure turns a decoding error into the refusal the client sees, keeping
// the library's own text in the log rather than the response.
func decodeFailure(err error, message string) error {
	var requestErr *requestError
	if errors.As(err, &requestErr) {
		return requestErr
	}
	switch {
	case errors.Is(err, errTooManyOperands):
		return &requestError{
			code:    calculator.CodeInvalidOperandCount,
			message: fmt.Sprintf("an operation takes at most %d operands", calculator.MaxOperands),
		}
	case errors.Is(err, errOperandsShape):
		return &requestError{code: codeInvalidJSON, message: errOperandsShape.Error()}
	case errors.Is(err, io.EOF):
		return &requestError{code: codeInvalidJSON, message: "request body is empty"}
	}
	return &requestError{code: codeInvalidJSON, message: message, detail: err}
}
