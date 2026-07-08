package tui

// Version information set at build time via -ldflags
var (
	// Version is the semantic version tag, set at build time
	Version = "dev"
	// GitSHA is the short git commit hash, set at build time
	GitSHA = "dev"
)

// LogDestination is set by cmd/root.go before model creation.
// Valid values: "journal", "file", "stderr"
var LogDestination = "file"
