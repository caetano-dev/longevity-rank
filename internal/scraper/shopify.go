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
		// Cache-busting logic maintained
		url := fmt.Sprintf("%s?page=%d&_t=%d", vendor.URL, page, time.Now().Unix())
		
		req, _ := http.NewRequest("GET", url, nil)
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
				Images   []struct { // NEW
					Src string `json:"src"`
				} `json:"images"`
				Variants []struct {
					Price     string `json:"price"`
					Title     string `json:"title"`
					Available bool   `json:"available"`
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
			// Extract Image
			img := ""
			if len(p.Images) > 0 {
				img = p.Images[0].Src
			}

			newProd := models.Product{
				ID:       strconv.FormatInt(p.ID, 10),
				Title:    p.Title,
				Handle:   p.Handle,
				BodyHTML: p.BodyHTML,
				ImageURL: img, // MAP
			}

			for _, v := range p.Variants {
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