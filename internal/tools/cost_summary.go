package tools

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"
)

// CostSummaryResult is the top-level result for glpi_cost_summary.
type CostSummaryResult struct {
	GeneratedAt          string             `json:"generated_at"`
	EntityID             int                `json:"entity_id,omitempty"`
	AssetTypeCosts       []AssetTypeCost    `json:"asset_type_costs"`
	ContractCosts        []ContractCost     `json:"contract_costs,omitempty"`
	BudgetAllocations    []BudgetAllocation `json:"budget_allocations,omitempty"`
	TotalAssetCost       float64            `json:"total_asset_cost"`
	TotalContractCost    float64            `json:"total_contract_cost"`
	TotalBudgetAllocated float64            `json:"total_budget_allocated"`
	GrandTotal           float64            `json:"grand_total"`
	HasErrors            bool               `json:"has_errors"`
	ErrorMessages        []string           `json:"error_messages,omitempty"`
}

// AssetTypeCost represents the aggregated cost for a single asset type.
type AssetTypeCost struct {
	Itemtype    string  `json:"itemtype"`
	Count       int     `json:"count"`
	TotalCost   float64 `json:"total_cost"`
	AverageCost float64 `json:"average_cost"`
}

// ContractCost represents the cost of a single contract.
type ContractCost struct {
	ID      int     `json:"id"`
	Name    string  `json:"name"`
	Cost    float64 `json:"cost"`
	EndDate string  `json:"end_date,omitempty"`
}

// BudgetAllocation represents a single budget item.
type BudgetAllocation struct {
	ID    int     `json:"id"`
	Name  string  `json:"name"`
	Value float64 `json:"value"`
}

// CostSummaryInput represents the input for the glpi_cost_summary tool.
type CostSummaryInput struct {
	EntityID         int  `json:"entity_id,omitempty"`
	IncludeContracts bool `json:"include_contracts,omitempty"`
	IncludeBudgets   bool `json:"include_budgets,omitempty"`
}

// CostSummaryTool provides aggregated cost summaries across GLPI inventory.
type CostSummaryTool struct {
	client ToolClient
}

// NewCostSummaryTool creates a new cost summary tool.
// Returns an error if the client is nil.
func NewCostSummaryTool(client ToolClient) (*CostSummaryTool, error) {
	if client == nil {
		return nil, fmt.Errorf("cost summary tool: client cannot be nil")
	}
	return &CostSummaryTool{client: client}, nil
}

// Name returns the tool name for registration.
func (c *CostSummaryTool) Name() string {
	return "glpi_cost_summary"
}

// Description returns the tool description.
func (c *CostSummaryTool) Description() string {
	return "Return a cost summary with total purchase value by asset type, contract costs, and budget allocations"
}

// GetInput returns a new input struct for the tool.
func (c *CostSummaryTool) GetInput() *CostSummaryInput {
	return &CostSummaryInput{}
}

