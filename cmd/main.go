package main

import (
	"flag"
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

	var cmdFlag string
	var trimHistoryFlag bool
	var decayHistoryFlag bool

	flag.StringVar(&cmdFlag, "c", "", "Execute a command and exit")
	flag.BoolVar(&trimHistoryFlag, "trim-history", false, "Trim argument history database")
	flag.BoolVar(&decayHistoryFlag, "decay-history", false, "Apply decay factor to argument history")
	flag.Parse()

	log.Printf("Session started at %s by user %d (%s)", time.Now(), os.Geteuid(), os.Getenv("USER"))

	// If -c flag is provided, execute the command and exit
	if cmdFlag != "" {
		executeCommand(cmdFlag)
		return
	}

	// If maintenance flags are provided without interactive mode, exit after processing
	if (trimHistoryFlag || decayHistoryFlag) && cmdFlag == "" {
		fmt.Println("Maintenance operations completed.")
		return
	}

	fmt.Println("Welcome to gosh Shell")
	fmt.Println("Tab completion is ready to use")

	jobManager := gosh.NewJobManager()

	// Declare completer variable at the beginning
	var completer readline.AutoCompleter

	// Create argument history database for smart argument completion
	argHistory, err := gosh.NewArgHistoryDB("")
	if err != nil {
		log.Printf("Warning: Could not initialize argument history: %v", err)
		// If we can't load the argument history, fall back to basic completion
		completer = gosh.NewCompleter(gosh.Builtins())
	} else {
		// Process history maintenance flags
		if trimHistoryFlag {
			fmt.Println("Trimming argument history database...")
			argHistory.Trim(100) // Keep top 100 arguments per command
			argHistory.Save()
		}

		if decayHistoryFlag {
			fmt.Println("Applying decay factor to argument history...")
			argHistory.ApplyDecay(0.9) // Apply 10% decay
			argHistory.Save()
		}

		// Use smart completer with argument history and cycling behavior
		completer = gosh.NewSmartCompleter(gosh.Builtins(), argHistory)
	}

	rl, err := readline.NewEx(&readline.Config{
		Prompt:            gosh.GetPrompt(),
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

	// Tab completion is now ready to use immediately

	for {
		rl.SetPrompt(gosh.GetPrompt()) // Update the prompt before each readline
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

		// Record command usage for tab completion learning
		if len(line) > 0 && strings.Fields(line)[0] != "" {
			parts := strings.Fields(line)
			firstCmd := parts[0]

			// Record command usage for basic command completion
			if sc, ok := completer.(*gosh.SmartCompleter); ok {
				sc.Completer.RecordCommandUsage(firstCmd)
			} else if bc, ok := completer.(*gosh.Completer); ok {
				bc.RecordCommandUsage(firstCmd)
			}

			// Record argument usage for argument completion
			if argHistory != nil && len(parts) > 1 {
				argHistory.RecordArgUsage(firstCmd, parts[1:])
			}
		}

		if historyManager != nil {
			err = historyManager.Insert(command, 0) // Replace 0 with actual session ID
			if err != nil {
				log.Printf("Failed to insert command into history: %v", err)
			}
		}

		rl.SaveHistory(line)
	}

	// Save argument history database before exiting
	if argHistory != nil {
		argHistory.Save()
	}
}

func executeCommand(cmd string) {
	jobManager := gosh.NewJobManager()
	command, err := gosh.NewCommand(cmd, jobManager)
	if err != nil {
		log.Fatalf("Error creating command: %v", err)
	}

	command.Stdin = os.Stdin
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	command.Run()
}
