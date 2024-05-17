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
	{Name: "Word", Pattern: `\S+`},
	{Name: "String", Pattern: `"(\\\\"|[^"])*"`},
	{Name: "Operator", Pattern: `[<>|&;]+`},
	{Name: "Whitespace", Pattern: `\s+`},
})

// Types for various parts of the shell grammar.
type Word struct {
	Value string `parser:"@Word"`
}

type Command struct {
	SimpleCommand   *SimpleCommand   `parser:"@@?"`
	PipelineCommand *PipelineCommand `parser:"| @@"`
	ForLoop         *ForLoop         `parser:"| @@"`
	IfCondition     *IfCondition     `parser:"| @@"`
}

type SimpleCommand struct {
	Items []*Word `parser:"@@+"`
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
	participle.Unquote(),
	participle.Elide("Whitespace"),
)

// Parsing function to handle complex command structures.
func Parse(input string) (*Command, error) {
	log.Printf("Parsing input: %s", input)
	command := &Command{}
	command, err := parser.ParseString("", input)
	if err != nil {
		log.Printf("Failed to parse command string: %s, error: %v", input, err)
		return nil, fmt.Errorf("parse error: %v", err)
	}
	log.Printf("Parsed command: %+v", command)
	return command, nil
}
