package parser

import (
	"fmt"
	"log"

	"github.com/alecthomas/participle/v2"
	"github.com/alecthomas/participle/v2/lexer"
)

var shellLexer = lexer.MustSimple([]lexer.SimpleRule{
	{Name: "Ident", Pattern: `[a-zA-Z_][a-zA-Z0-9_]*`},
	{Name: "Path", Pattern: `/?[\w./-]+`},
	{Name: "Option", Pattern: `-\w+`},
	{Name: "String", Pattern: `"(\\\\"|[^"])*"`},
	{Name: "SingleQuotedString", Pattern: `'(\\\\'|[^'])*'`},
	{Name: "Operator", Pattern: `[<>|&;]+`},
	{Name: "Whitespace", Pattern: `\s+`},
})

type SimpleCommandElement struct {
	Word string `parser:"@Ident | @Path | @Option | @String"`
}

type SimpleCommand struct {
	Command  string                  `parser:"@Ident"`
	Options  []string                `parser:"(@Whitespace @Option)*"`
	Args     []string                `parser:"(@Whitespace (@Ident | @Path | @String))*"`
	Elements []*SimpleCommandElement `parser:"@@*"`
}

type ShellCommand struct {
	SimpleCommand *SimpleCommand `parser:"@@"`
}

type PipelineCommand struct {
	Bang         bool            `parser:"@'!'?"`
	Timespec     *Timespec       `parser:"@@?"`
	PipeCommands []*ShellCommand `parser:"@@ ( '|' @@ )*"`
}

type Timespec struct {
	Opt string `parser:"'time' @Ident?"`
}

type Command struct {
	SimpleCommand   *SimpleCommand   `parser:"@@"`
	PipelineCommand *PipelineCommand `parser:"| @@"`
}

var parser = participle.MustBuild[Command](
	participle.Lexer(shellLexer),
	participle.Unquote(),
)

func Parse(input string) (*Command, error) {
	command, err := parser.ParseString("", input)
	if err != nil {
		log.Printf("Failed to parse command string: %s, error: %v", input, err)
		return nil, fmt.Errorf("parse error: %v", err)
	}
	log.Printf("Parsed command: %+v", command)
	return command, nil
}
