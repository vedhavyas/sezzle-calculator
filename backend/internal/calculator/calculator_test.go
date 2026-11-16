package calculator

import (
	"errors"
	"math"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/shopspring/decimal"
)

// operands parses test values into the decimals Calculate takes.
func operands(values ...string) []decimal.Decimal {
	operands := make([]decimal.Decimal, len(values))
	for index, value := range values {
		parsed, err := decimal.NewFromString(value)
		if err != nil {
			panic("invalid operand in test: " + value)
		}
		operands[index] = parsed
	}
	return operands
}

func TestCalculate(t *testing.T) {
	cases := []struct {
		op       Operation
		operands []string
		want     string
		wantCode string
	}{
		{op: Add, operands: []string{"1", "2"}, want: "3"},
		{op: Add, operands: []string{"0.1", "0.2"}, want: "0.3"},
		{op: Add, operands: []string{"-1", "1"}, want: "0"},
		{op: Add, operands: []string{"12345678901234567890123456789012345678901234567890", "1"}, want: "12345678901234567890123456789012345678901234567891"},
		{op: Subtract, operands: []string{"1", "2"}, want: "-1"},
		{op: Subtract, operands: []string{"0.3", "0.1"}, want: "0.2"},
		{op: Multiply, operands: []string{"0.1", "0.2"}, want: "0.02"},
		{op: Multiply, operands: []string{"2", "3"}, want: "6"},
		{op: Multiply, operands: []string{"-2", "3"}, want: "-6"},
		{op: Multiply, operands: []string{"1e5", "1e5"}, want: "10000000000"},
		{op: Divide, operands: []string{"10", "4"}, want: "2.5"},
		{op: Divide, operands: []string{"1", "8"}, want: "0.125"},
		{op: Divide, operands: []string{"1", "3"}, want: "0.3333333333333333"},
		{op: Divide, operands: []string{"2", "3"}, want: "0.6666666666666667"},
		{op: Divide, operands: []string{"10", "0"}, wantCode: CodeDivisionByZero},
		{op: Divide, operands: []string{"0", "0"}, wantCode: CodeDivisionByZero},
		{op: Divide, operands: []string{"5", "100000000000000000"}, want: "0.00000000000000005"},
		{op: Divide, operands: []string{"-2", "3"}, want: "-0.6666666666666667"},
		{op: Divide, operands: []string{"-5", "100000000000000000"}, want: "-0.00000000000000005"},
		{op: Divide, operands: []string{"1", "12345678901234567890"}, want: "0.00000000000000000008100000072900001"},
		{op: Power, operands: []string{"2", "10"}, want: "1024"},
		{op: Power, operands: []string{"2", "-1"}, want: "0.5"},
		{op: Power, operands: []string{"0", "0"}, want: "1"},
		{op: Power, operands: []string{"0", "-1"}, wantCode: CodeDivisionByZero},
		{op: Power, operands: []string{"0", "-1001"}, wantCode: CodeDivisionByZero},
		{op: Power, operands: []string{"-8", "1001.5"}, wantCode: CodeUndefinedPower},
		{op: Power, operands: []string{"-2", "3"}, want: "-8"},
		{op: Power, operands: []string{"-2", "2"}, want: "4"},
		{op: Power, operands: []string{"-2", "-3"}, want: "-0.125"},
		{op: Power, operands: []string{"-8", "0.5"}, wantCode: CodeUndefinedPower},
		{op: Power, operands: []string{"-8", "0.3333333333333333"}, wantCode: CodeUndefinedPower},
		{op: Power, operands: []string{"2", "0.5"}, want: "1.414213562373095"},
		{op: Power, operands: []string{"200", "0.5"}, want: "14.14213562373095"},
		{op: Power, operands: []string{"1e-33", "0.5"}, want: "0.00000000000000003162277660168379"},
		{op: Power, operands: []string{"300", "-2"}, want: "0.00001111111111111111"},
		{op: Power, operands: []string{"1e1000", "99.5"}, wantCode: CodeResultTooLarge},
		{op: Power, operands: []string{"1e10000", "1000"}, wantCode: CodeResultTooLarge},
		{op: Power, operands: []string{"0.5", "100000"}, wantCode: CodeResultTooLarge},
		{op: Power, operands: []string{"2", "100000"}, wantCode: CodeResultTooLarge},
		{op: Power, operands: []string{"2", "1e10000"}, wantCode: CodeResultTooLarge},
		{op: Power, operands: []string{"1." + strings.Repeat("9", 98), "1000"}, wantCode: CodeResultTooLarge},
		{op: Power, operands: []string{"-1e10000", "0.5"}, wantCode: CodeUndefinedPower},
		{op: Power, operands: []string{"1", "100000"}, want: "1"},
		{op: Power, operands: []string{"1", "1e400"}, want: "1"},
		{op: Power, operands: []string{"1.0", "1e400"}, want: "1"},
		{op: Power, operands: []string{"10e-1", "99"}, want: "1"},
		{op: Power, operands: []string{"-1.0", "3"}, want: "-1"},
		{op: Power, operands: []string{"-1.0", "2"}, want: "1"},
		{op: Power, operands: []string{"-1.0", "-3"}, want: "-1"},
		{op: Power, operands: []string{"-1", "2e10000"}, want: "1"},
		{op: Power, operands: []string{"1." + strings.Repeat("0", 98), "1000"}, want: "1"},
		{op: Power, operands: []string{"-1", "1001"}, want: "-1"},
		{op: Power, operands: []string{"0", "1000.5"}, want: "0"},
		{op: Sqrt, operands: []string{"16"}, want: "4"},
		{op: Sqrt, operands: []string{"2"}, want: "1.414213562373095"},
		{op: Sqrt, operands: []string{"0.25"}, want: "0.5"},
		{op: Sqrt, operands: []string{"0"}, want: "0"},
		{op: Sqrt, operands: []string{"1e-100"}, want: "0.00000000000000000000000000000000000000000000000001"},
		{op: Sqrt, operands: []string{"1e-33"}, want: "0.00000000000000003162277660168379"},
		{op: Sqrt, operands: []string{"-4"}, wantCode: CodeNegativeSqrt},
		{op: Percent, operands: []string{"15", "200"}, want: "30"},
		{op: Percent, operands: []string{"-15", "200"}, want: "-30"},
		{op: Percent, operands: []string{"1", "3"}, want: "0.03"},
		{op: Percent, operands: []string{"0.5", "100"}, want: "0.5"},
		{op: Percent, operands: []string{"33.33", "100"}, want: "33.33"},
		{op: Operation("sqrtt"), operands: []string{"1"}, wantCode: CodeUnknownOperation},
		{op: Operation("sqrtt"), operands: []string{"1", "2"}, wantCode: CodeUnknownOperation},
		{op: Operation(""), operands: []string{"1"}, wantCode: CodeUnknownOperation},
		{op: Add, operands: []string{"1"}, wantCode: CodeInvalidOperandCount},
		{op: Sqrt, operands: []string{"1", "2"}, wantCode: CodeInvalidOperandCount},
	}

	for _, tc := range cases {
		name := string(tc.op) + "(" + strings.Join(tc.operands, ",") + ")"
		t.Run(name, func(t *testing.T) {
			got, err := Calculate(tc.op, operands(tc.operands...))

			if tc.wantCode != "" {
				var calcErr *Error
				if !errors.As(err, &calcErr) {
					t.Fatalf("%s(%v): want error with code %q, got %v", tc.op, tc.operands, tc.wantCode, err)
				}
				if calcErr.Code != tc.wantCode {
					t.Fatalf("%s(%v): want code %q, got %q", tc.op, tc.operands, tc.wantCode, calcErr.Code)
				}
				if err.Error() != calcErr.Message {
					t.Fatalf("%s(%v): Error() = %q, want the message %q", tc.op, tc.operands, err.Error(), calcErr.Message)
				}
				if !got.IsZero() {
					t.Fatalf("%s(%v): want no result alongside the error, got %s", tc.op, tc.operands, got)
				}
				return
			}

			if err != nil {
				t.Fatalf("%s(%v): unexpected error: %v", tc.op, tc.operands, err)
			}
			if got.String() != tc.want {
				t.Fatalf("%s(%v): want %s, got %s", tc.op, tc.operands, tc.want, got)
			}
		})
	}
}

