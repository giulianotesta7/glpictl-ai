package tools

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strconv"
	"sync"
)

// RackEquipment represents a single piece of equipment placed in a rack.
type RackEquipment struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	ItemType    string `json:"itemtype"`
	Position    int    `json:"position"`
	Orientation string `json:"orientation"`
}

// RackCapacity represents the capacity data for a single rack.
type RackCapacity struct {
	RackID         int             `json:"rack_id"`
	RackName       string          `json:"rack_name"`
	TotalU         int             `json:"total_u"`
	UsedU          int             `json:"used_u"`
	AvailableU     int             `json:"available_u"`
	UtilizationPct float64         `json:"utilization_pct"`
	Equipment      []RackEquipment `json:"equipment"`
}

// UnplacedItem represents equipment not assigned to any rack.
type UnplacedItem struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	ItemType string `json:"itemtype"`
}

// RackCapacityReport is the top-level result for glpi_rack_capacity.
type RackCapacityReport struct {
	RackCount             int            `json:"rack_count"`
	TotalRackU            int            `json:"total_rack_u"`
	TotalUsedU            int            `json:"total_used_u"`
	TotalAvailableU       int            `json:"total_available_u"`
	OverallUtilizationPct float64        `json:"overall_utilization_pct"`
	Racks                 []RackCapacity `json:"racks"`
	UnplacedEquipment     []UnplacedItem `json:"unplaced_equipment,omitempty"`
}

// RackCapacityInput represents the input for the glpi_rack_capacity tool.
type RackCapacityInput struct {
	RackID          int  `json:"rack_id,omitempty"`
	IncludeUnplaced bool `json:"include_unplaced,omitempty"`
}

// rackInfo holds basic rack data fetched during discovery.
type rackInfo struct {
	id     int
	name   string
	height int // total U capacity
}

// RackCapacityTool provides rack capacity and utilization reporting.
type RackCapacityTool struct {
	client ToolClient
}

// NewRackCapacityTool creates a new rack capacity tool.
// Returns an error if the client is nil.
func NewRackCapacityTool(client ToolClient) (*RackCapacityTool, error) {
	if client == nil {
		return nil, fmt.Errorf("rack capacity tool: client cannot be nil")
	}
	return &RackCapacityTool{client: client}, nil
}

// Name returns the tool name for registration.
func (r *RackCapacityTool) Name() string {
	return "glpi_rack_capacity"
}

// Description returns the tool description.
func (r *RackCapacityTool) Description() string {
	return "Return rack capacity and utilization report for DCIM management, with equipment positions and optional unplaced equipment listing"
}

// GetInput returns a new input struct for the tool.
func (r *RackCapacityTool) GetInput() *RackCapacityInput {
	return &RackCapacityInput{}
}

