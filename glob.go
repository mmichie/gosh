package gosh

import (
	"path/filepath"
	"strings"
)

func ExpandWildcards(args []string) []string {
	var expandedArgs []string

	for _, arg := range args {
		if strings.ContainsAny(arg, "*?") {
			matches, err := filepath.Glob(arg)
			if err != nil || len(matches) == 0 {
				// If there's an error or no matches, use the original argument
				expandedArgs = append(expandedArgs, arg)
			} else {
				expandedArgs = append(expandedArgs, matches...)
			}
		} else {
			expandedArgs = append(expandedArgs, arg)
		}
	}

	return expandedArgs
}
