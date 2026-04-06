package tools

import (
	"context"
	"encoding/csv"
	"fmt"
	"strconv"
	"strings"
)

// ExportInput represents the input for the glpi_export tool.
type ExportInput struct {
	ItemType string            `json:"itemtype"` // GLPI item type (e.g., Computer, Printer)
	Criteria []SearchCriterion `json:"criteria"` // Search criteria
	Fields   []string          `json:"fields"`   // Fields to export
	Limit    int               `json:"limit"`    // Max items to export (default: 1000)
}

// ExportResult represents the result of an export operation.
type ExportResult struct {
	ItemType   string `json:"itemtype"`
	TotalFound int    `json:"total_found"`
	Exported   int    `json:"exported"`
	CSV        string `json:"csv"` // CSV content as string
}

// ExportTool provides export functionality to CSV format.
type ExportTool struct {
	client        ToolClient
	fieldNameToID map[string]int // Maps field UID to numeric ID
	fieldIDToName map[int]string // Maps numeric ID to readable name
}

// NewExportTool creates a new export tool with the given client.
// Returns an error if the client is nil.
func NewExportTool(client ToolClient) (*ExportTool, error) {
	if client == nil {
		return nil, fmt.Errorf("export tool: client cannot be nil")
	}
	return &ExportTool{
		client:        client,
		fieldNameToID: make(map[string]int),
		fieldIDToName: make(map[int]string),
	}, nil
}

// Name returns the tool name for registration.
func (e *ExportTool) Name() string {
	return "glpi_export"
}

// Description returns the tool description.
func (e *ExportTool) Description() string {
	return "Export GLPI items to CSV format with optional search criteria"
}

// GetInput returns a new input struct for the tool.
func (e *ExportTool) GetInput() *ExportInput {
	return &ExportInput{}
}

// loadFieldMappings loads the field mappings for CSV header translation.
func (e *ExportTool) loadFieldMappings(ctx context.Context, itemtype string) error {
	searchOptions, err := e.client.GetSearchOptions(ctx, itemtype)
	if err != nil {
		return err
	}

	// Cache field mappings for CSV header generation
	for _, f := range searchOptions.Fields {
		e.fieldNameToID[f.UID] = f.ID
		e.fieldIDToName[f.ID] = f.Name
	}

	return nil
}

// Execute exports GLPI items to CSV format.
// Uses search to get items, then extracts data using field IDs that GLPI returns.
func (e *ExportTool) Execute(ctx context.Context, itemtype string, criteria []SearchCriterion, fields []string, limit int) (*ExportResult, error) {
	if itemtype == "" {
		return nil, fmt.Errorf("itemtype is required")
	}
	if !ValidateItemType(itemtype) {
		return nil, fmt.Errorf("invalid itemtype: %q", itemtype)
	}

	// Default limit
	if limit <= 0 {
		limit = 1000
	}

	// If no fields specified, use default (id=2, name=1 for most itemtypes)
	if len(fields) == 0 {
		fields = []string{"2", "1"} // id and name field IDs
	} else {
		// Translate common field names to numeric IDs
		translated := make([]string, len(fields))
		for i, f := range fields {
			switch f {
			case "id":
				translated[i] = "2"
			case "name":
				translated[i] = "1"
			default:
				translated[i] = f
			}
		}
		fields = translated
	}

	// Ensure we have field mappings for CSV headers
	if len(e.fieldNameToID) == 0 {
		_ = e.loadFieldMappings(ctx, itemtype) // non-fatal if fails
	}

	// Search to get items - use empty criteria if none provided
	if len(criteria) == 0 {
		criteria = []SearchCriterion{{Field: 1, SearchType: "contains", Value: ""}}
	}

	searchRange := &SearchRange{Start: 0, End: limit - 1}
	searchResult, err := (&SearchTool{client: e.client}).Execute(ctx, itemtype, criteria, fields, searchRange)
	if err != nil {
		return nil, fmt.Errorf("search items for export: %w", err)
	}

	if len(searchResult.Data) == 0 {
		return &ExportResult{
			ItemType:   itemtype,
			TotalFound: searchResult.TotalCount,
			Exported:   0,
			CSV:        "id,name\n",
		}, nil
	}

	// Build CSV - use field IDs as headers (GLPI returns data with numeric keys)
	csvContent, err := buildCSVWithNumericKeys(e.fieldIDToName, fields, searchResult.Data)
	if err != nil {
		return nil, fmt.Errorf("build CSV: %w", err)
	}

	return &ExportResult{
		ItemType:   itemtype,
		TotalFound: searchResult.TotalCount,
		Exported:   len(searchResult.Data),
		CSV:        csvContent,
	}, nil
}

