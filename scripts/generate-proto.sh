#!/bin/bash
# Script to generate Protocol Buffer code
export PATH="$PATH:$(go env GOPATH)/bin"

# Install protoc if not present
if ! command -v protoc &> /dev/null; then
    echo "protoc not found. Please install Protocol Buffers compiler:"
    echo "  macOS: brew install protobuf"
    echo "  Linux: apt-get install protobuf-compiler"
    exit 1
fi

# Install Go protobuf plugin if not present
if ! command -v protoc-gen-go &> /dev/null; then
    echo "Installing protoc-gen-go..."
    go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
fi

npm ci

# Generate Go code from proto files
echo "Generating Protocol Buffer code..."
protoc --go_out=internal/protocol \
    --go_opt=paths=source_relative \
    --proto_path=internal/protocol \
    internal/protocol/messages.proto

# Generate TypeScript code with JSON support
echo "Generating TypeScript Protocol Buffer code..."
npx protoc --ts_out=internal/protocol \
    --proto_path=internal/protocol \
    internal/protocol/messages.proto
