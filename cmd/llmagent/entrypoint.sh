#!/bin/sh
set -e

# Start the LLM agent, passing all arguments
# Claude auth is provided via mounted ~/.claude/ volume
exec /llmagent "$@"