// Execute generates a cost summary across all asset types and financial itemtypes.
// The entityID parameter is optional; if > 0, results are filtered to that entity.
// The includeContracts parameter defaults to true; when false, contract costs are omitted.
// The includeBudgets parameter defaults to true; when false, budget allocations are omitted.
// Returns a structured cost summary even if some queries fail.
func (c *CostSummaryTool) Execute(ctx context.Context, entityID int, includeContracts, includeBudgets bool) (*CostSummaryResult, error) {
	// Shared data structures for concurrent aggregation.
	type assetTypeResult struct {
		costs []AssetTypeCost
		err   error
	}

	var mu sync.Mutex
	var assetCosts []AssetTypeCost
	var contractCosts []ContractCost
	var budgetAllocations []BudgetAllocation
	var errorMessages []string

	var wg sync.WaitGroup

	// Phase 1: Aggregate asset costs concurrently.
	wg.Add(1)
	go func() {
		defer wg.Done()
		costs, errs, err := c.aggregateAssetCosts(ctx, entityID)
		mu.Lock()
		if err != nil {
			errorMessages = append(errorMessages, fmt.Sprintf("asset_costs: %v", err))
		} else {
			assetCosts = costs
			errorMessages = append(errorMessages, errs...)
		}
		mu.Unlock()
	}()

	// Phase 2: Aggregate contract costs (if requested).
	if includeContracts {
		wg.Add(1)
		go func() {
			defer wg.Done()
			costs, err := c.aggregateContractCosts(ctx, entityID)
			mu.Lock()
			if err != nil {
				errorMessages = append(errorMessages, fmt.Sprintf("contract_costs: %v", err))
			} else {
				contractCosts = costs
			}
			mu.Unlock()
		}()
	}

	// Phase 3: Aggregate budget allocations (if requested).
	if includeBudgets {
		wg.Add(1)
		go func() {
			defer wg.Done()
			allocs, err := c.aggregateBudgetAllocations(ctx, entityID)
			mu.Lock()
			if err != nil {
				errorMessages = append(errorMessages, fmt.Sprintf("budget_allocations: %v", err))
			} else {
				budgetAllocations = allocs
			}
			mu.Unlock()
		}()
	}

	wg.Wait()

	// Calculate totals.
	var totalAssetCost, totalContractCost, totalBudgetAllocated float64

	for _, ac := range assetCosts {
		totalAssetCost += ac.TotalCost
	}
	for _, cc := range contractCosts {
		totalContractCost += cc.Cost
	}
	for _, ba := range budgetAllocations {
		totalBudgetAllocated += ba.Value
	}

	grandTotal := totalAssetCost + totalContractCost

	// Sort asset costs by total_cost descending.
	sort.Slice(assetCosts, func(i, j int) bool {
		return assetCosts[i].TotalCost > assetCosts[j].TotalCost
	})

	// Sort contract costs by cost descending.
	sort.Slice(contractCosts, func(i, j int) bool {
		return contractCosts[i].Cost > contractCosts[j].Cost
	})

	// Sort budget allocations by value descending.
	sort.Slice(budgetAllocations, func(i, j int) bool {
		return budgetAllocations[i].Value > budgetAllocations[j].Value
	})

	result := &CostSummaryResult{
		GeneratedAt:          time.Now().Format(time.RFC3339),
		EntityID:             entityID,
		AssetTypeCosts:       assetCosts,
		ContractCosts:        contractCosts,
		BudgetAllocations:    budgetAllocations,
		TotalAssetCost:       totalAssetCost,
		TotalContractCost:    totalContractCost,
		TotalBudgetAllocated: totalBudgetAllocated,
		GrandTotal:           grandTotal,
		HasErrors:            len(errorMessages) > 0,
		ErrorMessages:        errorMessages,
	}

	return result, nil
}

// aggregateAssetCosts queries each hardware itemtype and aggregates purchase costs.
// Returns partial results even if some itemtype queries fail.
func (c *CostSummaryTool) aggregateAssetCosts(ctx context.Context, entityID int) ([]AssetTypeCost, []string, error) {
	searchTool, err := NewSearchTool(c.client)
	if err != nil {
		return nil, nil, fmt.Errorf("create search tool: %w", err)
	}

	type itemtypeCost struct {
		itemtype  string
		totalCost float64
		count     int
		err       error
	}

	results := make([]itemtypeCost, len(DefaultHardwareItemtypes))
	var mu sync.Mutex
	var wg sync.WaitGroup

	for i, itemtype := range DefaultHardwareItemtypes {
		wg.Add(1)
		go func(idx int, it string) {
			defer wg.Done()

			criteria := []SearchCriterion{
				{
					FieldName:  fmt.Sprintf("%s.warranty_date", it),
					SearchType: "exists",
					Value:      "",
				},
			}

			if entityID > 0 {
				criteria = append(criteria, SearchCriterion{
					FieldName:  fmt.Sprintf("%s.entities_id", it),
					SearchType: "equals",
					Value:      fmt.Sprintf("%d", entityID),
				})
			}

			result, err := searchTool.Execute(ctx, it, criteria, []string{}, nil)
			if err != nil {
				mu.Lock()
				results[idx] = itemtypeCost{itemtype: it, err: err}
				mu.Unlock()
				return
			}

			var totalCost float64
			for _, item := range result.Data {
				if item.Data == nil {
					continue
				}
				totalCost += extractPurchaseCost(item.Data, it)
			}

			mu.Lock()
			results[idx] = itemtypeCost{
				itemtype:  it,
				count:     result.TotalCount,
				totalCost: totalCost,
			}
			mu.Unlock()
		}(i, itemtype)
	}

	wg.Wait()

	var costs []AssetTypeCost
	var errorMessages []string
	for _, r := range results {
		if r.err != nil {
			errorMessages = append(errorMessages, fmt.Sprintf("%s: %v", r.itemtype, r.err))
			continue
		}
		if r.count > 0 {
			avgCost := r.totalCost / float64(r.count)
			costs = append(costs, AssetTypeCost{
				Itemtype:    r.itemtype,
				Count:       r.count,
				TotalCost:   r.totalCost,
				AverageCost: avgCost,
			})
		}
	}

	return costs, errorMessages, nil
}

