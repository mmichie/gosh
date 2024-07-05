package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"gosh"

	"github.com/chzyer/readline"
)

func main() {
	log.SetFlags(0)
	log.SetPrefix("")

	log.Printf("Session started at %s by user %d (%s)", time.Now(), os.Geteuid(), os.Getenv("USER"))

	fmt.Println("Welcome to gosh Shell")

	jobManager := gosh.NewJobManager()
	completer := gosh.NewCompleter(gosh.Builtins())

	rl, err := readline.NewEx(&readline.Config{
		Prompt:            "> ",
		HistoryFile:       "/tmp/gosh_readline_history",
		InterruptPrompt:   "^C",
		EOFPrompt:         "exit",
		AutoComplete:      completer,
		HistorySearchFold: true,
	})
	if err != nil {
		panic(err)
	}
	defer rl.Close()

	historyManager, err := gosh.NewHistoryManager("")
	if err != nil {
		log.Printf("Failed to create history manager: %v", err)
	}

	// Set up signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGTSTP, syscall.SIGINT, syscall.SIGCHLD)

	go func() {
		for sig := range sigChan {
			switch sig {
			case syscall.SIGTSTP:
				fmt.Println("\nReceived SIGTSTP")
				jobManager.StopForegroundJob()
			case syscall.SIGINT:
				fmt.Println("\nReceived SIGINT")
				jobManager.StopForegroundJob()
			case syscall.SIGCHLD:
				jobManager.ReapChildren()
			}
		}
	}()

	fmt.Println("Tab completion is being initialized in the background. It will be fully functional shortly.")

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

		command, err := gosh.NewCommand(line, jobManager)
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
