package gosh

import (
	"fmt"
	"strconv"
)

// shiftCommand implements the shift builtin command
// Shifts positional parameters to the left by n positions (default 1)
func shiftCommand(cmd *Command) error {
	args := extractCommandArgs(cmd, "shift")

	n := 1 // Default shift by 1
	if len(args) > 0 {
		var err error
		n, err = strconv.Atoi(args[0])
		if err != nil {
			fmt.Fprintf(cmd.Stderr, "shift: %s: numeric argument required\n", args[0])
			cmd.ReturnCode = 1
			return nil // Don't return error, just set return code
		}
	}

	state := GetGlobalState()
	err := state.ShiftPositionalParams(n)
	if err != nil {
		fmt.Fprintf(cmd.Stderr, "shift: %v\n", err)
		cmd.ReturnCode = 1
		return nil // Don't return error, just set return code
	}

	cmd.ReturnCode = 0
	return nil
}
