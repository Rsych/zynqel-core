FROM node:22-bookworm-slim

# Install Qwen Code CLI globally
RUN npm install -g @qwen-code/qwen-code@latest && \
    npm cache clean --force

# Install common tools
RUN apt-get update && \
    apt-get install -y --no-install-recommends \
        git \
        curl \
        ca-certificates \
        jq \
        procps \
    && rm -rf /var/lib/apt/lists/*

# Create workspace directory
RUN mkdir -p /workspace
WORKDIR /workspace

# Keep container alive for exec
CMD ["sleep", "infinity"]
