// Package gosh provides M28-powered record stream processing operators.
//
// These builtins enable Lisp-based stream processing in gosh pipelines:
//   - rmap: Transform each record using an M28 expression
//   - rfilter: Filter records using an M28 predicate
//   - rreduce: Reduce records to a single value using M28
//   - rtake: Take first N records
//   - rdrop: Drop first N records
package gosh

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/mmichie/m28/core"
)

func init() {
	// Register stream processing builtins
	builtins["rmap"] = rmapCommand
	builtins["rfilter"] = rfilterCommand
	builtins["rreduce"] = rreduceCommand
	builtins["rtake"] = rtakeCommand
	builtins["rdrop"] = rdropCommand
	builtins["reach"] = reachCommand
}

// recordToM28Value converts a gosh Record to an M28 DictValue
func recordToM28Value(record Record) *core.DictValue {
	dict := core.NewDict()
	for key, val := range record {
		dict.Set(key, goValueToM28(val))
	}
	return dict
}

// recordToMakeRecordExpr converts a gosh Record to a (make-record ...) expression string
// that can be evaluated by M28. This is needed because M28 doesn't support dict literal syntax.
func recordToMakeRecordExpr(record Record) string {
	if len(record) == 0 {
		return "(make-record)"
	}

	var parts []string
	for key, val := range record {
		parts = append(parts, fmt.Sprintf("%q", key))
		parts = append(parts, goValueToM28Expr(val))
	}
	return fmt.Sprintf("(make-record %s)", strings.Join(parts, " "))
}

// goValueToM28Expr converts a Go value to an M28 expression string
func goValueToM28Expr(v interface{}) string {
	switch val := v.(type) {
	case string:
		return fmt.Sprintf("%q", val)
	case float64:
		return fmt.Sprintf("%v", val)
	case int:
		return fmt.Sprintf("%d", val)
	case int64:
		return fmt.Sprintf("%d", val)
	case bool:
		if val {
			return "True"
		}
		return "False"
	case nil:
		return "nil"
	case map[string]interface{}:
		// Recursively convert nested records
		rec := make(Record)
		for k, v := range val {
			rec[k] = v
		}
		return recordToMakeRecordExpr(rec)
	case []interface{}:
		var items []string
		for _, item := range val {
			items = append(items, goValueToM28Expr(item))
		}
		return fmt.Sprintf("(list %s)", strings.Join(items, " "))
	default:
		return fmt.Sprintf("%q", fmt.Sprintf("%v", val))
	}
}

// goValueToM28 converts a Go value to an M28 Value
func goValueToM28(v interface{}) core.Value {
	switch val := v.(type) {
	case string:
		return core.StringValue(val)
	case float64:
		return core.NumberValue(val)
	case int:
		return core.NumberValue(float64(val))
	case int64:
		return core.NumberValue(float64(val))
	case bool:
		return core.BoolValue(val)
	case nil:
		return nil
	case map[string]interface{}:
		dict := core.NewDict()
		for k, v := range val {
			dict.Set(k, goValueToM28(v))
		}
		return dict
	case []interface{}:
		list := core.NewList()
		for _, item := range val {
			list.Append(goValueToM28(item))
		}
		return list
	default:
		return core.StringValue(fmt.Sprintf("%v", val))
	}
}

// m28ValueToGo converts an M28 Value to a Go value for JSON serialization
func m28ValueToGo(v core.Value) interface{} {
	if v == nil {
		return nil
	}

	switch val := v.(type) {
	case core.StringValue:
		return string(val)
	case core.NumberValue:
		return float64(val)
	case core.BoolValue:
		return bool(val)
	case *core.DictValue:
		result := make(map[string]interface{})
		for _, key := range val.Keys() {
			value, _ := val.Get(key)
			result[key] = m28ValueToGo(value)
		}
		return result
	case *core.ListValue:
		result := make([]interface{}, val.Len())
		for i := 0; i < val.Len(); i++ {
			item, _ := val.GetItem(i)
			result[i] = m28ValueToGo(item)
		}
		return result
	default:
		return v.String()
	}
}

// m28ValueToRecord converts an M28 DictValue to a gosh Record
func m28ValueToRecord(v core.Value) (Record, error) {
	dict, ok := v.(*core.DictValue)
	if !ok {
		return nil, fmt.Errorf("expected dict, got %T", v)
	}

	record := make(Record)
	for _, key := range dict.Keys() {
		value, _ := dict.Get(key)
		record[key] = m28ValueToGo(value)
	}
	return record, nil
}

