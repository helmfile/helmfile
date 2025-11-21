package app

import (
	"encoding/json"
	"fmt"

	"github.com/helmfile/helmfile/pkg/yaml"
)

// PrintEnv prints the parsed environment configuration
func (a *App) PrintEnv(c PrintEnvConfigProvider) error {
	firstDoc := true

	// Open JSON array if needed
	if c.Output() == "json" {
		fmt.Println("[")
	}

	err := a.ForEachState(func(run *Run) (_ bool, errs []error) {
		st := run.state

		// Get merged values (includes secrets if present)
		values, err := st.Env.GetMergedValues()
		if err != nil {
			return false, []error{fmt.Errorf("failed to get merged values: %w", err)}
		}

		// Get full absolute path to identify which helmfile this environment comes from
		filePath := st.FilePath
		if fullPath, err := st.FullFilePath(); err != nil {
			a.Logger.Warnf("failed to get full file path for %s: %v", st.FilePath, err)
		} else {
			filePath = fullPath
		}

		// Prepare output structure - include file path to identify source
		output := map[string]any{
			"filePath":    filePath,
			"name":        st.Env.Name,
			"kubeContext": st.Env.KubeContext,
			"values":      values,
		}

		// Marshal based on output format
		var outputBytes []byte
		switch c.Output() {
		case "json":
			// For JSON, print array of documents
			if !firstDoc {
				fmt.Println(",")
			}
			outputBytes, err = json.MarshalIndent(output, "  ", "  ")
			if err != nil {
				return false, []error{fmt.Errorf("failed to marshal to JSON: %w", err)}
			}
			fmt.Print("  ")
			fmt.Print(string(outputBytes))
		case "yaml", "":
			// For YAML, use multi-document format with --- separator
			if !firstDoc {
				fmt.Println("---")
			}
			outputBytes, err = yaml.Marshal(output)
			if err != nil {
				return false, []error{fmt.Errorf("failed to marshal to YAML: %w", err)}
			}
			fmt.Print(string(outputBytes))
		default:
			return false, []error{fmt.Errorf("unsupported output format: %s (supported: yaml, json)", c.Output())}
		}

		firstDoc = false
		return false, nil
	}, false)

	// Close JSON array
	if c.Output() == "json" {
		fmt.Println()
		fmt.Println("]")
	}

	// Suppress "no releases found" error - print-env doesn't need releases
	if _, ok := err.(*NoMatchingHelmfileError); ok {
		return nil
	}

	return err
}
