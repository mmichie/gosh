package gosh

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"unicode"
)

// EvaluateArithmetic parses and evaluates a single arithmetic expression
// against the shell's variable namespace. Side effects (assignments,
// increments) write back through the same path as parameter expansion's
// :=  form: locals when shadowing a function local, environment otherwise.
//
// Returns the final integer value of the expression. The whole expression
// must be consumed; trailing junk is an error.
func EvaluateArithmetic(expr string) (int64, error) {
	p := &arithParser{src: expr}
	v, err := p.parseExpression(0)
	if err != nil {
		return 0, err
	}
	p.skipSpace()
	if p.pos < len(p.src) {
		return 0, fmt.Errorf("arithmetic: unexpected character %q at offset %d", p.src[p.pos], p.pos)
	}
	return v.toInt(), nil
}

// ExpandArithmetic finds $((...)) in s and replaces each with the result of
// evaluating the inner expression. Balanced parentheses inside the
// expression are honored so $((1 + (2 * 3))) is parsed correctly.
func ExpandArithmetic(s string) (string, error) {
	var b strings.Builder
	i := 0
	for i < len(s) {
		if s[i] == '\\' && i+1 < len(s) {
			b.WriteByte(s[i])
			b.WriteByte(s[i+1])
			i += 2
			continue
		}
		if i+2 < len(s) && s[i] == '$' && s[i+1] == '(' && s[i+2] == '(' {
			end := findArithClose(s, i+3)
			if end == -1 {
				b.WriteByte(s[i])
				i++
				continue
			}
			inner := s[i+3 : end]
			expanded, err := ExpandSpecialVariablesE(inner)
			if err != nil {
				return "", err
			}
			result, err := EvaluateArithmetic(expanded)
			if err != nil {
				return "", err
			}
			b.WriteString(strconv.FormatInt(result, 10))
			i = end + 2
			continue
		}
		b.WriteByte(s[i])
		i++
	}
	return b.String(), nil
}

// findArithClose returns the index of the first ) that closes the )) of an
// arithmetic expansion starting at startIdx (after the opening $((). Returns
// -1 if no balanced close is found.
func findArithClose(s string, startIdx int) int {
	depth := 1
	for i := startIdx; i < len(s)-1; i++ {
		switch s[i] {
		case '\\':
			i++
		case '(':
			depth++
		case ')':
			if s[i+1] == ')' && depth == 1 {
				return i
			}
			depth--
		}
	}
	return -1
}

// arithValue carries an int and a name (when the value originated from a
// bare identifier). The name is needed by the assignment, ++, and -- forms.
type arithValue struct {
	v       int64
	name    string
	hasName bool
}

func intVal(v int64) arithValue { return arithValue{v: v} }

func (a arithValue) toInt() int64 { return a.v }

// arithParser is a Pratt-style parser for arithmetic expressions. The
// precedence table is encoded by the precedence values returned from
// parseExpression on operator lookup.
type arithParser struct {
	src string
	pos int
}

func (p *arithParser) skipSpace() {
	for p.pos < len(p.src) && (p.src[p.pos] == ' ' || p.src[p.pos] == '\t' || p.src[p.pos] == '\n') {
		p.pos++
	}
}

func (p *arithParser) peek() byte {
	if p.pos >= len(p.src) {
		return 0
	}
	return p.src[p.pos]
}

func (p *arithParser) peekAt(offset int) byte {
	if p.pos+offset >= len(p.src) {
		return 0
	}
	return p.src[p.pos+offset]
}

// parseExpression parses an expression while binding tighter than minPrec.
// minPrec=0 parses a complete expression; recursive calls pass the current
// operator's precedence to enforce associativity.
func (p *arithParser) parseExpression(minPrec int) (arithValue, error) {
	left, err := p.parseUnary()
	if err != nil {
		return arithValue{}, err
	}

	for {
		p.skipSpace()
		opStr, prec, rightAssoc, ok := p.peekBinaryOp()
		if !ok || prec < minPrec {
			break
		}
		p.pos += len(opStr)

		// Ternary: condition ? a : b
		if opStr == "?" {
			thenV, err := p.parseExpression(0)
			if err != nil {
				return arithValue{}, err
			}
			p.skipSpace()
			if p.peek() != ':' {
				return arithValue{}, fmt.Errorf("arithmetic: expected ':' in ternary")
			}
			p.pos++
			elseV, err := p.parseExpression(prec)
			if err != nil {
				return arithValue{}, err
			}
			if left.v != 0 {
				left = thenV
			} else {
				left = elseV
			}
			continue
		}

		// Assignment family writes through `left.name`, so capture the LHS
		// name BEFORE evaluating RHS.
		if isAssignOp(opStr) {
			if !left.hasName {
				return arithValue{}, fmt.Errorf("arithmetic: assignment to non-variable")
			}
			name := left.name
			rhs, err := p.parseExpression(prec)
			if err != nil {
				return arithValue{}, err
			}
			newVal, err := applyAssignOp(opStr, left.v, rhs.v)
			if err != nil {
				return arithValue{}, err
			}
			if err := writeArithVar(name, newVal); err != nil {
				return arithValue{}, err
			}
			left = intVal(newVal)
			continue
		}

		nextMin := prec + 1
		if rightAssoc {
			nextMin = prec
		}
		right, err := p.parseExpression(nextMin)
		if err != nil {
			return arithValue{}, err
		}
		v, err := applyBinaryOp(opStr, left.v, right.v)
		if err != nil {
			return arithValue{}, err
		}
		left = intVal(v)
	}
	return left, nil
}

