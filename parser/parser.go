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
	{Name: "Background", Pattern: `&`}, // Add & for background execution
	{Name: "Redirect", Pattern: `>>|>|<`},
	{Name: "Quote", Pattern: `'[^']*'|"[^"]*"`},
	{Name: "Word", Pattern: `[^\s|><&'";]+`}, // Updated to exclude semicolons
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
	Commands   []*SimpleCommand `parser:"@@ ( '|' @@ )*"`
	Background bool             `parser:"@Background?"`
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

func ProcessCommand(cmd *SimpleCommand) (string, []string, string, string, string, string) {
	if len(cmd.Parts) == 0 {
		return "", []string{}, "", "", "", ""
	}

	command := cmd.Parts[0]

	// Handle arguments (all parts after the first one)
	args := []string{}
	if len(cmd.Parts) > 1 {
		args = cmd.Parts[1:]
	}
	var inputRedirectType, inputFilename, outputRedirectType, outputFilename string

	for _, redirect := range cmd.Redirects {
		// Process redirection
		if redirect.Type == "<" {
			inputRedirectType = redirect.Type
			inputFilename = redirect.File
		} else if redirect.Type == ">" || redirect.Type == ">>" {
			outputRedirectType = redirect.Type
			outputFilename = redirect.File
		}
	}

	return command, args, inputRedirectType, inputFilename, outputRedirectType, outputFilename
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

	// Format each command in the pipeline
	for j, simpleCmd := range pipeline.Commands {
		if j > 0 {
			result.WriteString(" | ")
		}
		result.WriteString(strings.Join(simpleCmd.Parts, " "))
		for _, redirect := range simpleCmd.Redirects {
			result.WriteString(" ")
			result.WriteString(redirect.Type)
			result.WriteString(" ")
			result.WriteString(redirect.File)
		}

		// Add & symbol if the individual command should run in the background
		// (This is for backward compatibility, but normally we'll use the pipeline background flag)
		if simpleCmd.Background {
			result.WriteString(" &")
		}
	}

	// Add & symbol if the entire pipeline should run in the background
	if pipeline.Background {
		result.WriteString(" &")
	}

	return result.String()
}
