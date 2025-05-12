# Design Document: Gosh Shell Testability and Programmatic Interfaces

## 1. Executive Summary
This document outlines a comprehensive design for improving the testability of the Gosh shell and creating programmatic interfaces for external program interaction. The design addresses two main goals:

1. Create a testable architecture with proper interfaces and mocks
2. Provide programmatic interfaces for non-interactive automation

## 2. Core Interface Abstractions

### 2.1 FileSystem Interface
```go
package interfaces

import (
    "io"
    "os"
    "time"
)

type FileSystem interface {
    Open(name string) (File, error)
    Create(name string) (File, error)
    OpenFile(name string, flag int, perm os.FileMode) (File, error)
    Stat(name string) (os.FileInfo, error)
    MkdirAll(path string, perm os.FileMode) error
    Remove(name string) error
    RemoveAll(path string) error
    Chdir(dir string) error
    Getwd() (string, error)
    ReadDir(dirname string) ([]os.FileInfo, error)
}

type File interface {
    io.Reader
    io.Writer
    io.Closer
    Name() string
    Stat() (os.FileInfo, error)
}
```

### 2.2 CommandExecutor Interface
```go
package interfaces

import (
    "io"
    "os/exec"
)

type CommandExecutor interface {
    Command(name string, arg ...string) Command
    Start(cmd Command) error
    Run(cmd Command) error
    Wait(cmd Command) error
    LookPath(file string) (string, error)
}

type Command interface {
    SetDir(dir string)
    SetEnv(env []string)
    SetStdin(reader io.Reader)
    SetStdout(writer io.Writer)
    SetStderr(writer io.Writer)
    Start() error
    Run() error
    Wait() error
}
```

### 2.3 EnvironmentManager Interface
```go
package interfaces

type EnvironmentManager interface {
    Getenv(key string) string
    Setenv(key, value string) error
    Unsetenv(key string) error
    Environ() []string
    Getuid() int
    Geteuid() int
}
```

### 2.4 ProcessManager Interface
```go
package interfaces

import (
    "os"
)

type ProcessManager interface {
    StartProcess(name string, argv []string, attr *os.ProcAttr) (*os.Process, error)
    FindProcess(pid int) (*os.Process, error)
    Signal(sig os.Signal) error
}
```

## 3. Mock Implementations

### 3.1 MockFileSystem
```go
package testing

type MockFileSystem struct {
    files     map[string]*MockFile
    cwd       string
    
    // Methods to help set up test scenarios
    AddFile(path, content string, mode os.FileMode) error
    AddDir(path string, mode os.FileMode) error
    RemoveFile(path string) error
    
    // Implementation of FileSystem interface
    // ...
}
```

### 3.2 MockCommandExecutor
```go
package testing

type MockCommandExecutor struct {
    commandOutputs map[string]struct {
        stdout   string
        stderr   string
        exitCode int
        err      error
    }
    executedCommands []string
    
    // Methods to set up test scenarios
    SetCommandOutput(cmd string, stdout, stderr string, exitCode int, err error)
    GetExecutedCommands() []string
    
    // Implementation of CommandExecutor interface
    // ...
}
```

### 3.3 Test Environment
```go
package testing

type TestShellEnvironment struct {
    FileSystem      *MockFileSystem
    CommandExecutor *MockCommandExecutor
    Environment     *MockEnvironmentManager
    ProcessManager  *MockProcessManager
    JobManager      *gosh.JobManager
    
    StdoutBuffer    *bytes.Buffer
    StderrBuffer    *bytes.Buffer
    
    // Helper methods
    RunCommand(cmdString string) (*gosh.Command, error)
    AssertExitCode(expected int) bool
    AssertStdoutContains(substring string) bool
    AssertFileExists(path string) bool
}
```

## 4. Programmatic Interfaces

### 4.1 Shell API (Library Interface)
```go
package api

type Shell struct {
    Stdin      io.Reader
    Stdout     io.Writer
    Stderr     io.Writer
    JobManager *gosh.JobManager
    Environment interfaces.EnvironmentManager
    FileSystem  interfaces.FileSystem
}

func NewShell(stdin io.Reader, stdout, stderr io.Writer) *Shell {
    // Initialize with real implementations
}

func (s *Shell) Execute(cmdString string) (int, error) {
    // Execute a single command and return exit code
}

func (s *Shell) ExecuteScript(script string) ([]int, error) {
    // Execute multiple commands from a script string
}

func (s *Shell) GetEnvironment(key string) string {
    // Get environment variable
}

func (s *Shell) SetEnvironment(key, value string) error {
    // Set environment variable
}
```

