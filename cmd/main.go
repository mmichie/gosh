package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"gosh"
)

func main() {
	log.SetFlags(0)
	log.SetPrefix("")

	log.Printf("Session started at %s by user %d (%s)", time.Now(), os.Geteuid(), os.Getenv("USER"))

	fmt.Println("Welcome to gosh Shell")
	fmt.Print("> ")

	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		input := scanner.Text()
		input = strings.TrimSpace(input)

		if input == "exit" || input == "quit" {
			fmt.Println("Exiting gosh Shell...")
			break
		}

		if input == "" {
			fmt.Print("> ")
			continue
		}

		command, err := gosh.NewCommand(input)
		if err != nil {
			log.Printf("Error creating command: %v", err)
			fmt.Print("> ")
			continue
		}

		command.Stdin = os.Stdin
		command.Stdout = os.Stdout
		command.Run()

		fmt.Print("> ")
	}

	if err := scanner.Err(); err != nil {
		log.Printf("Error reading input: %v", err)
	}
}
