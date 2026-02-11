#!/bin/bash

set -e

cd "$(dirname "$0")"

echo "Starting MCP server..."
./scriptling mcp serve --tools ./mcp-tool-test &
SERVER_PID=$!

# Wait for server to start
sleep 2

# Cleanup function
cleanup() {
    echo ""
    echo "Stopping server..."
    kill $SERVER_PID 2>/dev/null || true
    wait $SERVER_PID 2>/dev/null || true
}
trap cleanup EXIT

echo "Running tests..."
echo ""

# Test 1: Health check
echo "Test 1: Health check"
curl -s http://127.0.0.1:8000/health
echo ""
echo ""

# Test 2: List tools
echo "Test 2: List tools"
curl -s -X POST http://127.0.0.1:8000/mcp \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/list"}' | jq .
echo ""

# Test 3: Call hello tool with name only
echo "Test 3: Call hello tool (name only)"
curl -s -X POST http://127.0.0.1:8000/mcp \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"hello","arguments":{"name":"World"}}}' | jq .
echo ""

# Test 4: Call hello tool with name and times
echo "Test 4: Call hello tool (name and times)"
curl -s -X POST http://127.0.0.1:8000/mcp \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"hello","arguments":{"name":"Alice","times":3}}}' | jq .
echo ""

# Test 5: Call with missing required parameter (should fail)
echo "Test 5: Call with missing required parameter (should fail)"
curl -s -X POST http://127.0.0.1:8000/mcp \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"hello","arguments":{}}}' | jq .
echo ""

echo "All tests completed!"
