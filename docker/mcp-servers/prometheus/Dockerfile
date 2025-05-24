# Dockerfile for MCP Server Prometheus
FROM python:3.11-slim

# Install system dependencies
RUN apt-get update && apt-get install -y \
    git \
    && rm -rf /var/lib/apt/lists/*

# Create a non-root user
RUN useradd -m -s /bin/bash mcpuser

# Switch to non-root user
USER mcpuser
WORKDIR /home/mcpuser

# Install UV package manager
RUN pip install --user uv

# Clone the prometheus-mcp-server repository
RUN git clone https://github.com/giantswarm/prometheus-mcp-server.git

# Install dependencies using UV
WORKDIR /home/mcpuser/prometheus-mcp-server
RUN /home/mcpuser/.local/bin/uv pip install --user -r requirements.txt

# Install mcp-proxy via npx (requires Node.js)
USER root
RUN apt-get update && apt-get install -y \
    curl \
    && curl -fsSL https://deb.nodesource.com/setup_20.x | bash - \
    && apt-get install -y nodejs \
    && rm -rf /var/lib/apt/lists/*

USER mcpuser

# Expose the default mcp-proxy port
EXPOSE 3000

# Set default environment variables
ENV PROMETHEUS_URL=http://localhost:8080/prometheus
ENV ORG_ID=giantswarm

# Start mcp-proxy with the prometheus server
ENTRYPOINT ["npx", "mcp-proxy", "--port", "3000", "--pass-environment", "--", "/home/mcpuser/.local/bin/uv", "run", "src/prometheus_mcp_server/main.py"] 