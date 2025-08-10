package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"time"
)

type JSONRPCMessage struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id,omitempty"`
	Method  string      `json:"method,omitempty"`
	Params  interface{} `json:"params,omitempty"`
	Result  interface{} `json:"result,omitempty"`
	Error   *RPCError   `json:"error,omitempty"`
}

type RPCError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

func sendMessage(stdin io.Writer, msg JSONRPCMessage) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	fmt.Fprintf(stdin, "%s\n", string(data))
	return nil
}

func main() {
	// Start the MCP server as a subprocess
	cmd := exec.Command("go", "run", "server.go", ".")

	// Set up pipes for communication
	stdin, err := cmd.StdinPipe()
	if err != nil {
		log.Fatal(err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Fatal(err)
	}

	cmd.Stderr = os.Stderr

	// Start the server
	if err := cmd.Start(); err != nil {
		log.Fatal(err)
	}

	defer func() {
		stdin.Close()
		cmd.Wait()
	}()

	// Create scanner for reading responses
	scanner := bufio.NewScanner(stdout)

	// Helper function to send message and read response
	sendAndRead := func(msg JSONRPCMessage) {
		data, _ := json.Marshal(msg)
		fmt.Printf("Sending: %s\n", string(data))

		if err := sendMessage(stdin, msg); err != nil {
			log.Printf("Error sending message: %v", err)
			return
		}

		if scanner.Scan() {
			response := scanner.Text()
			fmt.Printf("Received: %s\n\n", response)
		}
	}

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	// Test 1: Initialize
	fmt.Println("=== Test 1: Initialize ===")
	initMsg := JSONRPCMessage{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "initialize",
		Params: map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"capabilities": map[string]interface{}{
				"roots": map[string]bool{
					"listChanged": true,
				},
			},
			"clientInfo": map[string]string{
				"name":    "test-client",
				"version": "1.0.0",
			},
		},
	}
	sendAndRead(initMsg)

	// Test 2: List Resources
	fmt.Println("=== Test 2: List Resources ===")
	listMsg := JSONRPCMessage{
		JSONRPC: "2.0",
		ID:      2,
		Method:  "resources/list",
	}
	sendAndRead(listMsg)

	// Test 3: Read a specific resource (you'll need to adjust the URI based on your directory)
	fmt.Println("=== Test 3: Read Resource ===")
	readMsg := JSONRPCMessage{
		JSONRPC: "2.0",
		ID:      3,
		Method:  "resources/read",
		Params: map[string]string{
			"uri": "file://" + getCurrentDir() + "/main.go",
		},
	}
	sendAndRead(readMsg)
}

func getCurrentDir() string {
	dir, err := os.Getwd()
	if err != nil {
		return "."
	}
	return dir
}

/*
Setup Instructions:

1. Create a new directory for your MCP server:
   mkdir mcp-file-server
   cd mcp-file-server

2. Initialize Go module:
   go mod init mcp-file-server

3. Save the main server code as main.go

4. Save this test client code as test_client.go

5. Create some test files in the directory:
   echo "Hello World" > test.txt
   echo '{"name": "test", "value": 42}' > data.json

6. Run the server directly:
   go run main.go .

   Then in another terminal, you can send JSON-RPC messages manually:
   echo '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}' | go run main.go .

7. Or run the automated test client:
   go run test_client.go

8. To specify a different directory to serve:
   go run main.go /path/to/directory

Building and Installation:

1. Build the executable:
   go build -o mcp-file-server main.go

2. Make it executable and install:
   chmod +x mcp-file-server
   sudo cp mcp-file-server /usr/local/bin/

3. You can then run it from anywhere:
   mcp-file-server /path/to/serve

Usage with Claude Desktop or other MCP clients:

Add to your MCP client configuration (like Claude Desktop's config):
{
  "servers": {
    "file-server": {
      "command": "/usr/local/bin/mcp-file-server",
      "args": ["/path/to/directory/to/serve"]
    }
  }
}

Security Notes:
- The server only serves files within the specified directory
- Path traversal attacks are prevented by checking absolute paths
- All file access is read-only
- Logging goes to stderr to not interfere with stdio transport

Features:
- Lists all files recursively in the specified directory
- Serves file contents as text resources
- Supports common MIME type detection
- Proper error handling and logging
- Follows MCP 2024-11-05 protocol specification
*/