func TestLargeExactMultiplicationIsReturned(t *testing.T) {
	// Two compact operands multiply into a long exact result: 1e9999 squared is
	// a one followed by 19,998 zeros. It is returned, not refused, because by
	// the time a result exists the work is already done.
	got, err := Calculate(Multiply, operands("1e9999", "1e9999"))
	if err != nil {
		t.Fatalf("1e9999 * 1e9999: unexpected error: %v", err)
	}
	text := got.String()
	if len(text) != 19_999 || text[0] != '1' || strings.Trim(text[1:], "0") != "" {
		t.Fatalf("1e9999 * 1e9999 = %d characters, want 1 followed by 19,998 zeros", len(text))
	}
}

func TestPowerAcceptsExponentAtTheCap(t *testing.T) {
	const want = "10715086071862673209484250490600018105614048117055336074437503883703510511249361224931983788156958581275946729175531468251871452856923140435984577574698574803934567774824230985421074605062371141877954182153046474983581941267398767559165543946077062914571196477686542167660429831652624386837205668069376"

	got, err := Calculate(Power, operands("2", "1000"))
	if err != nil {
		t.Fatalf("2^1000: unexpected error: %v", err)
	}
	if got.String() != want {
		t.Fatalf("2^1000 = %s, want %s", got, want)
	}
}

