# LLM Optimizer MCP Server - Docker proxy image
#
# This image runs mcp-remote to proxy stdio MCP connections to the
# hosted LLM Optimizer MCP endpoint at https://llmopt.metavert.io/mcp
#
# Usage (with API key):
#   docker run -i --rm -e API_KEY=lok_xxx ghcr.io/metavert-io/llmopt
#
# Usage (OAuth flow):
#   docker run -i --rm ghcr.io/metavert-io/llmopt
#
# MCP client config (Claude Desktop, etc.):
#   {
#     "command": "docker",
#     "args": ["run", "-i", "--rm", "-e", "API_KEY=lok_xxx", "ghcr.io/metavert-io/llmopt"]
#   }

FROM node:22-alpine
RUN npm install -g mcp-remote

ENV API_KEY=""

COPY docker-entrypoint.sh /usr/local/bin/docker-entrypoint.sh
RUN chmod +x /usr/local/bin/docker-entrypoint.sh

ENTRYPOINT ["/usr/local/bin/docker-entrypoint.sh"]
