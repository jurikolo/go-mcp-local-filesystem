package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
)

// MCP Protocol Message Types

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

type InitializeParams struct {
	ProtocolVersion string                 `json:"protocolVersion"`
	Capabilities    ClientCapabilities     `json:"capabilities"`
	ClientInfo      ClientInfo             `json:"clientInfo"`
	Meta            map[string]interface{} `json:"meta,omitempty"`
}

type ClientCapabilities struct {
	Roots    *RootsCapability    `json:"roots,omitempty"`
	Sampling *SamplingCapability `json:"sampling,omitempty"`
}

type RootsCapability struct {
	ListChanged bool `json:"listChanged,omitempty"`
}

type SamplingCapability struct{}

type ClientInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type InitializeResult struct {
	ProtocolVersion string             `json:"protocolVersion"`
	Capabilities    ServerCapabilities `json:"capabilities"`
	ServerInfo      ServerInfo         `json:"serverInfo"`
}

type ServerCapabilities struct {
	Resources *ResourcesCapability `json:"resources,omitempty"`
	Tools     *ToolsCapability     `json:"tools,omitempty"`
}

type ResourcesCapability struct {
	Subscribe   bool `json:"subscribe,omitempty"`
	ListChanged bool `json:"listChanged,omitempty"`
}

type ToolsCapability struct {
	ListChanged bool `json:"listChanged,omitempty"`
}

type ServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type ListResourcesResult struct {
	Resources []Resource `json:"resources"`
}

type Resource struct {
	URI         string                 `json:"uri"`
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	MimeType    string                 `json:"mimeType,omitempty"`
	Meta        map[string]interface{} `json:"meta,omitempty"`
}

type ReadResourceParams struct {
	URI string `json:"uri"`
}

type ReadResourceResult struct {
	Contents []ResourceContent `json:"contents"`
}

type ResourceContent struct {
	URI      string `json:"uri"`
	MimeType string `json:"mimeType,omitempty"`
	Text     string `json:"text,omitempty"`
	Blob     string `json:"blob,omitempty"`
}

type ListToolsResult struct {
	Tools []Tool `json:"tools"`
}

type Tool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"inputSchema"`
}

type CallToolParams struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments,omitempty"`
}

type CallToolResult struct {
	Content []ToolContent `json:"content"`
	IsError bool          `json:"isError,omitempty"`
}

type ToolContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// MCP Server Implementation

type MCPServer struct {
	baseDir string
	scanner *bufio.Scanner
}

func NewMCPServer(baseDir string) *MCPServer {
	return &MCPServer{
		baseDir: baseDir,
		scanner: bufio.NewScanner(os.Stdin),
	}
}

func (s *MCPServer) sendMessage(msg JSONRPCMessage) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	fmt.Println(string(data))
	return nil
}

func (s *MCPServer) sendError(id interface{}, code int, message string) error {
	msg := JSONRPCMessage{
		JSONRPC: "2.0",
		ID:      id,
		Error: &RPCError{
			Code:    code,
			Message: message,
		},
	}
	return s.sendMessage(msg)
}

func (s *MCPServer) sendResult(id interface{}, result interface{}) error {
	msg := JSONRPCMessage{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	}
	return s.sendMessage(msg)
}

func (s *MCPServer) sendToolResult(id interface{}, text string, isError bool) error {
	result := CallToolResult{
		Content: []ToolContent{
			{
				Type: "text",
				Text: text,
			},
		},
		IsError: isError,
	}
	return s.sendResult(id, result)
}

func (s *MCPServer) handleInitialize(id interface{}, params InitializeParams) error {
	log.Printf("Initialize request from client: %s %s", params.ClientInfo.Name, params.ClientInfo.Version)

	result := InitializeResult{
		ProtocolVersion: "2024-11-05",
		Capabilities: ServerCapabilities{
			Resources: &ResourcesCapability{
				Subscribe:   false,
				ListChanged: false,
			},
			Tools: &ToolsCapability{
				ListChanged: false,
			},
		},
		ServerInfo: ServerInfo{
			Name:    "file-server",
			Version: "1.0.0",
		},
	}

	return s.sendResult(id, result)
}

