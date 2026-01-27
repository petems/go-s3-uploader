package main

import "fmt"

var (
	// Version is the semantic version number
	Version = "dev"
	// GitCommit is the git commit SHA
	GitCommit = "unknown"
	// BuildDate is the date the binary was built
	BuildDate = "unknown"
)

// GetVersion returns the full version string
func GetVersion() string {
	return fmt.Sprintf("go-s3-uploader version %s (commit: %s, built: %s)", Version, GitCommit, BuildDate)
}
