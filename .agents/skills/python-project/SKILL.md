# Skill: python-project

Python coding patterns and conventions specific to glpictl-ai.

## When to Use

- Before writing ANY Python code for this project
- When structuring new modules, commands, or models
- When choosing patterns for HTTP clients, CLI commands, or data models

## Project Stack

| Tool | Purpose | Version |
|------|---------|---------|
| Python | Language | 3.12+ |
| Click | CLI framework | 8.x |
| httpx | HTTP client | 0.27+ |
| Pydantic | Data validation | 2.x |
| Rich | Terminal output | 13.x |
| uv | Package manager | latest |
| pytest | Testing | 8.x |

## Critical Patterns

### Click Command Structure

```python
import click
from rich.console import Console

console = Console()

@click.group()
def cli():
    """glpictl-ai — GLPI inventory management for AI agents."""
    pass

@cli.group()
def search():
    """Search GLPI assets."""
    pass

@search.command("computer")
@click.option("--name", "-n", help="Filter by name (partial match)")
@click.option("--serial", "-s", help="Filter by serial number")
@click.option("--location", "-l", help="Filter by location")
@click.option("--state", help="Filter by state")
@click.option("--limit", default=50, help="Max results")
@click.option("--json", "as_json", is_flag=True, help="Output as JSON")
@click.pass_context
def search_computer(ctx, name, serial, location, state, limit, as_json):
    """Search computers in GLPI inventory."""
    client = ctx.obj["client"]
    filters = {k: v for k, v in {
        "name": name, "serial": serial,
        "locations_id": location, "states_id": state
    }.items() if v is not None}

    results = client.search("Computer", filters, limit=limit)

    if as_json:
        import json
        click.echo(json.dumps(results, indent=2))
    else:
        from glpictl_ai.output import print_search_table
        print_search_table(results, "Computer")
```

### Pydantic Models

```python
from pydantic import BaseModel, Field
from typing import Optional
from datetime import datetime

class Asset(BaseModel):
    """Base model for all GLPI assets."""
    id: int
    name: str
    entities_id: str = ""
    serial: Optional[str] = None
    otherserial: Optional[str] = None
    contact: Optional[str] = None
    comment: Optional[str] = None
    date_mod: Optional[datetime] = None
    is_dynamic: bool = False
    is_deleted: bool = False

    model_config = {"populate_by_name": True}

class Computer(Asset):
    """GLPI Computer itemtype."""
    computermodels_id: Optional[str] = None
    computertypes_id: Optional[str] = None
    manufacturers_id: Optional[str] = None
    users_id: Optional[str] = None
    groups_id: Optional[str] = None
    states_id: Optional[str] = None
    locations_id: Optional[str] = None
    uuid: Optional[str] = None

class SearchResult(BaseModel):
    """Generic search result wrapper."""
    totalcount: int
    data: list[dict]
    range: str = ""
```

### HTTP Client Pattern

```python
import httpx
from contextlib import contextmanager

class GLPIClient:
    """GLPI REST API client with session management."""

    def __init__(self, base_url: str, user_token: str, app_token: str = ""):
        self.base_url = base_url.rstrip("/")
        self.api_url = f"{self.base_url}/apirest.php"
        self.user_token = user_token
        self.app_token = app_token
        self.session_token: str | None = None
        self._client: httpx.Client | None = None

    @property
    def headers(self) -> dict:
        h = {"Content-Type": "application/json"}
        if self.app_token:
            h["App-Token"] = self.app_token
        if self.session_token:
            h["Session-Token"] = self.session_token
        return h

    def connect(self):
        """Initialize session with GLPI."""
        self._client = httpx.Client(timeout=30.0)
        resp = self._client.get(
            f"{self.api_url}/initSession",
            headers={
                **self.headers,
                "Authorization": f"user_token {self.user_token}"
            }
        )
        resp.raise_for_status()
        self.session_token = resp.json()["session_token"]

    def disconnect(self):
        """Kill session."""
        if self.session_token and self._client:
            self._client.get(
                f"{self.api_url}/killSession",
                headers=self.headers
            )
        if self._client:
            self._client.close()

    def __enter__(self):
        self.connect()
        return self

    def __exit__(self, *args):
        self.disconnect()

    def get(self, endpoint: str, **params) -> dict | list:
        """GET request to GLPI API."""
        resp = self._client.get(
            f"{self.api_url}/{endpoint}",
            headers=self.headers,
            params=params
        )
        resp.raise_for_status()
        return resp.json()
```

### Rich Output

```python
from rich.console import Console
from rich.table import Table
from rich import print_json

console = Console()

def print_search_table(results: dict, itemtype: str):
    """Print search results as a Rich table."""
    table = Table(title=f"{itemtype} Search Results")
    table.add_column("ID", style="cyan", no_wrap=True)
    table.add_column("Name", style="green")
    table.add_column("Serial", style="yellow")
    table.add_column("Entity", style="blue")

    for row in results.get("data", []):
        table.add_row(
            str(row.get("2", "")),  # ID
            str(row.get("1", "")),  # Name
            str(row.get("5", "")),  # Serial (varies by type)
            str(row.get("80", "")), # Entity
        )

    console.print(table)
    console.print(f"Total: {results.get('totalcount', 0)} results")
```

## Config Loading (Priority: flags > env > toml > defaults)

```python
import os
import tomllib
from pathlib import Path
from pydantic import BaseModel

class Config(BaseModel):
    url: str = ""
    user_token: str = ""
    app_token: str = ""
    entity_id: int = 0
    output: str = "table"
    page_size: int = 50

def load_config(
    config_path: str | None = None,
    url: str | None = None,
    user_token: str | None = None,
) -> Config:
    """Load config with priority: params > env > toml > defaults."""
    # 1. TOML file
    path = Path(config_path or "~/.config/glpictl-ai/config.toml").expanduser()
    toml_cfg = {}
    if path.exists():
        with open(path, "rb") as f:
            toml_cfg = tomllib.load(f).get("glpi", {})

    # 2. Env vars override TOML
    env_cfg = {
        "url": os.environ.get("GLPI_URL"),
        "user_token": os.environ.get("GLPI_USER_TOKEN"),
        "app_token": os.environ.get("GLPI_APP_TOKEN"),
    }
    env_cfg = {k: v for k, v in env_cfg.items() if v is not None}

    # 3. CLI params override env
    param_cfg = {}
    if url:
        param_cfg["url"] = url
    if user_token:
        param_cfg["user_token"] = user_token

    # Merge: defaults <- toml <- env <- params
    merged = {**toml_cfg, **env_cfg, **param_cfg}
    return Config(**merged)
```

## Commands

```bash
# Development
uv sync                          # Install dependencies
uv run glpictl-ai --help         # Run CLI locally
uv run pytest                    # Run tests
uv run ruff check .              # Lint
uv run ruff format .             # Format

# Build
uv build                         # Build package
uv publish                       # Publish to PyPI
```
