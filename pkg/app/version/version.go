package version

import (
	classyversion "go.szostok.io/version"
)

var unknownVersion = "(devel)"

func Version() string {
	currentVersion := classyversion.Get().Version

	if currentVersion == unknownVersion {
		return "0.0.0-dev"
	}
	return currentVersion
}
