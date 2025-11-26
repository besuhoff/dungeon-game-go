#!/bin/bash
# Script to generate Protocol Buffer code

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

# Generate Go code from proto files
echo "Generating Protocol Buffer code..."
protoc --go_out=. --go_opt=paths=source_relative \
    internal/protocol/messages.proto

echo "Done! Generated files:"
ls -lh internal/protocol/*.pb.go
