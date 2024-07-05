package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"

	"gosh"

	"github.com/chzyer/readline"
)

func main() {
	log.SetFlags(0)
	log.SetPrefix("")

	log.Printf("Session started at %s by user %d (%s)", time.Now(), os.Geteuid(), os.Getenv("USER"))

	fmt.Println("Welcome to gosh Shell")

	rl, err := readline.NewEx(&readline.Config{
		Prompt:          "> ",
		HistoryFile:     "/tmp/gosh_readline_history",
		InterruptPrompt: "^C",
		EOFPrompt:       "exit",
	})
	if err != nil {
		panic(err)
	}
	defer rl.Close()

	historyManager, err := gosh.NewHistoryManager("")
	if err != nil {
		log.Printf("Failed to create history manager: %v", err)
	}

	for {
		line, err := rl.Readline()
		if err != nil {
			if err == readline.ErrInterrupt {
				continue
			} else if err == io.EOF {
				break
			}
			fmt.Println("Error reading input:", err)
			continue
		}

		line = strings.TrimSpace(line)

		if line == "exit" || line == "quit" {
			fmt.Println("Exiting gosh Shell...")
			break
		}

		if line == "" {
			continue
		}

		command, err := gosh.NewCommand(line)
		if err != nil {
			log.Printf("Error creating command: %v", err)
			continue
		}

		command.Stdin = os.Stdin
		command.Stdout = os.Stdout
		command.Stderr = os.Stderr
		command.Run()

		if historyManager != nil {
			err = historyManager.Insert(command, 0) // Replace 0 with actual session ID
			if err != nil {
				log.Printf("Failed to insert command into history: %v", err)
			}
		}

		rl.SaveHistory(line)
	}
}
