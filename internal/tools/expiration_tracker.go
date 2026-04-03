package tools

import (
	"context"
	"fmt"
	"math"
	"sort"
	"sync"
	"time"
)

// ExpirationReport is the top-level result for glpi_expiration_tracker.
type ExpirationReport struct {
	GeneratedAt   string                   `json:"generated_at"`
	DaysAhead     int                      `json:"days_ahead"`
	TotalExpiring int                      `json:"total_expiring"`
	ByItemtype    map[string][]ExpiredItem `json:"by_itemtype"`
	HasErrors     bool                     `json:"has_errors"`
	ErrorMessages []string                 `json:"error_messages,omitempty"`
}

// ExpiredItem represents a single item with expiration data.
type ExpiredItem struct {
	ID              int    `json:"id"`
	Name            string `json:"name"`
	ExpirationDate  string `json:"expiration_date"`
	DaysUntilExpiry int    `json:"days_until_expiry"`
	Itemtype        string `json:"itemtype"`
	EntityName      string `json:"entity_name,omitempty"`
	IsComputed      bool   `json:"is_computed"`
}

// ExpirationField is a registry entry mapping an itemtype to its expiration field config.
type ExpirationField struct {
	ItemType     string // GLPI itemtype (e.g., "Certificate")
	DateFieldUID string // Search UID for the date field (e.g., "Certificate.date_expiration")
	SearchType   string // "direct" or "computed"
	// For computed warranties:
	WarrantyDateFieldUID     string // e.g., "Computer.warranty_date" (only if computed)
	WarrantyDurationFieldUID string // e.g., "Computer.warranty_duration" (only if computed)
}

// ExpirationTrackerTool provides expiration checking across multiple GLPI itemtypes.
type ExpirationTrackerTool struct {
	client ToolClient
}

// NewExpirationTrackerTool creates a new expiration tracker tool.
// Returns an error if the client is nil.
func NewExpirationTrackerTool(client ToolClient) (*ExpirationTrackerTool, error) {
	if client == nil {
		return nil, fmt.Errorf("expiration tracker tool: client cannot be nil")
	}
	return &ExpirationTrackerTool{client: client}, nil
}

// Name returns the tool name for registration.
func (e *ExpirationTrackerTool) Name() string {
	return "glpi_expiration_tracker"
}

// Description returns the tool description.
func (e *ExpirationTrackerTool) Description() string {
	return "Check expiration dates across multiple GLPI itemtypes (certificates, domains, contracts, software licenses, hardware warranties) and return a consolidated report"
}

// ExpirationTrackerInput represents the input for the glpi_expiration_tracker tool.
type ExpirationTrackerInput struct {
	DaysAhead int      `json:"days_ahead"`
	Itemtypes []string `json:"itemtypes,omitempty"`
	EntityID  int      `json:"entity_id,omitempty"`
}

// GetInput returns a new input struct for the tool.
func (e *ExpirationTrackerTool) GetInput() *ExpirationTrackerInput {
	return &ExpirationTrackerInput{}
}

// defaultExpirationItemtypes is the sorted list of all default itemtypes checked for expiration.
// Kept in sync with expirationFieldRegistry to ensure deterministic iteration order.
var defaultExpirationItemtypes = []string{
	"Certificate",
	"Computer",
	"Contract",
	"Domain",
	"Monitor",
	"NetworkEquipment",
	"Peripheral",
	"Phone",
	"Printer",
	"SoftwareLicense",
}

