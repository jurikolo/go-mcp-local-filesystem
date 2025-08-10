# go-mcp-local-filesystem
Simple MCP server and client that uses stdio and allows interactions with local filesystem

# How to build and run MCP server
```sh
go build -o mcp-file-server server.go
./mcp-file-server
```

# How to build and run MCP client
```sh
go build -o mcp-client client.go
./mcp-client
```

# How to test MCP server locally
Test initialization:
```sh
echo '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}' | go run server.go . | jq .
```

Test file list:
```sh
echo '{"jsonrpc":"2.0","id":1,"method":"resources/list","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}' | go run server.go . | jq .
```

Test file read:
```sh
echo '{"jsonrpc":"2.0","id":1,"method":"resources/read","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"},"uri":"file://go.mod"}}' | go run server.go . | jq .
```

# How to integrate with local AI
```sh
# install https://ollama.com/download
curl -fsSL https://ollama.com/install.sh | sh
# pull a model that supports function calling
ollama pull qwen2.5:7b-instruct
# use Ollama MCP Bridge
git clone https://github.com/patruff/ollama-mcp-bridge
cd ollama-mcp-bridge
# configure MCP server binary location and working directory as argument (must be same dir as MCP server or subdir)
cat bridge_config.json
{
  "mcpServers": {
    "filesystem": {
      "command": "/home/$USER/git/go-mcp-local-filesystem/mcp-file-server",
      "args": [
        "/home/$USER/git/go-mcp-local-filesystem"
      ]
  }
  },
  "llm": {
    "model": "qwen2.5:7b-instruct",
    "baseUrl": "http://localhost:11434",
    "apiKey": "ollama",
    "temperature": 0.7,
    "maxTokens": 1000
  },
  "systemPrompt": "You are a helpful assistant that can use various tools to help answer questions. You have access to multiple MCPs including filesystem operations, GitHub interactions, Brave search, Gmail, Google Drive, and Flux for image generation. When using these tools, make sure to respect their specific requirements and limitations."
}

# install dependencies and run MCP bridge
npm install
npm start
```
