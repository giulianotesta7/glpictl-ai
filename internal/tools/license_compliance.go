package tools

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
)

// ComplianceStatus represents the license compliance state.
type ComplianceStatus string

const (
	// StatusCompliant means purchased >= installed, both > 0.
	StatusCompliant ComplianceStatus = "compliant"
	// StatusOverInstalled means installed > purchased.
	StatusOverInstalled ComplianceStatus = "over-installed"
	// StatusUnlicensed means purchased = 0, installed > 0.
	StatusUnlicensed ComplianceStatus = "unlicensed"
	// StatusUnused means purchased > 0, installed = 0 — licenses exist but are not assigned.
	StatusUnused ComplianceStatus = "unused"
	// StatusUnderUtilized means purchased > installed > 0 — some licenses are assigned but not all.
	StatusUnderUtilized ComplianceStatus = "under-utilized"
)

// LicenseDetail represents compliance data for a single software license.
type LicenseDetail struct {
	LicenseID      int    `json:"license_id"`
	LicenseName    string `json:"license_name"`
	PurchasedSeats int    `json:"purchased_seats"`
	InstalledCount int    `json:"installed_count"`
	ComplianceGap  int    `json:"compliance_gap"`
	Status         string `json:"status"`
}

// ComplianceReport is the top-level result for glpi_license_compliance.
type ComplianceReport struct {
	SoftwareID     int              `json:"software_id"`
	SoftwareName   string           `json:"software_name"`
	PurchasedCount int              `json:"purchased_count"`
	InstalledCount int              `json:"installed_count"`
	ComplianceGap  int              `json:"compliance_gap"` // positive = surplus, negative = deficit
	Status         ComplianceStatus `json:"status"`         // compliant | over-installed | unlicensed | unused | under-utilized
	Licenses       []LicenseDetail  `json:"licenses"`
}

// LicenseComplianceTool provides license compliance checking by cross-referencing
// purchased SoftwareLicense records against actual Item_SoftwareVersion installations.
type LicenseComplianceTool struct {
	client ToolClient
}

// NewLicenseComplianceTool creates a new license compliance tool.
// Returns an error if the client is nil.
func NewLicenseComplianceTool(client ToolClient) (*LicenseComplianceTool, error) {
	if client == nil {
		return nil, fmt.Errorf("license compliance tool: client cannot be nil")
	}
	return &LicenseComplianceTool{client: client}, nil
}

// Name returns the tool name for registration.
func (l *LicenseComplianceTool) Name() string {
	return "glpi_license_compliance"
}

// Description returns the tool description.
func (l *LicenseComplianceTool) Description() string {
	return "Return a software license compliance report comparing purchased licenses vs actual installations"
}

// LicenseComplianceInput represents the input for the glpi_license_compliance tool.
type LicenseComplianceInput struct {
	SoftwareID int `json:"software_id"`
	EntityID   int `json:"entity_id,omitempty"`
}

// GetInput returns a new input struct for the tool.
func (l *LicenseComplianceTool) GetInput() *LicenseComplianceInput {
	return &LicenseComplianceInput{}
}

