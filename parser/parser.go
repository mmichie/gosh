package parser

import (
	"fmt"
	"log"

	"github.com/alecthomas/participle/v2"
	"github.com/alecthomas/participle/v2/lexer"
)

// Lexer rules enhanced for additional elements.
var shellLexer = lexer.MustSimple([]lexer.SimpleRule{
	{Name: "Ident", Pattern: `[a-zA-Z_][a-zA-Z0-9_]*`},
	{Name: "Word", Pattern: `[^\s><|&;]+`},
	{Name: "String", Pattern: `"(\\"|[^"])*"`},
	{Name: "Redirect", Pattern: `[><]`},
	{Name: "Pipe", Pattern: `\|`},
	{Name: "Whitespace", Pattern: `\s+`},
})

// Types for various parts of the shell grammar.
type Word struct {
	Value string `parser:"@(Ident|Word|String)"`
}

type Command struct {
	SimpleCommand   *SimpleCommand   `parser:"@@"`
	PipelineCommand *PipelineCommand `parser:"| @@"`
	ForLoop         *ForLoop         `parser:"| @@"`
	IfCondition     *IfCondition     `parser:"| @@"`
}

type SimpleCommand struct {
	Command     []*Word      `parser:"@@+"`
	Redirection *Redirection `parser:"@@?"`
}

type Redirection struct {
	Type     string `parser:"@Redirect"`
	Filename *Word  `parser:"@@"`
}

type PipelineCommand struct {
	Commands []*SimpleCommand `parser:"@@ ( '|' @@ )*"`
}

type ForLoop struct {
	Variable string       `parser:"'for' @Ident"`
	DoBlock  *CommandList `parser:"'do' @@ 'done'"`
}

type IfCondition struct {
	Condition *CommandList `parser:"'if' @@"`
	ThenBlock *CommandList `parser:"'then' @@"`
	ElseBlock *CommandList `parser:"'else' @@ 'fi'"`
}

type CaseStatement struct {
	Expression string      `parser:"'case' @Word 'in'"`
	Cases      []*CasePart `parser:"@@* 'esac'"`
}

type CasePart struct {
	Pattern     string       `parser:"@Word"`
	CommandList *CommandList `parser:"')' @@ ';;'"`
}

type CommandList struct {
	Commands []*Command `parser:"@@ (';' @@)*"`
}

var parser = participle.MustBuild[Command](
	participle.Lexer(shellLexer),
	participle.Unquote("String"),
	participle.Elide("Whitespace"),
)

// Parse function to handle complex command structures.
func Parse(input string) (*Command, error) {
	log.Printf("Parsing input: %s", input)
	command, err := parser.ParseString("", input)
	if err != nil {
		log.Printf("Failed to parse command string: %s, error: %v", input, err)
		return nil, fmt.Errorf("parse error: %v", err)
	}
	log.Printf("Parsed command: %+v", command)
	if command.SimpleCommand != nil {
		log.Printf("Simple command: %+v", command.SimpleCommand.Command)
		log.Printf("Simple command redirection: %+v", command.SimpleCommand.Redirection)
	}
	return command, nil
}
