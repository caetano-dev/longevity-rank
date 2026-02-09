package scraper

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url" // Added for dynamic URL parsing
	"regexp"
	"strings"
	"time"

	"longevity-ranker/internal/models"
)

// Generic LD+JSON structures
type LdJsonGraph struct {
	Graph []LdNode `json:"@graph"`
}

type LdNode struct {
	Type       interface{} `json:"@type"`
	Name       string      `json:"name"`
	HasVariant []LdVariant `json:"hasVariant"`
	Offers     *LdOffer    `json:"offers,omitempty"`
}

type LdVariant struct {
	Name   string  `json:"name"`
	Offers LdOffer `json:"offers"`
}

type LdOffer struct {
	Price         interface{} `json:"price"`
	PriceCurrency string      `json:"priceCurrency"`
}

func FetchLdJsonProducts(vendor models.Vendor) ([]models.Product, error) {
	client := &http.Client{Timeout: 30 * time.Second}
	fmt.Printf("🔍 Crawling %s (%s)...\n", vendor.Name, vendor.Type)

	// 1. Parse the Vendor Base URL (e.g. https://www.jinfiniti.com/shop/)
	baseURL, err := url.Parse(vendor.URL)
	if err != nil {
		return nil, fmt.Errorf("invalid vendor URL: %v", err)
	}

	shopBody, err := fetchBody(client, vendor.URL)
	if err != nil {
		return nil, err
	}

	// 2. Regex to find ALL product links
	// WooCommerce standard is often /product/..., but we capture the href content broadly
	// We look for anything that contains "/product/" in the path to filter out blog posts/pages
	reProductLink := regexp.MustCompile(`href="([^"]*?)"`)
	matches := reProductLink.FindAllStringSubmatch(string(shopBody), -1)

	uniqueLinks := make(map[string]bool)
	for _, m := range matches {
		rawLink := m[1]

		// Parse the found link relative to the base URL
		relURL, err := url.Parse(rawLink)
		if err != nil {
			continue // Skip invalid links
		}

		// Dynamically resolve it (handles "/product/x", "./x", and full "https://..." links)
		absURL := baseURL.ResolveReference(relURL)

		// Filter Logic:
		// 1. Must be on the same host (don't crawl external ads)
		// 2. Must contain "/product/" (specific to WooCommerce structure)
		if absURL.Host == baseURL.Host && strings.Contains(absURL.Path, "/product/") {
			uniqueLinks[absURL.String()] = true
		}
	}

	fmt.Printf("   -> Found %d unique product pages.\n", len(uniqueLinks))

	var products []models.Product

	// 3. Visit each product page
	for link := range uniqueLinks {
		// Polite rate limiting
		time.Sleep(300 * time.Millisecond)

		pageBody, err := fetchBody(client, link)
		if err != nil {
			fmt.Printf("Error fetching %s: %v\n", link, err)
			continue
		}

		// 4. Extract the LD+JSON block
		// We look for any script with type="application/ld+json"
		reSchema := regexp.MustCompile(`(?s)<script type="application/ld\+json"[^>]*>(.*?)</script>`)
		schemaMatches := reSchema.FindAllStringSubmatch(string(pageBody), -1)

		for _, match := range schemaMatches {
			var graph LdJsonGraph
			if err := json.Unmarshal([]byte(match[1]), &graph); err != nil {
				continue
			}

			// 5. Parse the Graph
			for _, node := range graph.Graph {
				if !isProductType(node.Type) {
					continue
				}

				// CASE A: Product Group (Variants like 30g, 100g)
				if len(node.HasVariant) > 0 {
					for _, v := range node.HasVariant {
						products = append(products, models.Product{
							ID:     v.Name,
							Title:  v.Name, // Using Variant Name as Title
							Handle: link,
							Variants: []models.Variant{
								{
									Price: fmt.Sprintf("%v", v.Offers.Price),
									Title: v.Name,
								},
							},
						})
					}
				} else if node.Offers != nil {
					// CASE B: Simple Product
					products = append(products, models.Product{
						ID:     node.Name,
						Title:  node.Name,
						Handle: link,
						Variants: []models.Variant{
							{
								Price: fmt.Sprintf("%v", node.Offers.Price),
								Title: node.Name,
							},
						},
					})
				}
			}
		}
	}

	return products, nil
}

// Helper to check if @type is "Product" or ["Product", ...]
func isProductType(t interface{}) bool {
	if s, ok := t.(string); ok {
		return s == "Product" || s == "ProductGroup"
	}
	if arr, ok := t.([]interface{}); ok {
		for _, v := range arr {
			if s, ok := v.(string); ok && (s == "Product" || s == "ProductGroup") {
				return true
			}
		}
	}
	return false
}

func fetchBody(client *http.Client, url string) ([]byte, error) {
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}
