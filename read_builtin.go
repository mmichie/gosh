package gosh

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"golang.org/x/term"
)

// readCommand implements the read builtin command
// Reads a line from stdin and assigns it to variables
func readCommand(cmd *Command) error {
	// Parse arguments
	args := extractCommandArgs(cmd, "read")

	// Parse flags and variable names
	var prompt string
	var silent bool
	var charCount int
	var timeout time.Duration
	var variables []string

	i := 0
	for i < len(args) {
		arg := args[i]

		if !strings.HasPrefix(arg, "-") {
			// This is a variable name
			variables = append(variables, args[i:]...)
			break
		}

		// Parse flags
		if arg == "-p" {
			// Prompt
			i++
			if i >= len(args) {
				return fmt.Errorf("read: -p requires an argument")
			}
			prompt = args[i]
			i++
		} else if arg == "-s" {
			// Silent mode (don't echo input)
			silent = true
			i++
		} else if arg == "-n" {
			// Read exactly N characters
			i++
			if i >= len(args) {
				return fmt.Errorf("read: -n requires an argument")
			}
			var err error
			_, err = fmt.Sscanf(args[i], "%d", &charCount)
			if err != nil || charCount < 1 {
				return fmt.Errorf("read: -n: invalid count: %s", args[i])
			}
			i++
		} else if arg == "-t" {
			// Timeout in seconds
			i++
			if i >= len(args) {
				return fmt.Errorf("read: -t requires an argument")
			}
			var seconds float64
			_, err := fmt.Sscanf(args[i], "%f", &seconds)
			if err != nil || seconds < 0 {
				return fmt.Errorf("read: -t: invalid timeout: %s", args[i])
			}
			timeout = time.Duration(seconds * float64(time.Second))
			i++
		} else {
			return fmt.Errorf("read: invalid option: %s", arg)
		}
	}

	// Default variable name is REPLY if none specified
	if len(variables) == 0 {
		variables = []string{"REPLY"}
	}

	// Display prompt if specified
	if prompt != "" {
		fmt.Fprint(cmd.Stdout, prompt)
	}

	// Read input
	var input string
	var err error

	if silent {
		// Silent mode - disable echo
		input, err = readSilent(cmd.Stdin, charCount, timeout)
		if prompt != "" {
			// Print newline after silent input
			fmt.Fprintln(cmd.Stdout)
		}
	} else if charCount > 0 {
		// Read specific number of characters
		input, err = readNChars(cmd.Stdin, charCount, timeout)
	} else if timeout > 0 {
		// Read with timeout
		input, err = readWithTimeout(cmd.Stdin, timeout)
	} else {
		// Normal line reading
		input, err = readLine(cmd.Stdin)
	}

	if err != nil {
		if err == io.EOF {
			// EOF is not an error for read, just return non-zero
			cmd.ReturnCode = 1
			return nil
		}
		return err
	}

	// Split input into fields based on IFS (default: space, tab, newline)
	ifs := os.Getenv("IFS")
	if ifs == "" {
		ifs = " \t\n"
	}

	// Split the input
	fields := splitByIFS(input, ifs)

	// Assign fields to variables
	for i, varName := range variables {
		var value string
		if i < len(fields) {
			if i == len(variables)-1 {
				// Last variable gets all remaining fields
				value = strings.Join(fields[i:], " ")
			} else {
				value = fields[i]
			}
		}
		// Set environment variable
		os.Setenv(varName, value)
	}

	cmd.ReturnCode = 0
	return nil
}

// extractCommandArgs extracts arguments from a Command, skipping the command name
func extractCommandArgs(cmd *Command, cmdName string) []string {
	if len(cmd.Command.LogicalBlocks) == 0 ||
		cmd.Command.LogicalBlocks[0].FirstPipeline == nil ||
		len(cmd.Command.LogicalBlocks[0].FirstPipeline.Commands) == 0 {
		return []string{}
	}

	parts := getCommandParts(cmd.Command.LogicalBlocks[0].FirstPipeline.Commands[0])
	if len(parts) <= 1 {
		return []string{}
	}

	// Skip the command name
	return parts[1:]
}

