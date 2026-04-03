package tools

import (
	"context"
	"fmt"
	"strings"
	"sync"
)

// PortInfo represents a single network port with its connection details.
type PortInfo struct {
	PortID   int    `json:"port_id"`
	PortName string `json:"port_name"`
	PortType string `json:"port_type"` // instantiation_type (NetworkPortEthernet, NetworkPortAggregate, etc.)
	MAC      string `json:"mac,omitempty"`
	IP       string `json:"ip,omitempty"`

	// Device owning this port
	DeviceID   int    `json:"device_id"`
	DeviceName string `json:"device_name"`
	DeviceType string `json:"device_type"`

	// Connection info (populated when port is connected)
	ConnectedPortID     int    `json:"connected_port_id,omitempty"`
	ConnectedPortName   string `json:"connected_port_name,omitempty"`
	ConnectedDeviceID   int    `json:"connected_device_id,omitempty"`
	ConnectedDeviceName string `json:"connected_device_name,omitempty"`
	ConnectedDeviceType string `json:"connected_device_type,omitempty"`
	CableID             int    `json:"cable_id,omitempty"`
}

// VLANInfo represents VLAN assignment on a network port.
type VLANInfo struct {
	PortID   int    `json:"port_id"`
	VLANID   int    `json:"vlan_id"`
	VLANName string `json:"vlan_name"`
	VLANTag  int    `json:"vlan_tag"`
}

// TopologyResult is the output of the glpi_network_topology tool.
type TopologyResult struct {
	Mode           string     `json:"mode"` // "port" or "device"
	Ports          []PortInfo `json:"ports"`
	VLANs          []VLANInfo `json:"vlans,omitempty"`
	TotalPorts     int        `json:"total_ports"`
	ConnectedPorts int        `json:"connected_ports"`
	TotalVLANs     int        `json:"total_vlans,omitempty"`
}

// NetworkTopologyInput represents the input for the glpi_network_topology tool.
type NetworkTopologyInput struct {
	PortID     int    `json:"port_id,omitempty"`     // Trace a specific port
	DeviceID   int    `json:"device_id,omitempty"`   // Show all ports for a device
	DeviceType string `json:"device_type,omitempty"` // Itemtype of the device (e.g., Computer, NetworkEquipment)
	ShowVLANs  bool   `json:"show_vlans,omitempty"`  // Include VLAN info on ports
}

// NetworkTopologyTool provides network topology tracing for GLPI inventory.
type NetworkTopologyTool struct {
	client ToolClient
}

// NewNetworkTopologyTool creates a new network topology tool.
// Returns an error if the client is nil.
func NewNetworkTopologyTool(client ToolClient) (*NetworkTopologyTool, error) {
	if client == nil {
		return nil, fmt.Errorf("network topology tool: client cannot be nil")
	}
	return &NetworkTopologyTool{client: client}, nil
}

// Name returns the tool name for registration.
func (n *NetworkTopologyTool) Name() string {
	return "glpi_network_topology"
}

// Description returns the tool description.
func (n *NetworkTopologyTool) Description() string {
	return "Trace network port connections and cable topology in GLPI inventory, with optional VLAN info"
}

// GetInput returns a new input struct for the tool.
func (n *NetworkTopologyTool) GetInput() *NetworkTopologyInput {
	return &NetworkTopologyInput{}
}

// Execute returns network topology data.
// When portID > 0, traces connections for that specific port.
// When deviceID > 0 and deviceType is set, shows all ports and connections for that device.
// When showVLANs is true, includes VLAN assignments on ports.
func (n *NetworkTopologyTool) Execute(ctx context.Context, portID int, deviceID int, deviceType string, showVLANs bool) (*TopologyResult, error) {
	// Validate input: must have either portID or (deviceID + deviceType)
	if deviceID > 0 && deviceType == "" {
		return nil, fmt.Errorf("device_type is required when device_id is provided")
	}

	if portID <= 0 && (deviceID <= 0 || deviceType == "") {
		return nil, fmt.Errorf("either port_id or (device_id + device_type) is required")
	}

	if deviceType != "" && !ValidateItemType(deviceType) {
		return nil, fmt.Errorf("invalid device_type: %q", deviceType)
	}

	var mode string
	var ports []PortInfo
	var allVLANs []VLANInfo

	if portID > 0 {
		// Single port mode
		mode = "port"
		port, err := n.fetchPortDetails(ctx, portID)
		if err != nil {
			return nil, fmt.Errorf("network topology [port %d]: %w", portID, err)
		}
		ports = []PortInfo{port}
	} else {
		// Device mode
		mode = "device"
		var err error
		ports, err = n.fetchDevicePorts(ctx, deviceID, deviceType)
		if err != nil {
			return nil, fmt.Errorf("network topology [device %s/%d]: %w", deviceType, deviceID, err)
		}
	}

	// Fetch connection details for each port concurrently
	if err := n.resolveConnections(ctx, ports); err != nil {
		return nil, fmt.Errorf("network topology [resolve connections]: %w", err)
	}

	// Fetch VLANs if requested
	if showVLANs {
		var err error
		allVLANs, err = n.fetchAllVLANs(ctx, ports)
		if err != nil {
			return nil, fmt.Errorf("network topology [vlans]: %w", err)
		}
	}

	// Count connected ports
	connectedCount := 0
	for _, p := range ports {
		if p.ConnectedPortID > 0 {
			connectedCount++
		}
	}

	return &TopologyResult{
		Mode:           mode,
		Ports:          ports,
		VLANs:          allVLANs,
		TotalPorts:     len(ports),
		ConnectedPorts: connectedCount,
		TotalVLANs:     len(allVLANs),
	}, nil
}

