package app

import (
	"path/filepath"
)

var (
	renderExts = []string{".gotmpl"}
)

func isRequiredRender(helmfile string) bool {
	for _, ext := range renderExts {
		if filepath.Ext(helmfile) == ext {
			return true
		}
	}
	return false
}
