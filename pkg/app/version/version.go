// Package version is used to get the version of the Helmfile CLI.
package version

import "fmt"

// Version is the version of Helmfile
var Version string

// Commit is the git revision
var Commit string

func GetVersion() string {
	if Version == "" {
		Version = "0.0.0-dev"
	}
	return Version
}

func GetCommit() string {
	if Commit == "" {
		Commit = "unknown_commit"
	}
	return Commit
}

func GetVersionWithCommit() string {
	return fmt.Sprintf("%s-%s", GetVersion(), GetCommit())
}
