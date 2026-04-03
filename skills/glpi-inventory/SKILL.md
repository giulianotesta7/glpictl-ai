---
name: glpi-inventory
description: >
  Query and manage GLPI inventory entities through MCP tools.
  Trigger: When the AI needs to interact with GLPI inventory items (search, get, create, update, delete, summary).
license: Apache-2.0
metadata:
  author: giulianotesta7
  version: "1.0"
---

## When to Use

- Searching across GLPI inventory types
- Getting detailed item information
- Creating, updating, or deleting inventory items
- Updating items by human-friendly name
- Bulk-updating multiple items at once
- Getting an inventory summary dashboard

## Critical Patterns

### Discover fields before searching
Always call `glpi_list_fields` first to discover searchable fields and their UIDs.

### Prefer UID for search criteria
Use exact UID when available (e.g. `Computer.name`). Do not guess numeric field IDs.

### Exact match for safe mutation
Use `glpi_update_by_name` for single-item updates. It enforces exact-match-only and never auto-selects duplicates.

### Use include for richer context
Use `glpi_get` with `include` to request related details (software, ports, contracts, etc.) before deciding next actions.

## Tools Reference

| Tool | Purpose |
|------|---------|
| `ping` | Test GLPI connection |
| `glpi_list_fields` | Discover searchable fields for an itemtype |
| `glpi_search` | Search items in a single itemtype |
| `glpi_global_search` | Search across multiple itemtypes at once |
| `glpi_get` | Get a single item with optional related details |
| `glpi_create` | Create a new item |
| `glpi_update` | Update an item by ID |
| `glpi_update_by_name` | Update a single item by exact name match |
| `glpi_bulk_update` | Update multiple items at once |
| `glpi_delete` | Delete an item by ID |
| `glpi_summary` | Dashboard with item counts by type |
| `glpi_user_assets` | Get all assets assigned to a specific user |
| `glpi_group_assets` | Get all assets assigned to a specific group |
| `glpi_license_compliance` | Get software license compliance report |
| `glpi_expiration_tracker` | Check expiration dates across multiple itemtypes |
| `glpi_rack_capacity` | Get rack utilization and equipment placement |
| `glpi_network_topology` | Trace network port connections and device topology |
| `glpi_warranty_report` | Get warranty status report for hardware assets |
| `glpi_cost_summary` | Get cost aggregation across assets, contracts, budgets |

## Commands

```
# Discover fields for Computer
glpi_list_fields(itemtype="Computer")

# Search by name
glpi_search(itemtype="Computer", criteria=[{"field_name":"Computer.name","searchtype":"contains","value":"laptop"}])

# Get item with software details
glpi_get(itemtype="Computer", id=5, include=["software"])

# Update by exact name
glpi_update_by_name(itemtype="Computer", name="PC-001", data={"comment":"updated"})

# Bulk update by name and ID
glpi_bulk_update(items=[{"itemtype":"Computer","name":"PC-001","data":{"state_id":5}},{"itemtype":"Printer","id":12,"data":{"comment":"moved"}}])

# Inventory summary
glpi_summary()

# Search users
glpi_search(itemtype="User", criteria=[{"field_name":"User.name","searchtype":"contains","value":"john"}])

# Get assets assigned to a user
glpi_user_assets(user_id=42)

# Get assets assigned to a group
glpi_group_assets(group_id=3)

# Check license compliance for software
glpi_license_compliance(software_id=10)

# Check all expiring items in the next 90 days
glpi_expiration_tracker(days_ahead=90)

# Get rack utilization
glpi_rack_capacity()

# Trace network port connections
glpi_network_topology(port_id=15)

# Get warranty status for hardware
glpi_warranty_report()

# Get cost summary
glpi_cost_summary()
```
