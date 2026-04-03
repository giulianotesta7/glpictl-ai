package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/giulianotesta7/glpictl-ai/internal/glpi"
)

// mockTopologyClient implements ToolClient for testing network topology.
type mockTopologyClient struct {
	getFunc        func(ctx context.Context, endpoint string, result interface{}) error
	searchFunc     func(ctx context.Context, itemtype string, criteria []SearchCriterion, fields []string, searchRange *SearchRange) (*SearchResult, error)
	searchOptsFunc func(ctx context.Context, itemtype string) (*glpi.SearchOptionsResult, error)
}

func (m *mockTopologyClient) InitSession(ctx context.Context) error { return nil }
func (m *mockTopologyClient) KillSession(ctx context.Context) error { return nil }
func (m *mockTopologyClient) SessionToken() string                  { return "test-token" }
func (m *mockTopologyClient) GLPIURL() string                       { return "http://test" }
func (m *mockTopologyClient) GetGLPIVersion(ctx context.Context) (string, error) {
	return "10.0.0", nil
}
func (m *mockTopologyClient) GetSearchOptions(ctx context.Context, itemtype string) (*glpi.SearchOptionsResult, error) {
	if m.searchOptsFunc != nil {
		return m.searchOptsFunc(ctx, itemtype)
	}
	return &glpi.SearchOptionsResult{ItemType: itemtype, Fields: []glpi.SearchOption{}}, nil
}
func (m *mockTopologyClient) Get(ctx context.Context, endpoint string, result interface{}) error {
	if m.getFunc != nil {
		return m.getFunc(ctx, endpoint, result)
	}
	return nil
}
func (m *mockTopologyClient) Post(ctx context.Context, endpoint string, body interface{}, result interface{}) error {
	return nil
}
func (m *mockTopologyClient) Put(ctx context.Context, endpoint string, body interface{}, result interface{}) error {
	return nil
}
func (m *mockTopologyClient) Delete(ctx context.Context, endpoint string, result interface{}) error {
	return nil
}

func TestNewNetworkTopologyTool(t *testing.T) {
	tests := []struct {
		name    string
		client  ToolClient
		wantErr bool
	}{
		{
			name:    "nil client returns error",
			client:  nil,
			wantErr: true,
		},
		{
			name:    "valid client returns tool",
			client:  &mockTopologyClient{},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tool, err := NewNetworkTopologyTool(tt.client)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				if tool != nil {
					t.Error("expected nil tool, got non-nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if tool == nil {
					t.Error("expected non-nil tool, got nil")
				}
			}
		})
	}
}

func TestNetworkTopologyTool_Name(t *testing.T) {
	tool, _ := NewNetworkTopologyTool(&mockTopologyClient{})
	if got := tool.Name(); got != "glpi_network_topology" {
		t.Errorf("Name() = %q, want %q", got, "glpi_network_topology")
	}
}

func TestNetworkTopologyTool_Description(t *testing.T) {
	tool, _ := NewNetworkTopologyTool(&mockTopologyClient{})
	desc := tool.Description()
	if desc == "" {
		t.Error("Description() returned empty string")
	}
}

func TestNetworkTopologyTool_GetInput(t *testing.T) {
	tool, _ := NewNetworkTopologyTool(&mockTopologyClient{})
	input := tool.GetInput()
	if input == nil {
		t.Error("GetInput() returned nil")
	}
}