// fetchPortDetails retrieves a single NetworkPort by ID and resolves its connection.
func (n *NetworkTopologyTool) fetchPortDetails(ctx context.Context, portID int) (PortInfo, error) {
	var portData map[string]interface{}
	endpoint := fmt.Sprintf("/NetworkPort/%d", portID)
	if err := n.client.Get(ctx, endpoint, &portData); err != nil {
		return PortInfo{}, fmt.Errorf("fetch port %d: %w", portID, err)
	}

	if portData == nil || len(portData) == 0 {
		return PortInfo{}, fmt.Errorf("network port with ID %d not found", portID)
	}

	port := PortInfo{
		PortID:   portID,
		PortName: extractString(portData, "name"),
		PortType: extractString(portData, "instantiation_type"),
		MAC:      extractString(portData, "mac"),
		IP:       extractString(portData, "ip"),
		DeviceID: extractInt(portData, "items_id"),
	}

	// Device type from itemtype field
	if itemType, ok := portData["itemtype"]; ok {
		if s, ok := itemType.(string); ok {
			port.DeviceType = s
		}
	}

	// Fetch device name
	if port.DeviceID > 0 && port.DeviceType != "" {
		var deviceData map[string]interface{}
		deviceEndpoint := fmt.Sprintf("/%s/%d", port.DeviceType, port.DeviceID)
		if err := n.client.Get(ctx, deviceEndpoint, &deviceData); err == nil && deviceData != nil {
			port.DeviceName = extractString(deviceData, "name")
		}
	}

	// Resolve connection if present
	n.resolvePortConnection(ctx, &port, portData)

	return port, nil
}

// fetchDevicePorts searches for all NetworkPorts belonging to a specific device.
func (n *NetworkTopologyTool) fetchDevicePorts(ctx context.Context, deviceID int, deviceType string) ([]PortInfo, error) {
	searchTool, err := NewSearchTool(n.client)
	if err != nil {
		return nil, fmt.Errorf("create search tool: %w", err)
	}

	// Search for NetworkPorts where items_id = deviceID and itemtype = deviceType.
	// Use numeric field IDs directly to avoid search options resolution dependency:
	// NetworkPort field 4 = items_id, field 5 = itemtype
	result, err := searchTool.Execute(ctx, "NetworkPort", []SearchCriterion{
		{
			Field:      4, // items_id
			SearchType: "equals",
			Value:      fmt.Sprintf("%d", deviceID),
		},
		{
			Field:      5, // itemtype
			SearchType: "equals",
			Value:      deviceType,
			Link:       "AND",
		},
	}, []string{}, nil)
	if err != nil {
		return nil, fmt.Errorf("search ports for %s/%d: %w", deviceType, deviceID, err)
	}

	// Fetch device name once
	deviceName := ""
	var deviceData map[string]interface{}
	deviceEndpoint := fmt.Sprintf("/%s/%d", deviceType, deviceID)
	if err := n.client.Get(ctx, deviceEndpoint, &deviceData); err == nil && deviceData != nil {
		deviceName = extractString(deviceData, "name")
	}

	ports := make([]PortInfo, 0, len(result.Data))
	for _, item := range result.Data {
		data := item.Data
		if data == nil {
			continue
		}

		portID := extractInt(data, "id")
		if portID == 0 {
			portID = extractInt(data, "1")
		}
		if portID == 0 {
			portID = extractInt(data, "2")
		}
		if portID == 0 {
			continue
		}

		port := PortInfo{
			PortID:     portID,
			PortName:   extractString(data, "name"),
			PortType:   extractString(data, "instantiation_type"),
			MAC:        extractString(data, "mac"),
			IP:         extractString(data, "ip"),
			DeviceID:   deviceID,
			DeviceName: deviceName,
			DeviceType: deviceType,
		}
		ports = append(ports, port)
	}

	return ports, nil
}

