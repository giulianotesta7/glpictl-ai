---
name: glpi-software
description: >
  Manage GLPI software catalog, versions, installations, and licenses.
  Trigger: When the AI needs to work with software inventory, license tracking, compliance, or software installations.
license: Apache-2.0
metadata:
  author: giulianotesta7
  version: "1.0"
---

## When to Use

- Searching the software catalog across GLPI
- Viewing software versions and their installations
- Creating, assigning, or tracking software licenses
- Checking license compliance (used vs available)
- Finding software installed on specific assets
- Tracking license expiration dates

## Critical Patterns

### Software is an itemtype, not a sub-resource
Use `glpi_search` with `itemtype="Software"` to query the catalog. Software versions use `itemtype="SoftwareVersion"` and installations use `itemtype="SoftwareLicense"`.

### Discover fields before querying software
Always call `glpi_list_fields(itemtype="Software")` first to get valid search fields and UIDs.

### License compliance requires cross-referencing
To check compliance: search `SoftwareLicense` for a software, then compare `number` (purchased) against installation count. No single tool does this — you must compose queries.

### Use include to see installed software on an asset
Call `glpi_get(itemtype="Computer", id=N, include=["software"])` to see what's installed on a specific machine.

## Tools Reference

| Tool | Purpose |
|------|---------|
| `glpi_search` | Search software catalog, versions, or licenses |
| `glpi_get` | Get software details or installed software on an asset |
| `glpi_list_fields` | Discover searchable fields for Software/SoftwareVersion/SoftwareLicense |
| `glpi_create` | Create a new software or license entry |
| `glpi_update` | Update software or license details |
| `glpi_update_by_name` | Update software by exact name |
| `glpi_global_search` | Search across software and related types at once |
| `glpi_summary` | Get inventory totals including software counts |

## Commands

```
# Discover available fields for software
glpi_list_fields(itemtype="Software")

# Search software by name
glpi_search(itemtype="Software", criteria=[{"field_name":"Software.name","searchtype":"contains","value":"chrome"}])

# Search licenses
glpi_search(itemtype="SoftwareLicense", criteria=[{"field_name":"SoftwareLicense.name","searchtype":"contains","value":"office"}])

# Get software details with versions
glpi_get(itemtype="Software", id=42)

# Get installed software on a computer
glpi_get(itemtype="Computer", id=5, include=["software"])

# Create a software entry
glpi_create(itemtype="Software", data={"name":"MyApp","comment":"Internal tool"})

# Create a license
glpi_create(itemtype="SoftwareLicense", data={"name":"Office 365","software_id":10,"number":50})

# Update software by name
glpi_update_by_name(itemtype="Software", name="Chrome", data={"comment":"Updated browser"})

# Global search across software types
glpi_global_search(query="office", itemtypes=["Software","SoftwareLicense","SoftwareVersion"])
```
