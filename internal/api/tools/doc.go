// Package tools provides MCP tool implementations that expose envctl's API functionality
// through the aggregated MCP server.
//
// This package bridges the gap between envctl's internal API layer and the MCP protocol,
// allowing AI assistants and other MCP clients to programmatically control envctl services,
// clusters, and connections.
//
// Tool Categories:
//
//   - Service Management: Start, stop, restart, and query service status
//   - Cluster Management: List, switch, and query active clusters by role
//   - MCP Server Management: List servers, get server info, and query available tools
//   - K8s Connection Management: List connections and query connection details
//   - Port Forward Management: List and query port forward configurations
//
// All tools follow consistent patterns:
//   - Return structured JSON responses
//   - Provide clear error messages
//   - Use the envctl prefix (e.g., x_service_list) when exposed through the aggregator
//
// Example Usage:
//
// The tools are automatically registered with the aggregator server and become available
// to MCP clients. For example, to list all services:
//
//	{
//	  "method": "tools/call",
//	  "params": {
//	    "name": "x_service_list",
//	    "arguments": {}
//	  }
//	}
//
// Response:
//
//	{
//	  "services": [
//	    {
//	      "label": "my-service",
//	      "service_type": "MCPServer",
//	      "state": "running",
//	      "health": "healthy"
//	    }
//	  ],
//	  "total": 1
//	}
package tools
