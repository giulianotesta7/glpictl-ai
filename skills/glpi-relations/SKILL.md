---
name: glpi-relations
description: >
  Manage GLPI users, groups, entities, and asset assignments.
  Trigger: When the AI needs to work with user accounts, groups, entity switching, or asset-to-user/group assignments.
license: Apache-2.0
metadata:
  author: giulianotesta7
  version: "1.0"
---

## When to Use

- Searching users or groups in GLPI
- Finding what assets are assigned to a user or group
- Managing user-account properties
- Working with GLPI entities (multi-entity setups)
- Switching active entity context
- Querying group memberships

## Critical Patterns

### Users and groups are standard itemtypes
Use `itemtype="User"` for users and `itemtype="Group"` for groups. Standard search/get/create/update applies.

### Asset assignment uses glpi_user_assets
The dedicated `glpi_user_assets` tool retrieves all assets assigned to a user by `user_id`. Use this instead of manually searching the `User` itemtype for assignments.

### Entity switching changes query scope
GLPI is multi-entity. The active entity determines which items are visible. Use `glpi_get` on the `Entity` itemtype to inspect entities. Entity switching via the API is session-scoped.

### Discover user fields before complex queries
User fields have specific UIDs (e.g., `User.name`, `User.firstname`). Always call `glpi_list_fields(itemtype="User")` first.

### Group membership is not on the User itemtype
To find users in a group, search `Group_User` (the relationship itemtype) or use `glpi_get` on the Group with related data.

## Tools Reference

| Tool | Purpose |
|------|---------|
| `glpi_search` | Search users, groups, entities |
| `glpi_get` | Get user/group/entity details |
| `glpi_user_assets` | Get all assets assigned to a user |
| `glpi_list_fields` | Discover fields for User, Group, Entity |
| `glpi_create` | Create users or groups |
| `glpi_update` | Update user or group details |
| `glpi_update_by_name` | Update user by exact name |
| `glpi_global_search` | Search across users, groups, entities |

## Commands

```
# Discover fields for users
glpi_list_fields(itemtype="User")

# Search users by name
glpi_search(itemtype="User", criteria=[{"field_name":"User.name","searchtype":"contains","value":"john"}])

# Get user details
glpi_get(itemtype="User", id=42)

# Get all assets assigned to a user
glpi_user_assets(user_id=42)

# Search groups
glpi_search(itemtype="Group", criteria=[{"field_name":"Group.name","searchtype":"contains","value":"IT"}])

# Get group details
glpi_get(itemtype="Group", id=3)

# Search entities
glpi_search(itemtype="Entity", criteria=[{"field_name":"Entity.name","searchtype":"contains","value":"headquarters"}])

# Update user by name
glpi_update_by_name(itemtype="User", name="jdoe", data={"comment":"Transferred to IT dept"})

# Global search across relations
glpi_global_search(query="john", itemtypes=["User","Group","Entity"])
```
