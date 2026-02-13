package storage

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"

	"longevity-ranker/internal/models"
)

const DataDir = "data"

// EnsureDataDir creates the data directory if it doesn't exist
func EnsureDataDir() error {
	if _, err := os.Stat(DataDir); os.IsNotExist(err) {
		return os.Mkdir(DataDir, 0755)
	}
	return nil
}

func GetFilename(vendorName string) string {
	// Clean string: "Do Not Age" -> "do_not_age.json"
	clean := strings.ReplaceAll(strings.ToLower(vendorName), " ", "_")
	return filepath.Join(DataDir, clean+".json")
}

func SaveProducts(vendorName string, products []models.Product) error {
	filename := GetFilename(vendorName)
	
	// Pretty print JSON so it's readable by humans
	file, err := json.MarshalIndent(products, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filename, file, 0644)
}

// SaveReport writes the final analysis report to data/analysis_report.json.
// This is the sole integration point between the Go backend and the Next.js frontend.
func SaveReport(report []models.Analysis) error {
	filename := filepath.Join(DataDir, "analysis_report.json")

	file, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filename, file, 0644)
}

func LoadProducts(vendorName string) ([]models.Product, error) {
	filename := GetFilename(vendorName)
	
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	bytes, _ := io.ReadAll(file)
	
	var products []models.Product
	if err := json.Unmarshal(bytes, &products); err != nil {
		return nil, err
	}

	return products, nil
}