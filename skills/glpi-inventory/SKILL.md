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

## Update by name

When the agent needs to update a single item using a human-friendly name:

1. Use `glpi_update_by_name` with:
   - `itemtype` — the GLPI item type
   - `name` — the exact name to match
   - `data` — fields to update

2. The tool enforces exact-match-only semantics:
   - Updates only when exactly one item has that name
   - Returns a clear error if zero matches or multiple matches
   - Never auto-selects among duplicates

3. On ambiguity, the error includes candidate IDs so the agent can disambiguate.

4. If `glpi_update_by_name` returns not-found or ambiguous, fall back to `glpi_search` to investigate and then retry with a disambiguated name or use `glpi_update` with an explicit ID.