// expirationFieldRegistry maps itemtypes to their expiration field configurations.
var expirationFieldRegistry = map[string]ExpirationField{
	"Certificate": {
		ItemType:     "Certificate",
		DateFieldUID: "Certificate.date_expiration",
		SearchType:   "direct",
	},
	"Domain": {
		ItemType:     "Domain",
		DateFieldUID: "Domain.date_expiration",
		SearchType:   "direct",
	},
	"Contract": {
		ItemType:     "Contract",
		DateFieldUID: "Contract.end_date",
		SearchType:   "direct",
	},
	"SoftwareLicense": {
		ItemType:     "SoftwareLicense",
		DateFieldUID: "SoftwareLicense.expiration",
		SearchType:   "direct",
	},
	"Computer": {
		ItemType:                 "Computer",
		SearchType:               "computed",
		WarrantyDateFieldUID:     "Computer.warranty_date",
		WarrantyDurationFieldUID: "Computer.warranty_duration",
	},
	"Monitor": {
		ItemType:                 "Monitor",
		SearchType:               "computed",
		WarrantyDateFieldUID:     "Monitor.warranty_date",
		WarrantyDurationFieldUID: "Monitor.warranty_duration",
	},
	"Printer": {
		ItemType:                 "Printer",
		SearchType:               "computed",
		WarrantyDateFieldUID:     "Printer.warranty_date",
		WarrantyDurationFieldUID: "Printer.warranty_duration",
	},
	"NetworkEquipment": {
		ItemType:                 "NetworkEquipment",
		SearchType:               "computed",
		WarrantyDateFieldUID:     "NetworkEquipment.warranty_date",
		WarrantyDurationFieldUID: "NetworkEquipment.warranty_duration",
	},
	"Phone": {
		ItemType:                 "Phone",
		SearchType:               "computed",
		WarrantyDateFieldUID:     "Phone.warranty_date",
		WarrantyDurationFieldUID: "Phone.warranty_duration",
	},
	"Peripheral": {
		ItemType:                 "Peripheral",
		SearchType:               "computed",
		WarrantyDateFieldUID:     "Peripheral.warranty_date",
		WarrantyDurationFieldUID: "Peripheral.warranty_duration",
	},
}

// Execute performs expiration checking across multiple GLPI itemtypes concurrently.
// The daysAhead parameter is required and must be a positive integer.
// The itemtypes parameter is optional; if empty, all 10 supported types are queried.
// The entityID parameter is optional; if > 0, results are filtered to that entity.
// Returns a partial report even if some itemtype queries fail.
func (e *ExpirationTrackerTool) Execute(ctx context.Context, daysAhead int, itemtypes []string, entityID int) (*ExpirationReport, error) {
	if daysAhead <= 0 {
		return nil, fmt.Errorf("days_ahead must be a positive integer")
	}

	// Determine target itemtypes: default to all registry types if none specified.
	targetTypes := itemtypes
	if len(targetTypes) == 0 {
		targetTypes = make([]string, len(defaultExpirationItemtypes))
		copy(targetTypes, defaultExpirationItemtypes)
	}

	// Filter out invalid itemtypes silently.
	validTypes := make([]string, 0, len(targetTypes))
	for _, it := range targetTypes {
		if _, ok := expirationFieldRegistry[it]; ok {
			validTypes = append(validTypes, it)
		}
	}

	// Compute cutoff date.
	cutoffDate := time.Now().UTC().AddDate(0, 0, daysAhead)
	cutoffStr := cutoffDate.Format("2006-01-02")

	// Shared data structures for concurrent aggregation.
	results := make(map[string][]ExpiredItem)
	var mu sync.Mutex
	var firstErr error
	var errMu sync.Mutex
	var errorMessages []string
	var errMsgMu sync.Mutex

	var wg sync.WaitGroup

	for _, itemtype := range validTypes {
		wg.Add(1)
		go func(it string) {
			defer wg.Done()

			field := expirationFieldRegistry[it]
			var items []ExpiredItem
			var queryErr error

			if field.SearchType == "direct" {
				items, queryErr = e.queryDirectDate(ctx, it, field, cutoffStr, entityID)
			} else {
				items, queryErr = e.queryComputedWarranty(ctx, it, field, cutoffDate, entityID)
			}

			if queryErr != nil {
				errMu.Lock()
				if firstErr == nil {
					firstErr = fmt.Errorf("expiration tracker [%s]: %w", it, queryErr)
				}
				errMu.Unlock()

				errMsgMu.Lock()
				errorMessages = append(errorMessages, fmt.Sprintf("%s: %v", it, queryErr))
				errMsgMu.Unlock()
				return
			}

			// Sort by days_until_expiry ascending (most urgent first).
			sort.Slice(items, func(i, j int) bool {
				return items[i].DaysUntilExpiry < items[j].DaysUntilExpiry
			})

			mu.Lock()
			results[it] = items
			mu.Unlock()
		}(itemtype)
	}

	wg.Wait()

	// Build the report.
	totalExpiring := 0
	byItemtype := make(map[string][]ExpiredItem)
	for it, items := range results {
		if len(items) > 0 {
			byItemtype[it] = items
			totalExpiring += len(items)
		}
	}

	report := &ExpirationReport{
		GeneratedAt:   time.Now().Format(time.RFC3339),
		DaysAhead:     daysAhead,
		TotalExpiring: totalExpiring,
		ByItemtype:    byItemtype,
		HasErrors:     firstErr != nil,
		ErrorMessages: errorMessages,
	}

	return report, nil
}