// evaluateWithRecord evaluates an M28 expression with 'rec' bound to the record
// and returns the result as a string representation.
// Note: We use 'rec' (not '$rec') because M28 doesn't support $ in variable names.
func evaluateWithRecord(expr string, record Record) (string, error) {
	interpreter := GetM28Interpreter()

	// Build expression that binds 'rec' and evaluates the user's expression
	// Use recordToMakeRecordExpr because M28 doesn't support dict literal syntax
	wrappedExpr := fmt.Sprintf("((lambda (rec) %s) %s)", expr, recordToMakeRecordExpr(record))

	return interpreter.Execute(wrappedExpr)
}

// rmapCommand transforms each record using an M28 expression.
// Usage: rmap '(expression using rec)'
// Example: rmap '(set-field rec "doubled" (* (get-field rec "value") 2))'
// Note: Use 'rec' (not 'rec') to refer to the current record.
func rmapCommand(cmd *Command) error {
	args := getBuiltinArgs(cmd)

	if len(args) < 1 {
		return fmt.Errorf("rmap: requires an M28 expression\nUsage: rmap '(expression using rec)'")
	}

	expr := strings.Join(args, " ")

	// Read records from stdin
	records, err := readRecordsFromInput(cmd.Stdin)
	if err != nil {
		return fmt.Errorf("rmap: %v", err)
	}

	// Write magic header
	fmt.Fprint(cmd.Stdout, RecordMagic)

	interpreter := GetM28Interpreter()

	// Transform each record
	for _, record := range records {
		// Build expression that binds rec and evaluates the user's expression
		// Use recordToMakeRecordExpr because M28 doesn't support dict literal syntax
		wrappedExpr := fmt.Sprintf("((lambda (rec) %s) %s)", expr, recordToMakeRecordExpr(record))

		result, err := interpreter.Execute(wrappedExpr)
		if err != nil {
			fmt.Fprintf(cmd.Stderr, "rmap: error evaluating expression: %v\n", err)
			continue
		}

		// The result should be a dict - we need to parse it
		// interpreter.Execute returns the printed representation
		// We need to evaluate the result as a dict
		evalResult, err := interpreter.Execute(result)
		if err != nil {
			// If evaluation fails, try to use the result as-is
			evalResult = result
		}

		// Try to convert result to a record
		if evalResult == "" || evalResult == "nil" {
			continue
		}

		// Parse the result as JSON if it looks like JSON
		if strings.HasPrefix(evalResult, "{") {
			var parsedRecord Record
			if jsonErr := json.Unmarshal([]byte(evalResult), &parsedRecord); jsonErr == nil {
				data, _ := json.Marshal(parsedRecord)
				fmt.Fprintln(cmd.Stdout, string(data))
				continue
			}
		}

		// Try to evaluate it as M28 dict
		wrappedEval := fmt.Sprintf("((lambda (rec) %s) %s)", evalResult, recordToMakeRecordExpr(record))
		finalResult, _ := interpreter.Execute(wrappedEval)
		if strings.HasPrefix(finalResult, "{") {
			// It's a dict representation, output it
			fmt.Fprintln(cmd.Stdout, finalResult)
		} else {
			// Just output the original record for now
			data, _ := json.Marshal(record)
			fmt.Fprintln(cmd.Stdout, string(data))
		}
	}

	return nil
}

// rfilterCommand filters records using an M28 predicate.
// Usage: rfilter '(predicate using rec)'
// Example: rfilter '(> (get-field rec "age") 18)'
func rfilterCommand(cmd *Command) error {
	args := getBuiltinArgs(cmd)

	if len(args) < 1 {
		return fmt.Errorf("rfilter: requires an M28 predicate expression\nUsage: rfilter '(predicate using rec)'")
	}

	expr := strings.Join(args, " ")

	// Read records from stdin
	records, err := readRecordsFromInput(cmd.Stdin)
	if err != nil {
		return fmt.Errorf("rfilter: %v", err)
	}

	// Write magic header
	fmt.Fprint(cmd.Stdout, RecordMagic)

	interpreter := GetM28Interpreter()

	// Filter records
	for _, record := range records {
		// Build expression that binds rec and evaluates the predicate
		// Use recordToMakeRecordExpr because M28 doesn't support dict literal syntax
		wrappedExpr := fmt.Sprintf("((lambda (rec) %s) %s)", expr, recordToMakeRecordExpr(record))

		result, err := interpreter.Execute(wrappedExpr)
		if err != nil {
			fmt.Fprintf(cmd.Stderr, "rfilter: error evaluating predicate: %v\n", err)
			continue
		}

		// Check if result is truthy
		// In M28, True/False are capitalized, but also check lowercase variants
		result = strings.TrimSpace(result)
		isTruthy := result != "False" && result != "false" && result != "#f" && result != "nil" && result != ""

		if isTruthy {
			data, _ := json.Marshal(record)
			fmt.Fprintln(cmd.Stdout, string(data))
		}
	}

	return nil
}

