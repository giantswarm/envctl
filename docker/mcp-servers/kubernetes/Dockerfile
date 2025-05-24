# Dockerfile for MCP Server Kubernetes
FROM node:20-slim

# Install mcp-proxy and mcp-server-kubernetes
RUN npm install -g @modelcontextprotocol/server-proxy @modelcontextprotocol/server-kubernetes

# Create a non-root user
RUN useradd -m -s /bin/bash mcpuser

# Switch to non-root user
USER mcpuser
WORKDIR /home/mcpuser

# Copy kubeconfig if provided at build time (optional)
# This can also be mounted at runtime
# COPY --chown=mcpuser:mcpuser kubeconfig /home/mcpuser/.kube/config

# Expose the default mcp-proxy port
EXPOSE 3000

# Set default environment variables
ENV NODE_ENV=production

# Start mcp-proxy with mcp-server-kubernetes
ENTRYPOINT ["mcp-proxy", "--port", "3000", "--", "mcp-server-kubernetes"] 