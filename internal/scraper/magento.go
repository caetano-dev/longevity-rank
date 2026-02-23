package scraper

import (
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"longevity-ranker/internal/models"
)

// Hoisted compiled regexps — compiled once, used on every product page.
var (
	reProductLink = regexp.MustCompile(`class="(?:.*?(?:product-item-link|product-name|product-title).*?)"\s+href="([^"]+)"`)
	reScript      = regexp.MustCompile(`(?s)<script type="text/x-magento-init">(.+?)</script>`)
	reSchemaTitle = regexp.MustCompile(`<h1[^>]*itemprop="name"[^>]*>\s*(.*?)\s*</h1>`)
	reH1          = regexp.MustCompile(`<h1[^>]*>\s*(.*?)\s*</h1>`)
	reHTMLTag     = regexp.MustCompile(`<[^>]*>`)
	rePageTitle   = regexp.MustCompile(`<title>(.*?)</title>`)
	reMetaDesc    = regexp.MustCompile(`<meta name="description" content="([^"]*?)"`)
	reDescDiv     = regexp.MustCompile(`class="product attribute description"[^>]*>.*?<div class="value"[^>]*>(.*?)</div>`)
	reItemProp    = regexp.MustCompile(`<meta itemprop="image" content="([^"]*?)"`)
	reOgImage     = regexp.MustCompile(`<meta property="og:image" content="([^"]*?)"`)
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
			DnaIdToSku    map[string]string       `json:"dnaIdToSku"`
		} `json:"bulkBuyConfig"`
	} `json:"DoNotAge_BulkBuy/js/catalog/product/view/bulkbuy-options"`
}

type DnaTierInfo struct {
	Eligible   bool               `json:"eligible"`
	TierPrices map[string]float64 `json:"tierPrices"`
}

// --- Scraper Logic ---

func FetchMagentoProducts(vendor models.Vendor) ([]models.Product, error) {
	fmt.Printf("🔍 Crawling %s (Magento)...\n", vendor.Name)

	baseURL, err := url.Parse(vendor.URL)
	if err != nil {
		return nil, err
	}

	shopBody, err := FetchBody(vendor.URL)
	if err != nil {
		return nil, err
	}

	uniqueLinks := extractProductLinks(string(shopBody), baseURL)
	fmt.Printf("   -> Found %d potential products.\n", len(uniqueLinks))

	var products []models.Product
	for link := range uniqueLinks {
		time.Sleep(300 * time.Millisecond)

		pageBody, err := FetchBody(link)
		if err != nil {
			continue
		}

		products = append(products, parseMagentoProductPage(string(pageBody), link)...)
	}

	return products, nil
}