// Execute performs a license compliance check for a specific software ID by searching
// SoftwareLicense and Item_SoftwareVersion records concurrently, then cross-referencing
// the results. The softwareID parameter is required and must be a positive integer.
// The optional entityID filters by GLPI entity (0 = all entities).
func (l *LicenseComplianceTool) Execute(ctx context.Context, softwareID int, entityID int) (*ComplianceReport, error) {
	if softwareID <= 0 {
		return nil, fmt.Errorf("software_id must be a positive integer")
	}

	// Verify the software exists before running compliance searches.
	// This distinguishes "software doesn't exist" from "software exists but has no licenses/installations".
	var softwareData map[string]interface{}
	endpoint := fmt.Sprintf("/Software/%d", softwareID)
	if err := l.client.Get(ctx, endpoint, &softwareData); err != nil {
		return nil, fmt.Errorf("license compliance [software lookup]: %w", err)
	}
	if softwareData == nil || len(softwareData) == 0 {
		return nil, fmt.Errorf("software with ID %d not found", softwareID)
	}

	// Extract software name from the Get response for the report.
	softwareName := ""
	if name, ok := softwareData["name"].(string); ok {
		softwareName = name
	}

	// Shared data structures
	var licenses []map[string]interface{} // raw license data from search
	var installCount int                  // total installation count for this software
	var mu sync.Mutex
	var firstErr error
	var errMu sync.Mutex

	// Build base criteria with dynamically resolved field IDs
	var licenseCriteria []SearchCriterion
	var installCriteria []SearchCriterion

	// Resolve software reference field for SoftwareLicense
	softwareFieldLicense, err := l.resolveSoftwareFieldID(ctx, "SoftwareLicense")
	if err != nil {
		return nil, fmt.Errorf("license compliance [resolve software field for licenses]: %w", err)
	}
	licenseCriteria = append(licenseCriteria, SearchCriterion{
		Field:      softwareFieldLicense,
		SearchType: "equals",
		Value:      strconv.Itoa(softwareID),
	})

	// Resolve software reference field for Item_SoftwareVersion
	softwareFieldInstall, err := l.resolveSoftwareFieldID(ctx, "Item_SoftwareVersion")
	if err != nil {
		return nil, fmt.Errorf("license compliance [resolve software field for installations]: %w", err)
	}
	installCriteria = append(installCriteria, SearchCriterion{
		Field:      softwareFieldInstall,
		SearchType: "equals",
		Value:      strconv.Itoa(softwareID),
	})

	if entityID > 0 {
		entitiesFieldLicense, err := l.resolveEntityFieldID(ctx, "SoftwareLicense")
		if err == nil {
			licenseCriteria = append(licenseCriteria, SearchCriterion{
				Field:      entitiesFieldLicense,
				SearchType: "equals",
				Value:      strconv.Itoa(entityID),
			})
		}
		entitiesFieldInstall, err := l.resolveEntityFieldID(ctx, "Software")
		if err == nil {
			installCriteria = append(installCriteria, SearchCriterion{
				Field:      entitiesFieldInstall,
				SearchType: "equals",
				Value:      strconv.Itoa(entityID),
			})
		}
	}

	var wg sync.WaitGroup

	// Goroutine 1: Search SoftwareLicense for the given software_id
	wg.Add(1)
	go func() {
		defer wg.Done()

		searchTool, err := NewSearchTool(l.client)
		if err != nil {
			errMu.Lock()
			if firstErr == nil {
				firstErr = fmt.Errorf("license compliance [licenses]: %w", err)
			}
			errMu.Unlock()
			return
		}

		result, err := searchTool.Execute(ctx, "SoftwareLicense", licenseCriteria, []string{}, nil)
		if err != nil {
			errMu.Lock()
			if firstErr == nil {
				firstErr = fmt.Errorf("license compliance [licenses]: %w", err)
			}
			errMu.Unlock()
			return
		}

		mu.Lock()
		defer mu.Unlock()

		for _, item := range result.Data {
			data := item.Data
			if data == nil {
				continue
			}

			// GLPI field mappings for SoftwareLicense:
			// "2" = name (license name)
			// "31" = software_id (link to Software)
			// "34" = number (purchased count)
			// "5" = software name (via link)
			licenseName := toString(data["2"])
			softwareIDFromField := toInt(data["31"])
			purchased := toInt(data["34"])
			name := toString(data["5"])

			// If software_id is a nested object (expanded dropdown)
			if softwareIDFromField == 0 {
				if sw, ok := data["31"]; ok {
					if swMap, ok := sw.(map[string]interface{}); ok {
						softwareIDFromField = toInt(swMap["id"])
						if name == "" {
							name = toString(swMap["name"])
						}
					}
				}
			}

			// Capture software name from first license entry
			if softwareName == "" && name != "" {
				softwareName = name
			}

			licenses = append(licenses, map[string]interface{}{
				"id":        item.ID,
				"name":      licenseName,
				"purchased": purchased,
			})
		}
	}()

	// Goroutine 2: Search Item_SoftwareVersion (installations) for the given software_id
	wg.Add(1)
	go func() {
		defer wg.Done()

		searchTool, err := NewSearchTool(l.client)
		if err != nil {
			errMu.Lock()
			if firstErr == nil {
				firstErr = fmt.Errorf("license compliance [installations]: %w", err)
			}
			errMu.Unlock()
			return
		}

		result, err := searchTool.Execute(ctx, "Item_SoftwareVersion", installCriteria, []string{}, nil)
		if err != nil {
			errMu.Lock()
			if firstErr == nil {
				firstErr = fmt.Errorf("license compliance [installations]: %w", err)
			}
			errMu.Unlock()
			return
		}

		mu.Lock()
		defer mu.Unlock()

		// Count total installations for this software_id
		installCount = result.TotalCount
	}()

	wg.Wait()

	if firstErr != nil {
		return nil, firstErr
	}

	// Build per-license details
	licenseDetails := make([]LicenseDetail, 0, len(licenses))
	totalPurchased := 0

	for _, lic := range licenses {
		licenseID, _ := lic["id"].(int)
		licenseName, _ := lic["name"].(string)
		purchased, _ := lic["purchased"].(int)

		totalPurchased += purchased

		// Each license's installed count is proportional to its share of total purchased
		// If there's only one license, it gets all installations
		installed := 0
		if len(licenses) == 1 {
			installed = installCount
		} else if totalPurchased > 0 {
			// Distribute proportionally
			installed = (purchased * installCount) / totalPurchased
		}

		gap := purchased - installed
		status := computeStatus(purchased, installed)

		licenseDetails = append(licenseDetails, LicenseDetail{
			LicenseID:      licenseID,
			LicenseName:    licenseName,
			PurchasedSeats: purchased,
			InstalledCount: installed,
			ComplianceGap:  gap,
			Status:         string(status),
		})
	}

	// Sort details by license name for consistent output
	sort.Slice(licenseDetails, func(i, j int) bool {
		return licenseDetails[i].LicenseName < licenseDetails[j].LicenseName
	})

	// Compute overall compliance
	gap := totalPurchased - installCount
	status := computeStatus(totalPurchased, installCount)

	report := &ComplianceReport{
		SoftwareID:     softwareID,
		SoftwareName:   softwareName,
		PurchasedCount: totalPurchased,
		InstalledCount: installCount,
		ComplianceGap:  gap,
		Status:         status,
		Licenses:       licenseDetails,
	}

	return report, nil
}

