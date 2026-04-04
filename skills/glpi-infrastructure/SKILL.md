---
name: glpi-infrastructure
description: >
  Manage GLPI network infrastructure: equipment, ports, racks, cables, VLANs, and DCIM.
  Trigger: When the AI needs to work with network equipment, ports, rack positioning, cabling, VLANs, or data center infrastructure.
license: MIT
metadata:
  author: giulianotesta7
  version: "1.0"
---

## When to Use

- Searching network equipment (switches, routers, firewalls, access points)
- Viewing or managing network ports and connections
- Managing rack positioning and slot allocation
- Cable management and trace connections
- VLAN assignment and queries
- DCIM (Data Center Infrastructure Management) tasks

## Critical Patterns

### Network equipment uses specific itemtypes
Use `itemtype="NetworkEquipment"` for switches/routers/APs. Ports use `itemtype="NetworkPort"`. Cables use `itemtype="Cable"`. VLANs use `itemtype="Vlan"`.

### Racks have a dedicated capacity tool
Use `glpi_rack_capacity` to get rack utilization (total U, used U, available U, percentage) and list equipment per rack with position info. Optionally surfaces unplaced equipment.

### Network topology has a dedicated tracing tool
Use `glpi_network_topology` to trace port connections (local port -> cable -> remote port -> remote device) or show all ports on a device with their connection status.

### Discover fields before searching infrastructure
Always call `glpi_list_fields` for the specific itemtype first — network fields have unique UIDs that differ from computers.

## Tools Reference

| Tool | Purpose |
|------|---------|
| `glpi_search` | Search equipment, ports, racks, cables, VLANs |
| `glpi_get` | Get details of equipment, port connections, rack contents |
| `glpi_list_fields` | Discover fields for infrastructure itemtypes |
| `glpi_create` | Create equipment, ports, racks, cables, VLANs |
| `glpi_update` | Update infrastructure items by ID |
| `glpi_update_by_name` | Update equipment by exact name |
| `glpi_delete` | Remove infrastructure items |
| `glpi_global_search` | Search across infrastructure types at once |
| `glpi_rack_capacity` | Get rack utilization and equipment placement |
| `glpi_network_topology` | Trace port connections and device network topology |
| `glpi_expiration_tracker` | Check hardware warranty expiration dates |
| `glpi_warranty_report` | Get detailed warranty status report for hardware |

## Commands

```
# Discover fields for network equipment
glpi_list_fields(itemtype="NetworkEquipment")

# Search switches by name
glpi_search(itemtype="NetworkEquipment", criteria=[{"field_name":"NetworkEquipment.name","searchtype":"contains","value":"switch"}])

# Get rack details with contents
glpi_get(itemtype="Rack", id=3)

# Search VLANs
glpi_search(itemtype="Vlan", criteria=[{"field_name":"Vlan.name","searchtype":"contains","value":"production"}])

# Get network port details and connections
glpi_get(itemtype="NetworkPort", id=15)

# Search cables
glpi_search(itemtype="Cable", criteria=[{"field_name":"Cable.name","searchtype":"contains","value":"rack-1"}])

# Create a VLAN
glpi_create(itemtype="Vlan", data={"name":"VLAN-100","comment":"Production network"})

# Update network equipment by name
glpi_update_by_name(itemtype="NetworkEquipment", name="SW-CORE-01", data={"comment":"Updated firmware"})

# Global search across infrastructure
glpi_global_search(query="rack-1", itemtypes=["NetworkEquipment","Rack","Cable","NetworkPort"])

# Get rack utilization and equipment placement
glpi_rack_capacity()

# Trace network port connections
glpi_network_topology(port_id=15)

# Show all ports on a device with connection status
glpi_network_topology(device_id=5, device_type="NetworkEquipment")
```
