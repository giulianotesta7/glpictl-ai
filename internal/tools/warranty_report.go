package tools

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"
)

// DefaultHardwareItemtypes is the set of hardware itemtypes used for warranty reporting.
var DefaultHardwareItemtypes = []string{
	"Computer",
	"Monitor",
	"Printer",
	"NetworkEquipment",
	"Peripheral",
	"Phone",
}

// WarrantyStatus represents the warranty status of an asset.
type WarrantyStatus string

const (
	WarrantyStatusActive       WarrantyStatus = "active"
	WarrantyStatusExpired      WarrantyStatus = "expired"
	WarrantyStatusExpiringSoon WarrantyStatus = "expiring_soon"
)

// WarrantyReport is the top-level result for glpi_warranty_report.
type WarrantyReport struct {
	GeneratedAt       string                `json:"generated_at"`
	DaysWarning       int                   `json:"days_warning"`
	Summary           WarrantySummary       `json:"summary"`
	AssetDetails      []WarrantyAssetDetail `json:"asset_details"`
	TotalPurchaseCost float64               `json:"total_purchase_cost"`
	HasErrors         bool                  `json:"has_errors"`
	ErrorMessages     []string              `json:"error_messages,omitempty"`
}

// WarrantySummary provides aggregate counts by warranty status.
type WarrantySummary struct {
	Active       int `json:"active"`
	Expired      int `json:"expired"`
	ExpiringSoon int `json:"expiring_soon"`
	Total        int `json:"total"`
}

// WarrantyAssetDetail represents warranty information for a single asset.
type WarrantyAssetDetail struct {
	ID              int            `json:"id"`
	Name            string         `json:"name"`
	Itemtype        string         `json:"itemtype"`
	EntityName      string         `json:"entity_name,omitempty"`
	WarrantyDate    string         `json:"warranty_date"`
	WarrantyMonths  int            `json:"warranty_months"`
	ExpirationDate  string         `json:"expiration_date"`
	DaysUntilExpiry int            `json:"days_until_expiry"`
	Status          WarrantyStatus `json:"status"`
	PurchaseCost    float64        `json:"purchase_cost"`
}

// WarrantyReportTool provides warranty reporting for hardware assets.
type WarrantyReportTool struct {
	client ToolClient
}

// NewWarrantyReportTool creates a new warranty report tool.
// Returns an error if the client is nil.
func NewWarrantyReportTool(client ToolClient) (*WarrantyReportTool, error) {
	if client == nil {
		return nil, fmt.Errorf("warranty report tool: client cannot be nil")
	}
	return &WarrantyReportTool{client: client}, nil
}

// Name returns the tool name for registration.
func (w *WarrantyReportTool) Name() string {
	return "glpi_warranty_report"
}

// Description returns the tool description.
func (w *WarrantyReportTool) Description() string {
	return "Generate a warranty status report for hardware assets with active, expired, and expiring-soon categorization, including purchase cost aggregation"
}

// WarrantyReportInput represents the input for the glpi_warranty_report tool.
type WarrantyReportInput struct {
	DaysWarning int      `json:"days_warning,omitempty"`
	Itemtypes   []string `json:"itemtypes,omitempty"`
	EntityID    int      `json:"entity_id,omitempty"`
}

// GetInput returns a new input struct for the tool.
func (w *WarrantyReportTool) GetInput() *WarrantyReportInput {
	return &WarrantyReportInput{}
}

