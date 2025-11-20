package app

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/gosuri/uitable"
)

// trimTrailingWhitespace removes trailing whitespace from each line in the input string.
// This ensures consistent output formatting by removing spaces and tabs that table
// formatting libraries may add to pad empty columns.
func trimTrailingWhitespace(s string) string {
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		// Only modify lines that actually have trailing whitespace
		if trimmed := strings.TrimRight(line, " \t"); trimmed != line {
			lines[i] = trimmed
		}
	}
	return strings.Join(lines, "\n")
}

func FormatAsTable(releases []*HelmRelease) error {
	table := uitable.New()
	table.AddRow("NAME", "NAMESPACE", "ENABLED", "INSTALLED", "LABELS", "CHART", "VERSION")

	for _, r := range releases {
		table.AddRow(r.Name, r.Namespace, fmt.Sprintf("%t", r.Enabled), fmt.Sprintf("%t", r.Installed), r.Labels, r.Chart, r.Version)
	}

	output := trimTrailingWhitespace(table.String())
	fmt.Println(output)

	return nil
}

func FormatAsJson(releases []*HelmRelease) error {
	output, err := json.Marshal(releases)

	if err != nil {
		return fmt.Errorf("error generating json: %v", err)
	}

	fmt.Println(string(output))

	return nil
}