### 4.2 HTTP/JSON API
```go
package api

type CommandRequest struct {
    Command   string            `json:"command"`
    Env       map[string]string `json:"env,omitempty"`
    Directory string            `json:"directory,omitempty"`
    Timeout   int               `json:"timeout,omitempty"`
}

type CommandResponse struct {
    ExitCode int    `json:"exitCode"`
    Stdout   string `json:"stdout"`
    Stderr   string `json:"stderr"`
    Error    string `json:"error,omitempty"`
}

func RunAPIServer(addr string) error {
    // Start HTTP server with API endpoints
}

func handleCommandExecution(w http.ResponseWriter, r *http.Request) {
    // Handle POST /execute
}

func handleScriptExecution(w http.ResponseWriter, r *http.Request) {
    // Handle POST /script
}
```

### 4.3 MCP (Message Control Protocol) Interface

MCP is a bidirectional protocol for controlling and interacting with Gosh. This protocol provides a structured message-based approach for executing commands and receiving real-time updates.

#### 4.3.1 MCP Protocol Definition
```go
package mcp

type MessageType string

const (
    TypeCommand     MessageType = "command"     // Execute a command
    TypeScript      MessageType = "script"      // Execute a script
    TypeCancel      MessageType = "cancel"      // Cancel running command
    TypeSignal      MessageType = "signal"      // Send signal to process
    TypeInput       MessageType = "input"       // Send input to stdin
    TypeOutput      MessageType = "output"      // Stdout/stderr output
    TypeProgress    MessageType = "progress"    // Progress updates
    TypeStatus      MessageType = "status"      // Status updates
    TypeCompletion  MessageType = "completion"  // Command completion
    TypeError       MessageType = "error"       // Error messages
)

// Message represents the base structure for all MCP messages
type Message struct {
    ID        string      `json:"id"`         // Unique message ID
    Type      MessageType `json:"type"`       // Message type
    Timestamp int64       `json:"timestamp"`  // Unix timestamp
    Payload   interface{} `json:"payload"`    // Message-specific payload
}

// Command payload for executing shell commands
type CommandPayload struct {
    Command    string            `json:"command"`     // Command string to execute
    Env        map[string]string `json:"env,omitempty"`      // Environment variables
    Directory  string            `json:"directory,omitempty"`// Working directory
    Background bool              `json:"background,omitempty"` // Run in background
    Timeout    int               `json:"timeout,omitempty"`  // Timeout in milliseconds
}

// Script payload for executing multi-line scripts
type ScriptPayload struct {
    Script     string            `json:"script"`     // Multi-line script
    Env        map[string]string `json:"env,omitempty"`      // Environment variables
    Directory  string            `json:"directory,omitempty"`// Working directory
    StopOnError bool             `json:"stopOnError,omitempty"` // Stop on first error
}

// Output payload for stdout/stderr data
type OutputPayload struct {
    CommandID string `json:"commandId"`   // ID of the command producing output
    Stream    string `json:"stream"`      // "stdout" or "stderr"
    Data      string `json:"data"`        // Output content
}

// Status payload for command status updates
type StatusPayload struct {
    CommandID string `json:"commandId"`   // ID of the command
    Status    string `json:"status"`      // "running", "paused", "completed", "failed"
    PID       int    `json:"pid,omitempty"`      // Process ID if available
}

// Completion payload for command completion
type CompletionPayload struct {
    CommandID string `json:"commandId"`   // ID of the command
    ExitCode  int    `json:"exitCode"`    // Exit code
    Duration  int    `json:"duration"`    // Duration in milliseconds
}

// Error payload for error messages
type ErrorPayload struct {
    CommandID string `json:"commandId,omitempty"` // ID of the command (if applicable)
    Code      string `json:"code"`       // Error code
    Message   string `json:"message"`    // Error message
    Details   string `json:"details,omitempty"`   // Additional error details
}

// Input payload for sending input to a running command
type InputPayload struct {
    CommandID string `json:"commandId"`   // ID of the command
    Data      string `json:"data"`        // Input data
}

// Signal payload for sending signals to processes
type SignalPayload struct {
    CommandID string `json:"commandId"`   // ID of the command
    Signal    string `json:"signal"`      // Signal name (e.g., "SIGINT", "SIGTERM")
}
```