// buildCSVWithNumericKeys builds CSV when GLPI returns data with numeric field ID keys.
func buildCSVWithNumericKeys(fieldIDToName map[int]string, headers []string, rows []SearchData) (string, error) {
	var sb strings.Builder
	writer := csv.NewWriter(&sb)

	// Convert numeric header IDs to readable names
	csvHeaders := make([]string, len(headers))
	for i, h := range headers {
		if num, err := strconv.Atoi(h); err == nil {
			if name, ok := fieldIDToName[num]; ok {
				csvHeaders[i] = strings.TrimPrefix(name, itemtypePrefix(name))
			} else {
				csvHeaders[i] = h
			}
		} else {
			csvHeaders[i] = h
		}
	}

	// Write header
	if err := writer.Write(csvHeaders); err != nil {
		return "", fmt.Errorf("write header: %w", err)
	}

	// Write rows - GLPI returns data with numeric field IDs as keys
	for _, row := range rows {
		record := make([]string, len(headers))
		for i, header := range headers {
			// GLPI returns data with keys like "1", "2" (numeric field IDs)
			if val, ok := row.Data[header]; ok && val != nil {
				record[i] = fmt.Sprintf("%v", val)
			}
		}
		if err := writer.Write(record); err != nil {
			return "", fmt.Errorf("write row: %w", err)
		}
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		return "", fmt.Errorf("flush CSV: %w", err)
	}

	return sb.String(), nil
}

func itemtypePrefix(name string) string {
	// Extract prefix like "Computer." from field name
	for _, prefix := range []string{"Computer.", "Monitor.", "Printer.", "NetworkEquipment.", "Peripheral.", "Phone."} {
		if strings.HasPrefix(name, prefix) {
			return prefix
		}
	}
	return ""
}

// getDefaultFields returns default field names for export and caches the field mappings.
func (e *ExportTool) getDefaultFields(ctx context.Context, itemtype string) ([]string, error) {
	// Load field mappings
	if err := e.loadFieldMappings(ctx, itemtype); err != nil {
		return nil, err
	}

	// Get the name field ID (usually 1)
	nameFieldID := 0
	for id, name := range e.fieldIDToName {
		if name == "Name" {
			nameFieldID = id
			break
		}
	}
	if nameFieldID == 0 {
		nameFieldID = 1 // fallback
	}

	// Return default fields: id (2) and name (1)
	fields := []string{"2", fmt.Sprintf("%d", nameFieldID)}

	return fields, nil
}

// buildCSV creates CSV content with proper header names.
// fieldNameToID maps field UIDs (like "name") to numeric IDs (like 1).
// fieldIDToName maps numeric IDs to readable names for CSV headers.
func buildCSV(fieldNameToID map[string]int, fieldIDToName map[int]string, headers []string, rows []SearchData) (string, error) {
	var sb strings.Builder
	writer := csv.NewWriter(&sb)

	// Convert headers to readable names for CSV
	csvHeaders := make([]string, len(headers))
	for i, h := range headers {
		// Check if it's a numeric field ID
		if num, err := strconv.Atoi(h); err == nil {
			if name, ok := fieldIDToName[num]; ok {
				// Clean up the name (remove Computer. prefix)
				csvHeaders[i] = strings.TrimPrefix(name, "Computer.")
			} else {
				csvHeaders[i] = h
			}
		} else if name, ok := fieldNameToID[h]; ok {
			// It's a UID, get the readable name
			if readableName, ok := fieldIDToName[name]; ok {
				csvHeaders[i] = strings.TrimPrefix(readableName, "Computer.")
			} else {
				csvHeaders[i] = h
			}
		} else {
			csvHeaders[i] = h
		}
	}

	// Write header
	if err := writer.Write(csvHeaders); err != nil {
		return "", fmt.Errorf("write header: %w", err)
	}

	// Write rows - GLPI returns data with field IDs as keys (e.g., "1", "2")
	for _, row := range rows {
		record := make([]string, len(headers))
		for i, header := range headers {
			// First try the header as-is
			if val, ok := row.Data[header]; ok {
				record[i] = fmt.Sprintf("%v", val)
				continue
			}
			// Then try numeric field ID (header "1" -> data["1"])
			if headerNum, err := strconv.Atoi(header); err == nil {
				key := fmt.Sprintf("%d", headerNum)
				if val, ok := row.Data[key]; ok {
					record[i] = fmt.Sprintf("%v", val)
				}
			}
			// Finally try mapping UID to ID
			if fieldID, ok := fieldNameToID[header]; ok {
				key := fmt.Sprintf("%d", fieldID)
				if val, ok := row.Data[key]; ok {
					record[i] = fmt.Sprintf("%v", val)
				}
			}
		}
		if err := writer.Write(record); err != nil {
			return "", fmt.Errorf("write row: %w", err)
		}
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		return "", fmt.Errorf("flush CSV: %w", err)
	}

	return sb.String(), nil
}

// Ensure ExportTool implements the Tool interface.
var _ Tool = (*ExportTool)(nil)
