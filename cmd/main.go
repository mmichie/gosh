package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"gosh" // Assuming your module name is gosh; adjust based on your module declaration in go.mod
)

func main() {
	session := gosh.NewSession() // Initializes a new session

	// Example use: print session start time or user information
	fmt.Printf("Session started at %v by user %d (%s)\n", session.StartTime, session.UserID, session.UserName)

	reader := bufio.NewReader(os.Stdin)

	fmt.Println("Welcome to gosh Shell")
	for {
		fmt.Print("> ")
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)
		if input == "" {
			continue
		}

		command := gosh.NewCommand(input) // Same assumption as above
		command.Run()
	}
}
