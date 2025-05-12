# Parser Updates for OR Operator Support

The implementation of the OR operator requires significant changes to the parser structure. The field `AndCommands` has been replaced with `LogicalBlocks` to support both `&&` and `||` operators.

## Changes Required:

1. In `/Users/mim/src/gosh/builtins.go`: Replace all instances of `cmd.AndCommands` with `cmd.Command.LogicalBlocks`.

2. In `/Users/mim/src/gosh/parser/parser_test.go`: Update the test cases to use the new structure instead of `AndCommands`.

3. In other files: Any other references to `AndCommands` also need to be updated.

## Command Structure Update:

```go
// Old structure
type Command struct {
    AndCommands []*AndCommand `parser:"@@+"`
}

type AndCommand struct {
    Pipelines []*Pipeline `parser:"@@ ( '&&' @@ )*"`
}

// New structure
type Command struct {
    LogicalBlocks []*LogicalBlock `parser:"@@+"`
}

type LogicalBlock struct {
    FirstPipeline *Pipeline     `parser:"@@"`
    RestPipelines []*OpPipeline `parser:"@@*"`
}

type OpPipeline struct {
    Operator string    `parser:"@('&&' | '||')"`
    Pipeline *Pipeline `parser:"@@"`
}
```

## Update Strategy:

1. We need to modify the builtins.go file to handle the new structure.
2. We need to update parser tests to reflect the new structure.
3. The command.go implementation already has the updated Run method.

Note: This is a significant change that affects multiple files in the codebase.