#### 4.3.2 MCP Server Implementation
```go
package mcp

import (
    "encoding/json"
    "fmt"
    "net"
    "sync"
    "time"
    
    "gosh/api"
)

// MCPServer handles MCP protocol connections
type MCPServer struct {
    listener   net.Listener
    clients    map[*Client]bool
    clientsMu  sync.Mutex
    shellPool  *ShellPool
}

// Client represents a connected MCP client
type Client struct {
    conn      net.Conn
    server    *MCPServer
    shell     *api.Shell
    commands  map[string]*CommandInstance
    cmdMu     sync.Mutex
}

// CommandInstance represents a running command
type CommandInstance struct {
    ID        string
    Command   string
    Process   *os.Process
    StartTime time.Time
    ExitCode  int
    Done      bool
}

// ShellPool manages shell instances for clients
type ShellPool struct {
    shells map[*Client]*api.Shell
    mu     sync.Mutex
}

// NewMCPServer creates a new MCP server
func NewMCPServer(addr string) (*MCPServer, error) {
    listener, err := net.Listen("tcp", addr)
    if err != nil {
        return nil, err
    }
    
    server := &MCPServer{
        listener:  listener,
        clients:   make(map[*Client]bool),
        shellPool: NewShellPool(),
    }
    
    return server, nil
}

// Start begins listening for connections
func (s *MCPServer) Start() error {
    fmt.Printf("MCP server listening on %s\n", s.listener.Addr())
    
    for {
        conn, err := s.listener.Accept()
        if err != nil {
            return err
        }
        
        client := &Client{
            conn:     conn,
            server:   s,
            commands: make(map[string]*CommandInstance),
        }
        
        // Allocate a shell for this client
        client.shell = s.shellPool.GetShell(client)
        
        // Add client to list
        s.clientsMu.Lock()
        s.clients[client] = true
        s.clientsMu.Unlock()
        
        // Handle client in a goroutine
        go client.handle()
    }
}

// Client.handle processes incoming messages from the client
func (c *Client) handle() {
    defer func() {
        c.conn.Close()
        
        // Remove client and release resources
        c.server.clientsMu.Lock()
        delete(c.server.clients, c)
        c.server.clientsMu.Unlock()
        
        c.server.shellPool.ReleaseShell(c)
    }()
    
    decoder := json.NewDecoder(c.conn)
    
    for {
        var msg Message
        if err := decoder.Decode(&msg); err != nil {
            // Connection closed or error
            break
        }
        
        // Process message based on type
        switch msg.Type {
        case TypeCommand:
            c.handleCommand(msg)
        case TypeScript:
            c.handleScript(msg)
        case TypeInput:
            c.handleInput(msg)
        case TypeSignal:
            c.handleSignal(msg)
        case TypeCancel:
            c.handleCancel(msg)
        default:
            c.sendError(msg.ID, "unsupported_message_type", 
                        fmt.Sprintf("Unsupported message type: %s", msg.Type), "")
        }
    }
}

// handleCommand processes a command execution request
func (c *Client) handleCommand(msg Message) {
    var payload CommandPayload
    
    // Parse payload
    payloadData, err := json.Marshal(msg.Payload)
    if err != nil {
        c.sendError(msg.ID, "invalid_payload", "Invalid payload format", err.Error())
        return
    }
    
    if err := json.Unmarshal(payloadData, &payload); err != nil {
        c.sendError(msg.ID, "invalid_payload", "Invalid payload format", err.Error())
        return
    }
    
    // Setup command execution environment
    if payload.Directory != "" {
        c.shell.ChangeDirectory(payload.Directory)
    }
    
    for k, v := range payload.Env {
        c.shell.SetEnvironment(k, v)
    }
    
    // Create a command instance
    cmdInstance := &CommandInstance{
        ID:        msg.ID,
        Command:   payload.Command,
        StartTime: time.Now(),
    }
    
    c.cmdMu.Lock()
    c.commands[msg.ID] = cmdInstance
    c.cmdMu.Unlock()
    
    // Send status update
    c.sendStatus(msg.ID, "running", 0)
    
    // Execute command with output streaming
    go func() {
        var stdoutBuf, stderrBuf bytes.Buffer
        
        // Create multi-writers to capture and stream output
        stdoutWriter := io.MultiWriter(&stdoutBuf, &outputWriter{client: c, cmdID: msg.ID, stream: "stdout"})
        stderrWriter := io.MultiWriter(&stderrBuf, &outputWriter{client: c, cmdID: msg.ID, stream: "stderr"})
        
        // Create a shell instance for this command
        cmdShell := api.NewShell(nil, stdoutWriter, stderrWriter)
        
        // Set environment from parent shell
        // Apply timeout if specified
        
        // Execute the command
        exitCode, err := cmdShell.Execute(payload.Command)
        
        // Update command status
        c.cmdMu.Lock()
        if cmd, ok := c.commands[msg.ID]; ok {
            cmd.ExitCode = exitCode
            cmd.Done = true
        }
        c.cmdMu.Unlock()
        
        // Send completion message
        duration := int(time.Since(cmdInstance.StartTime).Milliseconds())
        c.sendCompletion(msg.ID, exitCode, duration)
        
        if err != nil {
            c.sendError(msg.ID, "execution_error", "Error executing command", err.Error())
        }
    }()
}

// Additional methods for handling scripts, input, signals, etc.
// ...

// sendMessage sends a message to the client
func (c *Client) sendMessage(msg Message) error {
    data, err := json.Marshal(msg)
    if err != nil {
        return err
    }
    
    data = append(data, '\n')
    _, err = c.conn.Write(data)
    return err
}

// Helper methods for sending various message types
func (c *Client) sendError(cmdID, code, message, details string) {
    // ...
}

func (c *Client) sendStatus(cmdID, status string, pid int) {
    // ...
}

func (c *Client) sendCompletion(cmdID string, exitCode, durationMs int) {
    // ...
}

// outputWriter implements io.Writer to stream output to the client
type outputWriter struct {
    client *Client
    cmdID  string
    stream string
    buffer bytes.Buffer
}

func (w *outputWriter) Write(p []byte) (int, error) {
    // Write to buffer
    n, err := w.buffer.Write(p)
    if err != nil {
        return n, err
    }
    
    // Send output to client
    w.client.sendOutput(w.cmdID, w.stream, string(p))
    return n, nil
}
```