// resolveSoftwareFieldID finds the numeric field ID for the software reference
// in the given itemtype by inspecting search options.
func (l *LicenseComplianceTool) resolveSoftwareFieldID(ctx context.Context, itemtype string) (int, error) {
	searchOptions, err := l.client.GetSearchOptions(ctx, itemtype)
	if err != nil {
		return 0, fmt.Errorf("get search options for %s: %w", itemtype, err)
	}

	// Look for a field whose UID starts with the itemtype and references Software
	// e.g., "SoftwareLicense.Software.name" — the itemtype prefix ensures we get the
	// reference field, not the software's own name field.
	prefix := itemtype + ".Software"
	for _, opt := range searchOptions.Fields {
		if strings.HasPrefix(opt.UID, prefix) {
			return opt.ID, nil
		}
	}

	// Fallback: look for any field with table containing "software" and field "name"
	// but exclude the main software table (we want the reference, not the software's own fields)
	for _, opt := range searchOptions.Fields {
		if strings.Contains(strings.ToLower(opt.Table), "software") &&
			strings.EqualFold(opt.Field, "name") &&
			!strings.EqualFold(opt.Table, "glpi_software") {
			return opt.ID, nil
		}
	}

	return 0, fmt.Errorf("no software reference field found for itemtype %q", itemtype)
}

// resolveEntityFieldID finds the numeric field ID for the entity field
// in the given itemtype by inspecting search options.
func (l *LicenseComplianceTool) resolveEntityFieldID(ctx context.Context, itemtype string) (int, error) {
	searchOptions, err := l.client.GetSearchOptions(ctx, itemtype)
	if err != nil {
		return 0, fmt.Errorf("get search options for %s: %w", itemtype, err)
	}

	// Look for entities_id field directly on the main table
	mainTable := "glpi_" + strings.ToLower(itemtype)
	for _, opt := range searchOptions.Fields {
		if strings.EqualFold(opt.Table, mainTable) && strings.EqualFold(opt.Field, "entities_id") {
			return opt.ID, nil
		}
	}

	// Fallback: look for Entity UID pattern
	for _, opt := range searchOptions.Fields {
		if strings.Contains(opt.UID, ".Entity.") {
			return opt.ID, nil
		}
	}

	return 0, fmt.Errorf("no entities_id field found for itemtype %q", itemtype)
}

// computeStatus determines the compliance status based on purchased and installed counts.
// Spec statuses:
//   - compliant:     purchased >= installed, both > 0
//   - over-installed: installed > purchased
//   - unlicensed:    purchased = 0, installed > 0
//   - unused:        purchased > 0, installed = 0
//   - under-utilized: purchased > installed, installed = 0
func computeStatus(purchased, installed int) ComplianceStatus {
	if purchased == 0 && installed == 0 {
		// Edge case: no data at all — treat as unused (no licenses, no installations)
		return StatusUnused
	}
	if purchased == 0 && installed > 0 {
		return StatusUnlicensed
	}
	if installed > purchased {
		return StatusOverInstalled
	}
	// purchased > 0 && installed <= purchased
	if installed == 0 {
		return StatusUnderUtilized
	}
	// purchased >= installed > 0
	return StatusCompliant
}

// toInt converts an interface{} to int, handling float64 (JSON numbers).
func toInt(v interface{}) int {
	if v == nil {
		return 0
	}
	switch val := v.(type) {
	case float64:
		return int(val)
	case int:
		return val
	case string:
		n, _ := strconv.Atoi(val)
		return n
	default:
		return 0
	}
}

// toString converts an interface{} to string.
func toString(v interface{}) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprintf("%v", v)
}

// Ensure LicenseComplianceTool implements the Tool interface.
var _ Tool = (*LicenseComplianceTool)(nil)