func TestDecimalOrder(t *testing.T) {
	cases := []struct {
		value string
		want  float64
	}{
		{value: "0", want: 0},
		{value: "1", want: 0},
		{value: "0.5", want: -0.30103},
		{value: "-0.5", want: -0.30103},
		{value: "15", want: 1.17609},
		{value: "-15", want: 1.17609},
		{value: "1e1000", want: 1000},
		{value: "-1e1000", want: 1000},
	}
	for _, tc := range cases {
		got := decimalOrder(decimal.RequireFromString(tc.value))
		if got < tc.want-0.001 || got > tc.want+0.001 {
			t.Fatalf("decimalOrder(%s) = %f, want about %f", tc.value, got, tc.want)
		}
	}
}

func TestPlacesFor(t *testing.T) {
	cases := []struct {
		order float64
		want  int32
	}{
		{order: 3, want: 12},
		{order: 0, want: 15},
		{order: -1, want: 16},
		{order: -20, want: 35},
		{order: 20, want: 0},
		{order: math.Inf(1), want: 0},
	}
	for _, tc := range cases {
		if got := placesFor(tc.order); got != tc.want {
			t.Fatalf("placesFor(%v) = %d, want %d", tc.order, got, tc.want)
		}
	}
}

func TestPowerBeyondTheOldExponentCap(t *testing.T) {
	big, err := Calculate(Power, operands("2", "1001"))
	if err != nil {
		t.Fatalf("2^1001: unexpected error: %v", err)
	}
	if got := big.String(); len(got) != 302 || got[:7] != "2143017" {
		t.Fatalf("2^1001 = %s..., want a 302-digit number starting 2143017", got[:16])
	}

	tiny, err := Calculate(Power, operands("2", "-1001"))
	if err != nil {
		t.Fatalf("2^-1001: unexpected error: %v", err)
	}
	if tiny.IsZero() {
		t.Fatalf("2^-1001 = 0, want a nonzero result")
	}
}

func TestFractionalPowersAreNotRefusedForSize(t *testing.T) {
	// A fractional exponent is rounded, so a shrinking base stays small however
	// large the exponent: these are all well inside the digit budget.
	cases := []struct {
		base, exponent string
		maxLength      int
	}{
		{base: "0.99", exponent: "6000.5", maxLength: 60},
		{base: "0.5", exponent: "10000.5", maxLength: 3100},
		{base: "0.9", exponent: "10001.5", maxLength: 500},
	}
	for _, tc := range cases {
		got, err := Calculate(Power, operands(tc.base, tc.exponent))
		if err != nil {
			t.Fatalf("%s^%s: unexpected refusal: %v", tc.base, tc.exponent, err)
		}
		if length := len(got.String()); length > tc.maxLength {
			t.Fatalf("%s^%s has %d characters, want at most %d", tc.base, tc.exponent, length, tc.maxLength)
		}
	}

	// An integer exponent is exact, so the same shrinking base expands.
	if _, err := Calculate(Power, operands("1."+strings.Repeat("9", 98), "1000")); err == nil {
		t.Fatal("an exact power with 98 fractional digits should still be refused")
	}
}

