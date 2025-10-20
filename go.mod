module gosh

go 1.24.0

toolchain go1.24.3

require (
	github.com/alecthomas/participle/v2 v2.1.1
	github.com/chzyer/readline v1.5.1
	github.com/google/uuid v1.6.0
	github.com/mattn/go-sqlite3 v1.14.22
	github.com/mmichie/m28 v0.0.0
	golang.org/x/term v0.36.0
)

require (
	github.com/shopspring/decimal v1.4.0 // indirect
	golang.org/x/sys v0.37.0 // indirect
)

replace github.com/mmichie/m28 => ../m28
