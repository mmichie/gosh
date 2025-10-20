package parser

import (
	"fmt"
	"log"
	"strings"

	"github.com/alecthomas/participle/v2"
	"github.com/alecthomas/participle/v2/lexer"
)

var shellLexer = lexer.MustSimple([]lexer.SimpleRule{
	{Name: "Whitespace", Pattern: `\s+`},
	{Name: "Semicolon", Pattern: `;`}, // Add semicolon pattern for command separation
	{Name: "Or", Pattern: `\|\|`},     // Add OR operator pattern before Pipe
	{Name: "Pipe", Pattern: `\|`},
	{Name: "And", Pattern: `&&`},
	{Name: "LParen", Pattern: `\(`},                         // Left parenthesis for subshells
	{Name: "RParen", Pattern: `\)`},                         // Right parenthesis for subshells
	{Name: "LBrace", Pattern: `\{`},                         // Left brace for command grouping
	{Name: "RBrace", Pattern: `\}`},                         // Right brace for command grouping
	{Name: "Redirect", Pattern: `2>&1|&>|>&|2>>|2>|>>|>|<`}, // Support advanced redirection (order matters!)
	{Name: "Background", Pattern: `&`},                      // Add & for background execution (after redirect)
	{Name: "Quote", Pattern: `'[^']*'|"[^"]*"`},
	{Name: "Word", Pattern: `[^\s|><&'";(){}]+`}, // Updated to exclude parentheses and braces
})

type Command struct {
	LogicalBlocks []*LogicalBlock `parser:"@@ ( ';' @@ )*"`
}

type LogicalBlock struct {
	FirstPipeline *Pipeline     `parser:"@@"`
	RestPipelines []*OpPipeline `parser:"@@*"`
}

type OpPipeline struct {
	Operator string    `parser:"@('&&' | '||')"` // Store the operator (either && or ||)
	Pipeline *Pipeline `parser:"@@"`
}

type Pipeline struct {
	Commands   []*CommandElement `parser:"@@ ( '|' @@ )*"`
	Background bool              `parser:"@Background?"`
}

// CommandElement represents either a subshell, command group, or simple command
type CommandElement struct {
	Subshell     *Subshell     `parser:"@@"`
	CommandGroup *CommandGroup `parser:"| @@"`
	Simple       *SimpleCommand `parser:"| @@"`
}

// Subshell represents commands executed in a subshell: ( commands )
type Subshell struct {
	Command    *Command    `parser:"'(' @@ ')'"`
	Redirects  []*Redirect `parser:"@@*"`
	Background bool        `parser:"@Background?"`
}

// CommandGroup represents commands grouped without subshell: { commands; }
type CommandGroup struct {
	Command    *Command    `parser:"'{' @@ '}'"`
	Redirects  []*Redirect `parser:"@@*"`
	Background bool        `parser:"@Background?"`
}

type SimpleCommand struct {
	Parts      []string    `parser:"@(Word | Quote)+"`
	Redirects  []*Redirect `parser:"@@*"`
	Background bool        `parser:"@Background?"`
}

type Redirect struct {
	Type string `parser:"@Redirect"`
	File string `parser:"@Word"`
}

var parser = participle.MustBuild[Command](
	participle.Lexer(shellLexer),
	participle.Elide("Whitespace"),
	participle.UseLookahead(2), // Increase lookahead to better handle & at the end of pipelines
)

func Parse(input string) (*Command, error) {
	// Handle empty input
	if strings.TrimSpace(input) == "" {
		return nil, fmt.Errorf("empty input")
	}

	// Preprocess input to handle trailing semicolons by removing them
	cleanInput := strings.TrimSpace(input)
	if strings.HasSuffix(cleanInput, ";") {
		cleanInput = strings.TrimSuffix(cleanInput, ";")
		cleanInput = strings.TrimSpace(cleanInput)

		// If after removing trailing semicolon, we have empty string, it's an error
		if cleanInput == "" {
			return nil, fmt.Errorf("no valid commands found")
		}
	}

	// Parse the cleaned input
	command, err := parser.ParseString("", cleanInput)
	if err != nil {
		log.Printf("Failed to parse command string: %s, error: %v", cleanInput, err)
		return nil, fmt.Errorf("parse error: %v", err)
	}

	if len(command.LogicalBlocks) == 0 {
		return nil, fmt.Errorf("no valid commands found")
	}

	return command, nil
}