// Execute generates a warranty report for hardware assets.
// The daysWarning parameter is optional (default 90); items expiring within this many days are flagged as expiring_soon.
// The itemtypes parameter is optional (defaults to hardware types: Computer, Monitor, Printer, NetworkEquipment, Peripheral, Phone).
// The entityID parameter is optional; if > 0, results are filtered to that entity.
// Returns a structured warranty report even if some itemtype queries fail.
func (w *WarrantyReportTool) Execute(ctx context.Context, daysWarning int, itemtypes []string, entityID int) (*WarrantyReport, error) {
	if daysWarning < 0 {
		return nil, fmt.Errorf("days_warning must be a non-negative integer")
	}

	// Default to hardware itemtypes if none specified.
	targetTypes := itemtypes
	if len(targetTypes) == 0 {
		targetTypes = DefaultHardwareItemtypes
	}

	// Filter to only hardware types that have warranty fields in the registry.
	validTypes := make([]string, 0, len(targetTypes))
	for _, it := range targetTypes {
		if field, ok := expirationFieldRegistry[it]; ok && field.SearchType == "computed" {
			validTypes = append(validTypes, it)
		}
	}

	// Shared data structures for concurrent aggregation.
	type assetResult struct {
		assets []WarrantyAssetDetail
		err    error
	}

	results := make(map[string]assetResult)
	var mu sync.Mutex

	var wg sync.WaitGroup

	for _, itemtype := range validTypes {
		wg.Add(1)
		go func(it string) {
			defer wg.Done()

			field := expirationFieldRegistry[it]
			assets, queryErr := w.queryWarranty(ctx, it, field, entityID)

			mu.Lock()
			results[it] = assetResult{assets: assets, err: queryErr}
			mu.Unlock()
		}(itemtype)
	}

	wg.Wait()

	// Build the report with status categorization.
	var allAssets []WarrantyAssetDetail
	var errorMessages []string
	var totalPurchaseCost float64
	summary := WarrantySummary{}

	for _, it := range validTypes {
		res := results[it]
		if res.err != nil {
			errorMessages = append(errorMessages, fmt.Sprintf("%s: %v", it, res.err))
			continue
		}

		for _, asset := range res.assets {
			// Categorize warranty status.
			days := asset.DaysUntilExpiry
			if days < 0 {
				asset.Status = WarrantyStatusExpired
				summary.Expired++
			} else if days <= daysWarning {
				asset.Status = WarrantyStatusExpiringSoon
				summary.ExpiringSoon++
			} else {
				asset.Status = WarrantyStatusActive
				summary.Active++
			}

			totalPurchaseCost += asset.PurchaseCost
			allAssets = append(allAssets, asset)
		}
	}

	// Sort assets: expired first, then expiring_soon, then active; within each group by days_until_expiry ascending.
	sort.Slice(allAssets, func(i, j int) bool {
		statusOrder := func(s WarrantyStatus) int {
			switch s {
			case WarrantyStatusExpired:
				return 0
			case WarrantyStatusExpiringSoon:
				return 1
			case WarrantyStatusActive:
				return 2
			default:
				return 3
			}
		}
		oi := statusOrder(allAssets[i].Status)
		oj := statusOrder(allAssets[j].Status)
		if oi != oj {
			return oi < oj
		}
		return allAssets[i].DaysUntilExpiry < allAssets[j].DaysUntilExpiry
	})

	summary.Total = summary.Active + summary.Expired + summary.ExpiringSoon

	report := &WarrantyReport{
		GeneratedAt:       time.Now().Format(time.RFC3339),
		DaysWarning:       daysWarning,
		Summary:           summary,
		AssetDetails:      allAssets,
		TotalPurchaseCost: totalPurchaseCost,
		HasErrors:         len(errorMessages) > 0,
		ErrorMessages:     errorMessages,
	}

	return report, nil
}

// queryWarranty searches for items with warranty dates and computes expiry.
func (w *WarrantyReportTool) queryWarranty(ctx context.Context, itemtype string, field ExpirationField, entityID int) ([]WarrantyAssetDetail, error) {
	searchTool, err := NewSearchTool(w.client)
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

	var assets []WarrantyAssetDetail
	today := time.Now()

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
		daysUntil := int(expiryDate.Sub(today).Hours() / 24)

		name := extractNameField(item.Data, itemtype)
		entityName := extractEntityName(item.Data)
		purchaseCost := extractPurchaseCost(item.Data, itemtype)

		assets = append(assets, WarrantyAssetDetail{
			ID:              item.ID,
			Name:            name,
			Itemtype:        itemtype,
			EntityName:      entityName,
			WarrantyDate:    warrantyDateStr,
			WarrantyMonths:  durationMonths,
			ExpirationDate:  expiryDate.Format("2006-01-02"),
			DaysUntilExpiry: daysUntil,
			PurchaseCost:    purchaseCost,
		})
	}

	return assets, nil
}

// extractPurchaseCost extracts the purchase cost from search result data.
// It tries multiple key formats: the full UID, the short field name, and common GLPI field IDs.
func extractPurchaseCost(data map[string]interface{}, itemtype string) float64 {
	// Try common purchase cost field patterns.
	costFields := []string{
		fmt.Sprintf("%s.buy_value", itemtype),
		"buy_value",
		"22", // GLPI field ID for buy_value in many itemtypes
	}

	for _, f := range costFields {
		if v, ok := data[f]; ok {
			switch val := v.(type) {
			case float64:
				return val
			case string:
				// Try parsing as number.
				if n, err := parseFloat(val); err == nil {
					return n
				}
			}
		}
	}

	return 0
}

// parseFloat parses a string to float64, handling common formats.
func parseFloat(s string) (float64, error) {
	var result float64
	_, err := fmt.Sscanf(s, "%f", &result)
	return result, err
}

// Ensure WarrantyReportTool implements the Tool interface.
var _ Tool = (*WarrantyReportTool)(nil)
