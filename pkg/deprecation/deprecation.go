package deprecation

var (
	deprecatedKeys = map[string]bool{
		"shell":      true,
		"silentuser": true,
	}
)

// Deprecated returns true if the key is deprecated
func Deprecated(k string) bool {
	if _, ok := deprecatedKeys[k]; ok {
		return deprecatedKeys[k]
	}
	return false
}
