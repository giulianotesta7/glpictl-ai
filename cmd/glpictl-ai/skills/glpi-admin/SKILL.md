---
name: glpi-admin
description: >
  GLPI administration: dashboards, alerts, certificate/domain tracking, reservations, and configuration.
  Trigger: When the AI needs to work with inventory summaries, expiration alerts, certificates, domain tracking, reservations, or GLPI configuration tasks.
license: MIT
metadata:
  author: giulianotesta7
  version: "1.0"
---

## When to Use

- Getting inventory dashboard summaries (totals by asset type)
- Checking expiring certificates, domains, licenses, warranties, or contracts
- Tracking reservation items
- Administrative configuration queries
- Grouped reporting by entity or state
- Alert-driven workflows (expiration monitoring)

## Critical Patterns

### glpi_summary gives the dashboard overview
The `glpi_summary` tool returns item counts grouped by itemtype. Use this as the starting point for admin tasks — it reveals what's in the inventory without querying each type individually.

### Expiration tracking is a single tool call
Use `glpi_expiration_tracker(days_ahead=N)` to check all expiring items at once across Certificate, Domain, Contract, SoftwareLicense, and hardware warranties. No need to compose multiple queries manually.

### Summary by entity requires filtering
To get per-entity summaries, use `glpi_search` with entity criteria on each itemtype, or use `glpi_get` on `Entity` with related data.

## Tools Reference

| Tool | Purpose |
|------|---------|
| `glpi_summary` | Dashboard with item counts by type |
| `glpi_search` | Search certificates, domains, reservations, or any itemtype |
| `glpi_get` | Get detailed info on certificates, domains, entities |
| `glpi_list_fields` | Discover fields for Certificate, Domain, ReservationItem |
| `glpi_global_search` | Search across admin types at once |
| `glpi_create` | Create certificates, domains, or reservation entries |
| `glpi_update` | Update admin items |
| `glpi_update_by_name` | Update items by exact name |
| `glpi_expiration_tracker` | Check all expiring items across multiple itemtypes |
| `glpi_cost_summary` | Get cost aggregation across assets, contracts, budgets |

## Commands

```
# Get inventory dashboard
glpi_summary()

# Discover fields for certificates
glpi_list_fields(itemtype="Certificate")

# Search expiring certificates
glpi_search(itemtype="Certificate", criteria=[{"field_name":"Certificate.date_expiration","searchtype":"less","value":"2026-06-30"}])

# Discover fields for domains
glpi_list_fields(itemtype="Domain")

# Search expiring domains
glpi_search(itemtype="Domain", criteria=[{"field_name":"Domain.date_expiration","searchtype":"less","value":"2026-12-31"}])

# Search reservation items
glpi_search(itemtype="ReservationItem", criteria=[{"field_name":"ReservationItem.is_active","searchtype":"equals","value":"1"}])

# Get entity details
glpi_get(itemtype="Entity", id=1)

# Create a certificate entry
glpi_create(itemtype="Certificate", data={"name":"*.example.com","serial":"ABC123","date_expiration":"2027-01-15"})

# Global search across admin types
glpi_global_search(query="expiring", itemtypes=["Certificate","Domain","SoftwareLicense","Contract"])

# Check all expiring items in the next 90 days
glpi_expiration_tracker(days_ahead=90)

# Check expiring items for a specific entity
glpi_expiration_tracker(days_ahead=30, entity_id=5)
```
