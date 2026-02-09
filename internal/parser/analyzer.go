package parser

import (
	"longevity-ranker/internal/models"
	"regexp"
	"strconv"
	"strings"
)

var (
	// Regex patterns compiled once for performance
	reMg    = regexp.MustCompile(`(?i)(\d+)\s*mg`)
	reCount = regexp.MustCompile(`(?i)(\d+)\s*(?:capsules|caps|servings|tabs|tablets|ct)`)
	reGrams = regexp.MustCompile(`(?i)(\d+)\s*grams?`)
	rePack  = regexp.MustCompile(`(?i)(\d+)\s*Pack`)
)

func AnalyzeProduct(vendorName string, p models.Product) *models.Analysis {
	if len(p.Variants) == 0 {
		return nil
	}

	price, _ := strconv.ParseFloat(p.Variants[0].Price, 64)
	
	// SEARCH STRATEGY:
	// We combine Title + Handle (URL) to increase chances of finding data.
	// Renue often puts "100-grams" in the URL but not the title.
	searchString := p.Title + " " + strings.ReplaceAll(p.Handle, "-", " ")

	capsuleMass := 0.0
	powderMass := 0.0
	packMultiplier := 1.0

	// 1. Detect Pills/Capsules
	mgMatch := reMg.FindStringSubmatch(searchString)
	countMatch := reCount.FindStringSubmatch(searchString)

	if len(mgMatch) > 1 && len(countMatch) > 1 {
		mg, _ := strconv.ParseFloat(mgMatch[1], 64)
		count, _ := strconv.ParseFloat(countMatch[1], 64)
		capsuleMass = (mg * count) / 1000.0
	}

	// 2. Detect Powder (Grams)
	gramMatch := reGrams.FindStringSubmatch(searchString)
	if len(gramMatch) > 1 {
		grams, _ := strconv.ParseFloat(gramMatch[1], 64)
		powderMass = grams
	}

	// 3. Detect Packs
	packMatch := rePack.FindStringSubmatch(searchString)
	if len(packMatch) > 1 {
		mult, _ := strconv.ParseFloat(packMatch[1], 64)
		packMultiplier = mult
	}

	// 4. Calculate Total Active Mass
	// Note: We use additive logic so bundles (Pills + Powder) work correctly
	totalGrams := (capsuleMass + powderMass) * packMultiplier

	if totalGrams <= 0 {
		return nil // Could not calculate, skip this product
	}

	productType := "Single"
	if packMultiplier > 1 {
		productType = "Multi-Pack"
	} else if capsuleMass > 0 && powderMass > 0 {
		productType = "Hybrid Bundle"
	} else if powderMass > 0 {
		productType = "Powder"
	}

	return &models.Analysis{
		Vendor:      vendorName,
		Name:        p.Title,
		Price:       price,
		TotalGrams:  totalGrams,
		CostPerGram: price / totalGrams,
		Type:        productType,
	}
}