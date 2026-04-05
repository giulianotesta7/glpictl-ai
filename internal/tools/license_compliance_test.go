package tools

import (
	"context"
	"strings"
	"testing"

	"github.com/giulianotesta7/glpictl-ai/internal/glpi"
)

// mockSearchOptionsForCompliance returns a SearchOptionsResult with field mappings for both
// SoftwareLicense and Item_SoftwareVersion itemtypes.
func mockSearchOptionsForCompliance() func(ctx context.Context, itemtype string) (*glpi.SearchOptionsResult, error) {
	return func(ctx context.Context, itemtype string) (*glpi.SearchOptionsResult, error) {
		switch itemtype {
		case "SoftwareLicense":
			return &glpi.SearchOptionsResult{
				ItemType: itemtype,
				Fields: []glpi.SearchOption{
					{ID: 1, UID: "SoftwareLicense.id", Field: "id", Name: "ID", DisplayName: "ID"},
					{ID: 2, UID: "SoftwareLicense.name", Field: "name", Name: "Name", DisplayName: "Name"},
					{ID: 5, UID: "SoftwareLicense.Software.name", Field: "name", Name: "Software", DisplayName: "Software"},
					{ID: 31, UID: "SoftwareLicense.Software.software", Field: "software", Name: "Software", DisplayName: "Software ID"},
					{ID: 34, UID: "SoftwareLicense.number", Field: "number", Name: "Number", DisplayName: "Number of licenses"},
					{ID: 80, UID: "SoftwareLicense.entities_id", Field: "entities_id", Name: "Entity", DisplayName: "Entity"},
				},
			}, nil
		case "Item_SoftwareVersion":
			return &glpi.SearchOptionsResult{
				ItemType: itemtype,
				Fields: []glpi.SearchOption{
					{ID: 1, UID: "Item_SoftwareVersion.id", Field: "id", Name: "ID", DisplayName: "ID"},
					{ID: 5, UID: "Item_SoftwareVersion.Software.name", Field: "name", Name: "Software", DisplayName: "Software ID"},
					{ID: 6, UID: "Item_SoftwareVersion.Software.entities_id", Field: "entities_id", Name: "Entity", DisplayName: "Entity"},
				},
			}, nil
		default:
			return &glpi.SearchOptionsResult{ItemType: itemtype}, nil
		}
	}
}

// mockGetFuncForCompliance returns a GetFunc that responds to search endpoints with the
// provided license and installation data, and to software lookup endpoints with a mock software object.
func mockGetFuncForCompliance(licenseData, installData map[string]interface{}) func(ctx context.Context, endpoint string, result interface{}) error {
	return func(ctx context.Context, endpoint string, result interface{}) error {
		resMap := result.(*map[string]interface{})
		if strings.Contains(endpoint, "/search/SoftwareLicense") {
			*resMap = licenseData
		} else if strings.Contains(endpoint, "/search/Item_SoftwareVersion") {
			*resMap = installData
		} else if strings.Contains(endpoint, "/Software/") {
			// Software lookup for existence check — extract ID from endpoint
			// e.g. "apirest.php/Software/42" → mock software with id=42
			parts := strings.Split(endpoint, "/")
			softwareID := parts[len(parts)-1]
			*resMap = map[string]interface{}{
				"id":   softwareID,
				"name": "Test Software",
			}
		} else {
			*resMap = map[string]interface{}{}
		}
		return nil
	}
}

