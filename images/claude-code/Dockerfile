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

# Ubuntu 24.04 ships with an 'ubuntu' user at uid 1000 — use it.
USER ubuntu
WORKDIR /home/ubuntu

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    curl \
    git \
    && rm -rf /var/lib/apt/lists/*

# Install Node.js LTS
RUN curl -fsSL https://deb.nodesource.com/setup_lts.x | bash - \
    && apt-get install -y nodejs \
    && rm -rf /var/lib/apt/lists/*

# Install claude via official install script then find where it landed
RUN curl -fsSL https://claude.ai/install.sh | bash; \
    find /root /home /usr/local /usr/bin -name "claude" -type f 2>/dev/null; \
    find /root /home -name "claude" 2>/dev/null

ENV PATH="/root/.local/bin:/usr/local/bin:/usr/bin:/bin"

RUN claude --version

COPY --from=build --chown=ubuntu:ubuntu /supervisor /supervisor

# Supervisor listens on 8080 (control) and 8081 (terminal).
EXPOSE 8080 8081

ENTRYPOINT ["/supervisor"]
