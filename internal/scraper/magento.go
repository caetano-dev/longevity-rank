package scraper

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"longevity-ranker/internal/models"
)

// --- Magento JSON Structures ---

type MagentoInit struct {
	SwatchOptions struct {
		MagentoSwatchesJsSwatchRenderer struct {
			JsonConfig MagentoJsonConfig `json:"jsonConfig"`
		} `json:"Magento_Swatches/js/swatch-renderer"`
	} `json:"[data-role=swatch-options]"`
}

type MagentoJsonConfig struct {
	Attributes   map[string]MagentoAttribute   `json:"attributes"`
	OptionPrices map[string]MagentoOptionPrice `json:"optionPrices"`
}

type MagentoAttribute struct {
	Code    string          `json:"code"`
	Label   string          `json:"label"`
	Options []MagentoOption `json:"options"`
}

type MagentoOption struct {
	ID       string   `json:"id"`
	Label    string   `json:"label"`
	Products []string `json:"products"`
}

type MagentoOptionPrice struct {
	FinalPrice struct {
		Amount float64 `json:"amount"`
	} `json:"finalPrice"`
}

// --- Scraper Logic ---

func FetchMagentoProducts(vendor models.Vendor) ([]models.Product, error) {
	client := &http.Client{Timeout: 30 * time.Second}
	fmt.Printf("🔍 Crawling %s (Magento)...\n", vendor.Name)

	baseURL, err := url.Parse(vendor.URL)
	if err != nil {
		return nil, err
	}

	shopBody, err := fetchBody(client, vendor.URL)
	if err != nil {
		return nil, err
	}

	// 1. ROBUST LINK FINDER
	// We look for hrefs inside anchors with specific classes known to be used in Magento themes.
	// - product-item-link (Standard Luma/Blank)
	// - product-name (DoNotAge/Goomento)
	// - product-title (Porto/Ultimo)
	reLinks := regexp.MustCompile(`class="(?:.*?(?:product-item-link|product-name|product-title).*?)"\s+href="([^"]+)"`)
	matches := reLinks.FindAllStringSubmatch(string(shopBody), -1)

	uniqueLinks := make(map[string]bool)
	for _, m := range matches {
		link := m[1]
		// Resolve relative URLs
		if !strings.HasPrefix(link, "http") {
			relURL, _ := url.Parse(link)
			link = baseURL.ResolveReference(relURL).String()
		}
		uniqueLinks[link] = true
	}

	fmt.Printf("   -> Found %d potential products.\n", len(uniqueLinks))

	var products []models.Product

	// 2. Visit Products
	for link := range uniqueLinks {
		time.Sleep(300 * time.Millisecond)

		pageBody, err := fetchBody(client, link)
		if err != nil {
			continue
		}

		// 3. Extract Title (Strategy Pattern)
		title := getTitleFromHTML(string(pageBody))
		if title == "Unknown Product" {
			// Optional: Log this for debugging
			// fmt.Printf("Skipping %s (No Title Found)\n", link)
			continue
		}

		// 4. Extract Configuration (Price/Variants)
		reScript := regexp.MustCompile(`(?s)<script type="text/x-magento-init">(.+?)</script>`)
		scripts := reScript.FindAllStringSubmatch(string(pageBody), -1)

		foundData := false

		for _, s := range scripts {
			scriptContent := s[1]
			if !strings.Contains(scriptContent, "jsonConfig") {
				continue
			}

			var specificData MagentoInit
			if err := json.Unmarshal([]byte(scriptContent), &specificData); err != nil {
				continue
			}
			
			config := specificData.SwatchOptions.MagentoSwatchesJsSwatchRenderer.JsonConfig
			if len(config.OptionPrices) == 0 {
				continue
			}

			// --- Variant Logic ---
			
			// A. Identify the "Purchase Option" attribute (One Time vs Subscription)
			var oneTimeProductIDs = make(map[string]bool)
			for _, attr := range config.Attributes {
				// Try to find attributes like "Purchase Option" or "Frequency"
				if strings.Contains(strings.ToLower(attr.Label), "purchase") || 
				   strings.Contains(strings.ToLower(attr.Code), "purchase") {
					for _, opt := range attr.Options {
						// We only want "One Time" or "Single" purchases for fair comparison
						if strings.Contains(strings.ToLower(opt.Label), "time") || 
						   strings.Contains(strings.ToLower(opt.Label), "single") {
							for _, pid := range opt.Products {
								oneTimeProductIDs[pid] = true
							}
						}
					}
				}
			}

			// B. Identify the "Size" or "Weight" attribute
			for _, attr := range config.Attributes {
				// Look for "Size", "Volume", "Weight", or "Pack"
				label := strings.ToLower(attr.Label)
				code := strings.ToLower(attr.Code)
				
				if strings.Contains(label, "size") || strings.Contains(label, "volume") || strings.Contains(code, "size") {
					for _, opt := range attr.Options {
						for _, variantID := range opt.Products {
							// Filter: If we found purchase options, enforce "One Time Only"
							if len(oneTimeProductIDs) > 0 && !oneTimeProductIDs[variantID] {
								continue
							}

							if priceData, ok := config.OptionPrices[variantID]; ok {
								products = append(products, models.Product{
									ID:     variantID,
									Title:  title,
									Handle: link,
									Variants: []models.Variant{
										{
											Price: fmt.Sprintf("%.2f", priceData.FinalPrice.Amount),
											Title: opt.Label, // This will be "100g", "60 Capsules", etc.
										},
									},
								})
							}
						}
					}
					foundData = true
				}
			}
		}

		// Fallback: If no complex JSON found, this might be a simple product.
		// (For this specific project, NMN usually has variants, so we can skip this for now to keep it clean)
		if !foundData {
			// fmt.Printf("   No variant data for %s\n", title)
		}
	}

	return products, nil
}

// getTitleFromHTML attempts multiple strategies to find the product name
func getTitleFromHTML(html string) string {
	// Strategy 1: Schema.org (Most reliable for SEO-optimized sites like DoNotAge)
	// Looks for: <h1 ... itemprop="name">Pure NMN</h1>
	reSchema := regexp.MustCompile(`<h1[^>]*itemprop="name"[^>]*>\s*(.*?)\s*</h1>`)
	if m := reSchema.FindStringSubmatch(html); len(m) > 1 {
		return strings.TrimSpace(m[1])
	}

	// Strategy 2: Standard Magento Luma
	// Looks for: <h1 class="page-title"><span class="base">Pure NMN</span></h1>
	reLuma := regexp.MustCompile(`<h1 class="page-title"[^>]*>\s*<span[^>]*>(.*?)</span>`)
	if m := reLuma.FindStringSubmatch(html); len(m) > 1 {
		return strings.TrimSpace(m[1])
	}

	// Strategy 3: Generic H1 with product-title class (DoNotAge fallback)
	reGeneric := regexp.MustCompile(`<h1 class="product-title"[^>]*>\s*(.*?)\s*</h1>`)
	if m := reGeneric.FindStringSubmatch(html); len(m) > 1 {
		return strings.TrimSpace(m[1])
	}

	return "Unknown Product"
}