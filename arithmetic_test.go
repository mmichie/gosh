package gosh

import (
	"os"
	"strings"
	"testing"
)

func TestEvaluateArithmeticBasic(t *testing.T) {
	cases := []struct {
		expr string
		want int64
	}{
		{"0", 0},
		{"42", 42},
		{"1 + 2", 3},
		{"7 - 3", 4},
		{"6 * 7", 42},
		{"20 / 3", 6},
		{"20 % 3", 2},
		{"2 ** 10", 1024},
		{"-5", -5},
		{"+5", 5},
		{"!0", 1},
		{"!42", 0},
		{"~0", -1},
	}
	for _, c := range cases {
		got, err := EvaluateArithmetic(c.expr)
		if err != nil {
			t.Fatalf("EvaluateArithmetic(%q) error: %v", c.expr, err)
		}
		if got != c.want {
			t.Errorf("EvaluateArithmetic(%q) = %d, want %d", c.expr, got, c.want)
		}
	}
}

func TestEvaluateArithmeticPrecedence(t *testing.T) {
	cases := []struct {
		expr string
		want int64
	}{
		{"1 + 2 * 3", 7},
		{"(1 + 2) * 3", 9},
		{"2 ** 3 ** 2", 512}, // right-associative: 2**(3**2) = 2**9 = 512
		{"10 - 3 - 2", 5},    // left-associative
		{"100 / 10 / 2", 5},
		{"1 << 4", 16},
		{"32 >> 2", 8},
		{"6 & 3", 2},
		{"6 | 3", 7},
		{"6 ^ 3", 5},
	}
	for _, c := range cases {
		got, err := EvaluateArithmetic(c.expr)
		if err != nil {
			t.Fatalf("EvaluateArithmetic(%q) error: %v", c.expr, err)
		}
		if got != c.want {
			t.Errorf("EvaluateArithmetic(%q) = %d, want %d", c.expr, got, c.want)
		}
	}
}

func TestEvaluateArithmeticComparison(t *testing.T) {
	cases := []struct {
		expr string
		want int64
	}{
		{"1 < 2", 1},
		{"2 < 1", 0},
		{"1 <= 1", 1},
		{"3 > 2", 1},
		{"2 >= 3", 0},
		{"5 == 5", 1},
		{"5 != 5", 0},
		{"1 && 1", 1},
		{"1 && 0", 0},
		{"0 || 0", 0},
		{"0 || 5", 1},
	}
	for _, c := range cases {
		got, err := EvaluateArithmetic(c.expr)
		if err != nil {
			t.Fatalf("EvaluateArithmetic(%q) error: %v", c.expr, err)
		}
		if got != c.want {
			t.Errorf("EvaluateArithmetic(%q) = %d, want %d", c.expr, got, c.want)
		}
	}
}

func TestEvaluateArithmeticTernary(t *testing.T) {
	cases := []struct {
		expr string
		want int64
	}{
		{"1 ? 10 : 20", 10},
		{"0 ? 10 : 20", 20},
		{"5 > 3 ? 100 : 200", 100},
		{"5 < 3 ? 100 : 200", 200},
		{"1 ? 2 ? 3 : 4 : 5", 3},
	}
	for _, c := range cases {
		got, err := EvaluateArithmetic(c.expr)
		if err != nil {
			t.Fatalf("EvaluateArithmetic(%q) error: %v", c.expr, err)
		}
		if got != c.want {
			t.Errorf("EvaluateArithmetic(%q) = %d, want %d", c.expr, got, c.want)
		}
	}
}

