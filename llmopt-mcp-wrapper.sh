#!/bin/bash
# Stdio-to-HTTP bridge for LLM Optimizer MCP server
# Used by Claude Desktop to connect to llmopt's Streamable HTTP MCP endpoint.
# Requires mcp-remote: npm install -g mcp-remote
exec mcp-remote https://llmopt.metavert.io/mcp 2>/dev/null
