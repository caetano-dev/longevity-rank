package scraper

import (
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"strconv"
	"time"

	"longevity-ranker/internal/models"
)

const maxShopifyPages = 1000

func FetchShopifyProducts(vendor models.Vendor) ([]models.Product, error) {
	var finalProducts []models.Product
	seenIDs := make(map[string]bool)
	page := 1

	fmt.Printf("🔌 Connecting to %s...\n", vendor.Name)

	baseURL, err := url.Parse(vendor.URL)
	if err != nil {
		return nil, fmt.Errorf("invalid vendor URL %q: %v", vendor.URL, err)
	}

	for page <= maxShopifyPages {
		// Build paginated URL preserving any existing query params (e.g. ?currency=USD)
		q := baseURL.Query()
		q.Set("page", strconv.Itoa(page))
		q.Set("_t", strconv.FormatInt(time.Now().Unix(), 10))
		baseURL.RawQuery = q.Encode()
		fetchURL := baseURL.String()

		req, err := NewRequest(fetchURL)
		if err != nil {
			return nil, fmt.Errorf("failed building request for page %d: %v", page, err)
		}
		req.Header.Set("Cache-Control", "no-cache, no-store, must-revalidate")
		req.Header.Set("Pragma", "no-cache")
		req.Header.Set("Expires", "0")

		resp, err := DefaultClient.Do(req)
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
				Images   []struct {
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

		newOnPage := 0
		for _, p := range rawData.Products {
			pid := strconv.FormatInt(p.ID, 10)
			if seenIDs[pid] {
				continue
			}
			seenIDs[pid] = true
			newOnPage++

			img := ""
			if len(p.Images) > 0 {
				img = p.Images[0].Src
			}

			newProd := models.Product{
				ID:       pid,
				Title:    p.Title,
				Handle:   p.Handle,
				BodyHTML: p.BodyHTML,
				ImageURL: img,
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

		fmt.Printf("   -> Page %d: %d items (%d new)\n", page, len(rawData.Products), newOnPage)

		if newOnPage == 0 {
			fmt.Printf("   ⚠️  No new products on page %d, stopping pagination.\n", page)
			break
		}
		page++
	}

	if page > maxShopifyPages {
		fmt.Printf("   ⚠️  Hit max page limit (%d) for %s.\n", maxShopifyPages, vendor.Name)
	}

	return finalProducts, nil
}