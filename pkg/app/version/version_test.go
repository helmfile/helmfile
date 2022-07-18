package version

import (
	"testing"

	"gotest.tools/assert"
)

func TestGetVersion(t *testing.T) {
	current := Version
	Version = ""
	assert.Equal(t, "0.0.0-dev", GetVersion())
	Version = "1.2.3"
	assert.Equal(t, "1.2.3", GetVersion())
	Version = current
}

func TestGetCommit(t *testing.T) {
	current := Commit
	Commit = ""
	assert.Equal(t, "unknown_commit", GetCommit())
	Commit = "abc123xyz"
	assert.Equal(t, "abc123xyz", GetCommit())
	Commit = current
}

func TestGetVersionWithCommit(t *testing.T) {
	currentVersion := Version
	currentCommit := Commit
	Version = ""
	Commit = ""
	assert.Equal(t, "0.0.0-dev-unknown_commit", GetVersionWithCommit())
	Version = "1.2.3"
	Commit = "abc123xyz"
	assert.Equal(t, "1.2.3-abc123xyz", GetVersionWithCommit())
	Version = currentVersion
	Commit = currentCommit
}
