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

# Install claude via official install script
RUN curl -fsSL https://claude.ai/install.sh | bash

RUN claude --version

# Ubuntu 24.04 ships with an 'ubuntu' user at uid 1000 — use it.
USER ubuntu
WORKDIR /home/ubuntu

COPY --from=build --chown=ubuntu:ubuntu /supervisor /supervisor

# Supervisor listens on 8080 (control) and 8081 (terminal).
EXPOSE 8080 8081

ENTRYPOINT ["/supervisor"]