#### 4.3.3 MCP Client Implementation (Go)
```go
package mcp

import (
    "bufio"
    "encoding/json"
    "fmt"
    "net"
    "sync"
    "time"
)

// Client for connecting to an MCP server
type Client struct {
    conn      net.Conn
    writeMu   sync.Mutex
    handlers  map[MessageType][]HandlerFunc
    respChan  map[string]chan Message
    respMu    sync.Mutex
}

// HandlerFunc is a function that handles a message
type HandlerFunc func(msg Message)

// Connect to an MCP server
func Connect(addr string) (*Client, error) {
    conn, err := net.Dial("tcp", addr)
    if err != nil {
        return nil, err
    }
    
    client := &Client{
        conn:     conn,
        handlers: make(map[MessageType][]HandlerFunc),
        respChan: make(map[string]chan Message),
    }
    
    // Start message receiver
    go client.receiveMessages()
    
    return client, nil
}

// Close the connection
func (c *Client) Close() error {
    return c.conn.Close()
}

// On registers a handler for a specific message type
func (c *Client) On(msgType MessageType, handler HandlerFunc) {
    c.handlers[msgType] = append(c.handlers[msgType], handler)
}

// Send a message to the server
func (c *Client) Send(msg Message) error {
    c.writeMu.Lock()
    defer c.writeMu.Unlock()
    
    data, err := json.Marshal(msg)
    if err != nil {
        return err
    }
    
    data = append(data, '\n')
    _, err = c.conn.Write(data)
    return err
}

// Wait for a response to a specific message
func (c *Client) WaitForResponse(msgID string, timeout time.Duration) (Message, error) {
    ch := make(chan Message, 1)
    
    c.respMu.Lock()
    c.respChan[msgID] = ch
    c.respMu.Unlock()
    
    defer func() {
        c.respMu.Lock()
        delete(c.respChan, msgID)
        c.respMu.Unlock()
    }()
    
    select {
    case msg := <-ch:
        return msg, nil
    case <-time.After(timeout):
        return Message{}, fmt.Errorf("timeout waiting for response")
    }
}

// Execute a command and wait for completion
func (c *Client) Execute(cmdString string, options CommandOptions) (int, error) {
    msgID := generateID()
    
    // Create command message
    msg := Message{
        ID:        msgID,
        Type:      TypeCommand,
        Timestamp: time.Now().Unix(),
        Payload: CommandPayload{
            Command:    cmdString,
            Env:        options.Env,
            Directory:  options.Directory,
            Background: options.Background,
            Timeout:    options.Timeout,
        },
    }
    
    // Register completion handler
    completionCh := make(chan int, 1)
    errorCh := make(chan error, 1)
    
    c.On(TypeCompletion, func(resp Message) {
        if resp.ID == msgID {
            var payload CompletionPayload
            
            payloadData, err := json.Marshal(resp.Payload)
            if err != nil {
                errorCh <- err
                return
            }
            
            if err := json.Unmarshal(payloadData, &payload); err != nil {
                errorCh <- err
                return
            }
            
            completionCh <- payload.ExitCode
        }
    })
    
    c.On(TypeError, func(resp Message) {
        if resp.ID == msgID {
            var payload ErrorPayload
            
            payloadData, err := json.Marshal(resp.Payload)
            if err != nil {
                errorCh <- err
                return
            }
            
            if err := json.Unmarshal(payloadData, &payload); err != nil {
                errorCh <- err
                return
            }
            
            errorCh <- fmt.Errorf("%s: %s", payload.Code, payload.Message)
        }
    })
    
    // Send the command
    if err := c.Send(msg); err != nil {
        return -1, err
    }
    
    // Wait for completion or error
    select {
    case exitCode := <-completionCh:
        return exitCode, nil
    case err := <-errorCh:
        return -1, err
    case <-time.After(time.Duration(options.Timeout) * time.Millisecond):
        return -1, fmt.Errorf("command execution timed out")
    }
}

// receiveMessages continuously receives messages from the server
func (c *Client) receiveMessages() {
    scanner := bufio.NewScanner(c.conn)
    
    for scanner.Scan() {
        line := scanner.Text()
        
        var msg Message
        if err := json.Unmarshal([]byte(line), &msg); err != nil {
            // Handle error
            continue
        }
        
        // Dispatch to handlers
        if handlers, ok := c.handlers[msg.Type]; ok {
            for _, handler := range handlers {
                go handler(msg)
            }
        }
        
        // Signal any waiting responses
        c.respMu.Lock()
        if ch, ok := c.respChan[msg.ID]; ok {
            ch <- msg
        }
        c.respMu.Unlock()
    }
    
    // Connection closed or error
    if err := scanner.Err(); err != nil {
        // Handle error
    }
}
```

