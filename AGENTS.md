# Agent Instructions for glpictl-ai

## Project Overview

glpictl-ai is a Go-based MCP server that wraps the GLPI REST API for IT inventory management by AI agents and humans.

## Stack

- **Language**: Go 1.23+
- **MCP SDK**: github.com/mark3labs/mcp-go
- **HTTP Client**: net/http (stdlib) or github.com/go-resty/resty/v2
- **Config**: github.com/BurntSushi/toml
- **TUI (future)**: github.com/charmbracelet/bubbletea

## Architecture

```
glpictl-ai/
├── cmd/glpictl-ai/main.go      # Entry point (MCP server)
├── internal/
│   ├── glpi/client.go           # GLPI REST API client (session mgmt)
│   ├── config/config.go         # Config loader (TOML)
│   └── tools/                   # MCP tools (search, get, create, update, delete)
└── skills/                      # SKILL.md files bundled for TUI installer
    ├── glpi-inventory/SKILL.md
    ├── glpi-software/SKILL.md
    ├── glpi-infrastructure/SKILL.md
    ├── glpi-financial/SKILL.md
    ├── glpi-relations/SKILL.md
    └── glpi-admin/SKILL.md
```

## Rules

- Standard Go project layout (`cmd/`, `internal/`, etc.)
- All exported functions have GoDoc comments
- Table-driven tests are encouraged when they improve clarity for behavior-heavy or multi-scenario coverage
- Error wrapping with `fmt.Errorf("context: %w", err)`
- Commits in English, conventional commits (feat, fix, refactor, test, chore)
- Never add "Co-Authored-By" or AI attribution to commits
- All work on feature branches (feat/, fix/), NEVER on main
- Config at `~/.config/glpictl-ai/config.toml` (XDG-style)

## GLPI API Notes

- Legacy API at `http://localhost/apirest.php` (NOT v2.2)
- Auth: `user_token` + `app_token` headers on initSession
- Session token replaces user_token after initSession
- App-Token header required on ALL requests
- `killSession` to clean up

## Skills (Auto-load based on context)

| Context | Skill to Load |
|---------|---------------|
| Writing Go code | `golang-pro` |
| Building MCP server | `go-mcp-server-generator` |
| Writing Go tests | `golang-pro` (testing.md reference) |
| Building TUI | `bubbletea` (future) |
| Creating skills | `skill-creator` |
| GLPI inventory CRUD (search, get, create, update, delete) | `glpi-inventory` |
| GLPI software, licenses, compliance | `glpi-software` |
| GLPI network equipment, ports, racks, VLANs | `glpi-infrastructure` |
| GLPI contracts, costs, budgets, depreciation | `glpi-financial` |
| GLPI users, groups, entities, assignments | `glpi-relations` |
| GLPI dashboards, alerts, certificates, domains | `glpi-admin` |
