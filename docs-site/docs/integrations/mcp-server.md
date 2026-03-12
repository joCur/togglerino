---
sidebar_position: 1
title: MCP Server
---

# MCP Server

The `@togglerino/mcp` package is a [Model Context Protocol](https://modelcontextprotocol.io/) (MCP) server that lets AI assistants manage feature flags in Togglerino.

## Installation

```bash
npm install -g @togglerino/mcp
```

Or use it directly with `npx`:

```bash
npx @togglerino/mcp
```

## Configuration

The MCP server requires two environment variables:

| Variable | Required | Description |
|----------|----------|-------------|
| `TOGGLERINO_URL` | Yes | Base URL of your Togglerino instance (e.g., `http://localhost:8080`) |
| `TOGGLERINO_API_KEY` | Yes | A personal access token (PAT) for authentication |
| `TOGGLERINO_PROJECT` | No | Default project key — tools will use this when no `projectKey` is provided |

### Creating a PAT

1. Log in to the Togglerino dashboard
2. Go to **Account** (top-right menu)
3. Under **API Tokens**, click **Create Token**
4. Copy the token — it is only shown once

See the [authentication docs](../api-reference/authentication.md#personal-access-tokens-programmatic-access) for more details.

## AI Assistant Setup

### Claude Code

Add to your project's `.claude/settings.json` or `~/.claude/settings.json`:

```json
{
  "mcpServers": {
    "togglerino": {
      "command": "npx",
      "args": ["@togglerino/mcp"],
      "env": {
        "TOGGLERINO_URL": "http://localhost:8080",
        "TOGGLERINO_API_KEY": "pat_your_token_here",
        "TOGGLERINO_PROJECT": "my-project"
      }
    }
  }
}
```

### Claude Desktop

Add to your Claude Desktop MCP config:

```json
{
  "mcpServers": {
    "togglerino": {
      "command": "npx",
      "args": ["@togglerino/mcp"],
      "env": {
        "TOGGLERINO_URL": "http://localhost:8080",
        "TOGGLERINO_API_KEY": "pat_your_token_here",
        "TOGGLERINO_PROJECT": "my-project"
      }
    }
  }
}
```

### Other MCP Clients

The server uses stdio transport. Run `togglerino-mcp` (or `npx @togglerino/mcp`) as a subprocess with the required environment variables.

## Available Tools

| Tool | Description |
|------|-------------|
| `list_projects` | List all projects |
| `list_flags` | List flags in a project (with optional search/tag filters) |
| `get_flag` | Get flag details including all environment configurations |
| `create_flag` | Create a new feature flag |
| `update_flag` | Update flag metadata (name, description, tags) |
| `toggle_flag` | Enable or disable a flag in a specific environment |
| `update_flag_config` | Update environment-specific config (targeting rules, rollout, default variant) |
| `list_environments` | List all environments in a project |
| `list_segments` | List targeting segments in a project |
| `get_segment` | Get segment details including conditions |

### Project Resolution

Most tools accept an optional `projectKey` parameter. If omitted, the server uses the `TOGGLERINO_PROJECT` environment variable. If neither is set, the tool returns an error suggesting you call `list_projects` first.

## Permissions

The MCP server inherits the permissions of the PAT's owner. The user must have appropriate project roles to perform actions:

- **Viewer**: Can list and read flags, environments, segments
- **Editor**: Can create/update flags, toggle flags, manage segments
- **Admin**: Full project access including settings

Environment-level access restrictions also apply — if a user's role doesn't have write access to an environment, toggle and config update operations for that environment will be denied.

## Example Usage

Once configured, ask your AI assistant to:

- "List all feature flags in the project"
- "Create a boolean release flag called `new-checkout-flow`"
- "Enable the `dark-mode` flag in the staging environment"
- "Add a targeting rule to roll out `new-checkout-flow` to 25% of users in production"