func TestLicenseComplianceTool_Execute(t *testing.T) {
	t.Run("compliant software — happy path", func(t *testing.T) {
		mockClient := &MockClient{
			searchOptionsFunc: mockSearchOptionsForCompliance(),
			GetFunc: mockGetFuncForCompliance(
				map[string]interface{}{
					"totalcount": float64(1),
					"data": []interface{}{
						map[string]interface{}{
							"id": float64(10),
							"2":  "Office 365 License",
							"5":  "Microsoft Office",
							"31": float64(42),
							"34": float64(10),
						},
					},
				},
				map[string]interface{}{
					"totalcount": float64(8),
					"data": []interface{}{
						map[string]interface{}{"id": float64(1), "5": float64(42)},
						map[string]interface{}{"id": float64(2), "5": float64(42)},
						map[string]interface{}{"id": float64(3), "5": float64(42)},
						map[string]interface{}{"id": float64(4), "5": float64(42)},
						map[string]interface{}{"id": float64(5), "5": float64(42)},
						map[string]interface{}{"id": float64(6), "5": float64(42)},
						map[string]interface{}{"id": float64(7), "5": float64(42)},
						map[string]interface{}{"id": float64(8), "5": float64(42)},
					},
				},
			),
		}

		tool, err := NewLicenseComplianceTool(mockClient)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		report, err := tool.Execute(context.Background(), 42, 0)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if report.SoftwareID != 42 {
			t.Errorf("SoftwareID = %d, want 42", report.SoftwareID)
		}
		if report.PurchasedCount != 10 {
			t.Errorf("PurchasedCount = %d, want 10", report.PurchasedCount)
		}
		if report.InstalledCount != 8 {
			t.Errorf("InstalledCount = %d, want 8", report.InstalledCount)
		}
		if report.ComplianceGap != 2 {
			t.Errorf("ComplianceGap = %d, want 2", report.ComplianceGap)
		}
		if report.Status != StatusCompliant {
			t.Errorf("Status = %q, want %q", report.Status, StatusCompliant)
		}
		if len(report.Licenses) != 1 {
			t.Fatalf("Licenses length = %d, want 1", len(report.Licenses))
		}
		d := report.Licenses[0]
		if d.LicenseID != 10 {
			t.Errorf("LicenseID = %d, want 10", d.LicenseID)
		}
		if d.PurchasedSeats != 10 {
			t.Errorf("PurchasedSeats = %d, want 10", d.PurchasedSeats)
		}
		if d.InstalledCount != 8 {
			t.Errorf("InstalledCount = %d, want 8", d.InstalledCount)
		}
		if d.ComplianceGap != 2 {
			t.Errorf("ComplianceGap = %d, want 2", d.ComplianceGap)
		}
	})

	t.Run("over-installed software", func(t *testing.T) {
		mockClient := &MockClient{
			searchOptionsFunc: mockSearchOptionsForCompliance(),
			GetFunc: mockGetFuncForCompliance(
				map[string]interface{}{
					"totalcount": float64(1),
					"data": []interface{}{
						map[string]interface{}{
							"id": float64(20),
							"2":  "Windows License",
							"5":  "Windows 11",
							"31": float64(99),
							"34": float64(5),
						},
					},
				},
				map[string]interface{}{
					"totalcount": float64(12),
					"data": []interface{}{
						map[string]interface{}{"id": float64(1), "5": float64(99)},
						map[string]interface{}{"id": float64(2), "5": float64(99)},
						map[string]interface{}{"id": float64(3), "5": float64(99)},
						map[string]interface{}{"id": float64(4), "5": float64(99)},
						map[string]interface{}{"id": float64(5), "5": float64(99)},
						map[string]interface{}{"id": float64(6), "5": float64(99)},
						map[string]interface{}{"id": float64(7), "5": float64(99)},
						map[string]interface{}{"id": float64(8), "5": float64(99)},
						map[string]interface{}{"id": float64(9), "5": float64(99)},
						map[string]interface{}{"id": float64(10), "5": float64(99)},
						map[string]interface{}{"id": float64(11), "5": float64(99)},
						map[string]interface{}{"id": float64(12), "5": float64(99)},
					},
				},
			),
		}

		tool, err := NewLicenseComplianceTool(mockClient)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		report, err := tool.Execute(context.Background(), 99, 0)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if report.PurchasedCount != 5 {
			t.Errorf("PurchasedCount = %d, want 5", report.PurchasedCount)
		}
		if report.InstalledCount != 12 {
			t.Errorf("InstalledCount = %d, want 12", report.InstalledCount)
		}
		if report.ComplianceGap != -7 {
			t.Errorf("ComplianceGap = %d, want -7", report.ComplianceGap)
		}
		if report.Status != StatusOverInstalled {
			t.Errorf("Status = %q, want %q", report.Status, StatusOverInstalled)
		}
	})

	t.Run("unlicensed — no purchases but installations exist", func(t *testing.T) {
		mockClient := &MockClient{
			searchOptionsFunc: mockSearchOptionsForCompliance(),
			GetFunc: mockGetFuncForCompliance(
				map[string]interface{}{
					"totalcount": float64(0),
					"data":       []interface{}{},
				},
				map[string]interface{}{
					"totalcount": float64(3),
					"data": []interface{}{
						map[string]interface{}{"id": float64(1), "5": float64(50)},
						map[string]interface{}{"id": float64(2), "5": float64(50)},
						map[string]interface{}{"id": float64(3), "5": float64(50)},
					},
				},
			),
		}

		tool, err := NewLicenseComplianceTool(mockClient)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		report, err := tool.Execute(context.Background(), 50, 0)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if report.PurchasedCount != 0 {
			t.Errorf("PurchasedCount = %d, want 0", report.PurchasedCount)
		}
		if report.InstalledCount != 3 {
			t.Errorf("InstalledCount = %d, want 3", report.InstalledCount)
		}
		if report.Status != StatusUnlicensed {
			t.Errorf("Status = %q, want %q", report.Status, StatusUnlicensed)
		}
	})

	t.Run("unused — purchased but no installations", func(t *testing.T) {
		mockClient := &MockClient{
			searchOptionsFunc: mockSearchOptionsForCompliance(),
			GetFunc: mockGetFuncForCompliance(
				map[string]interface{}{
					"totalcount": float64(1),
					"data": []interface{}{
						map[string]interface{}{
							"id": float64(30),
							"2":  "Unused License",
							"5":  "Unused Software",
							"31": float64(60),
							"34": float64(10),
						},
					},
				},
				map[string]interface{}{
					"totalcount": float64(0),
					"data":       []interface{}{},
				},
			),
		}

		tool, err := NewLicenseComplianceTool(mockClient)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		report, err := tool.Execute(context.Background(), 60, 0)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if report.PurchasedCount != 10 {
			t.Errorf("PurchasedCount = %d, want 10", report.PurchasedCount)
		}
		if report.InstalledCount != 0 {
			t.Errorf("InstalledCount = %d, want 0", report.InstalledCount)
		}
		if report.Status != StatusUnderUtilized {
			t.Errorf("Status = %q, want %q", report.Status, StatusUnderUtilized)
		}
	})

	t.Run("no licenses and no installations — unused", func(t *testing.T) {
		mockClient := &MockClient{
			searchOptionsFunc: mockSearchOptionsForCompliance(),
			GetFunc: mockGetFuncForCompliance(
				map[string]interface{}{"totalcount": float64(0), "data": []interface{}{}},
				map[string]interface{}{"totalcount": float64(0), "data": []interface{}{}},
			),
		}

		tool, err := NewLicenseComplianceTool(mockClient)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		report, err := tool.Execute(context.Background(), 70, 0)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if report.PurchasedCount != 0 {
			t.Errorf("PurchasedCount = %d, want 0", report.PurchasedCount)
		}
		if report.InstalledCount != 0 {
			t.Errorf("InstalledCount = %d, want 0", report.InstalledCount)
		}
		if report.Status != StatusUnused {
			t.Errorf("Status = %q, want %q", report.Status, StatusUnused)
		}
	})

	t.Run("invalid software_id returns error", func(t *testing.T) {
		mockClient := &MockClient{}

		tool, err := NewLicenseComplianceTool(mockClient)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		_, err = tool.Execute(context.Background(), 0, 0)
		if err == nil {
			t.Fatal("expected error for software_id=0")
		}
		if !strings.Contains(err.Error(), "software_id must be a positive integer") {
			t.Errorf("error should contain 'software_id must be a positive integer', got: %v", err)
		}

		_, err = tool.Execute(context.Background(), -1, 0)
		if err == nil {
			t.Fatal("expected error for software_id=-1")
		}
	})

	t.Run("software not found returns error", func(t *testing.T) {
		mockClient := &MockClient{
			searchOptionsFunc: mockSearchOptionsForCompliance(),
			GetFunc: func(ctx context.Context, endpoint string, result interface{}) error {
				resMap := result.(*map[string]interface{})
				if strings.Contains(endpoint, "/Software/") {
					// Simulate software not found — return empty map
					*resMap = map[string]interface{}{}
					return nil
				}
				*resMap = map[string]interface{}{
					"totalcount": float64(0),
					"data":       []interface{}{},
				}
				return nil
			},
		}

		tool, err := NewLicenseComplianceTool(mockClient)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		_, err = tool.Execute(context.Background(), 9999, 0)
		if err == nil {
			t.Fatal("expected error for non-existent software")
		}
		if !strings.Contains(err.Error(), "software with ID 9999 not found") {
			t.Errorf("error should contain 'software with ID 9999 not found', got: %v", err)
		}
	})

	t.Run("filter by entity_id", func(t *testing.T) {
		var licenseEndpoint, installEndpoint string
		mockClient := &MockClient{
			searchOptionsFunc: mockSearchOptionsForCompliance(),
			GetFunc: func(ctx context.Context, endpoint string, result interface{}) error {
				resMap := result.(*map[string]interface{})
				if strings.Contains(endpoint, "/search/SoftwareLicense") {
					licenseEndpoint = endpoint
					*resMap = map[string]interface{}{
						"totalcount": float64(0),
						"data":       []interface{}{},
					}
				} else if strings.Contains(endpoint, "/search/Item_SoftwareVersion") {
					installEndpoint = endpoint
					*resMap = map[string]interface{}{
						"totalcount": float64(0),
						"data":       []interface{}{},
					}
				} else if strings.Contains(endpoint, "/Software/") {
					*resMap = map[string]interface{}{
						"id":   float64(42),
						"name": "Test Software",
					}
				} else {
					*resMap = map[string]interface{}{}
				}
				return nil
			},
		}

		tool, err := NewLicenseComplianceTool(mockClient)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		_, err = tool.Execute(context.Background(), 42, 3)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Verify entity_id is included in the search criteria
		if !strings.Contains(licenseEndpoint, "criteria") {
			t.Errorf("license endpoint should contain search criteria, got: %s", licenseEndpoint)
		}
		if !strings.Contains(installEndpoint, "criteria") {
			t.Errorf("install endpoint should contain search criteria, got: %s", installEndpoint)
		}
	})

	t.Run("search error on licenses", func(t *testing.T) {
		mockClient := &MockClient{
			searchOptionsFunc: mockSearchOptionsForCompliance(),
			GetFunc: func(ctx context.Context, endpoint string, result interface{}) error {
				resMap := result.(*map[string]interface{})
				if strings.Contains(endpoint, "/search/SoftwareLicense") {
					return context.DeadlineExceeded
				} else if strings.Contains(endpoint, "/Software/") {
					*resMap = map[string]interface{}{
						"id":   float64(42),
						"name": "Test Software",
					}
					return nil
				}
				*resMap = map[string]interface{}{
					"totalcount": float64(0),
					"data":       []interface{}{},
				}
				return nil
			},
		}

		tool, err := NewLicenseComplianceTool(mockClient)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		_, err = tool.Execute(context.Background(), 42, 0)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "license compliance [licenses]") {
			t.Errorf("error should contain 'license compliance [licenses]', got: %v", err)
		}
	})

	t.Run("search error on installations", func(t *testing.T) {
		mockClient := &MockClient{
			searchOptionsFunc: mockSearchOptionsForCompliance(),
			GetFunc: func(ctx context.Context, endpoint string, result interface{}) error {
				if strings.Contains(endpoint, "/search/Item_SoftwareVersion") {
					return context.DeadlineExceeded
				} else if strings.Contains(endpoint, "/Software/") {
					resMap := result.(*map[string]interface{})
					*resMap = map[string]interface{}{
						"id":   float64(42),
						"name": "Test Software",
					}
					return nil
				}
				resMap := result.(*map[string]interface{})
				*resMap = map[string]interface{}{
					"totalcount": float64(1),
					"data": []interface{}{
						map[string]interface{}{
							"id": float64(10),
							"2":  "Test License",
							"5":  "Test Software",
							"31": float64(42),
							"34": float64(5),
						},
					},
				}
				return nil
			},
		}

		tool, err := NewLicenseComplianceTool(mockClient)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		_, err = tool.Execute(context.Background(), 42, 0)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "license compliance [installations]") {
			t.Errorf("error should contain 'license compliance [installations]', got: %v", err)
		}
	})

	t.Run("nil client", func(t *testing.T) {
		tool, err := NewLicenseComplianceTool(nil)
		if err == nil {
			t.Error("expected error on nil client")
		}
		if tool != nil {
			t.Error("expected nil tool on error")
		}
	})

	t.Run("multiple licenses with mixed status", func(t *testing.T) {
		mockClient := &MockClient{
			searchOptionsFunc: mockSearchOptionsForCompliance(),
			GetFunc: mockGetFuncForCompliance(
				map[string]interface{}{
					"totalcount": float64(3),
					"data": []interface{}{
						map[string]interface{}{
							"id": float64(1),
							"2":  "Alpha License",
							"5":  "Alpha Software",
							"31": float64(100),
							"34": float64(10),
						},
						map[string]interface{}{
							"id": float64(2),
							"2":  "Beta License",
							"5":  "Alpha Software",
							"31": float64(100),
							"34": float64(5),
						},
						map[string]interface{}{
							"id": float64(3),
							"2":  "Gamma License",
							"5":  "Alpha Software",
							"31": float64(100),
							"34": float64(5),
						},
					},
				},
				map[string]interface{}{
					"totalcount": float64(18),
					"data": []interface{}{
						// 18 installations for software 100
						map[string]interface{}{"id": float64(1), "5": float64(100)},
						map[string]interface{}{"id": float64(2), "5": float64(100)},
						map[string]interface{}{"id": float64(3), "5": float64(100)},
						map[string]interface{}{"id": float64(4), "5": float64(100)},
						map[string]interface{}{"id": float64(5), "5": float64(100)},
						map[string]interface{}{"id": float64(6), "5": float64(100)},
						map[string]interface{}{"id": float64(7), "5": float64(100)},
						map[string]interface{}{"id": float64(8), "5": float64(100)},
						map[string]interface{}{"id": float64(9), "5": float64(100)},
						map[string]interface{}{"id": float64(10), "5": float64(100)},
						map[string]interface{}{"id": float64(11), "5": float64(100)},
						map[string]interface{}{"id": float64(12), "5": float64(100)},
						map[string]interface{}{"id": float64(13), "5": float64(100)},
						map[string]interface{}{"id": float64(14), "5": float64(100)},
						map[string]interface{}{"id": float64(15), "5": float64(100)},
						map[string]interface{}{"id": float64(16), "5": float64(100)},
						map[string]interface{}{"id": float64(17), "5": float64(100)},
						map[string]interface{}{"id": float64(18), "5": float64(100)},
					},
				},
			),
		}

		tool, err := NewLicenseComplianceTool(mockClient)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		report, err := tool.Execute(context.Background(), 100, 0)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Total: 20 purchased, 18 installed -> compliant
		if report.PurchasedCount != 20 {
			t.Errorf("PurchasedCount = %d, want 20", report.PurchasedCount)
		}
		if report.InstalledCount != 18 {
			t.Errorf("InstalledCount = %d, want 18", report.InstalledCount)
		}
		if report.Status != StatusCompliant {
			t.Errorf("Status = %q, want %q", report.Status, StatusCompliant)
		}
		if len(report.Licenses) != 3 {
			t.Errorf("Licenses length = %d, want 3", len(report.Licenses))
		}
	})

	t.Run("Name returns correct tool name", func(t *testing.T) {
		mockClient := &MockClient{}
		tool, _ := NewLicenseComplianceTool(mockClient)
		if tool.Name() != "glpi_license_compliance" {
			t.Errorf("Name() = %q, want %q", tool.Name(), "glpi_license_compliance")
		}
	})

	t.Run("Description returns non-empty description", func(t *testing.T) {
		mockClient := &MockClient{}
		tool, _ := NewLicenseComplianceTool(mockClient)
		if tool.Description() == "" {
			t.Error("Description() should not be empty")
		}
	})
}

