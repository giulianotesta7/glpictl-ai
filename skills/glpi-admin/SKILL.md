---
name: glpi-admin
description: >
  GLPI administration: dashboards, alerts, certificate/domain tracking, reservations, and configuration.
  Trigger: When the AI needs to work with inventory summaries, expiration alerts, certificates, domain tracking, reservations, or GLPI configuration tasks.
license: Apache-2.0
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

### Expiration tracking requires cross-type queries
There's no single "alerts" tool. Expiration monitoring requires querying each type with date-range criteria: `Certificate`, `Domain`, `SoftwareLicense`, `Contract`, and asset warranty fields. Use `glpi_list_fields` to find the date field UID for each type.

### Certificates and domains are dedicated itemtypes
Use `itemtype="Certificate"` and `itemtype="Domain"` for tracking. These are separate from computers/network equipment.

### Reservations use itemtype="ReservationItem"
Reservation tracking queries `ReservationItem` linked to assets. Use `glpi_search` with this itemtype to find reservable items and their schedules.

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
```
