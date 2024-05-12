package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"gosh"
)

func main() {
	session := gosh.NewSession() // Initializes a new session
	fmt.Printf("Session started at %v by user %d (%s)\n", session.StartTime, session.UserID, session.UserName)

	reader := bufio.NewReader(os.Stdin)
	fmt.Println("Welcome to gosh Shell")

	for {
		fmt.Print("> ")
		input, err := reader.ReadString('\n')
		if err != nil {
			if err == bufio.ErrBufferFull {
				fmt.Println("Input buffer overflow. Please try again.")
				continue
			} else if err.Error() == "EOF" { // Handle EOF when Ctrl-D is pressed.
				fmt.Println("\nExit command received. Exiting gosh shell.")
				os.Exit(0)
			}
			fmt.Println("An error occurred:", err)
			continue
		}

		input = strings.TrimSpace(input)
		if input == "" {
			continue
		}

		command := gosh.NewCommand(input)
		command.Run()
	}
}