// Execute performs a rack capacity analysis.
// When rackID > 0, returns capacity for that specific rack only.
// When rackID == 0, returns capacity for all racks.
// When includeUnplaced is true, also lists equipment not assigned to any rack.
func (r *RackCapacityTool) Execute(ctx context.Context, rackID int, includeUnplaced bool) (*RackCapacityReport, error) {
	var racks []rackInfo

	if rackID > 0 {
		// Single rack mode: fetch the specific rack
		ri, err := r.fetchSingleRack(ctx, rackID)
		if err != nil {
			return nil, fmt.Errorf("rack capacity [single rack]: %w", err)
		}
		racks = []rackInfo{ri}
	} else {
		// All racks mode: search for all racks
		var err error
		racks, err = r.fetchAllRacks(ctx)
		if err != nil {
			return nil, fmt.Errorf("rack capacity [all racks]: %w", err)
		}
	}

	// Build capacity data for each rack concurrently
	rackCapacities := make([]RackCapacity, len(racks))
	var mu sync.Mutex
	var firstErr error
	var errMu sync.Mutex

	var wg sync.WaitGroup
	for i, rack := range racks {
		wg.Add(1)
		go func(idx int, ri rackInfo) {
			defer wg.Done()

			capacity, err := r.computeRackCapacity(ctx, ri)
			if err != nil {
				errMu.Lock()
				if firstErr == nil {
					firstErr = fmt.Errorf("rack capacity [rack %d]: %w", ri.id, err)
				}
				errMu.Unlock()
				return
			}

			mu.Lock()
			rackCapacities[idx] = capacity
			mu.Unlock()
		}(i, rack)
	}
	wg.Wait()

	if firstErr != nil {
		return nil, firstErr
	}

	// Sort racks by utilization percentage descending
	sort.Slice(rackCapacities, func(i, j int) bool {
		return rackCapacities[i].UtilizationPct > rackCapacities[j].UtilizationPct
	})

	// Compute aggregate totals
	totalRackU := 0
	totalUsedU := 0
	for _, rc := range rackCapacities {
		totalRackU += rc.TotalU
		totalUsedU += rc.UsedU
	}
	totalAvailableU := totalRackU - totalUsedU
	overallUtilizationPct := roundToOneDecimal(percentage(totalUsedU, totalRackU))

	report := &RackCapacityReport{
		RackCount:             len(rackCapacities),
		TotalRackU:            totalRackU,
		TotalUsedU:            totalUsedU,
		TotalAvailableU:       totalAvailableU,
		OverallUtilizationPct: overallUtilizationPct,
		Racks:                 rackCapacities,
	}

	// Fetch unplaced equipment if requested
	if includeUnplaced {
		unplaced, err := r.fetchUnplacedEquipment(ctx)
		if err != nil {
			return nil, fmt.Errorf("rack capacity [unplaced]: %w", err)
		}
		report.UnplacedEquipment = unplaced
	}

	return report, nil
}

// fetchSingleRack retrieves a specific rack by ID using the Get endpoint.
func (r *RackCapacityTool) fetchSingleRack(ctx context.Context, rackID int) (rackInfo, error) {
	var rackData map[string]interface{}
	endpoint := fmt.Sprintf("/Rack/%d", rackID)
	if err := r.client.Get(ctx, endpoint, &rackData); err != nil {
		return rackInfo{}, fmt.Errorf("fetch rack %d: %w", rackID, err)
	}
	if rackData == nil || len(rackData) == 0 {
		return rackInfo{}, fmt.Errorf("rack with ID %d not found", rackID)
	}

	return rackInfo{
		id:     rackID,
		name:   extractString(rackData, "name"),
		height: extractInt(rackData, "height"),
	}, nil
}

// fetchAllRacks searches for all Rack items and extracts basic info.
func (r *RackCapacityTool) fetchAllRacks(ctx context.Context) ([]rackInfo, error) {
	searchTool, err := NewSearchTool(r.client)
	if err != nil {
		return nil, fmt.Errorf("create search tool: %w", err)
	}

	// Search all racks with a criterion that matches everything
	result, err := searchTool.Execute(ctx, "Rack", []SearchCriterion{{
		FieldName:  "Rack.id",
		SearchType: "contains",
		Value:      "",
	}}, []string{}, nil)
	if err != nil {
		return nil, fmt.Errorf("search racks: %w", err)
	}

	racks := make([]rackInfo, 0, len(result.Data))
	for _, item := range result.Data {
		data := item.Data
		if data == nil {
			continue
		}

		// GLPI search results use field IDs as keys.
		// Common Rack fields: "1" = id, "3" = name, "5" = height
		// But with expand_dropdown or field selection, keys may be named.
		// Try named keys first, then fall back to field IDs.
		id := extractInt(data, "id")
		if id == 0 {
			id = extractInt(data, "1")
		}
		if id == 0 {
			id = extractInt(data, "2") // field "2" is often the ID in search results
		}

		name := extractString(data, "name")
		if name == "" {
			name = extractString(data, "3")
		}

		height := extractInt(data, "height")
		if height == 0 {
			height = extractInt(data, "5")
		}

		if id > 0 {
			racks = append(racks, rackInfo{
				id:     id,
				name:   name,
				height: height,
			})
		}
	}

	return racks, nil
}

