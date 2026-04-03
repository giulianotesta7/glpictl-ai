---
name: glpi-financial
description: >
  Manage GLPI financial data: contracts, costs, budgets, depreciation, and warranties.
  Trigger: When the AI needs to work with contracts, purchase costs, budgets, asset depreciation, warranty tracking, or financial reporting.
license: Apache-2.0
metadata:
  author: giulianotesta7
  version: "1.0"
---

## When to Use

- Searching or creating contracts
- Viewing financial info on assets (purchase cost, warranty, depreciation)
- Managing budgets
- Tracking contract expiration
- Linking assets to contracts
- Financial reporting and cost analysis

## Critical Patterns

### Contracts are a top-level itemtype
Use `itemtype="Contract"` for contract CRUD. Contracts are linked to assets via the asset's `include` — not stored on the contract itself.

### Financial info lives on the asset
Purchase cost, warranty end date, and budget fields are on the asset itemtype (Computer, etc.), not on a separate financial itemtype. Use `glpi_get` with `include=["contract"]` to see contract links.

### Budgets use itemtype="Budget"
Budget tracking is separate from contracts. Use `glpi_search(itemtype="Budget")` to query budgets and link them to assets.

### Warranty reporting is a single tool call
Use `glpi_warranty_report` to get warranty status (active/expired/expiring_soon) for all hardware assets. No need to compute warranty dates manually.

### Cost summary aggregates financial data
Use `glpi_cost_summary` to get purchase costs by asset type, contract costs, and budget allocations in a single call.

### Expiration tracking is a single tool call
Use `glpi_expiration_tracker` to check all expiring items (contracts, warranties, certificates) across multiple itemtypes at once.

### Discover fields per itemtype
Financial fields have unique UIDs. Always call `glpi_list_fields` for Contract, Budget, or the specific asset type before querying.

## Tools Reference

| Tool | Purpose |
|------|---------|
| `glpi_search` | Search contracts, budgets, or financial fields on assets |
| `glpi_get` | Get contract details or asset financial info |
| `glpi_list_fields` | Discover fields for Contract, Budget, or asset types |
| `glpi_create` | Create contracts or budgets |
| `glpi_update` | Update contract or budget details |
| `glpi_update_by_name` | Update contract by exact name |
| `glpi_delete` | Remove contracts or budgets |
| `glpi_global_search` | Search across financial types at once |
| `glpi_warranty_report` | Get warranty status report for all hardware assets |
| `glpi_cost_summary` | Get cost aggregation across assets, contracts, budgets |
| `glpi_expiration_tracker` | Check contract and warranty expiration dates |

## Commands

```
# Discover fields for contracts
glpi_list_fields(itemtype="Contract")

# Search contracts by name
glpi_search(itemtype="Contract", criteria=[{"field_name":"Contract.name","searchtype":"contains","value":"maintenance"}])

# Get contract details
glpi_get(itemtype="Contract", id=7)

# Get computer with contract info
glpi_get(itemtype="Computer", id=5, include=["contract"])

# Search budgets
glpi_search(itemtype="Budget", criteria=[{"field_name":"Budget.name","searchtype":"contains","value":"2026"}])

# Create a contract
glpi_create(itemtype="Contract", data={"name":"Support Contract","begin_date":"2026-01-01","end_date":"2026-12-31","cost":"5000"})

# Update contract by name
glpi_update_by_name(itemtype="Contract", name="Support Contract", data={"cost":"5500"})

# Search contracts expiring soon (requires field discovery first)
glpi_search(itemtype="Contract", criteria=[{"field_name":"Contract.end_date","searchtype":"less","value":"2026-06-30"}])

# Global search across financial types
glpi_global_search(query="warranty", itemtypes=["Contract","Budget"])

# Get warranty status for all hardware assets
glpi_warranty_report()

# Get cost summary across all asset types
glpi_cost_summary()

# Check all expiring contracts and warranties
glpi_expiration_tracker(days_ahead=90, itemtypes=["Contract","Computer","NetworkEquipment"])
```
