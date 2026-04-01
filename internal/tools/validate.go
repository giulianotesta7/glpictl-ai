package tools

import (
	"regexp"
)

// validItemTypeReg matches GLPI itemtype names (PascalCase alphanumeric).
var validItemTypeReg = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9]*$`)

// ValidateItemType checks if the given string is a valid GLPI itemtype.
// Valid itemtypes are PascalCase alphanumeric strings like "Computer", "NetworkEquipment".
func ValidateItemType(itemtype string) bool {
	return validItemTypeReg.MatchString(itemtype)
}