// computeRackCapacity searches for equipment in a rack and computes utilization.
func (r *RackCapacityTool) computeRackCapacity(ctx context.Context, ri rackInfo) (RackCapacity, error) {
	// Search for equipment assigned to this rack across common rackable itemtypes.
	rackableItemtypes := []string{
		"Computer",
		"Monitor",
		"NetworkEquipment",
		"Peripheral",
		"Phone",
		"Enclosure",
		"PDU",
	}

	var equipment []RackEquipment
	var mu sync.Mutex
	var firstErr error
	var errMu sync.Mutex

	var wg sync.WaitGroup
	for _, itemtype := range rackableItemtypes {
		wg.Add(1)
		go func(it string) {
			defer wg.Done()

			searchTool, err := NewSearchTool(r.client)
			if err != nil {
				errMu.Lock()
				if firstErr == nil {
					firstErr = fmt.Errorf("search equipment [%s]: %w", it, err)
				}
				errMu.Unlock()
				return
			}

			result, err := searchTool.Execute(ctx, it, []SearchCriterion{{
				FieldName:  "Rack.id",
				SearchType: "equals",
				Value:      strconv.Itoa(ri.id),
			}}, []string{}, nil)
			if err != nil {
				// Some itemtypes may not have Rack association — skip gracefully
				// Only treat as error if it's not a "field not found" type error
				return
			}

			mu.Lock()
			defer mu.Unlock()

			for _, item := range result.Data {
				data := item.Data
				if data == nil {
					continue
				}

				eqID := extractInt(data, "id")
				if eqID == 0 {
					eqID = extractInt(data, "1")
				}
				if eqID == 0 {
					eqID = extractInt(data, "2")
				}

				eqName := extractString(data, "name")
				if eqName == "" {
					eqName = extractString(data, "3")
				}

				// Position field varies by itemtype; common field IDs:
				// NetworkEquipment: "33" = position
				// Computer: varies
				// Try named key first, then common field IDs
				position := extractInt(data, "position")
				if position == 0 {
					position = extractInt(data, "33")
				}

				// Orientation: "front", "rear", "middle"
				orientation := extractString(data, "orientation")
				if orientation == "" {
					orientation = extractString(data, "34")
				}
				if orientation == "" {
					// GLPI may return orientation as a dropdown object
					if orientObj, ok := data["orientation"]; ok {
						if orientMap, ok := orientObj.(map[string]interface{}); ok {
							orientation = extractString(orientMap, "name")
						}
					}
				}

				// Map GLPI orientation codes to human-readable values
				orientation = mapOrientation(orientation)

				if eqID > 0 {
					equipment = append(equipment, RackEquipment{
						ID:          eqID,
						Name:        eqName,
						ItemType:    it,
						Position:    position,
						Orientation: orientation,
					})
				}
			}
		}(itemtype)
	}
	wg.Wait()

	if firstErr != nil {
		return RackCapacity{}, firstErr
	}

	// Sort equipment by position (items with position > 0 first, then by position)
	sort.Slice(equipment, func(i, j int) bool {
		if equipment[i].Position == 0 && equipment[j].Position == 0 {
			return equipment[i].Name < equipment[j].Name
		}
		if equipment[i].Position == 0 {
			return false
		}
		if equipment[j].Position == 0 {
			return true
		}
		return equipment[i].Position < equipment[j].Position
	})

	// Compute used U: count of equipment with a valid position (> 0)
	usedU := 0
	for _, eq := range equipment {
		if eq.Position > 0 {
			usedU++
		}
	}

	totalU := ri.height
	if totalU < 0 {
		totalU = 0
	}
	availableU := totalU - usedU
	if availableU < 0 {
		availableU = 0
	}

	utilizationPct := roundToOneDecimal(percentage(usedU, totalU))

	return RackCapacity{
		RackID:         ri.id,
		RackName:       ri.name,
		TotalU:         totalU,
		UsedU:          usedU,
		AvailableU:     availableU,
		UtilizationPct: utilizationPct,
		Equipment:      equipment,
	}, nil
}