// rreduceCommand reduces records to a single value using M28.
// Usage: rreduce initial-value '(expression using acc and rec)'
// Example: rreduce 0 '(+ acc (get-field rec "amount"))'
func rreduceCommand(cmd *Command) error {
	args := getBuiltinArgs(cmd)

	if len(args) < 2 {
		return fmt.Errorf("rreduce: requires initial value and expression\nUsage: rreduce initial-value '(expression using acc and rec)'")
	}

	initialValue := args[0]
	expr := strings.Join(args[1:], " ")

	// Read records from stdin
	records, err := readRecordsFromInput(cmd.Stdin)
	if err != nil {
		return fmt.Errorf("rreduce: %v", err)
	}

	interpreter := GetM28Interpreter()
	accumulator := initialValue

	// Reduce records
	for _, record := range records {
		// Build expression using lambda to bind acc and rec
		// M28 doesn't support let, so we use ((lambda (acc rec) expr) acc_val rec_val)
		// Use recordToMakeRecordExpr because M28 doesn't support dict literal syntax
		wrappedExpr := fmt.Sprintf("((lambda (acc rec) %s) %s %s)",
			expr, accumulator, recordToMakeRecordExpr(record))

		result, err := interpreter.Execute(wrappedExpr)
		if err != nil {
			return fmt.Errorf("rreduce: error evaluating expression: %v", err)
		}

		accumulator = result
	}

	// Output the final result
	fmt.Fprintln(cmd.Stdout, accumulator)

	return nil
}

// rtakeCommand takes the first N records.
// Usage: rtake N
// Example: rtake 10
func rtakeCommand(cmd *Command) error {
	args := getBuiltinArgs(cmd)

	if len(args) != 1 {
		return fmt.Errorf("rtake: requires exactly one argument\nUsage: rtake N")
	}

	n, err := strconv.Atoi(args[0])
	if err != nil {
		return fmt.Errorf("rtake: invalid number: %s", args[0])
	}

	if n < 0 {
		return fmt.Errorf("rtake: N must be non-negative")
	}

	// Read records from stdin
	records, err := readRecordsFromInput(cmd.Stdin)
	if err != nil {
		return fmt.Errorf("rtake: %v", err)
	}

	// Write magic header
	fmt.Fprint(cmd.Stdout, RecordMagic)

	// Take first N records
	count := 0
	for _, record := range records {
		if count >= n {
			break
		}
		data, _ := json.Marshal(record)
		fmt.Fprintln(cmd.Stdout, string(data))
		count++
	}

	return nil
}

// rdropCommand drops the first N records.
// Usage: rdrop N
// Example: rdrop 5
func rdropCommand(cmd *Command) error {
	args := getBuiltinArgs(cmd)

	if len(args) != 1 {
		return fmt.Errorf("rdrop: requires exactly one argument\nUsage: rdrop N")
	}

	n, err := strconv.Atoi(args[0])
	if err != nil {
		return fmt.Errorf("rdrop: invalid number: %s", args[0])
	}

	if n < 0 {
		return fmt.Errorf("rdrop: N must be non-negative")
	}

	// Read records from stdin
	records, err := readRecordsFromInput(cmd.Stdin)
	if err != nil {
		return fmt.Errorf("rdrop: %v", err)
	}

	// Write magic header
	fmt.Fprint(cmd.Stdout, RecordMagic)

	// Drop first N records
	count := 0
	for _, record := range records {
		count++
		if count <= n {
			continue
		}
		data, _ := json.Marshal(record)
		fmt.Fprintln(cmd.Stdout, string(data))
	}

	return nil
}

// reachCommand executes an M28 expression for each record (like forEach).
// Unlike rmap, this doesn't transform records - it's for side effects.
// Usage: reach '(expression using rec)'
// Example: reach '(print (get-field rec "name"))'
func reachCommand(cmd *Command) error {
	args := getBuiltinArgs(cmd)

	if len(args) < 1 {
		return fmt.Errorf("reach: requires an M28 expression\nUsage: reach '(expression using rec)'")
	}

	expr := strings.Join(args, " ")

	// Read records from stdin
	records, err := readRecordsFromInput(cmd.Stdin)
	if err != nil {
		return fmt.Errorf("reach: %v", err)
	}

	interpreter := GetM28Interpreter()

	// Execute expression for each record
	for _, record := range records {
		// Build expression that binds rec
		// Use recordToMakeRecordExpr because M28 doesn't support dict literal syntax
		wrappedExpr := fmt.Sprintf("((lambda (rec) %s) %s)", expr, recordToMakeRecordExpr(record))

		result, err := interpreter.Execute(wrappedExpr)
		if err != nil {
			fmt.Fprintf(cmd.Stderr, "reach: error evaluating expression: %v\n", err)
			continue
		}

		// Print non-nil results
		result = strings.TrimSpace(result)
		if result != "" && result != "nil" {
			fmt.Fprintln(cmd.Stdout, result)
		}
	}

	return nil
}
