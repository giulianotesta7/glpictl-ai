# glpictl-ai — Agent Instructions

## Project Overview

**glpictl-ai** is a Python CLI tool for managing GLPI IT inventory through AI agents and humans. It wraps GLPI's REST API with a clean, discoverable command-line interface.

- **Language**: Python 3.12+
- **CLI Framework**: Click
- **HTTP Client**: httpx
- **Data Validation**: Pydantic v2
- **Terminal Output**: Rich (tables, colors)
- **Config**: TOML (`~/.config/glpictl-ai/config.toml`) + env vars + CLI flags
- **Package Manager**: uv
- **Testing**: pytest

## Architecture

```
glpictl_ai/
├── __init__.py
├── cli.py              # Click CLI entrypoint + command groups
├── config.py            # Config loader (TOML → env → flags priority)
├── client.py            # GLPI REST API client (httpx, session management)
├── models.py            # Pydantic models for GLPI itemtypes
├── output.py            # Rich table/JSON/CSV formatters
└── commands/            # Command group modules
    ├── search.py        # search commands
    ├── get.py           # get/detail commands
    ├── create.py        # create commands
    ├── update.py        # update commands
    ├── delete.py        # delete commands
    └── summary.py       # summary/alerts commands
```

## Rules

### General

- Never add "Co-Authored-By" or AI attribution to commits. Use conventional commits only.
- Never build or install after changes unless explicitly asked.
- When asking a question, STOP and wait for response. Never continue or assume answers.
- Never agree with user claims without verification. Say "dejame verificar" and check code first.
- If user is wrong, explain WHY with evidence. If you were wrong, acknowledge with proof.
- Always propose alternatives with tradeoffs when relevant.

### Code Style

- Python 3.12+ with type hints everywhere
- Pydantic v2 for ALL data models (request and response)
- Use `httpx.AsyncClient` patterns even if CLI is sync (future-proof)
- Click commands: lowercase with hyphens (`search-computer`), options with hyphens (`--user-token`)
- Rich for ALL human output. JSON flag for machine output.
- No print() — use `rich.console.Console`
- Config follows priority: CLI flags > env vars > TOML file > defaults

### Naming Conventions

- GLPI itemtypes use PascalCase matching GLPI classes: `Computer`, `NetworkEquipment`, `Software`
- CLI commands use kebab-case: `search-computer`, `network-equipment`
- Python modules use snake_case: `network_equipment.py`
- Config keys use snake_case: `user_token`, `app_token`

### Git Workflow

- Branches: `feat/feature-name`, `fix/bug-name`, `docs/topic`
- Conventional commits: `feat:`, `fix:`, `docs:`, `refactor:`, `test:`, `chore:`
- Every feature gets a PR with detailed description
- No direct pushes to main
- Commits and PRs ALWAYS in English, NEVER use emojis

## Skills (Auto-load)

Load these skills BEFORE writing code when context matches:

| Context | Skill | When to load |
|---------|-------|-------------|
| Finishing a feature, committing, pushing, PR | `feature-workflow` | After completing any feature |
| Writing Python code for this project | `python-project` | Before writing any Python |
| Working with GLPI REST API | `glpi-api` | Before implementing API calls |
| Creating SKILL.md files | `skill-creator` | Before creating GLPI skill docs |

## GLPI API Context

- Base URL pattern: `{glpi_url}/apirest.php/{endpoint}`
- Auth: Session-based. Init session → get token → use token → kill session
- All requests need: `Content-Type: application/json`, `Session-Token`, optional `App-Token`
- Itemtypes: Computer, Printer, Monitor, NetworkEquipment, Peripheral, Phone, Software, Rack, Enclosure, Cable, etc.
- Search engine: `/search/{itemtype}/` with criteria array
- Pagination: `range` header (0-49 default), returns 206 Partial Content

## Language

- Spanish input → Rioplatense Spanish (voseo)
- English input → same warm energy
- Be direct, passionate, but caring about quality