// resolveConnections resolves connection details for all ports concurrently.
func (n *NetworkTopologyTool) resolveConnections(ctx context.Context, ports []PortInfo) error {
	var firstErr error
	var errMu sync.Mutex

	var wg sync.WaitGroup
	for i := range ports {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			// Fetch full port data to get connection info
			var portData map[string]interface{}
			endpoint := fmt.Sprintf("/NetworkPort/%d", ports[idx].PortID)
			if err := n.client.Get(ctx, endpoint, &portData); err != nil {
				errMu.Lock()
				if firstErr == nil {
					firstErr = fmt.Errorf("fetch port %d for connection: %w", ports[idx].PortID, err)
				}
				errMu.Unlock()
				return
			}

			n.resolvePortConnection(ctx, &ports[idx], portData)
		}(i)
	}
	wg.Wait()

	return firstErr
}

// resolvePortConnection extracts connection info from port data and resolves the remote endpoint.
func (n *NetworkTopologyTool) resolvePortConnection(ctx context.Context, port *PortInfo, portData map[string]interface{}) {
	// GLPI stores connection info in various fields depending on port type.
	// Common fields: itemtype_endpoint, items_id_endpoint for the connected port.
	// Cable info may be in a nested cable object or via Cable itemtype.

	endpointItemType := extractString(portData, "itemtype_endpoint")
	endpointItemID := extractInt(portData, "items_id_endpoint")

	if endpointItemType == "" || endpointItemID == 0 {
		// No direct connection — try NetworkPort_NetworkPort link
		endpointItemType = extractString(portData, "NetworkPort_NetworkPort.itemtype_endpoint")
		endpointItemID = extractInt(portData, "NetworkPort_NetworkPort.items_id_endpoint")
	}

	if endpointItemType == "" || endpointItemID == 0 {
		// Not connected or connection info not available
		return
	}

	port.ConnectedPortID = endpointItemID

	// Fetch connected port details
	var connectedPortData map[string]interface{}
	connectedEndpoint := fmt.Sprintf("/NetworkPort/%d", endpointItemID)
	if err := n.client.Get(ctx, connectedEndpoint, &connectedPortData); err != nil {
		return // Best effort — don't fail the whole topology
	}

	port.ConnectedPortName = extractString(connectedPortData, "name")
	port.ConnectedDeviceID = extractInt(connectedPortData, "items_id")

	if itemType, ok := connectedPortData["itemtype"]; ok {
		if s, ok := itemType.(string); ok {
			port.ConnectedDeviceType = s
		}
	}

	// Fetch connected device name
	if port.ConnectedDeviceID > 0 && port.ConnectedDeviceType != "" {
		var deviceData map[string]interface{}
		deviceEndpoint := fmt.Sprintf("/%s/%d", port.ConnectedDeviceType, port.ConnectedDeviceID)
		if err := n.client.Get(ctx, deviceEndpoint, &deviceData); err == nil && deviceData != nil {
			port.ConnectedDeviceName = extractString(deviceData, "name")
		}
	}

	// Try to find cable ID
	port.CableID = n.resolveCableID(ctx, port.PortID)
}

// resolveCableID attempts to find the Cable ID linking this port.
func (n *NetworkTopologyTool) resolveCableID(ctx context.Context, portID int) int {
	// Search for cables where this port is one endpoint
	searchTool, err := NewSearchTool(n.client)
	if err != nil {
		return 0
	}

	result, err := searchTool.Execute(ctx, "Cable", []SearchCriterion{
		{
			FieldName:  "Cable.itemtype_endpoint_a",
			SearchType: "equals",
			Value:      "NetworkPort",
		},
		{
			FieldName:  "Cable.items_id_endpoint_a",
			SearchType: "equals",
			Value:      fmt.Sprintf("%d", portID),
			Link:       "AND",
		},
	}, []string{}, nil)
	if err != nil || len(result.Data) == 0 {
		// Try the other endpoint (endpoint_b)
		result, err = searchTool.Execute(ctx, "Cable", []SearchCriterion{
			{
				FieldName:  "Cable.itemtype_endpoint_b",
				SearchType: "equals",
				Value:      "NetworkPort",
			},
			{
				FieldName:  "Cable.items_id_endpoint_b",
				SearchType: "equals",
				Value:      fmt.Sprintf("%d", portID),
				Link:       "AND",
			},
		}, []string{}, nil)
		if err != nil || len(result.Data) == 0 {
			return 0
		}
	}

	if len(result.Data) > 0 {
		return result.Data[0].ID
	}
	return 0
}

