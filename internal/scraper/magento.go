package scraper

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"slices"
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
	Attributes   map[string]MagentoAttribute    `json:"attributes"`
	OptionPrices map[string]MagentoOptionPrice  `json:"optionPrices"`
	Salable      map[string]map[string][]string `json:"salable"`
	Images       map[string][]MagentoImage      `json:"images"`
}

type MagentoImage struct {
	Img  string `json:"img"`
	Full string `json:"full"`
}

type MagentoAttribute struct {
	ID      string          `json:"id"`
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

	uniqueLinks := extractProductLinks(string(shopBody), baseURL)
	fmt.Printf("   -> Found %d potential products.\n", len(uniqueLinks))

	var products []models.Product

	for link := range uniqueLinks {
		time.Sleep(300 * time.Millisecond)

		pageBody, err := fetchBody(client, link)
		if err != nil {
			continue
		}

		pageProds := parseMagentoProductPage(string(pageBody), link)
		products = append(products, pageProds...)
	}

	return products, nil
}

// extractProductLinks finds all product URLs on the category page
func extractProductLinks(html string, baseURL *url.URL) map[string]bool {
	reLinks := regexp.MustCompile(`class="(?:.*?(?:product-item-link|product-name|product-title).*?)"\s+href="([^"]+)"`)
	matches := reLinks.FindAllStringSubmatch(html, -1)

	uniqueLinks := make(map[string]bool)
	for _, m := range matches {
		link := m[1]
		if !strings.HasPrefix(link, "http") {
			relURL, _ := url.Parse(link)
			link = baseURL.ResolveReference(relURL).String()
		}
		uniqueLinks[link] = true
	}
	return uniqueLinks
}

// parseMagentoProductPage processes a single product page HTML
func parseMagentoProductPage(html, link string) []models.Product {
	cleanTitle := getCleanTitle(html)
	seoContext := getSeoContext(html)
	description := getDescriptionFromHTML(html)
	fallbackImage := getImageFromHTML(html)

	stdConfig, bulkConfig, hasStdConfig := parseMagentoConfigs(html)

	if !hasStdConfig {
		return nil
	}

	oneTimeIDs, checkPurchaseOption := getOneTimePurchaseIDs(stdConfig)

	return extractVariants(stdConfig, bulkConfig, oneTimeIDs, checkPurchaseOption, cleanTitle, seoContext, description, fallbackImage, link)
}

// parseMagentoConfigs extracts the JSON blobs from the HTML scripts
func parseMagentoConfigs(html string) (MagentoJsonConfig, DnaBulkInit, bool) {
	var stdConfig MagentoJsonConfig
	var bulkConfig DnaBulkInit
	hasStdConfig := false

	reScript := regexp.MustCompile(`(?s)<script type="text/x-magento-init">(.+?)</script>`)
	scripts := reScript.FindAllStringSubmatch(html, -1)

	for _, s := range scripts {
		content := s[1]
		if strings.Contains(content, "jsonConfig") {
			var wrapper MagentoInit
			if err := json.Unmarshal([]byte(content), &wrapper); err == nil {
				stdConfig = wrapper.SwatchOptions.MagentoSwatchesJsSwatchRenderer.JsonConfig
				if len(stdConfig.OptionPrices) > 0 {
					hasStdConfig = true
				}
			}
		}
		if strings.Contains(content, "DoNotAge_BulkBuy") {
			var rawMap map[string]interface{}
			if err := json.Unmarshal([]byte(content), &rawMap); err == nil {
				if inner, ok := rawMap["*"]; ok {
					innerBytes, _ := json.Marshal(inner)
					json.Unmarshal(innerBytes, &bulkConfig)
				}
			}
		}
	}
	return stdConfig, bulkConfig, hasStdConfig
}

// getOneTimePurchaseIDs identifies products that are NOT subscriptions
func getOneTimePurchaseIDs(config MagentoJsonConfig) (map[string]bool, bool) {
	oneTimeIDs := make(map[string]bool)
	foundOption := false

	for _, attr := range config.Attributes {
		if strings.Contains(strings.ToLower(attr.Label), "purchase") {
			for _, opt := range attr.Options {
				lowerLabel := strings.ToLower(opt.Label)
				if strings.Contains(lowerLabel, "one time") || strings.Contains(lowerLabel, "single") {
					for _, pid := range opt.Products {
						oneTimeIDs[pid] = true
					}
					foundOption = true
				}
			}
		}
	}
	return oneTimeIDs, foundOption
}

