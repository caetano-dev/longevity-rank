package rules

import (
	"fmt"
	"strings"
	"longevity-ranker/internal/models"
)

// ProductSpec defines the "Physics" of a product that the scraper might miss
type ProductSpec struct {
	ForceType  string  // "Powder" or "Capsules"
	ForceMg    float64 // Dosage per serving/capsule
	ForceCount float64 // Total capsules or grams in the container
}

type VendorConfig struct {
	Blocklist []string
	Overrides map[string]ProductSpec
}

var Registry = map[string]VendorConfig{
	"Nutricost": {
		Blocklist: []string{"5-HTP", "Carnitine", "Caffeine", "Creatine", "Pre-Workout", "Gummies"},
	},
	"NMN Bio": {
		Blocklist: []string{"Bundle", "Endurance", "Book"},
		Overrides: map[string]ProductSpec{
			"nmn-supplement-500mg-capsules-30-caps": {
				ForceType:  "Capsules",
				ForceMg:    500,
				ForceCount: 30,
			},
		},
	},
	"Do Not Age": {
		Blocklist: []string{"Test", "Kit", "Consultation", "Subscription"},
	},
	"Renue By Science": {
		// Removed "Gel" and "NAD" from blocklist so we can track them
		Blocklist: []string{"Test", "Cream", "Serum", "Pet", "Cleanser", "Lotion"},
		Overrides: map[string]ProductSpec{
			// NMN Liposomal Gel: Website says "240mg" but misses "75 servings" (150ml bottle / 2ml serving)
			"nmn-lipo-gel-sublingual": {
				ForceType:  "Capsules", // Math-wise, a "Pump" behaves like a "Capsule" (Unit * Dosage)
				ForceMg:    240,        // 240mg per serving
				ForceCount: 75,         // 75 servings (150ml bottle)
			},
			// NAD+ Complete: Often misses count in title
			"lipo-nad-complete-powdered-liposomal": {
				ForceType:  "Capsules",
				ForceMg:    250, // Combined NAD+ precursors
				ForceCount: 90,
			},
			// Lipo NMN: Often misses count in title
			"lipo-nmn-powdered-liposomal-nmn2": {
				ForceType:  "Capsules",
				ForceMg:    250,
				ForceCount: 90,
			},
			// Fast Dissolve Tabs
			"nmns-240-ct-sublingual-tablets-2": {
				ForceType:  "Capsules",
				ForceMg:    125,
				ForceCount: 240,
			},
		},
	},
}

// ApplyRules enriches the product data with known facts from our database
func ApplyRules(vendorName string, p *models.Product) bool {
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
		// We append the hardcoded math data to the Context string.
		// The Analyzer will read this string and find the numbers it needs.
		extraContext := ""
		if spec.ForceMg > 0 {
			extraContext += fmt.Sprintf(" %0.fmg", spec.ForceMg)
		}
		if spec.ForceCount > 0 {
			if spec.ForceType == "Powder" {
				extraContext += fmt.Sprintf(" %0.fg", spec.ForceCount)
			} else {
				// We use the word "capsules" because the regex looks for it.
				// This works even for Pumps/Gels.
				extraContext += fmt.Sprintf(" %0.f capsules", spec.ForceCount)
			}
		}
		p.Context += extraContext
	}

	return true
}