// peekBinaryOp looks ahead at the current position for a binary operator and
// returns it with its precedence and associativity. Returns ok=false if the
// next token isn't a binary operator.
func (p *arithParser) peekBinaryOp() (op string, prec int, rightAssoc, ok bool) {
	if p.pos >= len(p.src) {
		return "", 0, false, false
	}
	c := p.src[p.pos]
	c2 := p.peekAt(1)
	c3 := p.peekAt(2)

	// Three-char ops first.
	switch {
	case c == '<' && c2 == '<' && c3 == '=':
		return "<<=", 2, true, true
	case c == '>' && c2 == '>' && c3 == '=':
		return ">>=", 2, true, true
	case c == '*' && c2 == '*' && c3 == '=':
		return "**=", 2, true, true
	}
	// Two-char ops.
	switch string([]byte{c, c2}) {
	case "**":
		return "**", 13, true, true
	case "<<":
		return "<<", 10, false, true
	case ">>":
		return ">>", 10, false, true
	case "<=":
		return "<=", 9, false, true
	case ">=":
		return ">=", 9, false, true
	case "==":
		return "==", 8, false, true
	case "!=":
		return "!=", 8, false, true
	case "&&":
		return "&&", 4, false, true
	case "||":
		return "||", 3, false, true
	case "+=":
		return "+=", 2, true, true
	case "-=":
		return "-=", 2, true, true
	case "*=":
		return "*=", 2, true, true
	case "/=":
		return "/=", 2, true, true
	case "%=":
		return "%=", 2, true, true
	case "&=":
		return "&=", 2, true, true
	case "|=":
		return "|=", 2, true, true
	case "^=":
		return "^=", 2, true, true
	}
	switch c {
	case '*':
		return "*", 12, false, true
	case '/':
		return "/", 12, false, true
	case '%':
		return "%", 12, false, true
	case '+':
		return "+", 11, false, true
	case '-':
		return "-", 11, false, true
	case '<':
		return "<", 9, false, true
	case '>':
		return ">", 9, false, true
	case '&':
		return "&", 7, false, true
	case '^':
		return "^", 6, false, true
	case '|':
		return "|", 5, false, true
	case '?':
		return "?", 1, true, true
	case '=':
		return "=", 2, true, true
	}
	return "", 0, false, false
}

// parseUnary handles prefix operators (-, +, !, ~, ++, --) and parentheses,
// then dispatches to parsePrimary for atoms. Postfix ++/-- is handled here
// by peeking after the primary.
func (p *arithParser) parseUnary() (arithValue, error) {
	p.skipSpace()
	c := p.peek()
	c2 := p.peekAt(1)

	if c == '+' && c2 == '+' {
		p.pos += 2
		operand, err := p.parseUnary()
		if err != nil {
			return arithValue{}, err
		}
		if !operand.hasName {
			return arithValue{}, fmt.Errorf("arithmetic: ++ requires a variable")
		}
		newV := operand.v + 1
		if err := writeArithVar(operand.name, newV); err != nil {
			return arithValue{}, err
		}
		return intVal(newV), nil
	}
	if c == '-' && c2 == '-' {
		p.pos += 2
		operand, err := p.parseUnary()
		if err != nil {
			return arithValue{}, err
		}
		if !operand.hasName {
			return arithValue{}, fmt.Errorf("arithmetic: -- requires a variable")
		}
		newV := operand.v - 1
		if err := writeArithVar(operand.name, newV); err != nil {
			return arithValue{}, err
		}
		return intVal(newV), nil
	}
	switch c {
	case '+':
		p.pos++
		return p.parseUnary()
	case '-':
		p.pos++
		v, err := p.parseUnary()
		if err != nil {
			return arithValue{}, err
		}
		return intVal(-v.v), nil
	case '!':
		p.pos++
		v, err := p.parseUnary()
		if err != nil {
			return arithValue{}, err
		}
		if v.v == 0 {
			return intVal(1), nil
		}
		return intVal(0), nil
	case '~':
		p.pos++
		v, err := p.parseUnary()
		if err != nil {
			return arithValue{}, err
		}
		return intVal(^v.v), nil
	}

	primary, err := p.parsePrimary()
	if err != nil {
		return arithValue{}, err
	}

	// Postfix ++/--
	p.skipSpace()
	if p.peek() == '+' && p.peekAt(1) == '+' {
		if !primary.hasName {
			return arithValue{}, fmt.Errorf("arithmetic: postfix ++ requires a variable")
		}
		p.pos += 2
		if err := writeArithVar(primary.name, primary.v+1); err != nil {
			return arithValue{}, err
		}
		return intVal(primary.v), nil
	}
	if p.peek() == '-' && p.peekAt(1) == '-' {
		if !primary.hasName {
			return arithValue{}, fmt.Errorf("arithmetic: postfix -- requires a variable")
		}
		p.pos += 2
		if err := writeArithVar(primary.name, primary.v-1); err != nil {
			return arithValue{}, err
		}
		return intVal(primary.v), nil
	}
	return primary, nil
}