func ProcessCommand(cmd *SimpleCommand) (string, []string, string, string, string, string, string, string, string) {
	if len(cmd.Parts) == 0 {
		return "", []string{}, "", "", "", "", "", "", ""
	}

	command := cmd.Parts[0]

	// Handle arguments (all parts after the first one)
	args := []string{}
	if len(cmd.Parts) > 1 {
		args = cmd.Parts[1:]
	}
	var inputRedirectType, inputFilename string
	var outputRedirectType, outputFilename string
	var stderrRedirectType, stderrFilename string
	var fdDupType string

	for _, redirect := range cmd.Redirects {
		// Process redirection
		if redirect.Type == "<" {
			// Input redirection
			inputRedirectType = redirect.Type
			inputFilename = redirect.File
		} else if redirect.Type == ">" || redirect.Type == ">>" {
			// Standard output redirection
			outputRedirectType = redirect.Type
			outputFilename = redirect.File
		} else if redirect.Type == "2>" || redirect.Type == "2>>" {
			// Standard error redirection
			stderrRedirectType = redirect.Type
			stderrFilename = redirect.File
		} else if redirect.Type == "&>" || redirect.Type == ">&" {
			// Combined output redirection (both stdout and stderr)
			outputRedirectType = redirect.Type
			outputFilename = redirect.File
			stderrRedirectType = redirect.Type
			stderrFilename = redirect.File
		} else if redirect.Type == "2>&1" {
			// File descriptor duplication (stderr to stdout)
			fdDupType = redirect.Type
		}
	}

	return command, args, inputRedirectType, inputFilename, outputRedirectType, outputFilename, stderrRedirectType, stderrFilename, fdDupType
}

func FormatCommand(cmd *Command) string {
	var result strings.Builder
	for i, block := range cmd.LogicalBlocks {
		if i > 0 {
			result.WriteString(" ; ") // Blocks are separated by semicolons
		}

		// Format the first pipeline
		result.WriteString(formatPipeline(block.FirstPipeline))

		// Format the rest of the pipelines with their operators
		for _, opPipeline := range block.RestPipelines {
			result.WriteString(" ")
			result.WriteString(opPipeline.Operator) // && or ||
			result.WriteString(" ")
			result.WriteString(formatPipeline(opPipeline.Pipeline))
		}
	}
	return result.String()
}

func formatPipeline(pipeline *Pipeline) string {
	var result strings.Builder

	// Format each command element in the pipeline
	for j, cmdElem := range pipeline.Commands {
		if j > 0 {
			result.WriteString(" | ")
		}
		result.WriteString(formatCommandElement(cmdElem))
	}

	// Add & symbol if the entire pipeline should run in the background
	if pipeline.Background {
		result.WriteString(" &")
	}

	return result.String()
}

func formatCommandElement(elem *CommandElement) string {
	var result strings.Builder

	if elem.Subshell != nil {
		result.WriteString("( ")
		result.WriteString(FormatCommand(elem.Subshell.Command))
		result.WriteString(" )")
		for _, redirect := range elem.Subshell.Redirects {
			result.WriteString(" ")
			result.WriteString(redirect.Type)
			result.WriteString(" ")
			result.WriteString(redirect.File)
		}
		if elem.Subshell.Background {
			result.WriteString(" &")
		}
	} else if elem.CommandGroup != nil {
		result.WriteString("{ ")
		result.WriteString(FormatCommand(elem.CommandGroup.Command))
		result.WriteString(" }")
		for _, redirect := range elem.CommandGroup.Redirects {
			result.WriteString(" ")
			result.WriteString(redirect.Type)
			result.WriteString(" ")
			result.WriteString(redirect.File)
		}
		if elem.CommandGroup.Background {
			result.WriteString(" &")
		}
	} else if elem.Simple != nil {
		result.WriteString(strings.Join(elem.Simple.Parts, " "))
		for _, redirect := range elem.Simple.Redirects {
			result.WriteString(" ")
			result.WriteString(redirect.Type)
			result.WriteString(" ")
			result.WriteString(redirect.File)
		}
		if elem.Simple.Background {
			result.WriteString(" &")
		}
	}

	return result.String()
}

// SplitCommand splits a command string into parts (command and arguments)
func SplitCommand(cmdString string) []string {
	// Simple tokenization - this doesn't handle quotes or escapes properly
	// but is good enough for extracting the base command name
	return strings.Fields(cmdString)
}
