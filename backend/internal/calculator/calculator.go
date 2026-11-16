package calculator

import (
	"fmt"
	"math"
	"strconv"

	"github.com/shopspring/decimal"
)

// significantDigits is how many digits a rounded result keeps. Division is exact
// when its quotient terminates within exactPlaces; anything else, including a
// quotient that terminates beyond that window, keeps this many significant
// digits.
const significantDigits = 16

// maxResultDigits bounds a power result before it is computed. Without it,
// 1e1000^99.5 occupies a core for twenty seconds and 1e10000^1000 returns ten
// megabytes. Every other operation is bounded by its operands: the transport
// caps operand text at 100 characters and the decimal library refuses exponents
// past 10,000 at parse, so no result outside power exceeds about 20,000
// characters.
const maxResultDigits = 10_000

var (
	one = decimal.NewFromInt(1)
	two = decimal.NewFromInt(2)
)

// Calculate applies op to operands and returns the result in canonical decimal
// form. The operation and count are checked before the operation runs, so each
// operation can index the operands its arity promises.
func Calculate(op Operation, operands []decimal.Decimal) (decimal.Decimal, error) {
	s, ok := specs[op]
	if !ok {
		return decimal.Zero, unknownOperation(op)
	}
	if len(operands) != s.arity {
		return decimal.Zero, operandCount(op, s.arity, len(operands))
	}
	return s.apply(operands)
}

func add(operands []decimal.Decimal) (decimal.Decimal, error) {
	return operands[0].Add(operands[1]), nil
}

func subtract(operands []decimal.Decimal) (decimal.Decimal, error) {
	return operands[0].Sub(operands[1]), nil
}

func multiply(operands []decimal.Decimal) (decimal.Decimal, error) {
	return operands[0].Mul(operands[1]), nil
}

// exactPlaces is deep enough that any quotient which terminates in a practical
// sense is returned exactly; a genuinely repeating one falls back to
// significantDigits.
const exactPlaces int32 = 100

func divide(operands []decimal.Decimal) (decimal.Decimal, error) {
	dividend, divisor := operands[0], operands[1]
	if divisor.IsZero() {
		return decimal.Zero, divisionByZero()
	}
	exact := dividend.DivRound(divisor, exactPlaces)
	if exact.Mul(divisor).Equal(dividend) {
		return exact, nil
	}
	places := placesFor(decimalOrder(dividend) - decimalOrder(divisor))
	return dividend.DivRound(divisor, places), nil
}

// power checks the domain itself: decimal.Pow returns no error and answers 0 for
// 0^0, 0^-1 and a negative base with a fractional exponent.
func power(operands []decimal.Decimal) (decimal.Decimal, error) {
	base, exponent := operands[0], operands[1]
	exponentFloat, _ := exponent.Float64()

	switch {
	case base.IsZero() && exponent.IsZero():
		return one, nil
	case base.IsZero() && exponent.IsNegative():
		return decimal.Zero, divisionByZero()
	case base.IsNegative() && !exponent.IsInteger():
		return decimal.Zero, &Error{
			Code:    CodeUndefinedPower,
			Message: fmt.Sprintf("a negative base with the fractional exponent %s is not supported", briefText(exponent.String())),
		}
	}
	// A base of ±1 never grows, whatever the exponent. Answering it here also
	// stops an unnormalised coefficient such as "1.0" or "10e-1" from being
	// expanded by PowWithPrecision, which for a huge exponent would allocate a
	// big.Int of the exponent's size before rounding it back to 1.
	if base.Abs().Equal(one) {
		if base.IsNegative() && !exponent.Mod(two).IsZero() {
			return one.Neg(), nil
		}
		return one, nil
	}
	switch {
	case resultWouldBeTooLarge(base, exponent, exponentFloat):
		return decimal.Zero, &Error{
			Code:    CodeResultTooLarge,
			Message: fmt.Sprintf("the result would exceed %d characters", maxResultDigits),
		}
	}
	// PowWithPrecision rather than Pow: the latter reads package globals and returns
	// a zero on a precision error, which would reach the client as a real result.
	// PowWithPrecision takes decimal places, so the places are derived from the
	// result's order to keep significantDigits significant ones, as division and
	// square root do.
	result, err := base.PowWithPrecision(exponent, placesFor(decimalOrder(base)*exponentFloat))
	if err != nil {
		return decimal.Zero, &Error{
			Code:    CodeResultTooLarge,
			Message: fmt.Sprintf("the result of that power cannot be represented within %d digits", maxResultDigits),
		}
	}
	return result, nil
}