#### 4.3.4 MCP Client Example (Python)
```python
import json
import socket
import threading
import time
import uuid

class MCPClient:
    def __init__(self, host, port):
        self.socket = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
        self.socket.connect((host, port))
        self.handlers = {}
        self.response_events = {}
        self.response_data = {}
        self.lock = threading.Lock()
        
        # Start receiver thread
        self.running = True
        self.receiver_thread = threading.Thread(target=self._receive_messages)
        self.receiver_thread.daemon = True
        self.receiver_thread.start()
    
    def close(self):
        self.running = False
        self.socket.close()
    
    def on(self, msg_type, handler):
        if msg_type not in self.handlers:
            self.handlers[msg_type] = []
        self.handlers[msg_type].append(handler)
    
    def send(self, msg):
        data = json.dumps(msg) + "\n"
        self.socket.sendall(data.encode('utf-8'))
    
    def execute(self, command, env=None, directory=None, timeout=30000):
        msg_id = str(uuid.uuid4())
        
        # Create event for waiting
        event = threading.Event()
        with self.lock:
            self.response_events[msg_id] = event
            self.response_data[msg_id] = None
        
        # Create message
        msg = {
            "id": msg_id,
            "type": "command",
            "timestamp": int(time.time()),
            "payload": {
                "command": command,
                "env": env or {},
                "directory": directory or "",
                "timeout": timeout
            }
        }
        
        # Send command
        self.send(msg)
        
        # Collect outputs
        stdout_data = []
        stderr_data = []
        
        def on_output(msg):
            if msg["id"] == msg_id:
                payload = msg["payload"]
                if payload["stream"] == "stdout":
                    stdout_data.append(payload["data"])
                elif payload["stream"] == "stderr":
                    stderr_data.append(payload["data"])
        
        # Register output handler
        self.on("output", on_output)
        
        # Wait for completion
        if not event.wait(timeout / 1000):
            raise TimeoutError("Command execution timed out")
        
        # Get result
        with self.lock:
            result = self.response_data[msg_id]
            del self.response_events[msg_id]
            del self.response_data[msg_id]
        
        if "error" in result:
            raise Exception(f"Command failed: {result['error']}")
        
        return {
            "exit_code": result["exit_code"],
            "stdout": "".join(stdout_data),
            "stderr": "".join(stderr_data),
            "duration": result["duration"]
        }
    
    def _receive_messages(self):
        buffer = ""
        
        while self.running:
            try:
                data = self.socket.recv(4096).decode('utf-8')
                if not data:
                    break
                
                buffer += data
                lines = buffer.split("\n")
                
                # Process complete messages
                for i in range(len(lines) - 1):
                    line = lines[i]
                    try:
                        msg = json.loads(line)
                        self._handle_message(msg)
                    except json.JSONDecodeError:
                        pass
                
                # Keep incomplete message
                buffer = lines[-1]
            
            except Exception as e:
                print(f"Error receiving messages: {e}")
                break
    
    def _handle_message(self, msg):
        msg_type = msg["type"]
        msg_id = msg["id"]
        
        # Dispatch to handlers
        if msg_type in self.handlers:
            for handler in self.handlers[msg_type]:
                try:
                    handler(msg)
                except Exception as e:
                    print(f"Error in handler: {e}")
        
        # Handle completion
        if msg_type == "completion":
            payload = msg["payload"]
            with self.lock:
                if msg_id in self.response_events:
                    self.response_data[msg_id] = {
                        "exit_code": payload["exitCode"],
                        "duration": payload["duration"]
                    }
                    self.response_events[msg_id].set()
        
        # Handle error
        elif msg_type == "error":
            payload = msg["payload"]
            with self.lock:
                if msg_id in self.response_events:
                    self.response_data[msg_id] = {
                        "error": f"{payload['code']}: {payload['message']}"
                    }
                    self.response_events[msg_id].set()
```