// aggregateContractCosts queries all contracts and extracts their cost field.
func (c *CostSummaryTool) aggregateContractCosts(ctx context.Context, entityID int) ([]ContractCost, error) {
	searchTool, err := NewSearchTool(c.client)
	if err != nil {
		return nil, fmt.Errorf("create search tool: %w", err)
	}

	criteria := []SearchCriterion{
		{
			FieldName:  "Contract.name",
			SearchType: "contains",
			Value:      "",
		},
	}

	if entityID > 0 {
		criteria = append(criteria, SearchCriterion{
			FieldName:  "Contract.entities_id",
			SearchType: "equals",
			Value:      fmt.Sprintf("%d", entityID),
		})
	}

	result, err := searchTool.Execute(ctx, "Contract", criteria, []string{}, nil)
	if err != nil {
		return nil, fmt.Errorf("search contracts: %w", err)
	}

	var costs []ContractCost
	for _, item := range result.Data {
		if item.Data == nil {
			continue
		}

		cost := extractContractCost(item.Data)
		name := extractNameField(item.Data, "Contract")
		endDate := extractDateField(item.Data, "Contract.end_date")

		costs = append(costs, ContractCost{
			ID:      item.ID,
			Name:    name,
			Cost:    cost,
			EndDate: endDate,
		})
	}

	return costs, nil
}

// aggregateBudgetAllocations queries all budgets and extracts their value field.
func (c *CostSummaryTool) aggregateBudgetAllocations(ctx context.Context, entityID int) ([]BudgetAllocation, error) {
	searchTool, err := NewSearchTool(c.client)
	if err != nil {
		return nil, fmt.Errorf("create search tool: %w", err)
	}

	criteria := []SearchCriterion{
		{
			FieldName:  "Budget.name",
			SearchType: "contains",
			Value:      "",
		},
	}

	if entityID > 0 {
		criteria = append(criteria, SearchCriterion{
			FieldName:  "Budget.entities_id",
			SearchType: "equals",
			Value:      fmt.Sprintf("%d", entityID),
		})
	}

	result, err := searchTool.Execute(ctx, "Budget", criteria, []string{}, nil)
	if err != nil {
		return nil, fmt.Errorf("search budgets: %w", err)
	}

	var allocations []BudgetAllocation
	for _, item := range result.Data {
		if item.Data == nil {
			continue
		}

		value := extractBudgetValue(item.Data)
		name := extractNameField(item.Data, "Budget")

		allocations = append(allocations, BudgetAllocation{
			ID:    item.ID,
			Name:  name,
			Value: value,
		})
	}

	return allocations, nil
}

// extractContractCost extracts the cost value from contract search result data.
func extractContractCost(data map[string]interface{}) float64 {
	costFields := []string{
		"Contract.cost",
		"cost",
		"15", // GLPI field ID for cost in Contract
	}

	for _, f := range costFields {
		if v, ok := data[f]; ok {
			switch val := v.(type) {
			case float64:
				return val
			case string:
				if n, err := parseFloat(val); err == nil {
					return n
				}
			}
		}
	}

	return 0
}

// extractBudgetValue extracts the budget value from budget search result data.
func extractBudgetValue(data map[string]interface{}) float64 {
	valueFields := []string{
		"Budget.buy_value",
		"Budget.value",
		"buy_value",
		"value",
		"11", // GLPI field ID for buy_value in Budget
	}

	for _, f := range valueFields {
		if v, ok := data[f]; ok {
			switch val := v.(type) {
			case float64:
				return val
			case string:
				if n, err := parseFloat(val); err == nil {
					return n
				}
			}
		}
	}

	return 0
}

// Ensure CostSummaryTool implements the Tool interface.
var _ Tool = (*CostSummaryTool)(nil)