// fetchAllVLANs fetches VLAN assignments for all ports concurrently.
func (n *NetworkTopologyTool) fetchAllVLANs(ctx context.Context, ports []PortInfo) ([]VLANInfo, error) {
	var allVLANs []VLANInfo
	var mu sync.Mutex

	var wg sync.WaitGroup
	for _, port := range ports {
		wg.Add(1)
		go func(p PortInfo) {
			defer wg.Done()

			vlans, err := n.fetchVLANsForPort(ctx, p.PortID)
			if err != nil {
				// VLAN search may fail for some port types — skip gracefully
				return
			}

			mu.Lock()
			allVLANs = append(allVLANs, vlans...)
			mu.Unlock()
		}(port)
	}
	wg.Wait()

	return allVLANs, nil
}

// fetchVLANsForPort searches for VLANs assigned to a specific NetworkPort.
func (n *NetworkTopologyTool) fetchVLANsForPort(ctx context.Context, portID int) ([]VLANInfo, error) {
	searchTool, err := NewSearchTool(n.client)
	if err != nil {
		return nil, fmt.Errorf("create search tool: %w", err)
	}

	// Search NetworkPort_Vlan for this port
	result, err := searchTool.Execute(ctx, "NetworkPort_Vlan", []SearchCriterion{
		{
			FieldName:  "NetworkPort_Vlan.items_id",
			SearchType: "equals",
			Value:      fmt.Sprintf("%d", portID),
		},
	}, []string{}, nil)
	if err != nil {
		return nil, fmt.Errorf("search vlans for port %d: %w", portID, err)
	}

	vlans := make([]VLANInfo, 0, len(result.Data))
	for _, item := range result.Data {
		data := item.Data
		if data == nil {
			continue
		}

		vlan := VLANInfo{
			PortID:   portID,
			VLANID:   extractInt(data, "id"),
			VLANName: extractString(data, "name"),
			VLANTag:  extractInt(data, "tag"),
		}
		vlans = append(vlans, vlan)
	}

	return vlans, nil
}

// Ensure NetworkTopologyTool implements the Tool interface.
var _ Tool = (*NetworkTopologyTool)(nil)

// BuildTopologyText creates a human-readable topology summary.
func BuildTopologyText(result *TopologyResult) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("Network Topology (mode: %s)\n", result.Mode))
	sb.WriteString(fmt.Sprintf("Total ports: %d\n", result.TotalPorts))
	sb.WriteString(fmt.Sprintf("Connected ports: %d\n", result.ConnectedPorts))

	if len(result.Ports) == 0 {
		sb.WriteString("\nNo ports found.\n")
		return sb.String()
	}

	sb.WriteString("\n--- Ports ---\n")
	for _, p := range result.Ports {
		sb.WriteString(fmt.Sprintf("\nPort #%d: %s", p.PortID, p.PortName))
		if p.PortType != "" {
			sb.WriteString(fmt.Sprintf(" [%s]", p.PortType))
		}
		if p.MAC != "" {
			sb.WriteString(fmt.Sprintf(" MAC: %s", p.MAC))
		}
		if p.IP != "" {
			sb.WriteString(fmt.Sprintf(" IP: %s", p.IP))
		}
		sb.WriteString(fmt.Sprintf("\n  Device: %s (ID: %d, Type: %s)", p.DeviceName, p.DeviceID, p.DeviceType))

		if p.ConnectedPortID > 0 {
			sb.WriteString(fmt.Sprintf("\n  → Connected to: Port #%d %s", p.ConnectedPortID, p.ConnectedPortName))
			if p.ConnectedDeviceName != "" {
				sb.WriteString(fmt.Sprintf(" on %s (ID: %d, Type: %s)",
					p.ConnectedDeviceName, p.ConnectedDeviceID, p.ConnectedDeviceType))
			}
			if p.CableID > 0 {
				sb.WriteString(fmt.Sprintf(" [Cable #%d]", p.CableID))
			}
		} else {
			sb.WriteString("\n  → Not connected")
		}
	}

	if len(result.VLANs) > 0 {
		sb.WriteString(fmt.Sprintf("\n\n--- VLANs (%d) ---\n", result.TotalVLANs))
		for _, v := range result.VLANs {
			sb.WriteString(fmt.Sprintf("Port #%d: VLAN #%d (%s) tag=%d\n", v.PortID, v.VLANID, v.VLANName, v.VLANTag))
		}
	}

	return sb.String()
}
