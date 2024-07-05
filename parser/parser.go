package parser

import (
	"fmt"
	"log"
	"strings"

	"github.com/alecthomas/participle/v2"
	"github.com/alecthomas/participle/v2/lexer"
)

// Lexer rules enhanced for additional elements.
var shellLexer = lexer.MustSimple([]lexer.SimpleRule{
	{Name: "Word", Pattern: `[^\s|><]+`},
	{Name: "Pipe", Pattern: `\|`},
	{Name: "Redirect", Pattern: `>>|>`},
	{Name: "Whitespace", Pattern: `\s+`},
})

// Types for various parts of the shell grammar.
type Command struct {
	Pipelines []*Pipeline `parser:"@@+"`
}

type Pipeline struct {
	Commands []*SimpleCommand `parser:"@@ ( '|' @@ )*"`
}

type SimpleCommand struct {
	Parts []string `parser:"@Word+ ( @Redirect @Word )?"`
}

var parser = participle.MustBuild[Command](
	participle.Lexer(shellLexer),
	participle.Elide("Whitespace"),
)

// Parse function to handle command structures.
func Parse(input string) (*Command, error) {
	log.Printf("Parsing input: %s", input)
	command, err := parser.ParseString("", input)
	if err != nil {
		log.Printf("Failed to parse command string: %s, error: %v", input, err)
		return nil, fmt.Errorf("parse error: %v", err)
	}
	log.Printf("Parsed command: %+v", command)
	return command, nil
}

// ProcessCommand processes the parsed command and separates the command, arguments, and redirection.
func ProcessCommand(cmd *SimpleCommand) (string, []string, string, string) {
	if len(cmd.Parts) == 0 {
		return "", nil, "", ""
	}

	command := cmd.Parts[0]
	args := []string{}
	redirectType := ""
	filename := ""

	for i := 1; i < len(cmd.Parts); i++ {
		if cmd.Parts[i] == ">" || cmd.Parts[i] == ">>" {
			redirectType = cmd.Parts[i]
			if i+1 < len(cmd.Parts) {
				filename = cmd.Parts[i+1]
			}
			break
		}
		args = append(args, cmd.Parts[i])
	}

	return command, args, redirectType, filename
}

// FormatCommand formats a Command back into a string representation
func FormatCommand(cmd *Command) string {
	var result strings.Builder
	for i, pipeline := range cmd.Pipelines {
		if i > 0 {
			result.WriteString(" | ")
		}
		for j, simpleCmd := range pipeline.Commands {
			if j > 0 {
				result.WriteString(" | ")
			}
			result.WriteString(strings.Join(simpleCmd.Parts, " "))
		}
	}
	return result.String()
}
