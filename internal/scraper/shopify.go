package scraper

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"longevity-ranker/internal/models"
	"time"
)

func FetchAllProducts(vendor models.Vendor) ([]models.Product, error) {
	client := &http.Client{Timeout: 15 * time.Second}
	var allProducts []models.Product
	page := 1

	fmt.Printf("🔌 Connecting to %s...\n", vendor.Name)

	for {
		// Construct paginated URL
		url := fmt.Sprintf("%s?page=%d", vendor.URL, page)
		
		req, _ := http.NewRequest("GET", url, nil)
		// User-Agent is crucial to avoid being blocked
		req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko)")

		resp, err := client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("failed fetching page %d: %v", page, err)
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)

		var result models.ShopifyResponse
		if err := json.Unmarshal(body, &result); err != nil {
			// Some sites return HTML errors instead of JSON when done
			break 
		}

		if len(result.Products) == 0 {
			break // End of pagination
		}

		allProducts = append(allProducts, result.Products...)
		fmt.Printf("   -> Page %d: %d items\n", page, len(result.Products))
		page++
	}

	return allProducts, nil
}