// Package agent provides an MCP (Model Context Protocol) client implementation
// that can be used to debug and test the envctl aggregator's SSE server.
//
// The agent connects to the aggregator endpoint, performs the MCP handshake,
// lists available tools, and waits for notifications about tool changes.
// When tools are added or removed, it automatically fetches the updated list
// and displays the differences.
//
// This package is primarily used by the `envctl agent` command for debugging
// purposes, but can also be used programmatically to test MCP server implementations.
//
// Example usage:
//
//	logger := agent.NewLogger(true, true, false)  // verbose=true, color=true, jsonRPC=false
//	client := agent.NewClient("http://localhost:8090/sse", logger)
//
//	ctx := context.Background()
//	if err := client.Run(ctx); err != nil {
//	    log.Fatal(err)
//	}
package agent
