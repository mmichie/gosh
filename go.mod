module gosh

go 1.23.0

toolchain go1.24.3

require (
	github.com/alecthomas/participle/v2 v2.1.1
	github.com/chzyer/readline v1.5.1
	github.com/google/uuid v1.6.0
	github.com/mattn/go-sqlite3 v1.14.22
	github.com/mmichie/m28 v0.0.0
)

require golang.org/x/sys v0.33.0 // indirect

replace github.com/mmichie/m28 => ../m28
