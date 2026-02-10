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

	// 1. GATEKEEPER: Ensure it's NMN
	// We check everything: Title, Context (SEO Title), and Handle (URL)
	identityString := strings.ToLower(p.Title + " " + p.Context + " " + p.Handle)
	if !strings.Contains(identityString, "nmn") { // TODO: will need refactoring. We will list other products soon.
		return nil
	}

	price, _ := strconv.ParseFloat(p.Variants[0].Price, 64)
	
	// 2. BUILD SEARCH STRING
	// Combine Clean Title + Hidden Context + Variant Title
	// This gives the regex "X-Ray vision" to find "500mg" even if not in the title.
	searchString := p.Title + " " + p.Context + " " + p.Variants[0].Title + " " + strings.ReplaceAll(p.Handle, "-", " ")

	capsuleMass := 0.0
	powderMass := 0.0
	packMultiplier := 1.0

	// 3. PRIORITY: POWDER
	// Check specifically for powder in the Title/Variant first.
	// This prevents "100g" from matching with "500mg" context to create a fake pill.
	cleanSearch := p.Title + " " + p.Variants[0].Title
	gramMatch := reGrams.FindStringSubmatch(cleanSearch)
	kgMatch := reKg.FindStringSubmatch(cleanSearch)

	if len(gramMatch) > 1 {
		grams, _ := strconv.ParseFloat(gramMatch[1], 64)
		powderMass = grams
	} else if len(kgMatch) > 1 {
		kg, _ := strconv.ParseFloat(kgMatch[1], 64)
		powderMass = kg * 1000.0 // Convert kg to grams
	} else {
		// 4. FALLBACK: CAPSULES
		// Only look for pills if we confirmed it's NOT a powder variant
		mgMatch := reMg.FindStringSubmatch(searchString)
		countMatch := reCount.FindStringSubmatch(searchString)

		if len(mgMatch) > 1 && len(countMatch) > 1 {
			mg, _ := strconv.ParseFloat(mgMatch[1], 64)
			count, _ := strconv.ParseFloat(countMatch[1], 64)
			capsuleMass = (mg * count) / 1000.0
		}
	}

	// 5. PACKS (1, 3, 6 Units)
	packMatch := rePack.FindStringSubmatch(searchString)
	if len(packMatch) > 1 {
		mult, _ := strconv.ParseFloat(packMatch[1], 64)
		packMultiplier = mult
	}

	// 6. TOTAL CALCULATION
	totalGrams := (capsuleMass + powderMass) * packMultiplier

	if totalGrams <= 0 {
		return nil
	}

	// 7. DETERMINE TYPE
	productType := "Single"
	if packMultiplier > 1 {
		productType = "Multi-Pack"
	} else if powderMass > 0 {
		productType = "Powder"
	} else {
		productType = "Capsules"
	}

	// 8. FINAL NAME FORMATTING
	// "Pure NMN (60 Capsules)" or "Pure NMN (100g)"
	displayName := p.Title
	if p.Variants[0].Title != "" {
		displayName = displayName + " (" + p.Variants[0].Title + ")"
	}

	return &models.Analysis{
		Vendor:      vendorName,
		Name:        displayName,
		Price:       price,
		TotalGrams:  totalGrams,
		CostPerGram: price / totalGrams,
		Type:        productType,
	}
}