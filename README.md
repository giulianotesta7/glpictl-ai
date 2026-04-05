# glpictl-ai

[![Go CI](https://github.com/giulianotesta7/glpictl-ai/actions/workflows/go-ci.yml/badge.svg)](https://github.com/giulianotesta7/glpictl-ai/actions/workflows/go-ci.yml)
[![Release](https://github.com/giulianotesta7/glpictl-ai/actions/workflows/release.yml/badge.svg)](https://github.com/giulianotesta7/glpictl-ai/actions/workflows/release.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/giulianotesta7/glpictl-ai)](https://goreportcard.com/report/github.com/giulianotesta7/glpictl-ai)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

MCP (Model Context Protocol) server that wraps the GLPI REST API for IT inventory management. Designed for AI agents and humans.

## Features

- **20 MCP tools** covering all GLPI domains: inventory, software, infrastructure, financial, admin, and relationships
- **License compliance** — single-call comparison of purchased vs installed licenses
- **Expiration tracking** — monitor certificates, domains, contracts, and warranties across 10 itemtypes
- **DCIM rack capacity** — rack utilization reports with equipment placement
- **Network topology** — trace port connections through cables
- **Warranty reporting** — hardware warranty status with configurable warning thresholds
- **Cost summary** — financial aggregation across assets, contracts, and budgets
- **Group assets** — retrieve all assets assigned to a GLPI group
- **Installation scripts** — one-command setup for Linux, macOS, and Windows
- **Automatic MCP client setup** — configure OpenCode, Claude Code, and Claude Desktop with one command
- **Embedded GLPI skills** — AI agents get domain-specific instructions for all 6 GLPI domains

## Prerequisites

### GLPI Instance

You need a running GLPI instance (version 10.0+) with the **API enabled**. The API is not enabled by default.

### Enable the API in GLPI

> **Note**: glpictl-ai uses the **GLPI REST API (legacy)** at `/apirest.php`. This is the default API available in GLPI 10.x.

1. Log in to GLPI as a **Super-Admin**
2. Go to **Setup** → **General** → **API** tab
3. Set **Enable API login** to **Yes**
4. Click **Save**

### Create an API Client (App Token)

1. In the same API settings page, scroll to **API clients**
2. Click **Add an API client**
3. Fill in:
   - **Name**: `glpictl-ai` (or whatever you prefer)
   - **IPv4 range**: `127.0.0.1-255.255.255.255` (or your network range)
   - **IPv6 address**: leave blank or set as needed
4. Click **Save**
5. Copy the **App-Token** value — you'll need it for configuration

### Generate a User Token

1. Go to **Preferences** (click your username in the top-right corner)
2. Scroll to the **Personal access tokens** section
3. Click **Add a token**
4. Copy the generated **User Token** — you'll need it for configuration

## Quick Start

### Install

```bash
# Linux/macOS
curl -fsSL https://raw.githubusercontent.com/giulianotesta7/glpictl-ai/main/install.sh | bash

# Windows (PowerShell)
Invoke-Expression (Invoke-WebRequest -Uri "https://raw.githubusercontent.com/giulianotesta7/glpictl-ai/main/install.ps1").Content
```

### Configure

```bash
glpictl-ai configure
```

Or non-interactively:

```bash
glpictl-ai configure \
  --url http://your-glpi/apirest.php \
  --app-token "your-app-token" \
  --user-token "your-user-token"
```

Or via environment variables:

```bash
export GLPICTL_GLPI_URL=http://your-glpi/apirest.php
export GLPICTL_APP_TOKEN="your-app-token"
export GLPICTL_USER_TOKEN="your-user-token"
glpictl-ai configure
```

### Verify

```bash
glpictl-ai version
glpictl-ai ping
```

### Set Up MCP Clients

```bash
glpictl-ai setup-mcp
```

Select which clients to configure (OpenCode, Claude Code, Claude Desktop). The installer runs this automatically after configuration.

## MCP Tools

### Inventory

| Tool | Purpose |
|------|---------|
| `glpi_search` | Search items in a single itemtype |
| `glpi_get` | Get a single item with optional related details |
| `glpi_create` | Create a new item |
| `glpi_update` | Update an item by ID |
| `glpi_update_by_name` | Update a single item by exact name match |
| `glpi_bulk_update` | Update multiple items at once |
| `glpi_delete` | Delete an item by ID |
| `glpi_list_fields` | Discover searchable fields for an itemtype |

### Specialized

| Tool | Purpose |
|------|---------|
| `glpi_license_compliance` | Get software license compliance report (purchased vs installed) |
| `glpi_expiration_tracker` | Check expiration dates across 10 itemtypes |
| `glpi_group_assets` | Get all assets assigned to a GLPI group |
| `glpi_user_assets` | Get all assets assigned to a user |
| `glpi_rack_capacity` | Get rack utilization and equipment placement |
| `glpi_network_topology` | Trace network port connections and device topology |
| `glpi_warranty_report` | Get warranty status report for hardware assets |
| `glpi_cost_summary` | Get cost aggregation across assets, contracts, budgets |

### Utility

| Tool | Purpose |
|------|---------|
| `glpi_summary` | Dashboard with item counts by type |
| `glpi_global_search` | Search across multiple itemtypes at once |
| `glpi_ping` | Test GLPI connection |

## MCP Client Integration

After installing and configuring GLPI, the installer will prompt you to set up MCP clients automatically.

You can also run it manually at any time:

```bash
glpictl-ai setup-mcp
```

This interactive tool lets you select which clients to configure. For each client it:

1. Writes the MCP server config with your GLPI credentials
2. Installs 6 GLPI skills so the AI agent knows how to use each domain:
   - **glpi-inventory** — search, get, create, update, delete
   - **glpi-software** — licenses, compliance, auditing
   - **glpi-infrastructure** — network equipment, ports, racks, VLANs
   - **glpi-financial** — contracts, costs, budgets, depreciation
   - **glpi-relations** — users, groups, entities, assignments
   - **glpi-admin** — dashboards, alerts, certificates, domains

Supported clients:

| Client | MCP Config | Skills |
|--------|-----------|--------|
| **OpenCode** | `~/.config/opencode/opencode.json` | `~/.config/opencode/skills/glpictl-ai/` |
| **Claude Code** | `~/.claude/settings.json` | `~/.claude/skills/glpictl-ai/` |
| **Claude Desktop** | `~/.config/claude/claude_desktop_config.json` | Not supported |

> **Tip**: The setup reads your existing GLPI config from `~/.config/glpictl-ai/config.toml` automatically — no need to re-enter tokens.

## Example Usage

Once connected, you can ask your AI agent natural language questions about your IT infrastructure:

### Simple Query

> **You:** "Show me all computers assigned to the IT department"
>
> **Agent:** Calls `glpi_global_search` with `itemtype=Computer`, filters by group → returns 12 computers with status, serial numbers, and assigned users.

### License Compliance

> **You:** "What's our Microsoft Office license compliance status?"
>
> **Agent:** Calls `glpi_search` for Software named "Microsoft Office" → gets software ID → calls `glpi_license_compliance` with that ID → returns: "You have 50 licenses purchased, 63 installations detected — **13 over-licensed**."

### Infrastructure

> **You:** "Which racks are running out of space?"
>
> **Agent:** Calls `glpi_rack_capacity` → returns: "Rack A-03 is at 92% (42/46U), Rack B-01 at 88% (39/44U). 3 servers are unplaced and don't fit in any rack."

### Multi-Step Analysis

> **You:** "Generate a warranty report for all hardware expiring in the next 90 days and tell me the total replacement cost"
>
> **Agent:**
> 1. Calls `glpi_warranty_report` with `days_warning=90`
> 2. Gets 8 items with expiring warranties (3 computers, 2 monitors, 1 printer, 2 network switches)
> 3. Calls `glpi_cost_summary` to aggregate purchase values
> 4. Returns: "8 items expiring soon. Total replacement cost: **$14,250** (computers: $9,800, monitors: $1,200, printer: $450, switches: $2,800)"

### Network Troubleshooting

> **You:** "Trace the network path from the web server to the core switch"
>
> **Agent:** Calls `glpi_search` for the web server → gets device ID → calls `glpi_network_topology` with that ID → returns the full cable path through patch panels and intermediate switches.

## Configuration

Config is stored at `~/.config/glpictl-ai/config.toml`:

```toml
[glpi]
url = "http://your-glpi/apirest.php"
app_token = "your-app-token"
user_token = "your-user-token"
timeout = 30
insecure_ssl = false
```

### Environment Variables

| Variable | Description |
|----------|-------------|
| `GLPICTL_GLPI_URL` | GLPI API URL |
| `GLPICTL_APP_TOKEN` | GLPI application token |
| `GLPICTL_USER_TOKEN` | GLPI user token |
| `GLPICTL_TIMEOUT` | HTTP timeout in seconds (default: 30) |
| `GLPICTL_INSECURE_SSL` | Skip TLS verification (default: false) |

Priority: **flags > env vars > interactive prompts**

## Build from Source

```bash
git clone https://github.com/giulianotesta7/glpictl-ai.git
cd glpictl-ai
go build -o glpictl-ai ./cmd/glpictl-ai
```

With version info:

```bash
go build -ldflags "-s -w -X main.version=v1.0.0" -o glpictl-ai ./cmd/glpictl-ai
```

## Test

```bash
go vet ./...
go test -race ./...
```

## Architecture

```
glpictl-ai/
├── cmd/glpictl-ai/       # CLI entry point, configure, setup-mcp
├── internal/
│   ├── glpi/             # GLPI REST API client (session mgmt, auto-reconnect)
│   ├── config/           # Config loader (TOML, env vars, CLI flags)
│   └── tools/            # MCP tool implementations
├── skills/               # GLPI domain skills for AI agents
├── install.sh            # Linux/macOS installer
└── install.ps1           # Windows installer
```

## Acknowledgments

This project integrates with [GLPI](https://glpi-project.org) — a Free Asset and IT Management Software package — via its REST API. GLPI is licensed under the [GNU GPL v3](https://www.gnu.org/licenses/gpl-3.0.html).

## License

MIT. See [LICENSE](LICENSE).
