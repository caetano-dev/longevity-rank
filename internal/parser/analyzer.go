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
	reGrams = regexp.MustCompile(`(?i)(\d+)\s*(?:grams?|g)\b`) // \b ensures we don't match 'mg' as 'g'
	reKg    = regexp.MustCompile(`(?i)(\d+(?:\.\d+)?)\s*kg\b`)
	rePack  = regexp.MustCompile(`(?i)(\d+)\s*Pack`)
)

func AnalyzeProduct(vendorName string, p models.Product) *models.Analysis {
	if len(p.Variants) == 0 {
		return nil
	}

	identityString := strings.ToLower(p.Title + " " + p.Handle)

	if !strings.Contains(identityString, "nmn") { // TODO: Will need refactoring. We are going to add other products.
		return nil 
	}

	price, _ := strconv.ParseFloat(p.Variants[0].Price, 64)
	
	searchString := p.Title + " " + p.Variants[0].Title + " " + strings.ReplaceAll(p.Handle, "-", " ")

	capsuleMass := 0.0
	powderMass := 0.0
	packMultiplier := 1.0

	// A. Detect Pills (mg * count)
	mgMatch := reMg.FindStringSubmatch(searchString)
	countMatch := reCount.FindStringSubmatch(searchString)

	if len(mgMatch) > 1 && len(countMatch) > 1 {
		mg, _ := strconv.ParseFloat(mgMatch[1], 64)
		count, _ := strconv.ParseFloat(countMatch[1], 64)
		capsuleMass = (mg * count) / 1000.0 // Convert to grams
	}

	// B. Detect Powder (Grams or KG)
	gramMatch := reGrams.FindStringSubmatch(searchString)
	kgMatch := reKg.FindStringSubmatch(searchString)

	if len(gramMatch) > 1 {
		grams, _ := strconv.ParseFloat(gramMatch[1], 64)
		powderMass = grams
	} else if len(kgMatch) > 1 {
		kg, _ := strconv.ParseFloat(kgMatch[1], 64)
		powderMass = kg * 1000.0 // Convert kg to grams
	}

	// C. Detect "Packs" (Multipliers)
	packMatch := rePack.FindStringSubmatch(searchString)
	if len(packMatch) > 1 {
		mult, _ := strconv.ParseFloat(packMatch[1], 64)
		packMultiplier = mult
	}

	// D. Calculate Total Active NMN
	totalGrams := (capsuleMass + powderMass) * packMultiplier

	if totalGrams <= 0 {
		return nil // Could not calculate mass
	}

	// E. Determine Type for the Report
	productType := "Single"
	if packMultiplier > 1 {
		productType = "Multi-Pack"
	} else if capsuleMass > 0 && powderMass > 0 {
		productType = "Hybrid Bundle"
	} else if powderMass > 0 {
		productType = "Powder"
	} else {
		productType = "Capsules"
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