// queryDirectDate searches for items with a direct expiration date field before the cutoff.
func (e *ExpirationTrackerTool) queryDirectDate(ctx context.Context, itemtype string, field ExpirationField, cutoffDate string, entityID int) ([]ExpiredItem, error) {
	searchTool, err := NewSearchTool(e.client)
	if err != nil {
		return nil, fmt.Errorf("create search tool: %w", err)
	}

	criteria := []SearchCriterion{
		{
			FieldName:  field.DateFieldUID,
			SearchType: "lessthan",
			Value:      cutoffDate,
		},
	}

	if entityID > 0 {
		criteria = append(criteria, SearchCriterion{
			FieldName:  fmt.Sprintf("%s.entities_id", itemtype),
			SearchType: "equals",
			Value:      fmt.Sprintf("%d", entityID),
		})
	}

	result, err := searchTool.Execute(ctx, itemtype, criteria, []string{}, nil)
	if err != nil {
		return nil, fmt.Errorf("search %s: %w", itemtype, err)
	}

	var items []ExpiredItem
	today := time.Now().UTC()

	for _, item := range result.Data {
		if item.Data == nil {
			continue
		}

		expDate := extractDateField(item.Data, field.DateFieldUID)
		if expDate == "" {
			continue
		}

		expTime, err := time.Parse("2006-01-02", expDate)
		if err != nil {
			continue
		}

		daysUntil := int(math.Floor(expTime.Sub(today).Seconds() / 86400))

		name := extractNameField(item.Data, itemtype)
		entityName := extractEntityName(item.Data)

		items = append(items, ExpiredItem{
			ID:              item.ID,
			Name:            name,
			ExpirationDate:  expDate,
			DaysUntilExpiry: daysUntil,
			Itemtype:        itemtype,
			EntityName:      entityName,
			IsComputed:      false,
		})
	}

	return items, nil
}

