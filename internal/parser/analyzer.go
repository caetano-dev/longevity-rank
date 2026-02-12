package parser

import (
	"math"
	"regexp"
	"strconv"
	"strings"

	"longevity-ranker/internal/models"
)

var (
	reMg      = regexp.MustCompile(`(?i)(\d+)\s*mg`)
	// Updated reCount to ensure we catch tabs/tablets explicitly
	reCount   = regexp.MustCompile(`(?i)(\d+)\s*(?:capsules|caps|servings|tabs|tablets|ct)`)
	reGrams   = regexp.MustCompile(`(?i)(\d+)\s*(?:grams?|g)\b`)
	reKg      = regexp.MustCompile(`(?i)(\d+(?:\.\d+)?)\s*kg\b`)
	rePack    = regexp.MustCompile(`(?i)(\d+)\s*Pack`)
	reServing = regexp.MustCompile(`(?i)(\d+)\s*(?:capsules|caps).*?per\s*serving`)
)

func AnalyzeProduct(vendorName string, p models.Product) *models.Analysis {
	if len(p.Variants) == 0 {
		return nil
	}

	// 1. GATEKEEPER
	identityString := strings.ToLower(p.Title + " " + p.Context + " " + p.Handle)
	if !strings.Contains(identityString, "nmn") && !strings.Contains(identityString, "nad") {
		return nil
	}

	var bestAnalysis *models.Analysis
	minCostPerGram := math.MaxFloat64

	for _, v := range p.Variants {
		price, _ := strconv.ParseFloat(v.Price, 64)
		if price <= 0 {
			continue
		}

		// 2. SEARCH STRING
		searchString := p.Title + " " + p.Context + " " + v.Title + " " + strings.ReplaceAll(p.Handle, "-", " ") + " " + p.BodyHTML

		capsuleMass := 0.0
		powderMass := 0.0
		packMultiplier := 1.0

		// 3. PRIORITY: POWDER
		cleanSearch := p.Title + " " + v.Title
		gramMatch := reGrams.FindStringSubmatch(cleanSearch)
		kgMatch := reKg.FindStringSubmatch(cleanSearch)

		if len(gramMatch) > 1 {
			grams, _ := strconv.ParseFloat(gramMatch[1], 64)
			powderMass = grams
		} else if len(kgMatch) > 1 {
			kg, _ := strconv.ParseFloat(kgMatch[1], 64)
			powderMass = kg * 1000.0
		} else {
			// 4. FALLBACK: CAPSULES/TABLETS
			mgMatch := reMg.FindStringSubmatch(searchString)
			countMatch := reCount.FindStringSubmatch(searchString)

			if len(mgMatch) > 1 && len(countMatch) > 1 {
				mg, _ := strconv.ParseFloat(mgMatch[1], 64)
				count, _ := strconv.ParseFloat(countMatch[1], 64)

				servingMatch := reServing.FindStringSubmatch(searchString)
				servingSize := 1.0
				if len(servingMatch) > 1 {
					s, _ := strconv.ParseFloat(servingMatch[1], 64)
					if s > 0 {
						servingSize = s
					}
				}
				capsuleMass = (mg / servingSize * count) / 1000.0
			}
		}

		if powderMass == 0 && capsuleMass == 0 {
			gramMatchBody := reGrams.FindStringSubmatch(searchString)
			if len(gramMatchBody) > 1 {
				grams, _ := strconv.ParseFloat(gramMatchBody[1], 64)
				powderMass = grams
			}
		}

		// 5. PACKS
		packMatch := rePack.FindStringSubmatch(searchString)
		if len(packMatch) > 1 {
			mult, _ := strconv.ParseFloat(packMatch[1], 64)
			packMultiplier = mult
		}

		// 6. CALCULATE MASS
		totalGrams := (capsuleMass + powderMass) * packMultiplier
		if totalGrams <= 0 {
			continue
		}

		// 7. CALCULATE COSTS
		costPerGram := price / totalGrams
		
		// 8. DETERMINE TYPE
		// We use a restricted search string for Type to avoid pollution from the BodyHTML
		typeSearch := strings.ToLower(p.Title + " " + v.Title + " " + p.Handle + " " + p.Context)
		productType := "Single"

		if packMultiplier > 1 {
			productType = "Multi-Pack"
		} else if capsuleMass > 0 && powderMass > 0 {
			productType = "Hybrid Bundle"
		} else if powderMass > 0 {
			productType = "Powder"
		} else if strings.Contains(typeSearch, "gel") && !strings.Contains(typeSearch, "softgel") {
			productType = "Gel"
		} else if strings.Contains(typeSearch, "tab") { // Catches "Tablets" or "Tabs"
			productType = "Tablets"
		} else {
			productType = "Capsules"
		}

		// 9. BIOAVAILABILITY
		multiplier := 1.0
		
		// Liposomal gets highest bonus
		if strings.Contains(typeSearch, "liposomal") || strings.Contains(typeSearch, "lipo") {
			multiplier = 1.5 
		} else if strings.Contains(typeSearch, "sublingual") || productType == "Gel" || productType == "Tablets" {
			// Gels and Fast Dissolve Tablets are inherently sublingual
			multiplier = 1.1 
		}
		
		effectiveCost := costPerGram / multiplier

		// 10. COMPARE
		if costPerGram < minCostPerGram {
			minCostPerGram = costPerGram

			displayName := p.Title
			if v.Title != "" && !strings.EqualFold(v.Title, "Default Title") {
				displayName = displayName + " (" + v.Title + ")"
			}

			bestAnalysis = &models.Analysis{
				Vendor:        vendorName,
				Name:          displayName,
				Price:         price,
				TotalGrams:    totalGrams,
				CostPerGram:   costPerGram,
				EffectiveCost: effectiveCost,
				Type:          productType,
			}
		}
	}

	return bestAnalysis
}