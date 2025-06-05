// Package agent provides an MCP (Model Context Protocol) client implementation
// that can be used to debug and test the envctl aggregator's SSE server.
//
// The agent connects to the aggregator endpoint, performs the MCP handshake,
// lists available tools, and waits for notifications about tool changes.
// When tools are added or removed, it automatically fetches the updated list
// and displays the differences.
//
// The package also provides an interactive REPL (Read-Eval-Print Loop) mode
// that allows users to explore and execute MCP tools, view resources, and
// interact with prompts in real-time.
//
// This package is primarily used by the `envctl agent` command for debugging
// purposes, but can also be used programmatically to test MCP server implementations.
//
// Example usage (normal mode):
//
//	logger := agent.NewLogger(true, true, false)  // verbose=true, color=true, jsonRPC=false
//	client := agent.NewClient("http://localhost:8090/sse", logger)
//
//	ctx := context.Background()
//	if err := client.Run(ctx); err != nil {
//	    log.Fatal(err)
//	}
//
// Example usage (REPL mode):
//
//	logger := agent.NewLogger(true, true, false)
//	client := agent.NewClient("http://localhost:8090/sse", logger)
//	repl := agent.NewREPL(client, logger)
//
//	ctx := context.Background()
//	if err := repl.Run(ctx); err != nil {
//	    log.Fatal(err)
//	}
package agent