// queryComputedWarranty searches for items with warranty dates and computes expiry client-side.
func (e *ExpirationTrackerTool) queryComputedWarranty(ctx context.Context, itemtype string, field ExpirationField, cutoffDate time.Time, entityID int) ([]ExpiredItem, error) {
	searchTool, err := NewSearchTool(e.client)
	if err != nil {
		return nil, fmt.Errorf("create search tool: %w", err)
	}

	criteria := []SearchCriterion{
		{
			FieldName:  field.WarrantyDateFieldUID,
			SearchType: "exists",
			Value:      "",
		},
	}

	if entityID > 0 {
		criteria = append(criteria, SearchCriterion{
			FieldName:  fmt.Sprintf("%s.entities_id", itemtype),
			SearchType: "equals",
			Value:      fmt.Sprintf("%d", entityID),
		})
	}

	result, err := searchTool.Execute(ctx, itemtype, criteria, []string{}, nil)
	if err != nil {
		return nil, fmt.Errorf("search %s: %w", itemtype, err)
	}

	var items []ExpiredItem
	today := time.Now().UTC()

	for _, item := range result.Data {
		if item.Data == nil {
			continue
		}

		warrantyDateStr := extractDateField(item.Data, field.WarrantyDateFieldUID)
		if warrantyDateStr == "" {
			continue
		}

		warrantyDate, err := time.Parse("2006-01-02", warrantyDateStr)
		if err != nil {
			continue
		}

		durationMonths := extractDurationField(item.Data, field.WarrantyDurationFieldUID)
		if durationMonths <= 0 {
			continue
		}

		// Compute expiry: warranty_date + warranty_duration months.
		expiryDate := warrantyDate.AddDate(0, durationMonths, 0)

		// Only include items expiring within the cutoff window.
		if expiryDate.After(cutoffDate) {
			continue
		}

		daysUntil := int(math.Floor(expiryDate.Sub(today).Seconds() / 86400))

		name := extractNameField(item.Data, itemtype)
		entityName := extractEntityName(item.Data)

		items = append(items, ExpiredItem{
			ID:              item.ID,
			Name:            name,
			ExpirationDate:  expiryDate.Format("2006-01-02"),
			DaysUntilExpiry: daysUntil,
			Itemtype:        itemtype,
			EntityName:      entityName,
			IsComputed:      true,
		})
	}

	return items, nil
}

// extractDateField extracts a date value from search result data.
// It tries multiple key formats: the full UID, the short field name, and numeric field IDs.
func extractDateField(data map[string]interface{}, fieldUID string) string {
	// Try the full UID first (e.g., "Certificate.date_expiration").
	if v, ok := data[fieldUID]; ok {
		if s, ok := v.(string); ok && s != "" {
			return s
		}
	}

	// Try the short field name (e.g., "date_expiration").
	if shortName := extractShortFieldName(fieldUID); shortName != "" {
		if v, ok := data[shortName]; ok {
			if s, ok := v.(string); ok && s != "" {
				return s
			}
		}
	}

	return ""
}

// extractShortFieldName extracts the short field name from a UID like "Certificate.date_expiration".
func extractShortFieldName(uid string) string {
	for i := len(uid) - 1; i >= 0; i-- {
		if uid[i] == '.' {
			return uid[i+1:]
		}
	}
	return uid
}

// extractNameField extracts the name from search result data.
func extractNameField(data map[string]interface{}, itemtype string) string {
	// Try common name field patterns.
	nameFields := []string{
		"name",
		"1", // GLPI often returns name as field "1"
		fmt.Sprintf("%s.name", itemtype),
	}

	for _, f := range nameFields {
		if v, ok := data[f]; ok {
			if s, ok := v.(string); ok && s != "" {
				return s
			}
		}
	}

	return ""
}

// extractEntityName extracts the entity name from search result data.
func extractEntityName(data map[string]interface{}) string {
	// Try common entity name patterns.
	entityFields := []string{
		"entities_id",
		"80", // GLPI often returns entity as field "80"
	}

	for _, f := range entityFields {
		if v, ok := data[f]; ok {
			// Entity might be a string or a nested object.
			if s, ok := v.(string); ok && s != "" {
				return s
			}
			if m, ok := v.(map[string]interface{}); ok {
				if name, ok := m["name"].(string); ok && name != "" {
					return name
				}
			}
		}
	}

	return ""
}

// extractDurationField extracts a warranty duration (in months) from search result data.
func extractDurationField(data map[string]interface{}, fieldUID string) int {
	// Try the full UID.
	if v, ok := data[fieldUID]; ok {
		return toInt(v)
	}

	// Try the short field name.
	if shortName := extractShortFieldName(fieldUID); shortName != "" {
		if v, ok := data[shortName]; ok {
			return toInt(v)
		}
	}

	return 0
}

// Ensure ExpirationTrackerTool implements the Tool interface.
var _ Tool = (*ExpirationTrackerTool)(nil)
