# Build stage
FROM golang:1.21-bullseye AS builder

# Install build dependencies for CGO (SQLite)
RUN apt-get update && apt-get install -y gcc libc6-dev

WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the source code
COPY . .

# Build the application with CGO enabled
ENV CGO_ENABLED=1
RUN go build -o installer cmd/main.go


# Runtime stage
FROM python:3.11-slim-bullseye

WORKDIR /opt/installer

# Install ansible and SSH client
RUN apt-get update && \
    apt-get install -y openssh-client sshpass && \
    pip install ansible-core && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/*

# Copy the binary from builder
COPY --from=builder /app/installer /usr/local/bin/installer

# Copy playbooks and other necessary files
COPY playbooks ./playbooks

# Create directory for SSH keys
RUN mkdir -p /root/.ssh && chmod 700 /root/.ssh

# Expose the default port
EXPOSE 9091

# Run the installer
CMD ["installer"]


