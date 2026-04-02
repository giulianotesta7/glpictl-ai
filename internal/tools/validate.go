package tools

import (
	"regexp"
)

// validItemTypeReg matches GLPI itemtype names (PascalCase alphanumeric, with underscores for link tables).
var validItemTypeReg = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]*$`)

// ValidateItemType checks if the given string is a valid GLPI itemtype.
// Valid itemtypes are PascalCase alphanumeric strings like "Computer", "NetworkEquipment",
// or link tables with underscores like "Item_SoftwareVersion".
func ValidateItemType(itemtype string) bool {
	return validItemTypeReg.MatchString(itemtype)
}
