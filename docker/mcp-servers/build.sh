#!/bin/bash
# Build script for MCP server Docker images

set -e

# Configuration
REGISTRY="${REGISTRY:-ghcr.io/giantswarm}"
VERSION="${VERSION:-latest}"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${YELLOW}Building MCP Server Docker images...${NC}"

# List of servers to build
SERVERS=("kubernetes" "prometheus" "grafana")

# Build each server
for server in "${SERVERS[@]}"; do
    echo -e "\n${GREEN}Building $server...${NC}"
    
    if [ -d "$server" ] && [ -f "$server/Dockerfile" ]; then
        IMAGE_NAME="$REGISTRY/mcp-server-$server:$VERSION"
        
        docker build -t "$IMAGE_NAME" "$server/"
        
        if [ $? -eq 0 ]; then
            echo -e "${GREEN}✓ Successfully built $IMAGE_NAME${NC}"
        else
            echo -e "${RED}✗ Failed to build $IMAGE_NAME${NC}"
            exit 1
        fi
    else
        echo -e "${YELLOW}⚠ Skipping $server - Dockerfile not found${NC}"
    fi
done

echo -e "\n${GREEN}All builds completed successfully!${NC}"

# Optional: Push images if PUSH=true
if [ "$PUSH" = "true" ]; then
    echo -e "\n${YELLOW}Pushing images to registry...${NC}"
    for server in "${SERVERS[@]}"; do
        IMAGE_NAME="$REGISTRY/mcp-server-$server:$VERSION"
        echo -e "Pushing $IMAGE_NAME..."
        docker push "$IMAGE_NAME"
    done
    echo -e "${GREEN}All images pushed successfully!${NC}"
fi 