// parsePrimary handles parenthesized expressions, numeric literals
// (including 0x, 0, base#digits forms), and identifiers.
func (p *arithParser) parsePrimary() (arithValue, error) {
	p.skipSpace()
	c := p.peek()
	if c == '(' {
		p.pos++
		v, err := p.parseExpression(0)
		if err != nil {
			return arithValue{}, err
		}
		p.skipSpace()
		if p.peek() != ')' {
			return arithValue{}, fmt.Errorf("arithmetic: expected ')'")
		}
		p.pos++
		return intVal(v.v), nil
	}
	if c >= '0' && c <= '9' {
		return p.parseNumber()
	}
	if c == '_' || (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') {
		return p.parseIdent()
	}
	return arithValue{}, fmt.Errorf("arithmetic: unexpected token %q at offset %d", c, p.pos)
}

// parseNumber reads a numeric literal: decimal, hex (0x...), octal (leading
// 0), or base#digits form.
func (p *arithParser) parseNumber() (arithValue, error) {
	start := p.pos
	for p.pos < len(p.src) && (isDigit(p.src[p.pos]) || isHexLetter(p.src[p.pos]) || p.src[p.pos] == 'x' || p.src[p.pos] == 'X' || p.src[p.pos] == '#') {
		p.pos++
	}
	tok := p.src[start:p.pos]

	if strings.Contains(tok, "#") {
		parts := strings.SplitN(tok, "#", 2)
		base, err := strconv.Atoi(parts[0])
		if err != nil || base < 2 || base > 64 {
			return arithValue{}, fmt.Errorf("arithmetic: invalid base %q", parts[0])
		}
		v, err := strconv.ParseInt(parts[1], base, 64)
		if err != nil {
			return arithValue{}, fmt.Errorf("arithmetic: invalid digits for base %d: %q", base, parts[1])
		}
		return intVal(v), nil
	}

	if strings.HasPrefix(tok, "0x") || strings.HasPrefix(tok, "0X") {
		v, err := strconv.ParseInt(tok[2:], 16, 64)
		if err != nil {
			return arithValue{}, fmt.Errorf("arithmetic: invalid hex %q", tok)
		}
		return intVal(v), nil
	}
	if len(tok) > 1 && tok[0] == '0' {
		v, err := strconv.ParseInt(tok[1:], 8, 64)
		if err != nil {
			return arithValue{}, fmt.Errorf("arithmetic: invalid octal %q", tok)
		}
		return intVal(v), nil
	}
	v, err := strconv.ParseInt(tok, 10, 64)
	if err != nil {
		return arithValue{}, fmt.Errorf("arithmetic: invalid number %q", tok)
	}
	return intVal(v), nil
}

// parseIdent reads an identifier and looks it up. Unset variables yield 0,
// matching POSIX. The name is preserved on the returned value so assignment
// and ++/-- can write back to it.
func (p *arithParser) parseIdent() (arithValue, error) {
	start := p.pos
	for p.pos < len(p.src) && (p.src[p.pos] == '_' || isDigit(p.src[p.pos]) || isLetter(p.src[p.pos])) {
		p.pos++
	}
	name := p.src[start:p.pos]
	val := readArithVar(name)
	return arithValue{v: val, name: name, hasName: true}, nil
}

