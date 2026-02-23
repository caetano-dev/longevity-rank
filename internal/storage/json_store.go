package storage

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

const DataDir = "data"

// EnsureDataDir creates the data directory if it doesn't exist.
func EnsureDataDir() error {
	if _, err := os.Stat(DataDir); os.IsNotExist(err) {
		return os.Mkdir(DataDir, 0755)
	}
	return nil
}

// VendorFilename converts a vendor name to its JSON file path.
// Example: "Do Not Age" → "data/do_not_age.json"
func VendorFilename(vendorName string) string {
	clean := strings.ReplaceAll(strings.ToLower(vendorName), " ", "_")
	return filepath.Join(DataDir, clean+".json")
}

// SaveJSON marshals any value to pretty-printed JSON and writes it to path.
func SaveJSON[T any](path string, data T) error {
	bytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, bytes, 0644)
}

// LoadJSON reads a JSON file and unmarshals it into the target type.
func LoadJSON[T any](path string) (T, error) {
	var result T
	data, err := os.ReadFile(path)
	if err != nil {
		return result, err
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return result, err
	}
	return result, nil
}