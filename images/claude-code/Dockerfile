# syntax=docker/dockerfile:1
# Base image for the claude-code sandbox pod.
# Contains the supervisor binary + claude-code CLI.

# ── Build stage: compile the supervisor ────────────────────────────────────────
FROM golang:1.26-bookworm AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -o /supervisor ./cmd/supervisor

# ── Runtime stage ──────────────────────────────────────────────────────────────
FROM ubuntu:24.04

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    curl \
    git \
    sudo \
    && rm -rf /var/lib/apt/lists/* \
    && echo "ubuntu ALL=(root) NOPASSWD: /usr/bin/apt-get, /usr/bin/apt, /usr/bin/dpkg" \
       > /etc/sudoers.d/ubuntu-apt \
    && chmod 440 /etc/sudoers.d/ubuntu-apt

# Install GitHub CLI
RUN curl -fsSL https://cli.github.com/packages/githubcli-archive-keyring.gpg \
      | dd of=/usr/share/keyrings/githubcli-archive-keyring.gpg \
    && chmod go+r /usr/share/keyrings/githubcli-archive-keyring.gpg \
    && echo "deb [arch=$(dpkg --print-architecture) signed-by=/usr/share/keyrings/githubcli-archive-keyring.gpg] https://cli.github.com/packages stable main" \
      > /etc/apt/sources.list.d/github-cli.list \
    && apt-get update && apt-get install -y gh \
    && rm -rf /var/lib/apt/lists/*

# Install Node.js LTS
RUN curl -fsSL https://deb.nodesource.com/setup_lts.x | bash - \
    && apt-get install -y nodejs \
    && rm -rf /var/lib/apt/lists/*

# Install claude globally as root so it lands in /usr/local/bin
RUN npm install -g @anthropic-ai/claude-code

COPY --from=build /supervisor /supervisor
RUN chmod +x /supervisor

RUN claude --version

# Workspace directory writable by the ubuntu user (supervisor runs as uid 1000).
RUN mkdir /workspace && chown ubuntu:ubuntu /workspace

# Drop to non-root after all installs are done.
# Ubuntu 24.04 ships with an 'ubuntu' user at uid 1000.
USER ubuntu
ENV PATH="/home/ubuntu/.local/bin:/home/ubuntu/bin:/home/ubuntu/.cargo/bin:/home/ubuntu/go/bin:/usr/local/go/bin:/home/linuxbrew/.linuxbrew/bin:$PATH"
WORKDIR /home/ubuntu

EXPOSE 8080 8081
ENTRYPOINT ["/supervisor"]
