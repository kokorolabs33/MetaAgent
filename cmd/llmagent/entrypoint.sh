#!/bin/sh
set -e

# Set up Claude auth if token is provided
if [ -n "$CLAUDE_TOKEN" ]; then
    echo "Setting up Claude authentication..."
    claude setup-token "$CLAUDE_TOKEN"
fi

# Start the LLM agent, passing all arguments
exec /llmagent "$@"