func TestEvaluateArithmeticAssignment(t *testing.T) {
	os.Unsetenv("GOSH_ARITH_X")
	os.Unsetenv("GOSH_ARITH_Y")
	defer func() {
		os.Unsetenv("GOSH_ARITH_X")
		os.Unsetenv("GOSH_ARITH_Y")
	}()

	v, err := EvaluateArithmetic("GOSH_ARITH_X = 42")
	if err != nil {
		t.Fatalf("assign error: %v", err)
	}
	if v != 42 {
		t.Errorf("assign result: got %d, want 42", v)
	}
	if got := os.Getenv("GOSH_ARITH_X"); got != "42" {
		t.Errorf("env after assign: got %q, want %q", got, "42")
	}

	v, err = EvaluateArithmetic("GOSH_ARITH_X += 8")
	if err != nil {
		t.Fatalf("+= error: %v", err)
	}
	if v != 50 {
		t.Errorf("+= result: got %d, want 50", v)
	}

	v, err = EvaluateArithmetic("GOSH_ARITH_X *= 2")
	if err != nil {
		t.Fatalf("*= error: %v", err)
	}
	if v != 100 {
		t.Errorf("*= result: got %d, want 100", v)
	}

	v, err = EvaluateArithmetic("GOSH_ARITH_Y = GOSH_ARITH_X / 4")
	if err != nil {
		t.Fatalf("cross-var error: %v", err)
	}
	if v != 25 {
		t.Errorf("cross-var result: got %d, want 25", v)
	}
}

func TestEvaluateArithmeticIncrementDecrement(t *testing.T) {
	os.Unsetenv("GOSH_ARITH_I")
	defer os.Unsetenv("GOSH_ARITH_I")

	if _, err := EvaluateArithmetic("GOSH_ARITH_I = 10"); err != nil {
		t.Fatalf("init error: %v", err)
	}

	v, err := EvaluateArithmetic("++GOSH_ARITH_I")
	if err != nil {
		t.Fatalf("prefix ++ error: %v", err)
	}
	if v != 11 {
		t.Errorf("prefix ++ result: got %d, want 11", v)
	}

	v, err = EvaluateArithmetic("GOSH_ARITH_I++")
	if err != nil {
		t.Fatalf("postfix ++ error: %v", err)
	}
	if v != 11 {
		t.Errorf("postfix ++ returns pre-increment value: got %d, want 11", v)
	}
	if got := os.Getenv("GOSH_ARITH_I"); got != "12" {
		t.Errorf("after postfix ++, env: got %q, want %q", got, "12")
	}

	v, err = EvaluateArithmetic("--GOSH_ARITH_I")
	if err != nil {
		t.Fatalf("prefix -- error: %v", err)
	}
	if v != 11 {
		t.Errorf("prefix -- result: got %d, want 11", v)
	}

	v, err = EvaluateArithmetic("GOSH_ARITH_I--")
	if err != nil {
		t.Fatalf("postfix -- error: %v", err)
	}
	if v != 11 {
		t.Errorf("postfix -- returns pre-decrement value: got %d, want 11", v)
	}
	if got := os.Getenv("GOSH_ARITH_I"); got != "10" {
		t.Errorf("after postfix --, env: got %q, want %q", got, "10")
	}
}

func TestEvaluateArithmeticDivisionByZero(t *testing.T) {
	if _, err := EvaluateArithmetic("1 / 0"); err == nil {
		t.Errorf("expected error for division by zero")
	}
	if _, err := EvaluateArithmetic("1 % 0"); err == nil {
		t.Errorf("expected error for modulo by zero")
	}
}

func TestEvaluateArithmeticNumberForms(t *testing.T) {
	cases := []struct {
		expr string
		want int64
	}{
		{"0xff", 255},
		{"0XFF", 255},
		{"0x10", 16},
		{"010", 8},     // octal
		{"0777", 511},  // octal
		{"2#1010", 10}, // base#digits
		{"16#ff", 255}, // base 16
		{"8#17", 15},   // base 8
	}
	for _, c := range cases {
		got, err := EvaluateArithmetic(c.expr)
		if err != nil {
			t.Fatalf("EvaluateArithmetic(%q) error: %v", c.expr, err)
		}
		if got != c.want {
			t.Errorf("EvaluateArithmetic(%q) = %d, want %d", c.expr, got, c.want)
		}
	}
}

