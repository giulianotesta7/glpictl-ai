# glpi-inventory

Use this skill when querying GLPI inventory entities through MCP tools.

## Workflow (MVP)

1. **Discover fields first** with `glpi_list_fields`:
   - Input: `itemtype` (for example `Computer`, `Monitor`, `Printer`)
   - Inspect the returned `fields` list and identify the most stable field identifier.

2. **Prefer `uid` for search criteria**:
   - Use exact `uid` when available (for example `Computer.name`).
   - If no `uid` exists, use an exact technical field name.
   - Only use display names when no technical identifier is available.

3. **Search using `glpi_search` + `field_name`**:
   - Provide `field_name` and let the tool map it to the numeric GLPI field ID.
   - Do **not** guess numeric field IDs.

4. **Backward compatibility**:
   - Existing numeric `field` criteria still work.
   - Prefer `field_name` for readability and portability across environments.

## Example

1. `glpi_list_fields(itemtype="Computer")`
2. Pick `uid="Computer.name"`
3. `glpi_search(itemtype="Computer", criteria=[{"field_name":"Computer.name","searchtype":"contains","value":"laptop"}])`
