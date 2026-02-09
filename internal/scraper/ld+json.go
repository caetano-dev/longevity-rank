package scraper

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"longevity-ranker/internal/models"
	"regexp"
	"strings"
	"time"
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

	shopBody, err := fetchBody(client, vendor.URL)
	if err != nil {
		return nil, err
	}

	// 1. Regex to find ALL product links (WooCommerce standard is /product/...)
	reProductLink := regexp.MustCompile(`href="([^"]*?/product/[^"]+)"`)
	matches := reProductLink.FindAllStringSubmatch(string(shopBody), -1)

	uniqueLinks := make(map[string]bool)
	for _, m := range matches {
		link := m[1]
		
		// --- THE FIX ---
		// Don't check against vendor.URL (which contains /shop/).
		// Instead, check if it's relative or part of the same domain.
		if strings.HasPrefix(link, "/") {
			// Convert relative link to absolute
			// Assumption: vendor.URL is like https://domain.com/shop/
			// We need the base domain. 
			// For simplicity in this scraper, we assume absolute links usually, 
			// or we just prepend the domain manually if we know it.
			// But Jinfiniti uses absolute links, so we focus on that.
			uniqueLinks["https://www.jinfiniti.com"+link] = true // Hardcoded for safety in this specific case, or parse URL properly
		} else if strings.Contains(link, "jinfiniti.com/product/") {
			uniqueLinks[link] = true
		}
	}

	fmt.Printf("   -> Found %d unique product pages.\n", len(uniqueLinks))
	
	var products []models.Product

	// 2. Visit each page
	for link := range uniqueLinks {
		// Polite rate limiting
		time.Sleep(300 * time.Millisecond) 

		pageBody, err := fetchBody(client, link)
		if err != nil {
			fmt.Printf("Error fetching %s: %v\n", link, err)
			continue
		}

		// 3. Extract the LD+JSON block
		reSchema := regexp.MustCompile(`(?s)<script type="application/ld\+json"[^>]*>(.*?)</script>`)
		schemaMatches := reSchema.FindAllStringSubmatch(string(pageBody), -1)

		foundJson := false
		for _, match := range schemaMatches {
			var graph LdJsonGraph
			if err := json.Unmarshal([]byte(match[1]), &graph); err != nil {
				continue
			}

			// 4. Parse the Graph
			for _, node := range graph.Graph {
				if !isProductType(node.Type) {
					continue
				}
				foundJson = true

				// CASE A: Product Group (Variants like 30g, 100g)
				if len(node.HasVariant) > 0 {
					for _, v := range node.HasVariant {
						products = append(products, models.Product{
							ID:     v.Name, // Use Name as ID
							Title:  v.Name, // e.g. "Pure NMN Powder - 120g"
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
		
		if !foundJson {
			// Debug line to see if we missed the JSON on a valid page
			// fmt.Printf("No valid Product JSON found on %s\n", link)
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