### 4.4 Enhanced CLI Options
```go
// cmd/main.go additions
var jsonOutputFlag bool
var apiServerFlag bool
var apiAddr string
var scriptFlag string
var mcpServerFlag bool
var mcpAddr string

flag.BoolVar(&jsonOutputFlag, "json", false, "Output results in JSON format")
flag.BoolVar(&apiServerFlag, "api-server", false, "Run as an API server")
flag.StringVar(&apiAddr, "api-addr", ":8080", "API server address") 
flag.StringVar(&scriptFlag, "f", "", "Execute commands from script file")
flag.BoolVar(&mcpServerFlag, "mcp-server", false, "Run as an MCP server")
flag.StringVar(&mcpAddr, "mcp-addr", ":9090", "MCP server address")
```

## 5. Implementation Examples

### 5.1 Unit Test Example
```go
func TestRedirection(t *testing.T) {
    env := testing.NewTestShellEnvironment()
    
    // Set up test files
    env.FileSystem.AddFile("/mock/input.txt", "test content", 0644)
    
    // Run command with redirection
    cmd, err := env.RunCommand("cat /mock/input.txt > /mock/output.txt")
    if err != nil {
        t.Fatalf("Command creation failed: %v", err)
    }
    
    // Verify command succeeded
    if !env.AssertExitCode(0) {
        t.Error("Command failed unexpectedly")
    }
    
    // Verify output file exists with correct content
    if !env.AssertFileExists("/mock/output.txt") {
        t.Error("Output file not created")
    }
    
    content, _ := env.FileSystem.ReadFile("/mock/output.txt")
    if string(content) != "test content" {
        t.Errorf("Expected 'test content', got '%s'", content)
    }
}
```