func (s *MCPServer) handleNotificationInitialized() {
	// This is a notification, no response needed
	log.Printf("Received initialized notification")
}

func (s *MCPServer) handleListResources(id interface{}) error {
	log.Printf("Listing resources in directory: %s", s.baseDir)

	var resources []Resource

	err := filepath.WalkDir(s.baseDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			return nil
		}

		relPath, err := filepath.Rel(s.baseDir, path)
		if err != nil {
			return err
		}

		uri := "file://" + filepath.Join(s.baseDir, relPath)

		// Determine MIME type based on file extension
		mimeType := getMimeType(filepath.Ext(path))

		resource := Resource{
			URI:         uri,
			Name:        relPath,
			Description: fmt.Sprintf("File: %s", relPath),
			MimeType:    mimeType,
		}

		resources = append(resources, resource)
		return nil
	})

	if err != nil {
		log.Printf("Error walking directory: %v", err)
		return s.sendError(id, -32603, fmt.Sprintf("Failed to list resources: %v", err))
	}

	result := ListResourcesResult{
		Resources: resources,
	}

	log.Printf("Found %d resources", len(resources))
	return s.sendResult(id, result)
}

func (s *MCPServer) handleReadResource(id interface{}, params ReadResourceParams) error {
	log.Printf("Reading resource: %s", params.URI)

	// Parse URI to get file path
	if !strings.HasPrefix(params.URI, "file://") {
		return s.sendError(id, -32602, "Invalid URI scheme, expected file://")
	}

	filePath := strings.TrimPrefix(params.URI, "file://")

	// Security check: ensure the file is within the base directory
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return s.sendError(id, -32602, "Invalid file path")
	}

	absBaseDir, err := filepath.Abs(s.baseDir)
	if err != nil {
		return s.sendError(id, -32603, "Server configuration error")
	}

	if !strings.HasPrefix(absPath, absBaseDir) {
		return s.sendError(id, -32602, "Access denied: file outside allowed directory")
	}

	// Read file content
	content, err := os.ReadFile(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			return s.sendError(id, -32602, "File not found")
		}
		return s.sendError(id, -32603, fmt.Sprintf("Failed to read file: %v", err))
	}

	mimeType := getMimeType(filepath.Ext(absPath))

	resourceContent := ResourceContent{
		URI:      params.URI,
		MimeType: mimeType,
		Text:     string(content),
	}

	result := ReadResourceResult{
		Contents: []ResourceContent{resourceContent},
	}

	log.Printf("Successfully read file: %s (%d bytes)", absPath, len(content))
	return s.sendResult(id, result)
}

func (s *MCPServer) handleListTools(id interface{}) error {
	log.Printf("Listing available tools")

	tools := []Tool{
		{
			Name:        "read_file",
			Description: "Read the contents of a file",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"path": map[string]interface{}{
						"type":        "string",
						"description": "The path to the file to read",
					},
				},
				"required": []string{"path"},
			},
		},
		{
			Name:        "list_directory",
			Description: "List files and directories in a given path",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"path": map[string]interface{}{
						"type":        "string",
						"description": "The path to the directory to list (optional, defaults to base directory)",
					},
				},
				"required": []string{},
			},
		},
		{
			Name:        "search_files",
			Description: "Search for files by name pattern",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"pattern": map[string]interface{}{
						"type":        "string",
						"description": "The filename pattern to search for (supports wildcards)",
					},
				},
				"required": []string{"pattern"},
			},
		},
	}

	result := ListToolsResult{
		Tools: tools,
	}

	log.Printf("Returning %d tools", len(tools))
	return s.sendResult(id, result)
}

