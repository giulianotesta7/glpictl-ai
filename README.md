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
├── cmd/glpictl-ai/
│   ├── main.go              # Entry point, tool registration, signal handling
│   ├── configure.go          # CLI configure command
│   └── version.go            # CLI version command
├── internal/
│   ├── glpi/
│   │   └── client.go         # GLPI REST API client (session mgmt)
│   ├── config/
│   │   └── config.go         # Config loader (TOML), Save(), validation
│   └── tools/                # MCP tool implementations
│       ├── search.go
│       ├── get.go
│       ├── create.go
│       ├── update.go
│       ├── delete.go
│       ├── summary.go
│       ├── global_search.go
│       ├── list_fields.go
│       ├── user_assets.go
│       ├── group_assets.go
│       ├── bulk_update.go
│       ├── update_by_name.go
│       ├── license_compliance.go
│       ├── expiration_tracker.go
│       ├── rack_capacity.go
│       ├── network_topology.go
│       ├── warranty_report.go
│       ├── cost_summary.go
│       └── ping.go
├── skills/                   # SKILL.md files for AI agents
├── install.sh                # Linux/macOS installer
├── install.ps1               # Windows installer
└── .github/workflows/
    ├── go-ci.yml             # CI: vet, test, build
    └── release.yml           # Release: cross-compile binaries + checksums
```

## Acknowledgments

This project integrates with [GLPI](https://glpi-project.org) — a Free Asset and IT Management Software package — via its REST API. GLPI is licensed under the [GNU GPL v3](https://www.gnu.org/licenses/gpl-3.0.html).

## License

MIT. See [LICENSE](LICENSE).