func TestEvaluateArithmeticUnsetVarIsZero(t *testing.T) {
	os.Unsetenv("GOSH_ARITH_UNSET")
	v, err := EvaluateArithmetic("GOSH_ARITH_UNSET + 5")
	if err != nil {
		t.Fatalf("unset var error: %v", err)
	}
	if v != 5 {
		t.Errorf("unset var: got %d, want 5", v)
	}
}

func TestEvaluateArithmeticNonIntegerVarIsZero(t *testing.T) {
	os.Setenv("GOSH_ARITH_STR", "hello")
	defer os.Unsetenv("GOSH_ARITH_STR")
	v, err := EvaluateArithmetic("GOSH_ARITH_STR + 7")
	if err != nil {
		t.Fatalf("non-integer var error: %v", err)
	}
	if v != 7 {
		t.Errorf("non-integer var: got %d, want 7", v)
	}
}

func TestEvaluateArithmeticErrors(t *testing.T) {
	bad := []string{
		"1 +",
		"(1 + 2",
		"1 + ?",
		"5 = 3",      // assignment to non-variable
		"++5",        // ++ requires variable
		"5++",        // postfix ++ requires variable
		"1 ? 2",      // missing ternary else
		"1 + abc#xx", // bad base
	}
	for _, expr := range bad {
		if _, err := EvaluateArithmetic(expr); err == nil {
			t.Errorf("expected error for %q", expr)
		}
	}
}

func TestExpandArithmeticInString(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"echo $((1+2))", "echo 3"},
		{"a$((3*4))b", "a12b"},
		{"$((1+2)) $((3+4))", "3 7"},
		{"$(( 2 ** 8 ))", "256"},
		{"no expansion here", "no expansion here"},
		{"$(echo nested)", "$(echo nested)"}, // command sub, not arithmetic — pass through
		{"$((1 + (2 * 3)))", "7"},
	}
	for _, c := range cases {
		got, err := ExpandArithmetic(c.in)
		if err != nil {
			t.Fatalf("ExpandArithmetic(%q) error: %v", c.in, err)
		}
		if got != c.want {
			t.Errorf("ExpandArithmetic(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestExpandArithmeticWithVariable(t *testing.T) {
	os.Setenv("GOSH_ARITH_VAR", "10")
	defer os.Unsetenv("GOSH_ARITH_VAR")

	got, err := ExpandArithmetic("$((GOSH_ARITH_VAR + 5))")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if got != "15" {
		t.Errorf("got %q, want %q", got, "15")
	}

	got, err = ExpandArithmetic("$(($GOSH_ARITH_VAR * 2))")
	if err != nil {
		t.Fatalf("error with $-prefixed var: %v", err)
	}
	if got != "20" {
		t.Errorf("$-prefix: got %q, want %q", got, "20")
	}
}

func TestPreprocessArithmeticCommand(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"((x = 5))", `let "x = 5"`},
		{"((y++))", `let "y++"`},
		{"  ((a + b))", `  let "a + b"`},
		{"echo foo", "echo foo"}, // unaffected
		{"((x=1)) && echo ok", `let "x=1" && echo ok`},
		{"((x = (1 + 2)))", `let "x = (1 + 2)"`},
	}
	for _, c := range cases {
		got := PreprocessArithmeticCommand(c.in)
		if got != c.want {
			t.Errorf("PreprocessArithmeticCommand(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestPreprocessArithmeticCommandUnclosed(t *testing.T) {
	// Unclosed (( should be left alone, not corrupted.
	in := "((x = 5"
	got := PreprocessArithmeticCommand(in)
	if got != in {
		t.Errorf("unclosed (( should be passthrough, got %q", got)
	}
}

func TestExpandArithmeticEscape(t *testing.T) {
	// A backslash should let \$ pass through without triggering arithmetic.
	in := `\$((1+2))`
	got, err := ExpandArithmetic(in)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if !strings.Contains(got, `\$`) {
		t.Errorf("escaped $ should be preserved, got %q", got)
	}
}
