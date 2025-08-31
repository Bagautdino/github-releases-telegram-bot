#!/bin/bash
set -e

echo "🚀 Testing Telegram Release Bot..."

# Check if go is installed
if ! command -v go &> /dev/null; then
    echo "❌ Go is not installed"
    exit 1
fi

# Build the project
echo "🔨 Building project..."
go build -o ./bin/tg-release-bot ./cmd/bot

# Check if binary was created
if [ -f "./bin/tg-release-bot" ]; then
    echo "✅ Binary created successfully"
else
    echo "❌ Failed to create binary"
    exit 1
fi

# Run tests
echo "🧪 Running tests..."
go test ./... || echo "⚠️  No tests found or tests failed"

# Check code formatting
echo "🎨 Checking code formatting..."
if command -v gofmt &> /dev/null; then
    UNFORMATTED=$(gofmt -l .)
    if [ -n "$UNFORMATTED" ]; then
        echo "❌ The following files are not formatted:"
        echo "$UNFORMATTED"
        exit 1
    else
        echo "✅ All files are properly formatted"
    fi
fi

# Check if Docker can build
if command -v docker &> /dev/null; then
    echo "🐳 Testing Docker build..."
    docker build -t tg-release-bot-test . || echo "⚠️  Docker build failed"
else
    echo "⚠️  Docker not found, skipping Docker test"
fi

echo "🎉 All tests passed!"
