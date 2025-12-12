package envfile

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// Load reads a dotenv file and sets environment variables.
// It does NOT override existing environment variables.
// If the file doesn't exist, it silently returns nil (no error).
// Lines that cannot be parsed (e.g., missing '=' sign, empty key) are silently skipped.
// This follows docker-compose behavior for optional env files.
func Load(path string) error {
	if path == "" {
		return nil
	}

	// Clean the path to normalize it
	path = filepath.Clean(path)

	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			// File doesn't exist - silently continue (docker-compose style)
			return nil
		}
		return err
	}
	defer func() { _ = file.Close() }()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Parse KEY=VALUE
		key, value, ok := parseEnvLine(line)
		if !ok {
			continue
		}

		// Don't override existing environment variables
		if _, exists := os.LookupEnv(key); exists {
			continue
		}

		_ = os.Setenv(key, value)
	}

	return scanner.Err()
}

// parseEnvLine parses a single line from a dotenv file.
// Returns key, value, and whether parsing succeeded.
func parseEnvLine(line string) (string, string, bool) {
	// Find the first = sign
	idx := strings.Index(line, "=")
	if idx == -1 {
		return "", "", false
	}

	key := strings.TrimSpace(line[:idx])
	if key == "" {
		return "", "", false
	}

	value := line[idx+1:]
	value = trimQuotes(value)

	return key, value, true
}

// trimQuotes removes surrounding quotes from a value
func trimQuotes(s string) string {
	s = strings.TrimSpace(s)

	// Handle double quotes
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		return s[1 : len(s)-1]
	}

	// Handle single quotes
	if len(s) >= 2 && s[0] == '\'' && s[len(s)-1] == '\'' {
		return s[1 : len(s)-1]
	}

	return s
}