func isDigit(c byte) bool  { return c >= '0' && c <= '9' }
func isLetter(c byte) bool { return (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') }
func isHexLetter(c byte) bool {
	return (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
}

// readArithVar resolves name to an integer. Strings that don't parse as
// integers evaluate to 0 (matching bash behavior in arithmetic context).
func readArithVar(name string) int64 {
	val, ok := lookupShellVar(name)
	if !ok || val == "" {
		return 0
	}
	v, err := strconv.ParseInt(strings.TrimSpace(val), 10, 64)
	if err != nil {
		return 0
	}
	return v
}

// writeArithVar stores an integer back to the shell. Honors function-local
// scopes the same way as parameter expansion's := form.
func writeArithVar(name string, value int64) error {
	gs := GetGlobalState()
	str := strconv.FormatInt(value, 10)
	if gs.IsReadonly(name) {
		return fmt.Errorf("%s: readonly variable", name)
	}
	if gs.IsInFunction() {
		if _, isLocal := gs.GetLocalVar(name); isLocal {
			return gs.SetLocalVar(name, str)
		}
	}
	return os.Setenv(name, str)
}

func isAssignOp(op string) bool {
	switch op {
	case "=", "+=", "-=", "*=", "/=", "%=", "&=", "|=", "^=", "<<=", ">>=", "**=":
		return true
	}
	return false
}

func applyAssignOp(op string, left, right int64) (int64, error) {
	switch op {
	case "=":
		return right, nil
	case "+=":
		return left + right, nil
	case "-=":
		return left - right, nil
	case "*=":
		return left * right, nil
	case "/=":
		if right == 0 {
			return 0, fmt.Errorf("arithmetic: division by zero")
		}
		return left / right, nil
	case "%=":
		if right == 0 {
			return 0, fmt.Errorf("arithmetic: modulo by zero")
		}
		return left % right, nil
	case "&=":
		return left & right, nil
	case "|=":
		return left | right, nil
	case "^=":
		return left ^ right, nil
	case "<<=":
		return left << uint(right), nil
	case ">>=":
		return left >> uint(right), nil
	case "**=":
		return intPow(left, right), nil
	}
	return 0, fmt.Errorf("arithmetic: unknown assignment operator %q", op)
}

func applyBinaryOp(op string, left, right int64) (int64, error) {
	switch op {
	case "+":
		return left + right, nil
	case "-":
		return left - right, nil
	case "*":
		return left * right, nil
	case "/":
		if right == 0 {
			return 0, fmt.Errorf("arithmetic: division by zero")
		}
		return left / right, nil
	case "%":
		if right == 0 {
			return 0, fmt.Errorf("arithmetic: modulo by zero")
		}
		return left % right, nil
	case "**":
		return intPow(left, right), nil
	case "<<":
		return left << uint(right), nil
	case ">>":
		return left >> uint(right), nil
	case "&":
		return left & right, nil
	case "|":
		return left | right, nil
	case "^":
		return left ^ right, nil
	case "<":
		return boolInt(left < right), nil
	case ">":
		return boolInt(left > right), nil
	case "<=":
		return boolInt(left <= right), nil
	case ">=":
		return boolInt(left >= right), nil
	case "==":
		return boolInt(left == right), nil
	case "!=":
		return boolInt(left != right), nil
	case "&&":
		return boolInt(left != 0 && right != 0), nil
	case "||":
		return boolInt(left != 0 || right != 0), nil
	}
	return 0, fmt.Errorf("arithmetic: unknown operator %q", op)
}

func boolInt(b bool) int64 {
	if b {
		return 1
	}
	return 0
}

// intPow computes base**exp with integer semantics. Negative exponents
// return 0 (matching bash).
func intPow(base, exp int64) int64 {
	if exp < 0 {
		return 0
	}
	result := int64(1)
	for exp > 0 {
		if exp&1 == 1 {
			result *= base
		}
		base *= base
		exp >>= 1
	}
	return result
}

// PreprocessArithmeticCommand rewrites a leading ((expr)) command form into
// `let "expr"` so the existing parser can handle it. Only matches when the
// trimmed input starts with (( and ends with )); leaves everything else
// untouched.
func PreprocessArithmeticCommand(input string) string {
	trimmed := strings.TrimLeftFunc(input, unicode.IsSpace)
	if !strings.HasPrefix(trimmed, "((") {
		return input
	}
	// Walk the rest looking for the matching )) at top level.
	body := trimmed[2:]
	depth := 1
	closeIdx := -1
	for i := 0; i < len(body)-1; i++ {
		switch body[i] {
		case '\\':
			i++
		case '(':
			depth++
		case ')':
			if body[i+1] == ')' && depth == 1 {
				closeIdx = i
				i = len(body) // break outer
			} else {
				depth--
			}
		}
	}
	if closeIdx == -1 {
		return input
	}
	expr := strings.TrimSpace(body[:closeIdx])
	rest := strings.TrimLeftFunc(body[closeIdx+2:], unicode.IsSpace)
	leadingSpace := input[:len(input)-len(trimmed)]
	if rest == "" {
		return leadingSpace + "let " + strconv.Quote(expr)
	}
	return leadingSpace + "let " + strconv.Quote(expr) + " " + rest
}