func (s *MCPServer) handleCallTool(id interface{}, params CallToolParams) error {
	log.Printf("Calling tool: %s with arguments: %v", params.Name, params.Arguments)

	switch params.Name {
	case "read_file":
		return s.handleReadFileTool(id, params.Arguments)
	case "list_directory":
		return s.handleListDirectoryTool(id, params.Arguments)
	case "search_files":
		return s.handleSearchFilesTool(id, params.Arguments)
	default:
		return s.sendError(id, -32601, fmt.Sprintf("Tool not found: %s", params.Name))
	}
}

func (s *MCPServer) handleReadFileTool(id interface{}, args map[string]interface{}) error {
	pathArg, ok := args["path"]
	if !ok {
		return s.sendError(id, -32602, "Missing required argument: path")
	}

	path, ok := pathArg.(string)
	if !ok {
		return s.sendError(id, -32602, "Invalid path argument: must be string")
	}

	// Security check: ensure the file is within the base directory
	fullPath := filepath.Join(s.baseDir, path)
	absPath, err := filepath.Abs(fullPath)
	if err != nil {
		return s.sendError(id, -32602, "Invalid file path")
	}

	absBaseDir, err := filepath.Abs(s.baseDir)
	if err != nil {
		return s.sendError(id, -32603, "Server configuration error")
	}

	if !strings.HasPrefix(absPath, absBaseDir) {
		return s.sendError(id, -32602, "Access denied: file outside allowed directory")
	}

	// Read file content
	content, err := os.ReadFile(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			return s.sendToolResult(id, fmt.Sprintf("File not found: %s", path), true)
		}
		return s.sendToolResult(id, fmt.Sprintf("Failed to read file: %v", err), true)
	}

	result := fmt.Sprintf("Contents of %s:\n%s", path, string(content))
	return s.sendToolResult(id, result, false)
}

func (s *MCPServer) handleListDirectoryTool(id interface{}, args map[string]interface{}) error {
	var targetDir string

	if pathArg, ok := args["path"]; ok {
		if path, ok := pathArg.(string); ok {
			targetDir = filepath.Join(s.baseDir, path)
		} else {
			return s.sendError(id, -32602, "Invalid path argument: must be string")
		}
	} else {
		targetDir = s.baseDir
	}

	// Security check
	absPath, err := filepath.Abs(targetDir)
	if err != nil {
		return s.sendError(id, -32602, "Invalid directory path")
	}

	absBaseDir, err := filepath.Abs(s.baseDir)
	if err != nil {
		return s.sendError(id, -32603, "Server configuration error")
	}

	if !strings.HasPrefix(absPath, absBaseDir) {
		return s.sendError(id, -32602, "Access denied: directory outside allowed path")
	}

	// List directory contents
	entries, err := os.ReadDir(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			return s.sendToolResult(id, fmt.Sprintf("Directory not found: %s", targetDir), true)
		}
		return s.sendToolResult(id, fmt.Sprintf("Failed to list directory: %v", err), true)
	}

	var result strings.Builder
	relPath, _ := filepath.Rel(s.baseDir, absPath)
	if relPath == "." {
		result.WriteString("Contents of base directory:\n")
	} else {
		result.WriteString(fmt.Sprintf("Contents of %s:\n", relPath))
	}

	for _, entry := range entries {
		if entry.IsDir() {
			result.WriteString(fmt.Sprintf("📁 %s/\n", entry.Name()))
		} else {
			info, err := entry.Info()
			if err == nil {
				result.WriteString(fmt.Sprintf("📄 %s (%d bytes)\n", entry.Name(), info.Size()))
			} else {
				result.WriteString(fmt.Sprintf("📄 %s\n", entry.Name()))
			}
		}
	}

	return s.sendToolResult(id, result.String(), false)
}