func TestResultSizeGuardDecidesBeforeComputing(t *testing.T) {
	// The guard itself: without this assertion, deleting it still passes the table
	// test and the refusal of this input then costs seconds of CPU before it
	// happens.
	nines := decimal.RequireFromString("1." + strings.Repeat("9", 98))

	if !resultWouldBeTooLarge(nines, decimal.NewFromInt(1000), 1000) {
		t.Fatal("an exact power with 98 fractional digits should be refused before computing")
	}
	// The same base with a fractional exponent is rounded, so it is allowed.
	if resultWouldBeTooLarge(nines, decimal.RequireFromString("2.5"), 2.5) {
		t.Fatal("a rounded fractional power should not be refused for size")
	}
	// A shrinking base with a fractional exponent is allowed however large it is.
	half := decimal.RequireFromString("0.5")
	if resultWouldBeTooLarge(half, decimal.NewFromInt(10000), 10000) {
		t.Fatal("0.5^10000 is small and should not be refused")
	}
	// A growing base past the budget is refused before computing.
	if !resultWouldBeTooLarge(decimal.NewFromInt(10), decimal.RequireFromString("1e6"), 1e6) {
		t.Fatal("10^1000000 should be refused before computing")
	}
	// A value of exactly 1 never grows, even when the exponent overflows float64.
	if resultWouldBeTooLarge(decimal.NewFromInt(1), decimal.RequireFromString("1e400"), math.Inf(1)) {
		t.Fatal("1 to any power is 1")
	}

	// And the guard is actually consulted: without it this refusal takes about
	// eleven seconds because the result is computed before being refused.
	started := time.Now()
	if _, err := Calculate(Power, operands("1."+strings.Repeat("9", 98), "100000")); err == nil {
		t.Fatal("the exact power should be refused")
	}
	if elapsed := time.Since(started); elapsed > 500*time.Millisecond {
		t.Fatalf("the refusal took %s, so the result was computed before the refusal", elapsed)
	}
}

func TestPowerAcceptsResultsInsideTheDigitBudget(t *testing.T) {
	cases := []struct {
		base, exponent string
		wantDigits     int
	}{
		{base: "2", exponent: "10001", wantDigits: 3011},
		{base: "10", exponent: "5001", wantDigits: 5002},
		{base: "100", exponent: "2501", wantDigits: 5003},
	}
	for _, tc := range cases {
		got, err := Calculate(Power, operands(tc.base, tc.exponent))
		if err != nil {
			t.Fatalf("%s^%s: unexpected error: %v", tc.base, tc.exponent, err)
		}
		if digits := len(got.String()); digits != tc.wantDigits {
			t.Fatalf("%s^%s has %d digits, want %d", tc.base, tc.exponent, digits, tc.wantDigits)
		}
	}
}

func TestPowerAllowsALargeNegativeResult(t *testing.T) {
	got, err := Calculate(Power, operands("-1000000000", "1000"))
	if err != nil {
		t.Fatalf("(-10^9)^1000: unexpected error: %v", err)
	}
	if text := got.String(); len(text) != 9001 || !strings.HasPrefix(text, "1") {
		t.Fatalf("(-10^9)^1000 = %s..., want 1 followed by 9000 zeros", text[:12])
	}
}

func TestPowerOfOneDoesNotExpandTheCoefficient(t *testing.T) {
	// "1.0" is coefficient 10 with exponent -1. Without the early answer,
	// PowWithPrecision expands that coefficient by the exponent and only then
	// rounds it back to 1, which at ten million is already hundreds of
	// milliseconds and a multi-megabyte integer.
	const exponent = "10000000"
	for _, base := range []string{"1.0", "1.00", "10e-1", "100e-2"} {
		started := time.Now()
		got, err := Calculate(Power, operands(base, exponent))
		if err != nil {
			t.Fatalf("%s^%s: unexpected error: %v", base, exponent, err)
		}
		if got.String() != "1" {
			t.Fatalf("%s^%s = %s, want 1", base, exponent, got)
		}
		if elapsed := time.Since(started); elapsed > 200*time.Millisecond {
			t.Fatalf("%s^%s took %s, so the coefficient was expanded before rounding", base, exponent, elapsed)
		}
	}

	for exponent, want := range map[string]string{"10000000": "1", "9999999": "-1"} {
		got, err := Calculate(Power, operands("-1.0", exponent))
		if err != nil {
			t.Fatalf("(-1.0)^%s: unexpected error: %v", exponent, err)
		}
		if got.String() != want {
			t.Fatalf("(-1.0)^%s = %s, want %s", exponent, got, want)
		}
	}
}

