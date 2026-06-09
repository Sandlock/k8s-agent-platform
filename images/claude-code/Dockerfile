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
    && rm -rf /var/lib/apt/lists/*

# Install Node.js LTS
RUN curl -fsSL https://deb.nodesource.com/setup_lts.x | bash - \
    && apt-get install -y nodejs \
    && rm -rf /var/lib/apt/lists/*

# Install claude-code globally
RUN npm install -g @anthropic-ai/claude-code

ENV PATH="/usr/local/bin:/usr/bin:/bin:/usr/local/sbin:/usr/sbin:/sbin"

# Verify claude is accessible
RUN claude --version || true

# Run as a non-root user so claude --dangerously-skip-permissions works.
RUN useradd -m -u 1000 -s /bin/bash sandlock
USER sandlock
WORKDIR /home/sandlock

COPY --from=build --chown=sandlock:sandlock /supervisor /supervisor

# Supervisor listens on 8080 (control) and 8081 (terminal).
EXPOSE 8080 8081

ENTRYPOINT ["/supervisor"]
