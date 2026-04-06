package cli

import (
	"os"
)

// Args holds parsed CLI arguments.
type Args struct {
	Mode           string // "branch", "unstaged", "all", or "" for auto-detect
	CommitRange    string
	PathFilter     string
	Verbose        bool
	FilePath       string
	ExportComments string // "" = disabled, "-" = stdout, path = file
	ExportFormat   string // "markdown" or "json"
}

// ParseTarget determines whether a positional argument is a file path or commit range.
func ParseTarget(target string) (filePath, commitRange string) {
	if target == "" {
		return "", ""
	}
	if info, err := os.Stat(target); err == nil && !info.IsDir() {
		return target, ""
	}
	// Check if it's a directory
	if info, err := os.Stat(target); err == nil && info.IsDir() {
		return target, ""
	}
	return "", target
}
