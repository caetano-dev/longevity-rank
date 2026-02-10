package scraper

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
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

// --- DoNotAge Bulk Config ---
type DnaBulkInit struct {
	BulkOptions struct {
		BulkConfig struct {
			BulkBuyConfig map[string]DnaTierInfo `json:"bulkBuyConfig"`
			DnaIdToSku    map[string]string      `json:"dnaIdToSku"`
		} `json:"bulkBuyConfig"`
	} `json:"DoNotAge_BulkBuy/js/catalog/product/view/bulkbuy-options"`
}

type DnaTierInfo struct {
	Eligible   bool               `json:"eligible"`
	TierPrices map[string]float64 `json:"tierPrices"`
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

	// 1. Link Finder
	reLinks := regexp.MustCompile(`class="(?:.*?(?:product-item-link|product-name|product-title).*?)"\s+href="([^"]+)"`)
	matches := reLinks.FindAllStringSubmatch(string(shopBody), -1)

	uniqueLinks := make(map[string]bool)
	for _, m := range matches {
		link := m[1]
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

		// 3. Extract Data
		cleanTitle := getCleanTitle(string(pageBody)) // "Pure NMN"
		seoContext := getSeoContext(string(pageBody)) // "Buy NMN 500mg..."

		// 4. Find Script Tags
		reScript := regexp.MustCompile(`(?s)<script type="text/x-magento-init">(.+?)</script>`)
		scripts := reScript.FindAllStringSubmatch(string(pageBody), -1)

		var stdConfig MagentoJsonConfig
		var bulkConfig DnaBulkInit
		hasStdConfig := false
		hasBulkConfig := false

		// Parse Scripts
		for _, s := range scripts {
			content := s[1]
			
			// Standard Config (Base Prices)
			if strings.Contains(content, "jsonConfig") {
				var wrapper MagentoInit
				if err := json.Unmarshal([]byte(content), &wrapper); err == nil {
					stdConfig = wrapper.SwatchOptions.MagentoSwatchesJsSwatchRenderer.JsonConfig
					if len(stdConfig.OptionPrices) > 0 {
						hasStdConfig = true
					}
				}
			}

			// Bulk Config (DoNotAge Bundles)
			if strings.Contains(content, "DoNotAge_BulkBuy") {
				var rawMap map[string]interface{}
				if err := json.Unmarshal([]byte(content), &rawMap); err == nil {
					// Handle the "*" selector in Magento init
					if inner, ok := rawMap["*"]; ok {
						innerBytes, _ := json.Marshal(inner)
						json.Unmarshal(innerBytes, &bulkConfig)
						if len(bulkConfig.BulkOptions.BulkConfig.DnaIdToSku) > 0 {
							hasBulkConfig = true
						}
					}
				}
			}
		}

		if !hasStdConfig {
			continue
		}

		// 5. Filter for "One Time Purchase"
		var oneTimeIDs = make(map[string]bool)
		for _, attr := range stdConfig.Attributes {
			if strings.Contains(strings.ToLower(attr.Label), "purchase") {
				for _, opt := range attr.Options {
					if strings.Contains(strings.ToLower(opt.Label), "one time") || 
					   strings.Contains(strings.ToLower(opt.Label), "single") {
						for _, pid := range opt.Products {
							oneTimeIDs[pid] = true
						}
					}
				}
			}
		}
		checkPurchaseOption := len(oneTimeIDs) > 0

		// 6. Generate Products
		for _, attr := range stdConfig.Attributes {
			label := strings.ToLower(attr.Label)
			// Look for Size/Volume attributes
			if strings.Contains(label, "size") || strings.Contains(label, "volume") {
				
				for _, opt := range attr.Options {
					for _, pid := range opt.Products {
						
						if checkPurchaseOption && !oneTimeIDs[pid] {
							continue
						}

						priceInfo, ok := stdConfig.OptionPrices[pid]
						if !ok {
							continue
						}

						// --- A. Base Product (1 Unit) ---
						basePrice := priceInfo.FinalPrice.Amount
						products = append(products, models.Product{
							ID:      pid,
							Title:   cleanTitle,
							Context: seoContext,
							Handle:  link,
							Variants: []models.Variant{
								{
									Price: fmt.Sprintf("%.2f", basePrice),
									Title: opt.Label, // e.g. "60 Capsules"
								},
							},
						})

						// --- B. Bulk Products (3 Pack, 6 Pack) ---
						if hasBulkConfig {
							sku, skuFound := bulkConfig.BulkOptions.BulkConfig.DnaIdToSku[pid]
							if skuFound {
								tierInfo, tierFound := bulkConfig.BulkOptions.BulkConfig.BulkBuyConfig[sku]
								if tierFound && tierInfo.Eligible {
									
									for qtyStr, unitPrice := range tierInfo.TierPrices {
										qty, _ := strconv.Atoi(qtyStr)
										if qty > 1 {
											// DoNotAge sends UNIT PRICE ($72). We need TOTAL PRICE ($216).
											totalPrice := unitPrice * float64(qty)
											
											products = append(products, models.Product{
												ID:      fmt.Sprintf("%s-%s", pid, qtyStr),
												Title:   cleanTitle,
												Context: seoContext,
												Handle:  link,
												Variants: []models.Variant{
													{
														Price: fmt.Sprintf("%.2f", totalPrice),
														// Append "Pack" so Analyzer multiplies the mass
														Title: fmt.Sprintf("%s - %d Pack", opt.Label, qty), 
													},
												},
											})
										}
									}
								}
							}
						}

					}
				}
			}
		}
	}

	return products, nil
}

func getCleanTitle(html string) string {
	// 1. Schema.org (Best for clean display name)
	reSchema := regexp.MustCompile(`<h1[^>]*itemprop="name"[^>]*>\s*(.*?)\s*</h1>`)
	if m := reSchema.FindStringSubmatch(html); len(m) > 1 {
		return strings.TrimSpace(m[1])
	}
	// 2. Generic H1
	reH1 := regexp.MustCompile(`<h1[^>]*>\s*(.*?)\s*</h1>`)
	if m := reH1.FindStringSubmatch(html); len(m) > 1 {
		clean := regexp.MustCompile(`<[^>]*>`).ReplaceAllString(m[1], "")
		return strings.TrimSpace(clean)
	}
	return "Unknown Product"
}

func getSeoContext(html string) string {
	// <title> tag contains the hidden dosage info "500mg"
	reTitle := regexp.MustCompile(`<title>(.*?)</title>`)
	if m := reTitle.FindStringSubmatch(html); len(m) > 1 {
		return strings.TrimSpace(m[1])
	}
	return ""
}