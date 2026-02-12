package scraper

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"longevity-ranker/internal/models"
)

type LdJsonGraph struct {
	Graph []LdNode `json:"@graph"`
}

type LdNode struct {
	Type        interface{} `json:"@type"`
	Name        string      `json:"name"`
	Description string      `json:"description"`
	HasVariant  []LdVariant `json:"hasVariant"`
	Offers      *LdOffer    `json:"offers,omitempty"`
}

type LdVariant struct {
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Offers      LdOffer `json:"offers"`
}

type LdOffer struct {
	Price         interface{} `json:"price"`
	PriceCurrency string      `json:"priceCurrency"`
	Availability  string      `json:"availability"` // NEW: Schema.org Availability
}

func FetchLdJsonProducts(vendor models.Vendor) ([]models.Product, error) {
	client := &http.Client{Timeout: 30 * time.Second}
	fmt.Printf("🔍 Crawling %s (%s)...\n", vendor.Name, vendor.Type)

	baseURL, err := url.Parse(vendor.URL)
	if err != nil {
		return nil, fmt.Errorf("invalid vendor URL: %v", err)
	}

	shopBody, err := fetchBody(client, vendor.URL)
	if err != nil {
		return nil, err
	}

	reProductLink := regexp.MustCompile(`href="([^"]*?)"`)
	matches := reProductLink.FindAllStringSubmatch(string(shopBody), -1)

	uniqueLinks := make(map[string]bool)
	for _, m := range matches {
		rawLink := m[1]
		relURL, err := url.Parse(rawLink)
		if err != nil {
			continue
		}
		absURL := baseURL.ResolveReference(relURL)
		if absURL.Host == baseURL.Host && strings.Contains(absURL.Path, "/product/") {
			uniqueLinks[absURL.String()] = true
		}
	}

	fmt.Printf("   -> Found %d unique product pages.\n", len(uniqueLinks))

	var products []models.Product

	for link := range uniqueLinks {
		time.Sleep(300 * time.Millisecond)

		pageBody, err := fetchBody(client, link)
		if err != nil {
			continue
		}

		reSchema := regexp.MustCompile(`(?s)<script type="application/ld\+json"[^>]*>(.*?)</script>`)
		schemaMatches := reSchema.FindAllStringSubmatch(string(pageBody), -1)

		for _, match := range schemaMatches {
			var graph LdJsonGraph
			if err := json.Unmarshal([]byte(match[1]), &graph); err != nil {
				continue
			}

			for _, node := range graph.Graph {
				if !isProductType(node.Type) {
					continue
				}

				if len(node.HasVariant) > 0 {
					for _, v := range node.HasVariant {
						desc := v.Description
						if desc == "" { desc = node.Description }

						// Determine Availability
						isAvailable := strings.Contains(v.Offers.Availability, "InStock")

						products = append(products, models.Product{
							ID:       v.Name,
							Title:    v.Name,
							Handle:   link,
							BodyHTML: desc,
							Variants: []models.Variant{
								{
									Price:     fmt.Sprintf("%v", v.Offers.Price),
									Title:     v.Name,
									Available: isAvailable, // MAP
								},
							},
						})
					}
				} else if node.Offers != nil {
					isAvailable := strings.Contains(node.Offers.Availability, "InStock")

					products = append(products, models.Product{
						ID:       node.Name,
						Title:    node.Name,
						Handle:   link,
						BodyHTML: node.Description,
						Variants: []models.Variant{
							{
								Price:     fmt.Sprintf("%v", node.Offers.Price),
								Title:     node.Name,
								Available: isAvailable, // MAP
							},
						},
					})
				}
			}
		}
	}

	return products, nil
}

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