# Skill: glpi-api

GLPI REST API interaction patterns for glpictl-ai. Reference for implementing all API calls.

## When to Use

- Before implementing any GLPI API endpoint
- When adding new commands that talk to GLPI
- When debugging API responses
- When handling errors from GLPI

## API Base

```
{GLPI_URL}/apirest.php/{endpoint}
```

## Authentication Flow

```python
# 1. Init session (get session_token)
GET /initSession
Headers:
  Authorization: user_token {token}
  App-Token: {optional}
  Content-Type: application/json

# Response: {"session_token": "abc123..."}

# 2. Use session_token for ALL subsequent calls
Headers:
  Session-Token: {session_token}
  App-Token: {optional}
  Content-Type: application/json

# 3. Kill session when done
GET /killSession
```

## Core Endpoints

### Get Single Item

```
GET /:itemtype/:id
Params:
  expand_dropdowns=false    # Show names instead of IDs
  with_devices=false        # Include hardware components
  with_disks=false          # Include filesystems (Computer only)
  with_softwares=false      # Include installed software (Computer only)
  with_connections=false    # Include connected peripherals
  with_networkports=false   # Include network port details
  with_infocoms=false       # Include financial info
  with_contracts=false      # Include contracts
  with_tickets=false        # Include linked tickets
  with_logs=false           # Include change history
  add_keys_names=[]         # Friendly names for foreign keys
```

### Get All Items

```
GET /:itemtype/
Params:
  range=0-49                # Pagination (default 50 items)
  expand_dropdowns=false
  sort=1                    # Sort by searchOption ID
  order=ASC                 # ASC or DESC
  searchText[name]=laptop   # Quick filter
  is_deleted=false          # Include trashed items
```

Returns 206 Partial Content with headers:
- `Content-Range: 0-49/200` (offset-end/total)
- `Accept-Range: 990` (max for this itemtype)

### Search (Engine)

```
GET /search/:itemtype/
Params:
  criteria[0][field]=1                  # searchOption ID (1=name)
  criteria[0][searchtype]=contains      # contains, equals, notequals, lessthan, morethan
  criteria[0][value]=laptop             # Search value
  criteria[1][link]=AND                 # AND, OR, AND NOT, OR NOT
  criteria[1][field]=5                  # 5=serial
  criteria[1][searchtype]=equals
  criteria[1][value]=XYZ123
  range=0-49
  forcedisplay[0]=1                     # Force columns to display
  forcedisplay[1]=5
  forcedisplay[2]=80
  sort=1
  order=ASC
  uid_cols=true                         # Use UIDs instead of numeric IDs
```

Response:
```json
{
  "totalcount": 42,
  "range": "0-49",
  "data": [
    {"1": "laptop-name", "5": "XYZ123", "80": "Root Entity"},
    ...
  ]
}
```

### Create Item

```
POST /:itemtype/
Body: {"input": {"name": "New Computer", "serial": "ABC123"}}
Response: 201 {"id": 42}

# Bulk create:
Body: {"input": [{"name": "PC1"}, {"name": "PC2"}]}
Response: 207 [{"id": 1}, {"id": 2}]
```

### Update Item

```
PUT /:itemtype/:id
Body: {"input": {"states_id": 2, "users_id": 15}}
Response: 200 [{"42": true}]
```

### Delete Item

```
DELETE /:itemtype/:id
Params: force_purge=false   # false=trash, true=permanent
Response: 204 (single) or 200 (multiple)
```

## Search Options (by itemtype)

These are the field IDs for the search engine. Common ones:

### Computer
| ID | Field | UID |
|----|-------|-----|
| 1 | Name | Computer.name |
| 2 | ID | Computer.id |
| 3 | Location | Computer.Location.completename |
| 4 | Type | Computer.computertypes_id |
| 5 | Serial | Computer.serial |
| 19 | Manufacturer | Computer.manufacturers_id |
| 23 | Model | Computer.computermodels_id |
| 31 | User | Computer.users_id |
| 71 | Comment | Computer.comment |
| 80 | Entity | Entity.completename |

Use `GET /listSearchOptions/:itemtype` to discover ALL search options at runtime.

## Error Handling

| Code | Meaning | Action |
|------|---------|--------|
| 200 | OK | Process response |
| 201 | Created | Read new ID |
| 204 | No Content | Success, no body |
| 206 | Partial Content | Check Content-Range header for pagination |
| 400 | Bad Request | Check parameters |
| 401 | Unauthorized | Re-init session |
| 404 | Not Found | Item doesn't exist or wrong endpoint |
| 500 | Server Error | Retry or report |

## Pagination Pattern

```python
def get_all_items(client, itemtype: str, **filters) -> list:
    """Fetch ALL items using automatic pagination."""
    all_items = []
    offset = 0
    limit = 50

    while True:
        resp = client.search(
            itemtype,
            criteria=filters,
            range=f"{offset}-{offset + limit - 1}"
        )
        data = resp.get("data", [])
        all_items.extend(data)

        total = resp.get("totalcount", 0)
        offset += limit
        if offset >= total:
            break

    return all_items
```

## GLPI Itemtypes (for CLI)

| CLI name | GLPI itemtype | Description |
|----------|--------------|-------------|
| computer | Computer | PCs, servers, laptops |
| printer | Printer | Printers |
| monitor | Monitor | Screens |
| network-equipment | NetworkEquipment | Switches, routers |
| peripheral | Peripheral | Keyboards, mice, webcams |
| phone | Phone | Mobile and desk phones |
| rack | Rack | Server racks |
| enclosure | Enclosure | Blade chassis |
| pdu | PDU | Power distribution |
| cable | Cable | Network/power cables |
| software | Software | Applications |
| cartridge | Cartridge | Toner, ink cartridges |
| consumable | Consumable | Paper, generic supplies |
| sim | Line | SIM cards |

## Gotchas

- GET requests MUST have empty body (params in URL only)
- Sessions are read-only by default, add `session_write=true` for writes
- `searchText` is NOT the same as search engine criteria
- `expand_dropdowns` converts dropdown IDs to names (useful for display)
- Field IDs vary by itemtype — always use `listSearchOptions` to discover
- `&nbsp;` in responses means empty/null in GLPI
- `is_dynamic=true` means item was created by automatic inventory
- Pagination 206 response is NORMAL, not an error
