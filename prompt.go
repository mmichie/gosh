package gosh

import (
	"os"
	"strings"
	"time"
)

var defaultPrompt = "\033[1;36m%u@%h\033[0m:\033[1;34m%w\033[0m$ "

func GetPrompt() string {
	customPrompt := os.Getenv("GOSH_PROMPT")
	if customPrompt == "" {
		customPrompt = defaultPrompt
	}
	return expandPromptVariables(customPrompt)
}

func expandPromptVariables(prompt string) string {
	gs := GetGlobalState()
	username := os.Getenv("USER")
	hostname, _ := os.Hostname()

	replacements := map[string]string{
		"%u": username,
		"%h": hostname,
		"%w": gs.GetCWD(),
		"%W": shortenPath(gs.GetCWD()),
		"%d": time.Now().Format("2006-01-02"),
		"%t": time.Now().Format("15:04:05"),
		"%$": "$",
	}

	for key, value := range replacements {
		prompt = strings.ReplaceAll(prompt, key, value)
	}

	return prompt
}

func shortenPath(path string) string {
	home := os.Getenv("HOME")
	if strings.HasPrefix(path, home) {
		return "~" + path[len(home):]
	}
	return path
}

func SetPrompt(newPrompt string) error {
	return os.Setenv("GOSH_PROMPT", newPrompt)
}
