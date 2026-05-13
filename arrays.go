package gosh

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// ShellArray holds an indexed or associative array. Indexed uses sparse map
// keys to support `arr[100]=x` without allocating gaps.
type ShellArray struct {
	Indexed     map[int]string
	Associative map[string]string
	IsAssoc     bool
}

// NewIndexedArray returns an empty indexed array.
func NewIndexedArray() *ShellArray {
	return &ShellArray{Indexed: map[int]string{}}
}

// NewAssociativeArray returns an empty associative array.
func NewAssociativeArray() *ShellArray {
	return &ShellArray{Associative: map[string]string{}, IsAssoc: true}
}

// Set writes a value at a subscript. For indexed arrays the subscript must be
// an integer expression; for associative arrays it's a literal key.
func (a *ShellArray) Set(subscript, value string) error {
	if a.IsAssoc {
		if a.Associative == nil {
			a.Associative = map[string]string{}
		}
		a.Associative[subscript] = value
		return nil
	}
	idx, err := arraySubscriptIndex(subscript)
	if err != nil {
		return err
	}
	if a.Indexed == nil {
		a.Indexed = map[int]string{}
	}
	a.Indexed[idx] = value
	return nil
}

// Get reads a value by subscript. Returns "" and ok=false when unset.
// `@` and `*` are special — call Values instead for those.
func (a *ShellArray) Get(subscript string) (string, bool) {
	if a.IsAssoc {
		v, ok := a.Associative[subscript]
		return v, ok
	}
	if subscript == "" {
		// Bare ${arr} is equivalent to ${arr[0]}
		v, ok := a.Indexed[0]
		return v, ok
	}
	idx, err := arraySubscriptIndex(subscript)
	if err != nil {
		return "", false
	}
	v, ok := a.Indexed[idx]
	return v, ok
}

// Values returns elements in iteration order: numeric ascending for indexed,
// insertion-independent sorted-by-key for associative (bash actually uses
// hash order, but sorted is deterministic and more useful).
func (a *ShellArray) Values() []string {
	if a.IsAssoc {
		keys := make([]string, 0, len(a.Associative))
		for k := range a.Associative {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		out := make([]string, len(keys))
		for i, k := range keys {
			out[i] = a.Associative[k]
		}
		return out
	}
	keys := make([]int, 0, len(a.Indexed))
	for k := range a.Indexed {
		keys = append(keys, k)
	}
	sort.Ints(keys)
	out := make([]string, len(keys))
	for i, k := range keys {
		out[i] = a.Indexed[k]
	}
	return out
}

// Keys returns the subscripts in iteration order, as strings.
func (a *ShellArray) Keys() []string {
	if a.IsAssoc {
		keys := make([]string, 0, len(a.Associative))
		for k := range a.Associative {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		return keys
	}
	keys := make([]int, 0, len(a.Indexed))
	for k := range a.Indexed {
		keys = append(keys, k)
	}
	sort.Ints(keys)
	out := make([]string, len(keys))
	for i, k := range keys {
		out[i] = strconv.Itoa(k)
	}
	return out
}

// Length is the count of set elements.
func (a *ShellArray) Length() int {
	if a.IsAssoc {
		return len(a.Associative)
	}
	return len(a.Indexed)
}

// Unset removes a subscript. No-op if absent.
func (a *ShellArray) Unset(subscript string) {
	if a.IsAssoc {
		delete(a.Associative, subscript)
		return
	}
	if idx, err := arraySubscriptIndex(subscript); err == nil {
		delete(a.Indexed, idx)
	}
}

// Append adds values at the end of an indexed array using the next free
// (max+1) index. No-op for associative — bash requires an explicit key there.
func (a *ShellArray) Append(values ...string) {
	if a.IsAssoc {
		return
	}
	if a.Indexed == nil {
		a.Indexed = map[int]string{}
	}
	next := 0
	for k := range a.Indexed {
		if k+1 > next {
			next = k + 1
		}
	}
	for _, v := range values {
		a.Indexed[next] = v
		next++
	}
}

// arraySubscriptIndex parses an integer subscript. Negative indexes count
// from the end (matching bash) but only at read time — Set always uses the
// raw integer value.
func arraySubscriptIndex(s string) (int, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("array: empty subscript")
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		// Try arithmetic evaluation for expressions like ${arr[i+1]}.
		v, aerr := EvaluateArithmetic(s)
		if aerr != nil {
			return 0, fmt.Errorf("array subscript: %q is not an integer", s)
		}
		return int(v), nil
	}
	return n, nil
}

// ParseArrayLiteral parses the body of `arr=(a b c)` (the content between the
// parentheses, with leading and trailing whitespace already trimmed) into a
// list of elements. Honors single- and double-quoted strings as one element
// each, and `key=value` or `[idx]=value` element forms for indexed/assoc
// initialization.
//
// Returns the parsed elements as raw strings (quotes stripped); the caller is
// responsible for further expansion if needed.
func ParseArrayLiteral(body string) ([]string, error) {
	var elements []string
	i := 0
	var cur strings.Builder
	inElem := false
	flush := func() {
		if inElem {
			elements = append(elements, cur.String())
			cur.Reset()
			inElem = false
		}
	}
	for i < len(body) {
		c := body[i]
		switch {
		case c == ' ' || c == '\t' || c == '\n':
			flush()
			i++
		case c == '\'':
			inElem = true
			i++
			for i < len(body) && body[i] != '\'' {
				cur.WriteByte(body[i])
				i++
			}
			if i < len(body) {
				i++
			}
		case c == '"':
			inElem = true
			i++
			for i < len(body) && body[i] != '"' {
				if body[i] == '\\' && i+1 < len(body) {
					cur.WriteByte(body[i+1])
					i += 2
					continue
				}
				cur.WriteByte(body[i])
				i++
			}
			if i < len(body) {
				i++
			}
		case c == '\\':
			inElem = true
			if i+1 < len(body) {
				cur.WriteByte(body[i+1])
				i += 2
			} else {
				i++
			}
		default:
			inElem = true
			cur.WriteByte(c)
			i++
		}
	}
	flush()
	return elements, nil
}
