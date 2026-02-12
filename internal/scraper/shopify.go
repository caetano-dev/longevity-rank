package scraper

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"longevity-ranker/internal/models"
	"strconv"
	"strings"
	"time"
)

func FetchShopifyProducts(vendor models.Vendor) ([]models.Product, error) {
	client := &http.Client{Timeout: 15 * time.Second}
	var finalProducts []models.Product
	page := 1

	fmt.Printf("🔌 Connecting to %s...\n", vendor.Name)

	for {
		// FIX: Add Timestamp (_t) to force cache-busting
		// Shopify caches products.json aggressively. This forces a fresh lookup.
		url := fmt.Sprintf("%s?page=%d&_t=%d", vendor.URL, page, time.Now().Unix())
		
		req, _ := http.NewRequest("GET", url, nil)
		// Update User-Agent to look more like a real browser
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
		req.Header.Set("Cache-Control", "no-cache, no-store, must-revalidate")
		req.Header.Set("Pragma", "no-cache")
		req.Header.Set("Expires", "0")

		resp, err := client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("failed fetching page %d: %v", page, err)
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)

		var rawData struct {
			Products []struct {
				ID       int64  `json:"id"`
				Title    string `json:"title"`
				Handle   string `json:"handle"`
				BodyHTML string `json:"body_html"`
				Variants []struct {
					Price     string `json:"price"`
					Title     string `json:"title"`
					Available bool   `json:"available"` // Capture Stock Status
				} `json:"variants"`
			} `json:"products"`
		}

		if err := json.Unmarshal(body, &rawData); err != nil {
			break 
		}

		if len(rawData.Products) == 0 {
			break 
		}

		for _, p := range rawData.Products {
			newProd := models.Product{
				ID:       strconv.FormatInt(p.ID, 10),
				Title:    p.Title,
				Handle:   p.Handle,
				BodyHTML: p.BodyHTML,
			}

			for _, v := range p.Variants {
				// --- DEBUG PRINT ---
				// Let's verify what the scraper actually sees for the problem product
				if strings.Contains(p.Title, "NMN Pure Powder") {
					fmt.Printf("   >> DEBUG: Found 'NMN Pure Powder'. Available: %v\n", v.Available)
				}
				// -------------------

				newProd.Variants = append(newProd.Variants, models.Variant{
					Price:     v.Price,
					Title:     v.Title,
					Available: v.Available, 
				})
			}

			finalProducts = append(finalProducts, newProd)
		}

		fmt.Printf("   -> Page %d: %d items\n", page, len(rawData.Products))
		page++
	}

	return finalProducts, nil
}