func TestNetworkTopologyTool_Execute_Validation(t *testing.T) {
	client := &mockTopologyClient{}
	tool, _ := NewNetworkTopologyTool(client)

	tests := []struct {
		name        string
		portID      int
		deviceID    int
		deviceType  string
		showVLANs   bool
		wantErr     bool
		errContains string
	}{
		{
			name:        "no params returns error",
			portID:      0,
			deviceID:    0,
			deviceType:  "",
			wantErr:     true,
			errContains: "either port_id or",
		},
		{
			name:        "device_id without device_type returns error",
			portID:      0,
			deviceID:    5,
			deviceType:  "",
			wantErr:     true,
			errContains: "device_type is required",
		},
		{
			name:        "invalid device_type returns error",
			portID:      0,
			deviceID:    5,
			deviceType:  "invalid type!",
			wantErr:     true,
			errContains: "invalid device_type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tool.Execute(context.Background(), tt.portID, tt.deviceID, tt.deviceType, tt.showVLANs)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				} else if tt.errContains != "" && !containsStr(err.Error(), tt.errContains) {
					t.Errorf("error %q should contain %q", err.Error(), tt.errContains)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestNetworkTopologyTool_Execute_SinglePort(t *testing.T) {
	client := &mockTopologyClient{
		getFunc: func(ctx context.Context, endpoint string, result interface{}) error {
			out, ok := result.(*map[string]interface{})
			if !ok {
				return fmt.Errorf("invalid result type")
			}

			switch endpoint {
			case "/NetworkPort/42":
				*out = map[string]interface{}{
					"id":                 float64(42),
					"name":               "eth0",
					"instantiation_type": "NetworkPortEthernet",
					"mac":                "AA:BB:CC:DD:EE:FF",
					"items_id":           float64(10),
					"itemtype":           "Computer",
				}
			case "/Computer/10":
				*out = map[string]interface{}{
					"id":   float64(10),
					"name": "workstation-01",
				}
			default:
				*out = map[string]interface{}{}
			}
			return nil
		},
	}

	tool, _ := NewNetworkTopologyTool(client)
	result, err := tool.Execute(context.Background(), 42, 0, "", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Mode != "port" {
		t.Errorf("Mode = %q, want %q", result.Mode, "port")
	}
	if result.TotalPorts != 1 {
		t.Errorf("TotalPorts = %d, want 1", result.TotalPorts)
	}
	if len(result.Ports) != 1 {
		t.Fatalf("Ports length = %d, want 1", len(result.Ports))
	}

	port := result.Ports[0]
	if port.PortID != 42 {
		t.Errorf("PortID = %d, want 42", port.PortID)
	}
	if port.PortName != "eth0" {
		t.Errorf("PortName = %q, want %q", port.PortName, "eth0")
	}
	if port.PortType != "NetworkPortEthernet" {
		t.Errorf("PortType = %q, want %q", port.PortType, "NetworkPortEthernet")
	}
	if port.MAC != "AA:BB:CC:DD:EE:FF" {
		t.Errorf("MAC = %q, want %q", port.MAC, "AA:BB:CC:DD:EE:FF")
	}
	if port.DeviceID != 10 {
		t.Errorf("DeviceID = %d, want 10", port.DeviceID)
	}
	if port.DeviceName != "workstation-01" {
		t.Errorf("DeviceName = %q, want %q", port.DeviceName, "workstation-01")
	}
	if port.DeviceType != "Computer" {
		t.Errorf("DeviceType = %q, want %q", port.DeviceType, "Computer")
	}
}

func TestNetworkTopologyTool_Execute_SinglePort_NotFound(t *testing.T) {
	client := &mockTopologyClient{
		getFunc: func(ctx context.Context, endpoint string, result interface{}) error {
			out, ok := result.(*map[string]interface{})
			if !ok {
				return fmt.Errorf("invalid result type")
			}
			*out = map[string]interface{}{}
			return nil
		},
	}

	tool, _ := NewNetworkTopologyTool(client)
	_, err := tool.Execute(context.Background(), 999, 0, "", false)
	if err == nil {
		t.Error("expected error for non-existent port, got nil")
	}
}

func TestNetworkTopologyTool_Execute_SinglePort_Connected(t *testing.T) {
	client := &mockTopologyClient{
		getFunc: func(ctx context.Context, endpoint string, result interface{}) error {
			out, ok := result.(*map[string]interface{})
			if !ok {
				return fmt.Errorf("invalid result type")
			}

			switch endpoint {
			case "/NetworkPort/100":
				*out = map[string]interface{}{
					"id":                 float64(100),
					"name":               "uplink-1",
					"instantiation_type": "NetworkPortEthernet",
					"items_id":           float64(1),
					"itemtype":           "NetworkEquipment",
					"itemtype_endpoint":  "NetworkPort",
					"items_id_endpoint":  float64(200),
				}
			case "/NetworkPort/200":
				*out = map[string]interface{}{
					"id":       float64(200),
					"name":     "downlink-1",
					"items_id": float64(2),
					"itemtype": "Computer",
				}
			case "/NetworkEquipment/1":
				*out = map[string]interface{}{
					"id":   float64(1),
					"name": "core-switch-01",
				}
			case "/Computer/2":
				*out = map[string]interface{}{
					"id":   float64(2),
					"name": "server-02",
				}
			default:
				*out = map[string]interface{}{}
			}
			return nil
		},
	}

	tool, _ := NewNetworkTopologyTool(client)
	result, err := tool.Execute(context.Background(), 100, 0, "", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	port := result.Ports[0]
	if port.ConnectedPortID != 200 {
		t.Errorf("ConnectedPortID = %d, want 200", port.ConnectedPortID)
	}
	if port.ConnectedPortName != "downlink-1" {
		t.Errorf("ConnectedPortName = %q, want %q", port.ConnectedPortName, "downlink-1")
	}
	if port.ConnectedDeviceID != 2 {
		t.Errorf("ConnectedDeviceID = %d, want 2", port.ConnectedDeviceID)
	}
	if port.ConnectedDeviceName != "server-02" {
		t.Errorf("ConnectedDeviceName = %q, want %q", port.ConnectedDeviceName, "server-02")
	}
	if port.ConnectedDeviceType != "Computer" {
		t.Errorf("ConnectedDeviceType = %q, want %q", port.ConnectedDeviceType, "Computer")
	}
	if result.ConnectedPorts != 1 {
		t.Errorf("ConnectedPorts = %d, want 1", result.ConnectedPorts)
	}
}

func TestNetworkTopologyTool_Execute_DevicePorts(t *testing.T) {
	client := &mockTopologyClient{
		getFunc: func(ctx context.Context, endpoint string, result interface{}) error {
			out, ok := result.(*map[string]interface{})
			if !ok {
				return fmt.Errorf("invalid result type")
			}

			switch {
			case endpoint == "/NetworkEquipment/5":
				*out = map[string]interface{}{
					"id":   float64(5),
					"name": "switch-floor2",
				}
			case endpoint == "/NetworkPort/10":
				*out = map[string]interface{}{
					"id":                 float64(10),
					"name":               "port-1",
					"instantiation_type": "NetworkPortEthernet",
					"items_id":           float64(5),
					"itemtype":           "NetworkEquipment",
				}
			case endpoint == "/NetworkPort/11":
				*out = map[string]interface{}{
					"id":                 float64(11),
					"name":               "port-2",
					"instantiation_type": "NetworkPortEthernet",
					"items_id":           float64(5),
					"itemtype":           "NetworkEquipment",
				}
			case strings.HasPrefix(endpoint, "/search/NetworkPort"):
				*out = map[string]interface{}{
					"totalcount": float64(2),
					"data": []interface{}{
						map[string]interface{}{
							"id":                 float64(10),
							"1":                  float64(10),
							"2":                  float64(10),
							"3":                  "port-1",
							"name":               "port-1",
							"instantiation_type": "NetworkPortEthernet",
							"mac":                "AA:BB:CC:00:00:01",
						},
						map[string]interface{}{
							"id":                 float64(11),
							"1":                  float64(11),
							"2":                  float64(11),
							"3":                  "port-2",
							"name":               "port-2",
							"instantiation_type": "NetworkPortEthernet",
							"mac":                "AA:BB:CC:00:00:02",
						},
					},
				}
			case strings.HasPrefix(endpoint, "/search/Cable"):
				*out = map[string]interface{}{
					"totalcount": float64(0),
					"data":       []interface{}{},
				}
			default:
				*out = map[string]interface{}{}
			}
			return nil
		},
	}

	tool, _ := NewNetworkTopologyTool(client)
	result, err := tool.Execute(context.Background(), 0, 5, "NetworkEquipment", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Mode != "device" {
		t.Errorf("Mode = %q, want %q", result.Mode, "device")
	}
	if result.TotalPorts != 2 {
		t.Errorf("TotalPorts = %d, want 2", result.TotalPorts)
	}
	if len(result.Ports) != 2 {
		t.Fatalf("Ports length = %d, want 2", len(result.Ports))
	}

	// Verify device info is populated
	for _, p := range result.Ports {
		if p.DeviceID != 5 {
			t.Errorf("Port %d: DeviceID = %d, want 5", p.PortID, p.DeviceID)
		}
		if p.DeviceName != "switch-floor2" {
			t.Errorf("Port %d: DeviceName = %q, want %q", p.PortID, p.DeviceName, "switch-floor2")
		}
		if p.DeviceType != "NetworkEquipment" {
			t.Errorf("Port %d: DeviceType = %q, want %q", p.PortID, p.DeviceType, "NetworkEquipment")
		}
	}
}

func TestNetworkTopologyTool_Execute_DevicePorts_NoPorts(t *testing.T) {
	client := &mockTopologyClient{
		getFunc: func(ctx context.Context, endpoint string, result interface{}) error {
			out, ok := result.(*map[string]interface{})
			if !ok {
				return fmt.Errorf("invalid result type")
			}
			if strings.HasPrefix(endpoint, "/search/NetworkPort") {
				*out = map[string]interface{}{
					"totalcount": float64(0),
					"data":       []interface{}{},
				}
				return nil
			}
			*out = map[string]interface{}{
				"id":   float64(99),
				"name": "empty-device",
			}
			return nil
		},
	}

	tool, _ := NewNetworkTopologyTool(client)
	result, err := tool.Execute(context.Background(), 0, 99, "Computer", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.TotalPorts != 0 {
		t.Errorf("TotalPorts = %d, want 0", result.TotalPorts)
	}
	if result.ConnectedPorts != 0 {
		t.Errorf("ConnectedPorts = %d, want 0", result.ConnectedPorts)
	}
}

func TestBuildTopologyText(t *testing.T) {
	tests := []struct {
		name     string
		result   *TopologyResult
		contains []string
	}{
		{
			name: "single connected port",
			result: &TopologyResult{
				Mode:           "port",
				TotalPorts:     1,
				ConnectedPorts: 1,
				Ports: []PortInfo{
					{
						PortID:              100,
						PortName:            "eth0",
						PortType:            "NetworkPortEthernet",
						MAC:                 "AA:BB:CC:DD:EE:FF",
						DeviceID:            1,
						DeviceName:          "server-01",
						DeviceType:          "Computer",
						ConnectedPortID:     200,
						ConnectedPortName:   "uplink",
						ConnectedDeviceID:   2,
						ConnectedDeviceName: "core-switch",
						ConnectedDeviceType: "NetworkEquipment",
						CableID:             5,
					},
				},
			},
			contains: []string{
				"mode: port",
				"Total ports: 1",
				"Connected ports: 1",
				"eth0",
				"NetworkPortEthernet",
				"AA:BB:CC:DD:EE:FF",
				"server-01",
				"Connected to: Port #200 uplink",
				"core-switch",
				"Cable #5",
			},
		},
		{
			name: "unconnected port",
			result: &TopologyResult{
				Mode:           "port",
				TotalPorts:     1,
				ConnectedPorts: 0,
				Ports: []PortInfo{
					{
						PortID:     300,
						PortName:   "unused",
						DeviceID:   1,
						DeviceName: "server-01",
						DeviceType: "Computer",
					},
				},
			},
			contains: []string{
				"Not connected",
				"unused",
			},
		},
		{
			name: "with vlans",
			result: &TopologyResult{
				Mode:           "device",
				TotalPorts:     1,
				ConnectedPorts: 0,
				TotalVLANs:     1,
				Ports: []PortInfo{
					{
						PortID:     400,
						PortName:   "trunk",
						DeviceID:   1,
						DeviceName: "switch-01",
						DeviceType: "NetworkEquipment",
					},
				},
				VLANs: []VLANInfo{
					{PortID: 400, VLANID: 10, VLANName: "management", VLANTag: 100},
				},
			},
			contains: []string{
				"mode: device",
				"VLANs (1)",
				"management",
				"tag=100",
			},
		},
		{
			name: "no ports",
			result: &TopologyResult{
				Mode:       "device",
				TotalPorts: 0,
				Ports:      []PortInfo{},
			},
			contains: []string{
				"No ports found",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			text := BuildTopologyText(tt.result)
			for _, want := range tt.contains {
				if !containsStr(text, want) {
					t.Errorf("output should contain %q\nGot:\n%s", want, text)
				}
			}
		})
	}
}

func TestNetworkTopologyResult_JSONSerialization(t *testing.T) {
	result := &TopologyResult{
		Mode:           "port",
		TotalPorts:     1,
		ConnectedPorts: 1,
		TotalVLANs:     2,
		Ports: []PortInfo{
			{
				PortID:              1,
				PortName:            "eth0",
				PortType:            "NetworkPortEthernet",
				MAC:                 "AA:BB:CC:DD:EE:FF",
				IP:                  "192.168.1.1",
				DeviceID:            10,
				DeviceName:          "server-01",
				DeviceType:          "Computer",
				ConnectedPortID:     2,
				ConnectedPortName:   "uplink",
				ConnectedDeviceID:   20,
				ConnectedDeviceName: "switch-01",
				ConnectedDeviceType: "NetworkEquipment",
				CableID:             5,
			},
		},
		VLANs: []VLANInfo{
			{PortID: 1, VLANID: 1, VLANName: "default", VLANTag: 1},
		},
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var decoded TopologyResult
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if decoded.Mode != result.Mode {
		t.Errorf("Mode = %q, want %q", decoded.Mode, result.Mode)
	}
	if decoded.TotalPorts != result.TotalPorts {
		t.Errorf("TotalPorts = %d, want %d", decoded.TotalPorts, result.TotalPorts)
	}
	if len(decoded.Ports) != len(result.Ports) {
		t.Errorf("Ports length = %d, want %d", len(decoded.Ports), len(result.Ports))
	}
	if decoded.Ports[0].CableID != result.Ports[0].CableID {
		t.Errorf("CableID = %d, want %d", decoded.Ports[0].CableID, result.Ports[0].CableID)
	}
}

func TestNetworkTopologyTool_Execute_NetworkPortNotFound(t *testing.T) {
	client := &mockTopologyClient{
		getFunc: func(ctx context.Context, endpoint string, result interface{}) error {
			out, ok := result.(*map[string]interface{})
			if !ok {
				return fmt.Errorf("invalid result type")
			}
			*out = map[string]interface{}{}
			return nil
		},
	}

	tool, _ := NewNetworkTopologyTool(client)
	_, err := tool.Execute(context.Background(), 999, 0, "", false)
	if err == nil {
		t.Error("expected error for non-existent port, got nil")
	}
}