func (s *MCPServer) handleSearchFilesTool(id interface{}, args map[string]interface{}) error {
	patternArg, ok := args["pattern"]
	if !ok {
		return s.sendError(id, -32602, "Missing required argument: pattern")
	}

	pattern, ok := patternArg.(string)
	if !ok {
		return s.sendError(id, -32602, "Invalid pattern argument: must be string")
	}

	var matches []string

	err := filepath.WalkDir(s.baseDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			return nil
		}

		matched, err := filepath.Match(pattern, d.Name())
		if err != nil {
			return err
		}

		if matched {
			relPath, err := filepath.Rel(s.baseDir, path)
			if err != nil {
				return err
			}
			matches = append(matches, relPath)
		}

		return nil
	})

	if err != nil {
		return s.sendToolResult(id, fmt.Sprintf("Search failed: %v", err), true)
	}

	var result strings.Builder
	result.WriteString(fmt.Sprintf("Files matching pattern '%s':\n", pattern))

	if len(matches) == 0 {
		result.WriteString("No files found matching the pattern.")
	} else {
		for _, match := range matches {
			result.WriteString(fmt.Sprintf("📄 %s\n", match))
		}
	}

	return s.sendToolResult(id, result.String(), false)
}

func (s *MCPServer) handleMessage(msg JSONRPCMessage) error {
	switch msg.Method {
	case "initialize":
		var params InitializeParams
		if err := json.Unmarshal(mustMarshal(msg.Params), &params); err != nil {
			return s.sendError(msg.ID, -32602, "Invalid initialize parameters")
		}
		return s.handleInitialize(msg.ID, params)

	case "notifications/initialized":
		s.handleNotificationInitialized()
		return nil

	case "resources/list":
		return s.handleListResources(msg.ID)

	case "resources/read":
		var params ReadResourceParams
		if err := json.Unmarshal(mustMarshal(msg.Params), &params); err != nil {
			return s.sendError(msg.ID, -32602, "Invalid read resource parameters")
		}
		return s.handleReadResource(msg.ID, params)

	case "tools/list":
		return s.handleListTools(msg.ID)

	case "tools/call":
		var params CallToolParams
		if err := json.Unmarshal(mustMarshal(msg.Params), &params); err != nil {
			return s.sendError(msg.ID, -32602, "Invalid call tool parameters")
		}
		return s.handleCallTool(msg.ID, params)

	default:
		return s.sendError(msg.ID, -32601, fmt.Sprintf("Method not found: %s", msg.Method))
	}
}

func (s *MCPServer) Run() error {
	log.Printf("MCP Server starting, serving directory: %s", s.baseDir)
	log.Printf("Server ready, waiting for messages...")

	for s.scanner.Scan() {
		line := s.scanner.Text()
		if line == "" {
			continue
		}

		log.Printf("Received: %s", line)

		var msg JSONRPCMessage
		if err := json.Unmarshal([]byte(line), &msg); err != nil {
			log.Printf("Invalid JSON: %v", err)
			continue
		}

		if err := s.handleMessage(msg); err != nil {
			log.Printf("Error handling message: %v", err)
		}
	}

	if err := s.scanner.Err(); err != nil {
		return fmt.Errorf("scanner error: %v", err)
	}

	return nil
}

// Utility Functions

func getMimeType(ext string) string {
	switch strings.ToLower(ext) {
	case ".txt", ".md", ".markdown":
		return "text/plain"
	case ".json":
		return "application/json"
	case ".xml":
		return "application/xml"
	case ".html", ".htm":
		return "text/html"
	case ".css":
		return "text/css"
	case ".js":
		return "application/javascript"
	case ".go":
		return "text/plain"
	case ".py":
		return "text/plain"
	case ".java":
		return "text/plain"
	case ".c", ".cpp", ".h":
		return "text/plain"
	default:
		return "application/octet-stream"
	}
}

func mustMarshal(v interface{}) []byte {
	if v == nil {
		return []byte("{}")
	}
	data, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return data
}

func main() {
	// Default to current directory if no argument provided
	baseDir := "."
	if len(os.Args) > 1 {
		baseDir = os.Args[1]
	}

	// Ensure the directory exists
	if _, err := os.Stat(baseDir); os.IsNotExist(err) {
		log.Fatalf("Directory does not exist: %s", baseDir)
	}

	// Set up logging to stderr so it doesn't interfere with stdio communication
	log.SetOutput(os.Stderr)
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	server := NewMCPServer(baseDir)
	if err := server.Run(); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