func TestComputeStatus(t *testing.T) {
	tests := []struct {
		name      string
		purchased int
		installed int
		want      ComplianceStatus
	}{
		{"both zero -> unused", 0, 0, StatusUnused},
		{"exact match -> compliant", 10, 10, StatusCompliant},
		{"surplus licenses -> compliant", 10, 5, StatusCompliant},
		{"deficit licenses -> over-installed", 5, 10, StatusOverInstalled},
		{"purchased only -> under-utilized", 10, 0, StatusUnderUtilized},
		{"installed only -> unlicensed", 0, 5, StatusUnlicensed},
		{"single seat exact -> compliant", 1, 1, StatusCompliant},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := computeStatus(tt.purchased, tt.installed)
			if got != tt.want {
				t.Errorf("computeStatus(%d, %d) = %q, want %q", tt.purchased, tt.installed, got, tt.want)
			}
		})
	}
}

func TestHelperFunctions(t *testing.T) {
	t.Run("toInt with nil", func(t *testing.T) {
		if got := toInt(nil); got != 0 {
			t.Errorf("toInt(nil) = %d, want 0", got)
		}
	})

	t.Run("toInt with float64", func(t *testing.T) {
		if got := toInt(float64(42.0)); got != 42 {
			t.Errorf("toInt(42.0) = %d, want 42", got)
		}
	})

	t.Run("toInt with int", func(t *testing.T) {
		if got := toInt(99); got != 99 {
			t.Errorf("toInt(99) = %d, want 99", got)
		}
	})

	t.Run("toInt with string", func(t *testing.T) {
		if got := toInt("123"); got != 123 {
			t.Errorf("toInt(\"123\") = %d, want 123", got)
		}
	})

	t.Run("toInt with invalid string", func(t *testing.T) {
		if got := toInt("abc"); got != 0 {
			t.Errorf("toInt(\"abc\") = %d, want 0", got)
		}
	})

	t.Run("toString with nil", func(t *testing.T) {
		if got := toString(nil); got != "" {
			t.Errorf("toString(nil) = %q, want \"\"", got)
		}
	})

	t.Run("toString with string", func(t *testing.T) {
		if got := toString("hello"); got != "hello" {
			t.Errorf("toString(\"hello\") = %q, want \"hello\"", got)
		}
	})

	t.Run("toString with int", func(t *testing.T) {
		if got := toString(42); got != "42" {
			t.Errorf("toString(42) = %q, want \"42\"", got)
		}
	})
}
