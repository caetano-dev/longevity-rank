package scraper

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"longevity-ranker/internal/models"
	"strconv"
	"time"
)

func FetchShopifyProducts(vendor models.Vendor) ([]models.Product, error) {
	client := &http.Client{Timeout: 15 * time.Second}
	var finalProducts []models.Product
	page := 1

	fmt.Printf("🔌 Connecting to %s...\n", vendor.Name)

	for {
		url := fmt.Sprintf("%s?page=%d", vendor.URL, page)
		
		req, _ := http.NewRequest("GET", url, nil)
		req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko)")

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
					Price string `json:"price"`
					Title string `json:"title"`
				} `json:"variants"`
			} `json:"products"`
		}

		if err := json.Unmarshal(body, &rawData); err != nil {
			break 
		}

		if len(rawData.Products) == 0 {
			break // End of pagination
		}

		for _, p := range rawData.Products {
			newProd := models.Product{
				ID:       strconv.FormatInt(p.ID, 10),
				Title:    p.Title,
				Handle:   p.Handle,
				BodyHTML: p.BodyHTML, // Map the HTML
			}

			for _, v := range p.Variants {
				newProd.Variants = append(newProd.Variants, models.Variant{
					Price: v.Price,
					Title: v.Title, 
				})
			}

			finalProducts = append(finalProducts, newProd)
		}

		fmt.Printf("   -> Page %d: %d items\n", page, len(rawData.Products))
		page++
	}

	return finalProducts, nil
}