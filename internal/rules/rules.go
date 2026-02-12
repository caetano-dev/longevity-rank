package rules

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"longevity-ranker/internal/models"
)

// ProductSpec defines the "Physics" of a product that the scraper might miss
type ProductSpec struct {
	ForceType  string  `json:"forceType"`
	ForceMg    float64 `json:"forceMg"`
	ForceCount float64 `json:"forceCount"`
}

type VendorConfig struct {
	Blocklist []string               `json:"blocklist"`
	Overrides map[string]ProductSpec `json:"overrides"`
}

// Registry holds the loaded rules
var Registry map[string]VendorConfig

// LoadRules reads the JSON configuration from disk
func LoadRules(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("could not open rules file: %v", err)
	}
	defer file.Close()

	bytes, err := io.ReadAll(file)
	if err != nil {
		return fmt.Errorf("could not read rules file: %v", err)
	}

	if err := json.Unmarshal(bytes, &Registry); err != nil {
		return fmt.Errorf("could not parse rules file: %v", err)
	}

	return nil
}

// ApplyRules enriches the product data with known facts from our database
func ApplyRules(vendorName string, p *models.Product) bool {
	if Registry == nil {
		return true // No rules loaded, safe default
	}

	config, exists := Registry[vendorName]
	if !exists {
		return true
	}

	// 1. Check Blocklist
	identity := strings.ToLower(p.Title + " " + p.Handle + " " + p.Context)
	for _, blocked := range config.Blocklist {
		if strings.Contains(identity, strings.ToLower(blocked)) {
			return false // Reject product
		}
	}

	// 2. Apply Overrides (The "Manual OCR")
	if spec, ok := config.Overrides[p.Handle]; ok {
		extraContext := ""
		if spec.ForceMg > 0 {
			extraContext += fmt.Sprintf(" %0.fmg", spec.ForceMg)
		}
		if spec.ForceCount > 0 {
			if spec.ForceType == "Powder" {
				extraContext += fmt.Sprintf(" %0.fg", spec.ForceCount)
			} else if spec.ForceType == "Tablets" {
				// NEW: Support for Tablets
				extraContext += fmt.Sprintf(" %0.f tablets", spec.ForceCount)
			} else {
				extraContext += fmt.Sprintf(" %0.f capsules", spec.ForceCount)
			}
		}
		p.Context += extraContext
	}

	return true
}