package main

import (
	"strings"
	"testing"
)

func TestGetVersion(t *testing.T) {
	version := GetVersion()

	if !strings.Contains(version, "go-s3-uploader version") {
		t.Errorf("Expected version string to contain 'go-s3-uploader version', got: %s", version)
	}

	if !strings.Contains(version, "commit:") {
		t.Errorf("Expected version string to contain 'commit:', got: %s", version)
	}

	if !strings.Contains(version, "built:") {
		t.Errorf("Expected version string to contain 'built:', got: %s", version)
	}
}

func TestVersionVariables(t *testing.T) {
	// In test environment, these should have default values unless set at build time
	if Version == "" {
		t.Error("Version should not be empty")
	}

	if GitCommit == "" {
		t.Error("GitCommit should not be empty")
	}

	if BuildDate == "" {
		t.Error("BuildDate should not be empty")
	}
}