// extractVariants iterates through attributes and builds the product list
func extractVariants(
	stdConfig MagentoJsonConfig,
	bulkConfig DnaBulkInit,
	oneTimeIDs map[string]bool,
	checkPurchaseOption bool,
	title, context, desc, fallbackImg, link string,
) []models.Product {

	var products []models.Product

	for _, attr := range stdConfig.Attributes {
		label := strings.ToLower(attr.Label)
		// We only care about attributes that define size/volume
		if !strings.Contains(label, "size") && !strings.Contains(label, "volume") {
			continue
		}

		for _, opt := range attr.Options {
			for _, pid := range opt.Products {

				// Filter out subscription products if a choice exists
				if checkPurchaseOption && !oneTimeIDs[pid] {
					continue
				}

				priceInfo, ok := stdConfig.OptionPrices[pid]
				if !ok {
					continue
				}

				isAvailable := checkAvailability(stdConfig, attr.ID, opt.ID, pid)
				variantImage := resolveImage(stdConfig, pid, fallbackImg)
				basePrice := priceInfo.FinalPrice.Amount

				// 1. Add Single Unit Product
				products = append(products, models.Product{
					ID:       pid,
					Title:    title,
					Context:  context,
					BodyHTML: desc,
					ImageURL: variantImage,
					Handle:   link,
					Variants: []models.Variant{
						{
							Price:     fmt.Sprintf("%.2f", basePrice),
							Title:     opt.Label,
							Available: isAvailable,
						},
					},
				})

				// 2. Add Bulk Packs (if applicable)
				bulkProds := extractBulkVariants(bulkConfig, pid, title, context, desc, variantImage, link, opt.Label, isAvailable)
				products = append(products, bulkProds...)
			}
		}
	}
	return products
}

// extractBulkVariants handles the logic for "Buy 3, Buy 6" deals
func extractBulkVariants(
	bulkConfig DnaBulkInit,
	pid, title, context, desc, img, link, label string,
	isAvailable bool,
) []models.Product {
	var bulkProducts []models.Product

	sku, skuFound := bulkConfig.BulkOptions.BulkConfig.DnaIdToSku[pid]
	if !skuFound {
		return nil
	}

	tierInfo, tierFound := bulkConfig.BulkOptions.BulkConfig.BulkBuyConfig[sku]
	if !tierFound || !tierInfo.Eligible {
		return nil
	}

	for qtyStr, unitPrice := range tierInfo.TierPrices {
		qty, _ := strconv.Atoi(qtyStr)
		if qty <= 1 {
			continue
		}

		totalPrice := unitPrice * float64(qty)

		bulkProducts = append(bulkProducts, models.Product{
			ID:       fmt.Sprintf("%s-%s", pid, qtyStr),
			Title:    title,
			Context:  context,
			BodyHTML: desc,
			ImageURL: img,
			Handle:   link,
			Variants: []models.Variant{
				{
					Price:     fmt.Sprintf("%.2f", totalPrice),
					Title:     fmt.Sprintf("%s - %d Pack", label, qty),
					Available: isAvailable,
				},
			},
		})
	}

	return bulkProducts
}

// checkAvailability checks if a variant ID is in the Salable map
func checkAvailability(config MagentoJsonConfig, attrID, optID, pid string) bool {
	if len(config.Salable) == 0 {
		return true
	}

	optionsMap, ok := config.Salable[attrID]
	if !ok {
		return true
	}

	validIDs, ok := optionsMap[optID]
	if !ok {
		return false
	}

	return slices.Contains(validIDs, pid)
}

// resolveImage picks the variant specific image or falls back to page image
func resolveImage(config MagentoJsonConfig, pid, fallback string) string {
	if imgs, ok := config.Images[pid]; ok && len(imgs) > 0 {
		if imgs[0].Full != "" {
			return imgs[0].Full
		}
		if imgs[0].Img != "" {
			return imgs[0].Img
		}
	}
	return fallback
}

// --- Regex Helpers ---

func getCleanTitle(html string) string {
	reSchema := regexp.MustCompile(`<h1[^>]*itemprop="name"[^>]*>\s*(.*?)\s*</h1>`)
	if m := reSchema.FindStringSubmatch(html); len(m) > 1 {
		return strings.TrimSpace(m[1])
	}
	reH1 := regexp.MustCompile(`<h1[^>]*>\s*(.*?)\s*</h1>`)
	if m := reH1.FindStringSubmatch(html); len(m) > 1 {
		clean := regexp.MustCompile(`<[^>]*>`).ReplaceAllString(m[1], "")
		return strings.TrimSpace(clean)
	}
	return "Unknown Product"
}

func getSeoContext(html string) string {
	reTitle := regexp.MustCompile(`<title>(.*?)</title>`)
	if m := reTitle.FindStringSubmatch(html); len(m) > 1 {
		return strings.TrimSpace(m[1])
	}
	return ""
}

func getDescriptionFromHTML(html string) string {
	reMeta := regexp.MustCompile(`<meta name="description" content="([^"]*?)"`)
	if m := reMeta.FindStringSubmatch(html); len(m) > 1 {
		return m[1]
	}
	reDesc := regexp.MustCompile(`class="product attribute description"[^>]*>.*?<div class="value"[^>]*>(.*?)</div>`)
	if m := reDesc.FindStringSubmatch(html); len(m) > 1 {
		clean := regexp.MustCompile(`<[^>]*>`).ReplaceAllString(m[1], " ")
		return strings.TrimSpace(clean)
	}
	return ""
}

func getImageFromHTML(html string) string {
	reSchema := regexp.MustCompile(`<meta itemprop="image" content="([^"]*?)"`)
	if m := reSchema.FindStringSubmatch(html); len(m) > 1 {
		return m[1]
	}
	reOg := regexp.MustCompile(`<meta property="og:image" content="([^"]*?)"`)
	if m := reOg.FindStringSubmatch(html); len(m) > 1 {
		return m[1]
	}
	return ""
}