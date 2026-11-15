package calculator

import (
	"fmt"
	"sort"
	"strings"

	"github.com/shopspring/decimal"
)

// Operation names a calculation the service supports.
type Operation string

const (
	Add      Operation = "add"
	Subtract Operation = "subtract"
	Multiply Operation = "multiply"
	Divide   Operation = "divide"
	Power    Operation = "power"
	Sqrt     Operation = "sqrt"
	Percent  Operation = "percent"
)

// MaxOperands is the largest operand count any operation takes. The transport
// caps a request's operand list at this while decoding, so a body naming more
// never allocates for them.
const MaxOperands = 2

type spec struct {
	arity int
	apply func([]decimal.Decimal) (decimal.Decimal, error)
}

var specs = map[Operation]spec{
	Add:      {arity: 2, apply: add},
	Subtract: {arity: 2, apply: subtract},
	Multiply: {arity: 2, apply: multiply},
	Divide:   {arity: 2, apply: divide},
	Power:    {arity: 2, apply: power},
	Sqrt:     {arity: 1, apply: sqrt},
	Percent:  {arity: 2, apply: percent},
}

// operandCount builds the refusal for the wrong number of operands.
func operandCount(op Operation, arity, got int) error {
	return &Error{
		Code:    CodeInvalidOperandCount,
		Message: fmt.Sprintf("%s takes exactly %d operand(s), got %d", op, arity, got),
	}
}

// Names lists the supported operations in alphabetical order.
func Names() []string {
	names := make([]string, 0, len(specs))
	for name := range specs {
		names = append(names, string(name))
	}
	sort.Strings(names)
	return names
}

func unknownOperation(op Operation) error {
	if op == "" {
		return &Error{
			Code:    CodeUnknownOperation,
			Message: fmt.Sprintf("operation is required; supported: %s", strings.Join(Names(), ", ")),
		}
	}
	return &Error{
		Code:    CodeUnknownOperation,
		Message: fmt.Sprintf("unknown operation %q; supported: %s", briefText(string(op)), strings.Join(Names(), ", ")),
	}
}

// briefText keeps a refusal from echoing an unbounded input in full.
func briefText(text string) string {
	if len(text) <= 40 {
		return text
	}
	return text[:40] + "..."
}
