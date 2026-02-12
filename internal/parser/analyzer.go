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

	identityString := strings.ToLower(p.Title + " " + p.Context + " " + p.Handle)
	if !strings.Contains(identityString, "nmn") && !strings.Contains(identityString, "nad") {
		return nil
	}

	var bestAnalysis *models.Analysis
	minCostPerGram := math.MaxFloat64

	for _, v := range p.Variants {
		if !v.Available {
			continue
		}

		price, _ := strconv.ParseFloat(v.Price, 64)
		if price <= 0 {
			continue
		}

		searchString := p.Title + " " + p.Context + " " + v.Title + " " + strings.ReplaceAll(p.Handle, "-", " ") + " " + p.BodyHTML

		capsuleMass := 0.0
		powderMass := 0.0
		packMultiplier := 1.0

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

		packMatch := rePack.FindStringSubmatch(searchString)
		if len(packMatch) > 1 {
			mult, _ := strconv.ParseFloat(packMatch[1], 64)
			packMultiplier = mult
		}

		totalGrams := (capsuleMass + powderMass) * packMultiplier
		if totalGrams <= 0 {
			continue
		}

		costPerGram := price / totalGrams
		
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
		} else if strings.Contains(typeSearch, "tab") { 
			productType = "Tablets"
		} else {
			productType = "Capsules"
		}

		multiplier := 1.0
		
		if strings.Contains(typeSearch, "liposomal") || strings.Contains(typeSearch, "lipo") {
			multiplier = 1.5 
		} else if strings.Contains(typeSearch, "sublingual") || productType == "Gel" || productType == "Tablets" {
			multiplier = 1.1 
		}
		
		effectiveCost := costPerGram / multiplier

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
				ImageURL:      p.ImageURL, // PASSTHROUGH
			}
		}
	}

	return bestAnalysis
}