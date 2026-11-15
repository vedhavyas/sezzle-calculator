package calculator

// Error codes returned by Calculate. The transport layer maps each to a status.
const (
	CodeUnknownOperation    = "unknown_operation"
	CodeInvalidOperandCount = "invalid_operand_count"
	CodeDivisionByZero      = "division_by_zero"
	CodeNegativeSqrt        = "negative_sqrt"
	CodeUndefinedPower      = "undefined_power"
	CodeResultTooLarge      = "result_too_large"
)

// Error is a refusal by the calculator, carrying a stable code for callers.
type Error struct {
	Code    string
	Message string
}

func (e *Error) Error() string { return e.Message }

func divisionByZero() error {
	return &Error{Code: CodeDivisionByZero, Message: "division by zero is undefined"}
}