// readLine reads a complete line from the reader
func readLine(r io.Reader) (string, error) {
	reader := bufio.NewReader(r)
	line, err := reader.ReadString('\n')
	if err != nil {
		if err == io.EOF {
			// If we got EOF but have some data, that's ok
			if line != "" {
				return line, nil
			}
			// Immediate EOF with no data
			return "", io.EOF
		}
		return "", err
	}
	// Remove trailing newline
	return strings.TrimSuffix(line, "\n"), nil
}

// readNChars reads exactly N characters (or until EOF/timeout)
func readNChars(r io.Reader, n int, timeout time.Duration) (string, error) {
	buf := make([]byte, n)

	if timeout > 0 {
		// Use channel for timeout
		type result struct {
			n   int
			err error
		}
		ch := make(chan result, 1)

		go func() {
			bytesRead, err := io.ReadFull(r, buf)
			ch <- result{bytesRead, err}
		}()

		select {
		case res := <-ch:
			if res.err != nil && res.err != io.EOF && res.err != io.ErrUnexpectedEOF {
				return "", res.err
			}
			return string(buf[:res.n]), nil
		case <-time.After(timeout):
			return "", fmt.Errorf("read: timeout")
		}
	}

	bytesRead, err := io.ReadFull(r, buf)
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		return "", err
	}
	return string(buf[:bytesRead]), nil
}

// readWithTimeout reads a line with timeout
func readWithTimeout(r io.Reader, timeout time.Duration) (string, error) {
	type result struct {
		line string
		err  error
	}
	ch := make(chan result, 1)

	go func() {
		line, err := readLine(r)
		ch <- result{line, err}
	}()

	select {
	case res := <-ch:
		return res.line, res.err
	case <-time.After(timeout):
		return "", fmt.Errorf("read: timeout")
	}
}

// readSilent reads input without echoing (for passwords)
func readSilent(r io.Reader, charCount int, timeout time.Duration) (string, error) {
	// Check if stdin is a terminal
	fd := int(os.Stdin.Fd())
	if !term.IsTerminal(fd) {
		// Not a terminal, just read normally
		if charCount > 0 {
			return readNChars(r, charCount, timeout)
		}
		if timeout > 0 {
			return readWithTimeout(r, timeout)
		}
		return readLine(r)
	}

	// Save terminal state
	oldState, err := term.GetState(fd)
	if err != nil {
		return "", err
	}
	defer term.Restore(fd, oldState)

	// Read password
	var password []byte
	if charCount > 0 {
		// Read N characters
		password = make([]byte, charCount)
		bytesRead, err := io.ReadFull(os.Stdin, password)
		if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
			return "", err
		}
		return string(password[:bytesRead]), nil
	}

	// Read line
	if timeout > 0 {
		type result struct {
			password []byte
			err      error
		}
		ch := make(chan result, 1)

		go func() {
			pw, err := term.ReadPassword(fd)
			ch <- result{pw, err}
		}()

		select {
		case res := <-ch:
			if res.err != nil {
				return "", res.err
			}
			return string(res.password), nil
		case <-time.After(timeout):
			return "", fmt.Errorf("read: timeout")
		}
	}

	password, err = term.ReadPassword(fd)
	if err != nil {
		return "", err
	}

	return string(password), nil
}

// splitByIFS splits a string by IFS characters
func splitByIFS(s string, ifs string) []string {
	if ifs == "" {
		// If IFS is empty, don't split
		return []string{s}
	}

	// Split by any character in IFS
	var fields []string
	var current strings.Builder

	for _, char := range s {
		if strings.ContainsRune(ifs, char) {
			if current.Len() > 0 {
				fields = append(fields, current.String())
				current.Reset()
			}
		} else {
			current.WriteRune(char)
		}
	}

	// Add remaining field
	if current.Len() > 0 {
		fields = append(fields, current.String())
	}

	// If no fields, return array with empty string
	if len(fields) == 0 {
		fields = []string{""}
	}

	return fields
}
