default-targets := testall test format compile fmt
special-targets := install clean log run coverage test-background
dir-basename := $(notdir $(PWD))

.PHONY: $(default-targets) $(special-targets) all

all: install run

run: compile
	@echo "Running the application..."
	@./bin/$(dir-basename)

testall: test coverage

test:
	@echo "Running tests..."
	go test ./...

coverage:
	@echo "Running tests with coverage..."
	@mkdir -p reports
	@go test ./... -coverprofile=reports/coverage.out
	@go tool cover -html=reports/coverage.out -o reports/coverage.html
	@echo "Coverage report generated: reports/coverage.html"

format:
	@echo "Formatting Go files..."
	go fmt ./...

fmt: format

compile: format
	@echo "Compiling the application..."
	@mkdir -p ./bin
	go build -o ./bin/$(dir-basename) cmd/main.go

clean:
	@echo "Cleaning up..."
	rm -rf ./reports
	rm ./bin/$(dir-basename)

test-background: compile
	@echo "Running background pipeline tests..."
	@./test_background.sh

-include local.mk