// fetchUnplacedEquipment searches for equipment not assigned to any rack.
func (r *RackCapacityTool) fetchUnplacedEquipment(ctx context.Context) ([]UnplacedItem, error) {
	rackableItemtypes := []string{
		"Computer",
		"Monitor",
		"NetworkEquipment",
		"Peripheral",
		"Phone",
		"Enclosure",
		"PDU",
	}

	var unplaced []UnplacedItem
	var mu sync.Mutex
	var firstErr error
	var errMu sync.Mutex

	var wg sync.WaitGroup
	for _, itemtype := range rackableItemtypes {
		wg.Add(1)
		go func(it string) {
			defer wg.Done()

			searchTool, err := NewSearchTool(r.client)
			if err != nil {
				errMu.Lock()
				if firstErr == nil {
					firstErr = fmt.Errorf("search unplaced [%s]: %w", it, err)
				}
				errMu.Unlock()
				return
			}

			// Search for items where Rack.id is empty (not assigned to any rack)
			result, err := searchTool.Execute(ctx, it, []SearchCriterion{{
				FieldName:  "Rack.id",
				SearchType: "contains",
				Value:      "",
			}}, []string{}, nil)
			if err != nil {
				// Some itemtypes may not have Rack association — skip gracefully
				return
			}

			mu.Lock()
			defer mu.Unlock()

			for _, item := range result.Data {
				data := item.Data
				if data == nil {
					continue
				}

				eqID := extractInt(data, "id")
				if eqID == 0 {
					eqID = extractInt(data, "1")
				}
				if eqID == 0 {
					eqID = extractInt(data, "2")
				}

				eqName := extractString(data, "name")
				if eqName == "" {
					eqName = extractString(data, "3")
				}

				if eqID > 0 {
					unplaced = append(unplaced, UnplacedItem{
						ID:       eqID,
						Name:     eqName,
						ItemType: it,
					})
				}
			}
		}(itemtype)
	}
	wg.Wait()

	if firstErr != nil {
		return nil, firstErr
	}

	// Sort by itemtype then name for consistent output
	sort.Slice(unplaced, func(i, j int) bool {
		if unplaced[i].ItemType != unplaced[j].ItemType {
			return unplaced[i].ItemType < unplaced[j].ItemType
		}
		return unplaced[i].Name < unplaced[j].Name
	})

	return unplaced, nil
}

// extractInt safely extracts an integer from a map, handling float64 JSON numbers.
func extractInt(data map[string]interface{}, key string) int {
	v, ok := data[key]
	if !ok {
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

// extractString safely extracts a string from a map.
func extractString(data map[string]interface{}, key string) string {
	v, ok := data[key]
	if !ok {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprintf("%v", v)
}

// percentage computes (part / total * 100), returning 0 if total is 0.
func percentage(part, total int) float64 {
	if total == 0 {
		return 0
	}
	return float64(part) / float64(total) * 100
}

// roundToOneDecimal rounds a float64 to one decimal place.
func roundToOneDecimal(f float64) float64 {
	return math.Round(f*10) / 10
}

// mapOrientation converts GLPI orientation codes to human-readable values.
func mapOrientation(orientation string) string {
	switch orientation {
	case "0":
		return "front"
	case "1":
		return "rear"
	case "2":
		return "middle"
	case "front", "rear", "middle":
		return orientation
	case "":
		return ""
	default:
		return orientation
	}
}

// Ensure RackCapacityTool implements the Tool interface.
var _ Tool = (*RackCapacityTool)(nil)
