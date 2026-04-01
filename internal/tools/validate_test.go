package tools

import "testing"

func TestValidateItemType(t *testing.T) {
	tests := []struct {
		name      string
		itemtype  string
		wantValid bool
	}{
		{"valid Computer", "Computer", true},
		{"valid NetworkEquipment", "NetworkEquipment", true},
		{"valid User", "User", true},
		{"valid lowercase", "computer", true},
		{"valid with numbers", "Computer123", true},
		{"empty string", "", false},
		{"path traversal dots", "../admin", false},
		{"path traversal deep", "../../etc/passwd", false},
		{"slash separator", "Computer/5", false},
		{"semicolon injection", "Computer;DROP", false},
		{"dash not allowed", "Network-Equipment", false},
		{"space not allowed", "Computer Lab", false},
		{"underscore not allowed", "Computer_Type", false},
		{"starts with number", "123Computer", false},
		{"only numbers", "123", false},
		{"special chars", "Computer<script>", false},
		{"SQL injection", "'; DROP TABLE--", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ValidateItemType(tt.itemtype)
			if got != tt.wantValid {
				t.Errorf("ValidateItemType(%q) = %v, want %v", tt.itemtype, got, tt.wantValid)
			}
		})
	}
}