// extractProductLinks finds all product URLs on the category page.
func extractProductLinks(html string, baseURL *url.URL) map[string]bool {
	matches := reProductLink.FindAllStringSubmatch(html, -1)
	uniqueLinks := make(map[string]bool, len(matches))
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

// parseMagentoProductPage processes a single product page HTML.
func parseMagentoProductPage(html, link string) []models.Product {
	title := getCleanTitle(html)
	context := getSeoContext(html)
	desc := getDescriptionFromHTML(html)
	fallbackImg := getImageFromHTML(html)

	stdConfig, bulkConfig, ok := parseMagentoConfigs(html)
	if !ok {
		return nil
	}

	oneTimeIDs, checkPurchase := getOneTimePurchaseIDs(stdConfig)
	return extractVariants(stdConfig, bulkConfig, oneTimeIDs, checkPurchase, title, context, desc, fallbackImg, link)
}

// parseMagentoConfigs extracts the JSON blobs from the HTML scripts.
func parseMagentoConfigs(html string) (MagentoJsonConfig, DnaBulkInit, bool) {
	var stdConfig MagentoJsonConfig
	var bulkConfig DnaBulkInit
	hasStdConfig := false

	for _, s := range reScript.FindAllStringSubmatch(html, -1) {
		content := s[1]
		if strings.Contains(content, "jsonConfig") {
			var wrapper MagentoInit
			if err := json.Unmarshal([]byte(content), &wrapper); err == nil {
				stdConfig = wrapper.SwatchOptions.MagentoSwatchesJsSwatchRenderer.JsonConfig
				hasStdConfig = len(stdConfig.OptionPrices) > 0
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

// getOneTimePurchaseIDs identifies product IDs that are NOT subscriptions.
func getOneTimePurchaseIDs(config MagentoJsonConfig) (map[string]bool, bool) {
	oneTimeIDs := make(map[string]bool)
	found := false

	for _, attr := range config.Attributes {
		if !strings.Contains(strings.ToLower(attr.Label), "purchase") {
			continue
		}
		for _, opt := range attr.Options {
			lower := strings.ToLower(opt.Label)
			if strings.Contains(lower, "one time") || strings.Contains(lower, "single") {
				for _, pid := range opt.Products {
					oneTimeIDs[pid] = true
				}
				found = true
			}
		}
	}
	return oneTimeIDs, found
}

// extractVariants iterates through size/volume attributes and builds the product list.
func extractVariants(
	stdConfig MagentoJsonConfig,
	bulkConfig DnaBulkInit,
	oneTimeIDs map[string]bool,
	checkPurchase bool,
	title, context, desc, fallbackImg, link string,
) []models.Product {
	var products []models.Product

	for _, attr := range stdConfig.Attributes {
		label := strings.ToLower(attr.Label)
		if !strings.Contains(label, "size") && !strings.Contains(label, "volume") {
			continue
		}

		for _, opt := range attr.Options {
			for _, pid := range opt.Products {
				if checkPurchase && !oneTimeIDs[pid] {
					continue
				}

				priceInfo, ok := stdConfig.OptionPrices[pid]
				if !ok {
					continue
				}

				isAvailable := checkAvailability(stdConfig, attr.ID, opt.ID, pid)
				variantImage := resolveImage(stdConfig, pid, fallbackImg)
				basePrice := priceInfo.FinalPrice.Amount

				// Single unit product
				products = append(products, models.Product{
					ID:       pid,
					Title:    title,
					Context:  context,
					BodyHTML: desc,
					ImageURL: variantImage,
					Handle:   link,
					Variants: []models.Variant{{
						Price:     fmt.Sprintf("%.2f", basePrice),
						Title:     opt.Label,
						Available: isAvailable,
					}},
				})

				// Bulk packs
				products = append(products, extractBulkVariants(bulkConfig, pid, title, context, desc, variantImage, link, opt.Label, isAvailable)...)
			}
		}
	}
	return products
}

// extractBulkVariants handles "Buy 3, Buy 6" tier pricing.
func extractBulkVariants(
	bulkConfig DnaBulkInit,
	pid, title, context, desc, img, link, label string,
	isAvailable bool,
) []models.Product {
	sku, ok := bulkConfig.BulkOptions.BulkConfig.DnaIdToSku[pid]
	if !ok {
		return nil
	}
	tierInfo, ok := bulkConfig.BulkOptions.BulkConfig.BulkBuyConfig[sku]
	if !ok || !tierInfo.Eligible {
		return nil
	}

	var products []models.Product
	for qtyStr, unitPrice := range tierInfo.TierPrices {
		qty, _ := strconv.Atoi(qtyStr)
		if qty <= 1 {
			continue
		}
		products = append(products, models.Product{
			ID:       fmt.Sprintf("%s-%s", pid, qtyStr),
			Title:    title,
			Context:  context,
			BodyHTML: desc,
			ImageURL: img,
			Handle:   link,
			Variants: []models.Variant{{
				Price:     fmt.Sprintf("%.2f", unitPrice*float64(qty)),
				Title:     fmt.Sprintf("%s - %d Pack", label, qty),
				Available: isAvailable,
			}},
		})
	}
	return products
}

// checkAvailability checks if a variant ID is in the Salable map.
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

// resolveImage picks the variant-specific image or falls back to page image.
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

// --- HTML Extraction Helpers ---

func getCleanTitle(html string) string {
	if m := reSchemaTitle.FindStringSubmatch(html); len(m) > 1 {
		return strings.TrimSpace(m[1])
	}
	if m := reH1.FindStringSubmatch(html); len(m) > 1 {
		return strings.TrimSpace(reHTMLTag.ReplaceAllString(m[1], ""))
	}
	return "Unknown Product"
}

func getSeoContext(html string) string {
	if m := rePageTitle.FindStringSubmatch(html); len(m) > 1 {
		return strings.TrimSpace(m[1])
	}
	return ""
}

func getDescriptionFromHTML(html string) string {
	if m := reMetaDesc.FindStringSubmatch(html); len(m) > 1 {
		return m[1]
	}
	if m := reDescDiv.FindStringSubmatch(html); len(m) > 1 {
		return strings.TrimSpace(reHTMLTag.ReplaceAllString(m[1], " "))
	}
	return ""
}

func getImageFromHTML(html string) string {
	if m := reItemProp.FindStringSubmatch(html); len(m) > 1 {
		return m[1]
	}
	if m := reOgImage.FindStringSubmatch(html); len(m) > 1 {
		return m[1]
	}
	return ""
}