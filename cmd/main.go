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

	// Get remaining arguments after flags
	args := flag.Args()

	// Only log session start in interactive mode (no -c flag and no script file)
	isInteractive := cmdFlag == "" && len(args) == 0
	if isInteractive {
		log.Printf("Session started at %s by user %d (%s)", time.Now(), os.Geteuid(), os.Getenv("USER"))
	}

	// If -c flag is provided, execute the command and exit
	if cmdFlag != "" {
		executeCommand(cmdFlag)
		return
	}

	// If a script file is provided, execute it and exit
	if len(args) > 0 {
		scriptFile := args[0]
		scriptArgs := args // args[0] will be $0, args[1:] will be $1, $2, etc.
		executeScript(scriptFile, scriptArgs)
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
	gosh.SetGlobalJobManager(jobManager)

	// Declare completer variable at the beginning
	var completer readline.AutoCompleter

	// Create argument history database for smart argument completion
	argHistory, err := gosh.NewArgHistory("")
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
		argHistory.Close()
	}
}

func executeCommand(cmd string) {
	jobManager := gosh.NewJobManager()
	gosh.SetGlobalJobManager(jobManager)

	// Create argument history for command line mode
	argHistory, err := gosh.NewArgHistory("")
	if err != nil {
		log.Printf("Warning: Could not initialize argument history: %v", err)
	}

	command, err := gosh.NewCommand(cmd, jobManager)
	if err != nil {
		log.Fatalf("Error creating command: %v", err)
	}

	command.Stdin = os.Stdin
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	command.Run()

	// Record argument usage for the -c command
	if argHistory != nil && cmd != "" {
		parts := strings.Fields(cmd)
		if len(parts) > 1 {
			firstCmd := parts[0]
			argHistory.RecordArgUsage(firstCmd, parts[1:])
			argHistory.Save()
			argHistory.Close()
		}
	}
}

// executeScript executes a shell script from a file
func executeScript(scriptPath string, args []string) {
	// Read the script file
	data, err := os.ReadFile(scriptPath)
	if err != nil {
		log.Fatalf("Error reading script file %s: %v", scriptPath, err)
	}

	content := string(data)
	lines := strings.Split(content, "\n")

	// Handle shebang line
	if len(lines) > 0 && strings.HasPrefix(lines[0], "#!") {
		// Skip the shebang line
		lines = lines[1:]
	}

	// Set up positional parameters
	// args[0] is the script name, args[1:] are the script arguments
	gs := gosh.GetGlobalState()
	gs.SetScriptName(scriptPath)
	// For positional params, args[0] should be script path, and args[1:] should be $1, $2, etc.
	// We need to set params so that $1 = first arg, $2 = second arg, etc.
	if len(args) > 1 {
		gs.SetPositionalParams(args[1:])
	} else {
		gs.SetPositionalParams([]string{})
	}

	// Create job manager for the script
	jobManager := gosh.NewJobManager()
	gosh.SetGlobalJobManager(jobManager)

	// Execute each line of the script
	exitCode := 0
	for lineNum, line := range lines {
		line = strings.TrimSpace(line)

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		command, err := gosh.NewCommand(line, jobManager)
		if err != nil {
			log.Printf("Error at line %d: %v", lineNum+1, err)
			exitCode = 1
			break
		}

		command.Stdin = os.Stdin
		command.Stdout = os.Stdout
		command.Stderr = os.Stderr
		command.Run()

		// Track last exit code
		exitCode = command.ReturnCode
	}

	// Exit with the last command's exit code
	os.Exit(exitCode)
}