func TestNegativeExponentsAreNotRefusedForScale(t *testing.T) {
	// A negative integer exponent is a reciprocal: the result is bounded by the
	// base's magnitude, not by the length of its fractional part.
	cases := []struct {
		base     string
		exponent string
		maxLen   int
	}{
		{base: "0.5", exponent: "-10001", maxLen: 3100},
		{base: "0.9", exponent: "-10001", maxLen: 500},
		{base: "1.5", exponent: "-10001", maxLen: 1800},
		{base: "1.0000001", exponent: "-20000", maxLen: 30},
	}
	for _, tc := range cases {
		got, err := Calculate(Power, operands(tc.base, tc.exponent))
		if err != nil {
			t.Fatalf("%s^%s: unexpected refusal: %v", tc.base, tc.exponent, err)
		}
		if got.IsZero() {
			t.Fatalf("%s^%s = 0, want a non-zero result", tc.base, tc.exponent)
		}
		if length := len(got.String()); length > tc.maxLen {
			t.Fatalf("%s^%s has %d characters, want at most %d", tc.base, tc.exponent, length, tc.maxLen)
		}
	}
}

func TestQuotientTerminatingBeyondTheWindowIsRounded(t *testing.T) {
	// 2^332 is a 100-digit divisor, so the exact quotient terminates at 332
	// places, past the 100-place exact window. It must be rounded to 16
	// significant digits, not returned exactly.
	got, err := Calculate(Divide, operands("1", "8749002899132047697490008908470485461412677723572849745703082425639811996797503692894052708092215296"))
	if err != nil {
		t.Fatalf("1/2^332: unexpected error: %v", err)
	}
	if length := len(got.String()); length != 117 {
		t.Fatalf("1/2^332 has %d characters, want the 117 of a 16-significant-digit result", length)
	}
}

func TestRefusalMessagesStayBrief(t *testing.T) {
	_, err := Calculate(Sqrt, operands("-1e100"))
	if err == nil {
		t.Fatal("sqrt of a negative number should be refused")
	}
	if message := err.Error(); len(message) > 100 || !strings.Contains(message, "...") {
		t.Fatalf("message = %q, want the operand truncated", message)
	}
}

func TestFractionalDigits(t *testing.T) {
	cases := []struct {
		value string
		want  float64
	}{
		{value: "2", want: 0},
		{value: "0.5", want: 1},
		{value: "1.25", want: 2},
		{value: "1." + strings.Repeat("0", 98), want: 0},
		{value: "1e-10000", want: 10000},
	}
	for _, tc := range cases {
		if got := fractionalDigits(decimal.RequireFromString(tc.value)); got != tc.want {
			t.Fatalf("fractionalDigits(%s) = %v, want %v", tc.value, got, tc.want)
		}
	}
	if got := placesFor(-1_000_000); got != maxPlaces {
		t.Fatalf("placesFor(-1000000) = %d, want the %d clamp", got, maxPlaces)
	}
}

func TestMaxOperandsCoversEveryOperation(t *testing.T) {
	for op, s := range specs {
		if s.arity > MaxOperands {
			t.Fatalf("%s takes %d operands, above MaxOperands %d", op, s.arity, MaxOperands)
		}
	}
}

func TestErrorCodeLiterals(t *testing.T) {
	literals := map[string]string{
		CodeUnknownOperation:    "unknown_operation",
		CodeInvalidOperandCount: "invalid_operand_count",
		CodeDivisionByZero:      "division_by_zero",
		CodeNegativeSqrt:        "negative_sqrt",
		CodeUndefinedPower:      "undefined_power",
		CodeResultTooLarge:      "result_too_large",
	}
	for got, want := range literals {
		if got != want {
			t.Fatalf("wire code %q, want %q", got, want)
		}
	}
}

func TestNames(t *testing.T) {
	want := []string{"add", "divide", "multiply", "percent", "power", "sqrt", "subtract"}
	if got := Names(); !reflect.DeepEqual(got, want) {
		t.Fatalf("Names() = %v, want %v", got, want)
	}
}
