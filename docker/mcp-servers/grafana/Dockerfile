# Dockerfile for MCP Server Grafana
FROM node:20-slim

# Assuming mcp-grafana is a Node.js package
# Install mcp-proxy and mcp-grafana
RUN npm install -g @modelcontextprotocol/server-proxy mcp-grafana

# Create a non-root user
RUN useradd -m -s /bin/bash mcpuser

# Switch to non-root user
USER mcpuser
WORKDIR /home/mcpuser

# Expose the default mcp-proxy port
EXPOSE 3000

# Set default environment variables
ENV NODE_ENV=production
ENV GRAFANA_URL=http://localhost:3000

# Start mcp-proxy with mcp-grafana
ENTRYPOINT ["mcp-proxy", "--port", "3000", "--pass-environment", "--", "mcp-grafana"] 