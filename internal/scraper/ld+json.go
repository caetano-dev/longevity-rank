package scraper

import (
	"encoding/json"
	"fmt"
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
	Image       interface{} `json:"image"`
	HasVariant  []LdVariant `json:"hasVariant"`
	Offers      *LdOffer    `json:"offers,omitempty"`
}

type LdVariant struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Offers      LdOffer `json:"offers"`
}

type LdOffer struct {
	Price         interface{} `json:"price"`
	PriceCurrency string      `json:"priceCurrency"`
	Availability  string      `json:"availability"`
}

func FetchLdJsonProducts(vendor models.Vendor) ([]models.Product, error) {
	fmt.Printf("🔍 Crawling %s (%s)...\n", vendor.Name, vendor.Type)

	baseURL, err := url.Parse(vendor.URL)
	if err != nil {
		return nil, fmt.Errorf("invalid vendor URL: %v", err)
	}

	shopBody, err := FetchBody(vendor.URL)
	if err != nil {
		return nil, err
	}

	reProductLink := regexp.MustCompile(`href="([^"]*?)"`)
	matches := reProductLink.FindAllStringSubmatch(string(shopBody), -1)

	uniqueLinks := make(map[string]bool)
	for _, m := range matches {
		relURL, err := url.Parse(m[1])
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

		pageBody, err := FetchBody(link)
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

				imgURL := extractImageURL(node.Image)

				if len(node.HasVariant) > 0 {
					for _, v := range node.HasVariant {
						desc := v.Description
						if desc == "" {
							desc = node.Description
						}

						products = append(products, models.Product{
							ID:       v.Name,
							Title:    v.Name,
							Handle:   link,
							BodyHTML: desc,
							ImageURL: imgURL,
							Variants: []models.Variant{
								{
									Price:     fmt.Sprintf("%v", v.Offers.Price),
									Title:     v.Name,
									Available: strings.Contains(v.Offers.Availability, "InStock"),
								},
							},
						})
					}
				} else if node.Offers != nil {
					products = append(products, models.Product{
						ID:       node.Name,
						Title:    node.Name,
						Handle:   link,
						BodyHTML: node.Description,
						ImageURL: imgURL,
						Variants: []models.Variant{
							{
								Price:     fmt.Sprintf("%v", node.Offers.Price),
								Title:     node.Name,
								Available: strings.Contains(node.Offers.Availability, "InStock"),
							},
						},
					})
				}
			}
		}
	}

	return products, nil
}

// extractImageURL handles the polymorphic image field (string or []string).
func extractImageURL(img interface{}) string {
	if s, ok := img.(string); ok {
		return s
	}
	if arr, ok := img.([]interface{}); ok && len(arr) > 0 {
		if s, ok := arr[0].(string); ok {
			return s
		}
	}
	return ""
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