func sqrt(operands []decimal.Decimal) (decimal.Decimal, error) {
	value := operands[0]
	// SqrtRound rather than Sqrt: the latter reads decimal.DivisionPrecision, a
	// package global, so this package's precision would not govern it.
	root, err := value.SqrtRound(placesFor(decimalOrder(value) / 2))
	if err != nil {
		return decimal.Zero, &Error{
			Code:    CodeNegativeSqrt,
			Message: fmt.Sprintf("square root of the negative number %s is undefined", briefText(value.String())),
		}
	}
	return root, nil
}

func percent(operands []decimal.Decimal) (decimal.Decimal, error) {
	// Dividing by 100 only moves the decimal point, so this is exact.
	return operands[0].Mul(operands[1]).Shift(-2), nil
}

// decimalOrder approximates log10(|value|) from its leading digits, so a result's
// size can be judged before computing it.
func decimalOrder(value decimal.Decimal) float64 {
	if value.IsZero() {
		return 0
	}
	digits := value.Abs().Coefficient().String()
	leading := digits
	if len(leading) > 15 {
		leading = leading[:15]
	}
	mantissa, err := strconv.ParseFloat(leading, 64)
	if err != nil {
		return 0
	}
	return math.Log10(mantissa) + float64(len(digits)-len(leading)) + float64(value.Exponent())
}

// placesFor returns the decimal places that keep significantDigits significant
// digits for a result of the given order.
func placesFor(order float64) int32 {
	if math.IsInf(order, 0) || math.IsNaN(order) {
		return 0
	}
	places := significantDigits - int(math.Floor(order)) - 1
	if places < 0 {
		return 0
	}
	if int32(places) > maxPlaces {
		return maxPlaces
	}
	return int32(places)
}

// maxPlaces bounds the precision requested from the decimal library so an
// extreme order cannot make the places grow without limit.
const maxPlaces int32 = 100_000

func resultWouldBeTooLarge(base, exponent decimal.Decimal, exponentFloat float64) bool {
	if base.IsZero() {
		return false
	}
	baseOrder := decimalOrder(base)
	order := math.Abs(baseOrder)
	scale := fractionalDigits(base)
	// A value of exactly ±1 has neither an order nor a scale and never grows.
	if order == 0 && scale == 0 {
		return false
	}
	if math.IsInf(exponentFloat, 0) || math.IsNaN(exponentFloat) {
		return true
	}
	magnitude := order * math.Abs(exponentFloat)
	// The magnitude bounds every exponent: 1e1000^99.5 would be 99,500 digits.
	if magnitude > float64(maxResultDigits) {
		return true
	}
	// A fractional power is rounded and a negative exponent is a reciprocal, so
	// neither expands the base's fractional part.
	if !exponent.IsInteger() || exponent.IsNegative() {
		return false
	}
	// A positive integer exponent is computed exactly before it is returned, so
	// the exact decimal's length is what has to fit. A base below one has only
	// its fractional part, plus the leading "0."; a base at least one adds the
	// integer digits.
	n := math.Abs(exponentFloat)
	var exactDigits float64
	if baseOrder < 0 {
		exactDigits = scale*n + 2
	} else {
		exactDigits = math.Floor(magnitude) + 1 + scale*n
	}
	return exactDigits > float64(maxResultDigits)
}

// fractionalDigits counts the digits after the point that carry information, so
// 1.000… with any number of trailing zeros counts as zero.
func fractionalDigits(value decimal.Decimal) float64 {
	if value.Exponent() >= 0 {
		return 0
	}
	scale := -int(value.Exponent())
	digits := value.Abs().Coefficient().String()
	for scale > 0 && len(digits) > 0 && digits[len(digits)-1] == '0' {
		digits = digits[:len(digits)-1]
		scale--
	}
	if scale < 0 {
		return 0
	}
	return float64(scale)
}