### 5.2 HTTP API Example (Python Client)
```python
import requests
import json

# Execute single command
response = requests.post("http://localhost:8080/execute", json={
    "command": "ls -la",
    "env": {"PATH": "/usr/bin:/bin"},
    "directory": "/tmp"
})

result = response.json()
print(f"Exit code: {result['exitCode']}")
print(f"Output: {result['stdout']}")

# Execute script
script = """
mkdir -p test_dir
cd test_dir
echo "Hello, World!" > test.txt
cat test.txt
"""

response = requests.post("http://localhost:8080/script", json={
    "script": script,
    "directory": "/tmp"
})

results = response.json()
for i, result in enumerate(results):
    print(f"Command {i+1} exit code: {result['exitCode']}")
    if result['stdout']:
        print(f"Output: {result['stdout']}")
```

### 5.3 Library API Example
```go
package main

import (
    "bytes"
    "fmt"
    "os"
    
    "gosh/api"
)

func main() {
    stdout := &bytes.Buffer{}
    stderr := &bytes.Buffer{}
    
    // Create shell instance
    shell := api.NewShell(os.Stdin, stdout, stderr)
    
    // Execute command
    exitCode, err := shell.Execute("echo Hello, World!")
    if err != nil {
        fmt.Fprintf(os.Stderr, "Error: %v\n", err)
        os.Exit(1)
    }
    
    fmt.Printf("Exit code: %d\n", exitCode)
    fmt.Printf("Output: %s\n", stdout.String())
    
    // Execute script
    script := `
    mkdir -p test_dir
    cd test_dir
    echo "Hello from script" > test.txt
    cat test.txt
    `
    
    results, err := shell.ExecuteScript(script)
    if err != nil {
        fmt.Fprintf(os.Stderr, "Script error: %v\n", err)
    }
    
    fmt.Printf("Script completed with %d commands\n", len(results))
    fmt.Printf("Final output: %s\n", stdout.String())
}
```

### 5.4 MCP Client Example
```go
package main

import (
    "fmt"
    "os"
    
    "gosh/mcp"
)

func main() {
    // Connect to MCP server
    client, err := mcp.Connect("localhost:9090")
    if err != nil {
        fmt.Fprintf(os.Stderr, "Connection error: %v\n", err)
        os.Exit(1)
    }
    defer client.Close()
    
    // Register output handler
    client.On(mcp.TypeOutput, func(msg mcp.Message) {
        payload, ok := msg.Payload.(mcp.OutputPayload)
        if !ok {
            return
        }
        
        if payload.Stream == "stdout" {
            fmt.Print(payload.Data)
        } else {
            fmt.Fprint(os.Stderr, payload.Data)
        }
    })
    
    // Execute a command
    options := mcp.CommandOptions{
        Env: map[string]string{
            "TEST_VAR": "test_value",
        },
        Directory: "/tmp",
        Timeout:   30000, // 30 seconds
    }
    
    exitCode, err := client.Execute("echo $TEST_VAR && ls -la", options)
    if err != nil {
        fmt.Fprintf(os.Stderr, "Execution error: %v\n", err)
        os.Exit(1)
    }
    
    fmt.Printf("Command completed with exit code: %d\n", exitCode)
}
```

## 6. Questions for Design Decisions

1. **Database Abstractions**: Should we also abstract the database interfaces for history and arg history?

2. **Configuration Storage**: How should shell configuration be stored and accessed?

3. **Authentication**: Should the HTTP API and MCP protocol include authentication?

4. **Platform Compatibility**: Do we need specific interfaces for platform-dependent features?

5. **State Management**: How should we handle stateful operations between commands?

6. **Streaming Responses**: Should the HTTP API support streaming output for long-running commands?

7. **Error Standardization**: How should we standardize error formats across interfaces?

8. **Protocol Extensions**: Beyond HTTP and MCP, should we support other protocols (gRPC, WebSockets)?